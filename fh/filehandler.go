package fh

import (
	"fmt"
	"io"
	"sync"
)

// FileHandler stellt Methoden zur Verfügung, um mit einer drive Datei zu interagieren.
// Jeder FileHandler hat dabei seinen eigenen Cache der im RAM abgelegt ist.
type FileHandler struct {
	mutex  *sync.Mutex   // sorgt dafür, dass alle exportierten Methoden atomar sind
	resp   io.ReadCloser // Verbindung zur Datei auf Google Drive
	offset int64         // nächstes, noch nicht gelesenes Byte der Datei auf Google Drive

	// Der Cache ist eine verkettete Liste von CacheElements.
	firstCacheElement *CacheElement // das erste CacheElement
	lastCacheElement  *CacheElement // das letzte CacheElement
}

// CacheElement ist ein zusammenhngender Byte-Block. Eine Liste von CacheElement ist dann der Cache.
type CacheElement struct {
	b      []byte // die Bytes, die dieses CacheElement enthält (max. ReadBufferSize)
	offset int64  // an welcher Stelle in der Datei stehen diese Bytes

	previous *CacheElement // vorhergehendes CacheElement (verkettete Liste)
	next     *CacheElement // nächstes CacheElement (verkettete Liste)
}

// CloseAndClear schließt die Verbindung zur Datei und löscht den Cache.
func (fh *FileHandler) CloseAndClear() {

	// LOCK / UNLOCK
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	// Schließe die Verbindung zu Google Drive
	if fh.resp != nil {
		fh.resp.Close()
	}

	// Setze alle Variablen auf nil, damit der Garbage Collector den Speicher freigeben kann
	fh.resp = nil
	fh.offset = 0
	fh.firstCacheElement = nil
	fh.lastCacheElement = nil
}

// Download gibt Bytes ab dem gewünschten Offset zurück.
// Hinweis: die gewünschte Länge ist als max. zu verstehen und muss nicht erreicht werden.
func (fh *FileHandler) Download(requestedOffset int64, length int) ([]byte, error) {

	// LOCK / UNLOCK
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	// Kein Zurücklesen!
	// Wird versucht auf Daten vor dem Cache zu zugreifen, dann ist das ein Fehler!
	if requestedOffset < fh.firstCacheElement.offset {
		return nil, fmt.Errorf("can't read backward: requestedOffset=%d, cacheStartOffset=%d", requestedOffset, fh.firstCacheElement.offset)
	}

	// Keine weiten Sprünge!
	// Wird versucht einen großen Bereich beim Lesen zu überspringen, dann ist das ein Fehler! (nur sequenzielles Lesen!)
	if requestedOffset > fh.offset+MaxForwardJump {
		return nil, fmt.Errorf("requestedOffset too far away: requestedOffset=%d, fhOffset=%d", requestedOffset, fh.offset)
	}

	// Diese Schleife lädt Daten in den Cache, wenn sie angefordert werden.
	// ACHTUNG: Es kann sein, das bereits EOF erreicht wurde und dieser Wunsch nie erfüllt werden kann!
	for requestedOffset+int64(length) > fh.offset {

		// Liest einmal ein CacheElement.
		// Das kann bei einer leeren Datei auch 0 Bytes enthalten.
		b := make([]byte, ReadBufferSize)
		n, err := fh.resp.Read(b)
		if err != nil && err != io.EOF {
			return nil, err
		}
		b = b[:n] // trim buffer

		cacheElement := &CacheElement{
			b:      b,
			offset: fh.offset,
		}

		// Es wurden keine Bytes gelesen
		// Damit gibt es auch nichts mehr --> Schleife verlassen
		if n <= 0 {
			break
		}

		// Nun, da der fh auf die Datei in Google Drive weitergerückt wurde,
		// muss ein neuer fhOffset berechnet werden.
		// (fhOffset: nächstes, noch nicht gelesenes Byte der Datei auf Google Drive)
		fh.offset += int64(n)

		// Das neue CacheElement in die verkettete Liste einbaun
		cacheElement.previous = fh.lastCacheElement
		fh.lastCacheElement.next = cacheElement
		fh.lastCacheElement = cacheElement
	}

	/*
	 * Was ist nun sichergestellt?
	 * Die angeforderten Daten beginnen irgendwo im Bereich des Caches.
	 * Jedoch ist NICHT gesagt, dass auch alle Daten hier sind.
	 * Wurde das Ende des Datei auf Google Drive erreicht, dann können am Ende Daten fehlen!
	 */

	// Suche von Hinten nach dem CacheElement, in dem die angeforderten Daten beginnen.
	cache := fh.lastCacheElement
	for requestedOffset < cache.offset {
		cache = cache.previous
	}

	// Das CacheElement ist gefunden, nun wird der Offset innerhalb des CacheElement berechnet.
	// (Die angeforderten Daten werden nicht immer genau am CacheElement-Anfang beginnen!)
	innerOffset := requestedOffset - cache.offset

	// BUGFIX: Werden Daten abgefragt (requestedOffset), die hinter dem Dateiende liegen, aber noch nicht von
	// MaxForwardJump verhindert werden, dann würde die Zeile "cache.b[innerOffset:]" einen Fehler verursachen!
	// Daher wird der innerOffset so gesetzt, das retBytes 0 bytes enthält, aber kein Fehler verursacht wird.
	if innerOffset > int64(len(cache.b)) {
		innerOffset = int64(len(cache.b))
	}

	// Es werden Bytes für die Zurückgabe vorbereitet.
	// Für den Anfang sind das alle Bytes, die im aktuellen CacheElement vorhanden sind.
	// (Natürlich exklusive des innerOffset)
	retBytes := cache.b[innerOffset:]

	// Es kann jedoch sein, dass die angeforderten Daten auch in andere CacheElements hinein reichen.
	// Daher wird das retBytes solange angereichert, bis es größer ist als die angeforderte Datenmenge.
	// ACHTUNG: Hier könnte man in das EOF-Problem laufen!
	cache = cache.next
	for len(retBytes) < length && cache != nil {
		retBytes = append(retBytes, cache.b...)
		cache = cache.next
	}

	// Nun müssen nur noch eventuell zu viel gelesene Bytes abgeschnitten werden...
	if len(retBytes) > length {
		retBytes = retBytes[:length]
	}

	// Zuletzt muss der Cache auf seine maximale Größe verfkleinert werden
	fh.cleanupCache()

	// EOF (Kompatibilität)
	// Werden 0 bytes zurück gegeben, dann wird stattdessen ein EOF Error geworfen.
	var lastError error = nil
	if len(retBytes) <= 0 {
		lastError = io.EOF
	}
	return retBytes, lastError
}

// cleanupCache verkleinert den Cache, sollte er zu groß sein.
func (fh *FileHandler) cleanupCache() {
	size := 0                    // ermittelte Größe des Caches
	cache := fh.lastCacheElement // aktuelles CacheElement

	// Wir gehen von Hinten die verkettete Liste durch,
	// solange bis der Anfang der Liste erreicht wurde.
	for cache != nil {
		// Cache-Größe aufsummieren
		size += len(cache.b)

		// HUCH! DER CACHE IST ZU GROß!!
		if size > MaxCacheSize {
			// Hier hacken wir die Liste ab!
			fh.firstCacheElement = cache        // dieses Element ist nun der Anfang
			fh.firstCacheElement.previous = nil // und der Anfang hat kein Element vor sich
			break                               // Ende!
		}

		// nächstes Element
		cache = cache.previous
	}
}

package fh

import (
	"io"
	"sync"

	"splitfuseX/backbone"
)

// NewFileHandler erzeugt ein neues FileHandler Objekt. Mit fileId wird die Datei auf dem Drive angegeben.
// Der offset bestimmt, an welcher Stelle grob die Datei gelesen werden soll.
// Hinweis: Der offset wird von dieser Methode etwas nach unten korrigiert damit initial mehr Daten gelesen werden.
func NewFileHandler(client backbone.Client, fileId string, offset int64) (*FileHandler, error) {

	// Es wird ein bestimmter offset angefordert. (pre-load)
	// Aber zur sicherheit werden einige Bytes davor mit gelesen,
	// weil ein nachträgliches "vorlesen" nicht mehr möglich ist.
	offset = offset - PreloadSize
	if offset < 0 {
		offset = 0
	}

	// http response zur Datei holen
	resp, err := client.Read(fileId, offset, MaxFileSize)
	if err != nil {
		return nil, err
	}

	// Liest einmal ein CacheElement. (einen ganzen ReadBufferSize großen Block lesen)
	// Das kann bei einer leeren Datei auch 0 Bytes enthalten.
	b := make([]byte, ReadBufferSize)
	n, err := resp.Read(b)
	if err != nil && err != io.EOF { // alle Fehler AUßER EOF zurückgeben
		return nil, err
	}
	b = b[:n] // trim buffer

	cacheElement := &CacheElement{
		b:      b,
		offset: offset,
	}

	// Nun, da der fh auf die Datei in Google Drive weiter gerückt wurde,
	// muss ein neuer fhOffset berechnet werden.
	// (fhOffset: nächstes, noch nicht gelesenes Byte der Datei auf Google Drive)
	offset += int64(n)

	// return fh object
	return &FileHandler{
		mutex:  &sync.Mutex{},
		resp:   resp,
		offset: offset,

		firstCacheElement: cacheElement,
		lastCacheElement:  cacheElement,
	}, nil
}

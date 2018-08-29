package core

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	// CHUNKSIZE sollte ein vielfaches der FUSE-Puffers 131072 Byte (128 Kibibyte)
	// und einer Blockgröße der Festplatten (zB 4096 Byte) sein.
	CHUNKSIZE = 131072 * 4096 * 2 // 1073741824 Byte (1024 Mebibyte)

	// DBUFFERSIZE sollte sich zwischen 10 MB und 20 MB bewegen
	// und muss die CHUNKSIZE Ganzzahlig teilen können!
	BUFFERSIZE = 16777216 // 16777216 Byte (16 Mebibyte)
)

// gibt den Ordnerinhalt zurück
func readDirNames(dirname string) ([]FolderContent, error) {
	// Ordner öffnen
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	// Liste lesen
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	// Liste sortieren
	sort.Strings(names)

	// FolderContent Liste anlegen
	ret := make([]FolderContent, 0, len(names))
	for _, v := range names {
		// sub-element Datei oder Ordner?
		tmppath := filepath.Join(dirname, v)
		info, err := os.Stat(tmppath)
		if err != nil {
			return nil, err
		}
		isFile := !info.IsDir()
		// hinzufügen
		ret = append(ret, FolderContent{Name: v, IsFile: isFile})
	}

	return ret, nil
}

// ScanFolder scant einen ganzen Ordner und erstellt daraus eine db.
func ScanFolder(rootpath string, db SfDb, debug bool) (newDB SfDb, changed bool, summary string, retErr error) {
	// clone oldDB
	oldDB := make(SfDb, len(db))
	for k, v := range db {
		oldDB[k] = v
	}

	// init return values
	countNewOrUpdate := 0
	newDB = SfDb{}

	// Walk
	retErr = filepath.Walk(rootpath, func(path string, info os.FileInfo, err error) error {
		// Fehlerbehandlung der WalkFunc
		if err != nil {
			return err
		}

		// relativen Pfad ermitteln
		relPath, err := filepath.Rel(rootpath, path)
		if err != nil {
			return err
		}

		// Eckdaten des betrachteten Elements ermitteln
		isFile := !info.IsDir()
		mtime := uint64(info.ModTime().Unix())
		size := info.Size()

		// Ordnerinhalt ermitteln, wenn es ein Ordner ist
		var folderContent []FolderContent
		if !isFile {
			folderContent, err = readDirNames(path)
			if err != nil {
				// Fehlerbehandlung der readDir Func
				return err
			}
		}

		// Element in der alten DB suchen
		e, ok := oldDB[relPath]

		// Fälle, in denen das Element neu gelesen werden muss
		// andernfalls kann das Element aus der alten DB übernommen werden
		if !ok || e.Size != size || e.IsFile != isFile || e.Mtime != mtime {
			countNewOrUpdate++
			changed = true // Änderung festhalten
			scanDebug(debug, "new or changed: "+relPath)

			if isFile {
				// Ist es eine Datei: Element scannen
				e, err = scanFile(path)
				if err != nil {
					// Fehlerbehandlung der ScanFunc
					return err
				}
			} else {
				// ist es ein Ordner, dann neu baun
				e = SfFile{
					Size:          size,
					Mtime:         mtime,
					IsFile:        isFile,
					FolderContent: folderContent,
				}
			}
		}

		// FIX: Den Ordner Content immer setzen
		// Gibt es keine Änderungen bei den Dateien, dann wird die DB sowieso nicht neu geschrieben
		// Aber wenn es Änderungen gab, dann sind alle Ordner aktuell
		// Das mache ich so, well der Abgleich (equal) von folderContent nicht immer funktioniert
		e.FolderContent = folderContent

		// Ist die einzige Änderung, dass ein altes Element nicht mehr vorhanden ist,
		// dann muss ich das auch erkennen können. Sollange also das changed Flag nicht andeweitig gesetzt wurde,
		// muss ich alle übernommenen Elemente aus der alten Datenbank löschen. Bleibt am Ende etwas übrig, dann
		// gab es eine Änderung!
		delete(oldDB, relPath)

		// Element in die neue DB schreiben und funktion beenden
		newDB[relPath] = e
		return nil
	})

	// finale changed?
	if len(oldDB) > 0 {
		changed = true
	}

	// Statistik
	summary = fmt.Sprintf("SCAN: error=%v, sum=%d, changed=%v, newOrUpdate=%d, removed=%d", retErr, len(newDB), changed, countNewOrUpdate, len(oldDB))
	return
}

func scanDebug(debug bool, msg string) {
	if debug {
		println("DEBUG: " + msg)
	}
}

// scanFile liest eine Klartextdatei und berechnet die hashes der einzelnen Chunks
func scanFile(path string) (SfFile, error) {

	// Datei zum Lesen öffnen
	fh, err := os.Open(path)
	if err != nil {
		return SfFile{}, err
	}
	defer fh.Close()

	// Datei in Chunks teilen und hash berechnen
	var fileSize int64 = 0
	var chunkSize = 0
	var chunkHash = sha512.New()
	var chunkList = make([]ChunkHash, 0)

	for {
		// buffer-weise den chunk lesen
		buffer := make([]byte, BUFFERSIZE)
		n, readErr := fh.Read(buffer)
		buffer = buffer[:n] // buffer ist nur so groß, wie auch wirklich gelesen wurde

		// hash weiter berechnen
		if n > 0 {
			fileSize += int64(n)
			chunkSize += n
			chunkHash.Write(buffer)
		}

		// Chunk abschließen?  wegen Größe oder EOF
		if chunkSize >= CHUNKSIZE || readErr != nil {
			sfChunkHash, _ := sha512ToChunkHash(chunkHash.Sum(nil))

			// leere Dateien müssen eine leere Chunk-Liste haben
			// UND chunks mit der größe 0 dürfen auch nicht auf die Liste
			if fileSize > 0 && chunkSize > 0 {
				chunkList = append(chunkList, sfChunkHash)
			}

			// reset vars
			chunkSize = 0
			chunkHash = sha512.New()
		}

		// Lesen der Datei ist abgeschlossen (EOF)
		if readErr != nil {
			break
		}
	}

	// Datei Attribute ermitteln
	fileInfo, err := os.Stat(path)
	if err != nil {
		return SfFile{}, err
	}

	// Prüfungen: Wurde alles gelesen?
	if fileSize != fileInfo.Size() {
		return SfFile{}, errors.New("file was not completely read: " + path)
	}

	// SfFile Objekt erzeugen und zurück geben
	return SfFile{
		Size:       int64(fileSize),
		Mtime:      uint64(fileInfo.ModTime().Unix()),
		IsFile:     !fileInfo.IsDir(),
		FileChunks: chunkList,
	}, nil
}

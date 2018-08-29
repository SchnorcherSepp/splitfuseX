package fuse

import (
	"fmt"

	"splitfuseX/backbone"
	"splitfuseX/core"
	"splitfuseX/fh"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

// SplitFile wird von der Open() Funktion zurück gegeben
// und stellt die Read() Funktion zur verfügung..
type SplitFile struct {
	nodefs.File

	debug     bool
	dbFile    core.SfFile
	chunkKeys [][]byte
	fileIds   []string
	apiClient backbone.Client
	fh        map[int]*fh.FileHandler
}

// Release wird aufgerufen, wenn .close() auf die Datei im FUSE aufgerufen wird.
// Damit müssen auch alle offenen internen FH geschlossen werden.
func (f *SplitFile) Release() {
	if f.fh != nil {
		for k, v := range f.fh {
			debug(f.debug, LOGINFO, fmt.Sprintf("Release(): close fh for chunk %d", k), nil)
			v.CloseAndClear()
			delete(f.fh, k)
		}
	}
}

// Read liest bytes und gibt sie fürs FUSE zurück.
func (f *SplitFile) Read(buf []byte, offset int64) (fuse.ReadResult, fuse.Status) {

	// leere Dateien sofort zurückgeben
	if f.dbFile.Size < 1 {
		debug(f.debug, LOGINFO, "Read(): read empty file", nil)
		return fuse.ReadResultData([]byte{}), fuse.OK
	}

	// Berechnungen
	readLength := int64(len(buf))
	chunkOffset := offset % core.CHUNKSIZE
	chunkNr := int((offset - chunkOffset) / core.CHUNKSIZE)

	// FIX: Es gibt den Fall, dass am Ende noch einmal 4096 bytes über die Datei gelesen werden.
	// Dabei kann es vorkommen, dass sich die ChunkNr erhöht und es dazu keine Daten in chunkKey und chunkName gibt.
	if chunkNr >= len(f.chunkKeys) {
		// würde panic: runtime error: index out of range auslösen
		debug(f.debug, LOGINFO, "Read(): EOF FIX!", nil)
		return fuse.ReadResultData([]byte{}), fuse.OK
	}

	// Daten ermitteln
	chunkKey := f.chunkKeys[chunkNr]
	fileId := f.fileIds[chunkNr]

	// fh map initialisieren (wenn notwendig)
	if f.fh == nil {
		f.fh = make(map[int]*fh.FileHandler)
	}

	// Ich muss nun auf den chunk zugreifen und brauche dafür ein fh
	// Da diese Operation teuer ist, speichere ich alte filehandler und verwende sie wieder!
	var openErr error
	fhForChunk, ok := f.fh[chunkNr]
	if !ok {
		// gibt noch keinen FH für diesen Chunk
		debug(f.debug, LOGINFO, fmt.Sprintf("Read(): new fh for chunk %d (fileId=%s)", chunkNr, fileId), nil)

		// fhForChunk mit neuem FH beschreiben
		fhForChunk, openErr = fh.NewFileHandler(f.apiClient, fileId, chunkOffset)
		if openErr != nil {
			debug(f.debug, LOGERROR, fmt.Sprintf("Read(): can't open new fh for chunk %d (fileId=%s)", chunkNr, fileId), openErr)
			return fuse.ReadResultData([]byte{}), fuse.EIO
		}

		// fh speichern !!
		f.fh[chunkNr] = fhForChunk
	}

	// Daten lesen
	buf, openErr = fhForChunk.Download(chunkOffset, int(readLength))
	if openErr != nil {
		debug(f.debug, LOGERROR, fmt.Sprintf("Read(): can't read bytes [chunk=%d, fileId=%s, offset=%d, len=%d]", chunkNr, fileId, chunkOffset, readLength), openErr)
		return fuse.ReadResultData([]byte{}), fuse.EIO
	}

	// die gelesenen Daten entschlüsseln
	core.CryptBytes(buf, chunkOffset, chunkKey)

	// SONDERFALL: was ist, wenn knapp über einen chunk hinaus gelesen werden soll?
	// dann muss eine weitere abfrage abgesetzt werden!
	nextChunkBufferSize := chunkOffset + readLength - core.CHUNKSIZE
	if nextChunkBufferSize > 0 {
		debug(f.debug, LOGINFO, fmt.Sprintf("SPECIAL READ [chunk=%d, fileId=%s, offset=%d, len=%d, nextChunkRead=%d]", chunkNr, fileId, chunkOffset, readLength, nextChunkBufferSize), nil)

		// einen Puffer anlegen für meine eigenen Read() Funktion
		buf2 := make([]byte, nextChunkBufferSize)
		// ReadResult abholen
		res2, _ := f.Read(buf2, offset+readLength-nextChunkBufferSize)
		// []byte aus dem ReadResult extrahieren
		buf2, _ = res2.Bytes(buf2)
		// Göße des Puffers gegebenenfalls anpassen
		buf2 = buf2[:res2.Size()]

		// neuen großen Puffer anlegen
		buf = append(buf, buf2...)

		return fuse.ReadResultData(buf), fuse.OK
	}

	// NORMALFALL
	return fuse.ReadResultData(buf), fuse.OK
}
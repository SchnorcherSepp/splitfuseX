package core

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/gob"
	"errors"
	"io"
	"io/ioutil"
	"os"
)

// SfDb ist eine Map, dessen Key der Pfad eines Ordners oder einer Datei ist und
// dessen value ein SfFile Objekt ist. Das Root-Verzeichnis hat den Pfad: '.'
type SfDb map[string]SfFile

// SfFile enthält alle Daten, um eine Datei im FUSE darstellen und lesen zu können.
// Relevant sind nur die Attribute Size und Mtime, alles andere ist statisch.
// Ist das Objekt eine Datei, so wird FileChunks gesetzt. Ist es ein Ordner so ist FolderContent gesetzt.
type SfFile struct {
	// Attr
	Size  int64  // size in bytes
	Mtime uint64 // time of last modification

	// file or folder
	IsFile        bool            // true is file, false is folder
	FileChunks    []ChunkHash     // if file: the full chunk list of this file
	FolderContent []FolderContent // if folder: a list ob sub elements of this folder
}

// ChunkHash ist ein sha512 Hash (64 bytes) über den Klartext eines Chunks.
// Eine Liste dieser Chunks ergeben eine ganze Datei.
type ChunkHash [64]byte

// FolderContent speichert den Namen eines Elements (Ordner oder Datei) eines Ordners (Ordnerinhalt).
// Wird für ein performantes FUSE listDir() benötigt.
type FolderContent struct {
	Name   string
	IsFile bool
}

// ------------------------------------------------------------------------------------------------------------------ //

// dbToEncGOB serialized und verschlüsselt das SfDb Objekt und gibt nonce und den ciphertext zurück.
// Im Fehlerfall wird ein Error zurück gegeben und der ciphertext ist Null.
func dbToEncGOB(key []byte, db SfDb) (nonce []byte, ciphertext []byte, err error) {

	// serialisiertes Objekt als bytes (plaintext)
	var plaintext = bytes.Buffer{}
	encoder := gob.NewEncoder(&plaintext)
	err = encoder.Encode(db)
	if err != nil {
		return
	}

	// create AES cipher with 16, 24, or 32 bytes key
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	// Galois Counter Mode
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	// create random nonce with standard length
	nonce = make([]byte, aesgcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return
	}

	// encrypts and authenticates plaintext
	ciphertext = aesgcm.Seal(nil, nonce, plaintext.Bytes(), nil)

	// FIN
	return
}

// dbFromEncGOB entschlüsselt und authentisirt den ciphertext.
// Im Fehlerfall wird ein error zurück gegeben.
func dbFromEncGOB(key []byte, nonce []byte, ciphertext []byte) (db SfDb, err error) {

	// create AES cipher with 16, 24, or 32 bytes key
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	// Galois Counter Mode
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	// decrypts and authenticates ciphertext
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return
	}

	// decode the plaintext and update db *SfDb
	decoder := gob.NewDecoder(bytes.NewReader(plaintext))
	err = decoder.Decode(&db)
	return
}

// DbToFile schreibt die DB in eine Datei.
// ACHTUNG: Das Ziel wird dabei überschrieben!
// Bei Problemen wird ein Fehler zurück gegeben der behandelt werden muss!
func DbToFile(path string, key []byte, db SfDb) error {

	// Datei überschreiben
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()

	// DO IT
	return DbToWriter(fh, key, db)
}

// DbToWriter schreibt eine DB mit einem Writer wie zB einem FH von os.Create().
func DbToWriter(w io.Writer, key []byte, db SfDb) error {
	// db verschlüsseln
	nonce, ciphertext, err := dbToEncGOB(key, db)
	if err != nil {
		return err
	}

	// nonce schreiben
	n, err := w.Write(nonce)
	if err != nil {
		return err
	}
	if n != len(nonce) {
		return errors.New("write nonce failed")
	}

	// ciphertext schreiben
	n, err = w.Write(ciphertext)
	if err != nil {
		return err
	}
	if n != len(ciphertext) {
		return errors.New("write ciphertext failed")
	}

	// FIN
	return nil
}

// DbFromFile liest eine Datei und gibt ein SfDB Objekt zurück.
// Im Fehlerfall wird ein Error zurck gegebe, der behandelt werden muss.
// Ein Beispiel für einen Fähler wäre, das Lesen einer noch nicht fertig geschriebenen DB Datei.
// Existiert die Datei überhaupt nicht, dann wird eine leere DB zurück gegeben
func DbFromFile(path string, key []byte) (db SfDb, err error) {

	// keine Datei -> leere DB
	_, err = os.Stat(path)
	if err != nil {
		// datei existiert nicht
		return SfDb{}, nil
	}

	// Datei öffnen
	fh, err := os.Open(path)
	if err != nil {
		return // z.B. error: file not found
	}
	defer fh.Close()

	// mach mal
	db, err = DbFromReader(fh, key)

	// FIN
	return
}

// DbFromReader liest eine DB von einem Reader wie zB einem FH von os.Open().
func DbFromReader(r io.Reader, key []byte) (db SfDb, err error) {

	// alles lesen
	filebytes, err := ioutil.ReadAll(r)
	if err != nil {
		return // z.B. error: file to large
	}

	// Datei muss groß genug sein
	gcmStandardNonceSize := 12
	if len(filebytes) < gcmStandardNonceSize+1 {
		err = errors.New("db file is too short")
		return
	}

	// daten extrahieren
	nonce := filebytes[:gcmStandardNonceSize]
	ciphertext := filebytes[gcmStandardNonceSize:]

	// encrtypt
	db, err = dbFromEncGOB(key, nonce, ciphertext)
	if err != nil {
		return // z.B. error: Authentication failed
	}

	// FIN
	return
}

// ------------------------------------------------------------------------------------------------------------------ //

// Wandelt ein Sha512 Hash in ein ChunkHash Objekt um.
func sha512ToChunkHash(sha512 []byte) (ChunkHash, error) {
	// check input
	if len(sha512) != 64 {
		return ChunkHash{}, errors.New("sha512 hash must be 64 bytes long")
	}
	// build and return
	var retbytes [64]byte
	copy(retbytes[:], sha512)
	return retbytes, nil
}

// CalcChunkSize berechnet wie groß ein gewählter Chunk ist, bei einer bestimmten Klartextdateigröße
func CalcChunkSize(chunkNr int, fileSize int64) (chunkSize int64) {
	test1 := int64(chunkNr+1) * CHUNKSIZE
	test2 := test1 - fileSize

	if test1 <= fileSize {
		return CHUNKSIZE
	}

	if test2 > CHUNKSIZE {
		return 0
	}

	return fileSize % CHUNKSIZE
}

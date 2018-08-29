package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

// Schlüsselmanagement
type KeyFile struct {
	cryptSecret []byte // für die Verschlüsselung der Chunks
	hashSecret  []byte // für die Chunk Dateinamen
	indexSecret []byte // für die DB Verschlüsselung
}

// CalcChunkKey leitet den individuellen Schlüssel für die Chunk-Verschlüsselung ab.
// Es muss der hash über den Klartext des Chunks übergeben werden und es wird ein Schlüssel für AES-256 zurück gegeben.
func (k *KeyFile) CalcChunkKey(chunkHash []byte) []byte {
	chunkKey := pbkdf2.Key(k.cryptSecret, chunkHash, 10000, 32, sha256.New)
	return chunkKey
}

// CalcChunkName generiert den Dateinamen eines Chunks. Es muss dafür der hash über den Klartext des Chunks
// übergeben werden und es wird der verschlüsselte, 512 Bits lange Name zurück gegeben.
func (k *KeyFile) CalcChunkName(chunkHash []byte) []byte {
	chunkKey := pbkdf2.Key(k.hashSecret, chunkHash, 500, 64, sha512.New)
	return chunkKey
}

// DbKey gibt den Schlüssel für die Datenbank (index) zurück.
// Es wird ein Schlüssel für AES-256 zurück gegeben.
func (k *KeyFile) DbKey() []byte {
	dbKey := pbkdf2.Key(k.indexSecret, []byte("dbkey"), 5000, 32, sha256.New)
	return dbKey
}

//--------------------------------------------------------------------------------------------------------------------//

// cryptReader ist ein privates struct für CryptReader()
type cryptReader struct {
	chunkKey    []byte
	innerReader io.Reader
	offset      int64
}

// Read implementiert die Verschlüsselung im Reader
func (cr *cryptReader) Read(p []byte) (n int, err error) {
	n, err = cr.innerReader.Read(p)
	CryptBytes(p, cr.offset, cr.chunkKey)
	cr.offset += int64(n)
	return n, err
}

// CryptReader kapselt den übergebenen Reader und sort dafür, dass er verschlüsselt wird.
func CryptReader(r io.Reader, chunkKey []byte) io.Reader {
	// new crypt reader
	cr := &cryptReader{
		chunkKey:    chunkKey,
		innerReader: r,
		offset:      0,
	}
	// return
	return cr
}

//--------------------------------------------------------------------------------------------------------------------//

// CryptBytes ver- oder entschlüsselt die übergebenen bytes aus einem Chunk.
// Von wo im Chunks die bytes stammen, muss mit dem offset angegeben werden.
// ACHTUNG: Die Werte in data werden durch die Funktion verändert.
// Im Fehlerfall wird ein error zurück gegeben und die Daten bleiben unverändert
//
// Verschlüsselung: AES-CTR
//   Es gibt keinen Nonce. Jeder Chunk muss daher einen eigenen Key haben.
//   Der Counter beginnt bei 0 und wird je nach offset in dieser Methode berechnet.
//   Es gibt kein Padding.
//   [{"op":"AES Encrypt","args":[{"option":"Hex","string":"0101010101010...256 Bit PartKey...01010101010101"},
//   {"option":"Hex","string":"00000000000000000000000000000001"},
//   {"option":"Hex","string":""},"CTR","NoPadding","Key","Hex"]}]
func CryptBytes(data []byte, offset int64, chunkKey []byte) {

	// Berechnet den AES-Block, in dem die bytes starten (muss nicht der Blockanfang sein)
	// Diese Blocknummer ist dann auch der Counter, da wir bei 0 mit dem Zählen beginnen.
	var ivInt int64
	modulo := offset % aes.BlockSize
	if modulo == 0 {
		// Ist der offset durch die Blockgröße restlos teilbar, dann ist der Start auch der Blockanfang
		ivInt = offset / aes.BlockSize
	} else {
		// Gibt es einen Rest, dann müssen wir den Blockanfang berechnen
		ivInt = (offset - modulo) / aes.BlockSize
	}

	// Die ermittelte Blocknummer wird nun in ein Array umgewandelt für den Counter.
	iv := make([]byte, aes.BlockSize)
	for i := 0; i < len(iv); i++ {
		iv[i] = byte(ivInt >> uint((15-i)*8))
	}

	// AES Konfiguration
	block, err := aes.NewCipher(chunkKey)
	if err != nil {
		panic("can't crypt bytes with wrong key length")
	}
	stream := cipher.NewCTR(block, iv)

	// Starten wir NICHT am Blockanfang, so müssen wir zuerst einige Bytes überspringen
	if modulo != 0 {
		tmp := make([]byte, modulo)
		stream.XORKeyStream(tmp, tmp)
	}

	// Daten ent- oder verschlüsseln
	stream.XORKeyStream(data, data)
}

// Erzeugt ein neues Keyfile das genau 128 random bytes enthält.
// Existierende Dateien werden NICHT überschrieben.
// Im Fehlerfall wird mit panic abgebrochen.
func NewRandomKeyfile(path string) {
	// random key erzeugen
	randkey := make([]byte, 128)
	n, err := io.ReadFull(rand.Reader, randkey)
	if err != nil {
		panic(err)
	}
	if n != 128 || len(randkey) != 128 {
		panic("can't create 128 byte key")
	}

	// existiert die datei schon? -> nicht überschreiben
	if _, err := os.Stat(path); err == nil {
		panic("file already exists")
	}

	// Datei schreiben
	err = ioutil.WriteFile(path, randkey, 0600)
	if err != nil {
		panic(err)
	}

	// testweise lesen  (bricht mit panic ab, wenn was nicht stimmt)
	k := LoadKeyfile(path)
	k.DbKey()
}

// LoadKeyfile lädt das Keyfile (genau 128 bytes groß) und generiert daraus die Schlüssel.
// Im Fehlerfall wird mit panic abgebrochen.
//   cryptSecret: Daraus wird der individuelle Chunk Schlüssel für die Verschlüsselung (AES-256-CTR) abgeleitet.
//   hashSecret: Daraus wird der individuelle ChunkCryptHash für den Chunk Dateiname abgeleitet.
//   indexSecret: Damit wird die DB verschlüsselt.
func LoadKeyfile(path string) KeyFile {

	// Schlüsseldatei einlesen und im Fehlerfall mit panic abbrechen
	filebytes, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	// In der Datei müssen genau 128 bytes sein, sonst abbruch mit panic.
	readlen := len(filebytes)
	if readlen != 128 {
		errMsg := fmt.Sprintf("key file must be exactly 128 bytes long (read %d bytes)", readlen)
		panic(errMsg)
	}

	// Die Schlüssel ableiten:
	// Für cryptSecret sind die ersten 64 bytes,
	// hashSecret bekommt die zweiten 64 bytes und
	// indexSecret wird mit einem Mix erzeugt.
	k := KeyFile{}
	k.cryptSecret = pbkdf2.Key(filebytes[:64], []byte("master_secret"), 60000, 64, sha512.New)
	k.hashSecret = pbkdf2.Key(filebytes[64:], []byte("hash_secret"), 60000, 64, sha512.New)
	k.indexSecret = pbkdf2.Key(filebytes[32:96], []byte("index_secret"), 99999, 64, sha512.New)

	return k
}

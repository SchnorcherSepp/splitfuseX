package core

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var (
	key           []byte
	db            SfDb
	writeTestFile string
)

func init() {
	// create writeTestFile
	writeTestFile = filepath.Join(os.TempDir(), "database.test")

	// generate test key with sha256 and a string
	h := sha256.New()
	h.Write([]byte("was geht up key"))
	key = h.Sum(nil)

	// generate a test DB with some samples
	db = SfDb{
		"hallo": SfFile{
			Size:          444,
			Mtime:         34,
			IsFile:        true,
			FolderContent: nil,
		},
		"du": SfFile{
			Size:          444,
			Mtime:         36,
			IsFile:        true,
			FolderContent: nil,
		},
		"da": SfFile{
			Size:          444,
			Mtime:         44,
			IsFile:        true,
			FolderContent: nil,
		},
		"drüben": SfFile{
			Size:          0,
			Mtime:         34,
			IsFile:        false,
			FileChunks:    nil,
			FolderContent: []FolderContent{{"file", true}, {"folder", false}},
		},
		"großes haus": SfFile{
			Size:          9,
			Mtime:         34,
			IsFile:        true,
			FolderContent: []FolderContent{{"file", true}, {"folder", false}},
		},
		"jejejeje": SfFile{
			Size:   923923,
			Mtime:  34,
			IsFile: true,
		},
		"dingdOng": SfFile{
			Size:   1234567,
			Mtime:  34,
			IsFile: true,
		},
		"leer": SfFile{},
	}
}

// ================================================================================================================== //

func TestSha512ToChunkHash(t *testing.T) {
	// generate some test bytes with sha512
	hf := sha512.New()
	hf.Write([]byte("test"))
	h := hf.Sum(nil)

	// use sha512ToChunkHash
	ch, err := sha512ToChunkHash(h)
	if err != nil {
		t.Error(err)
	}

	// check result
	hs, _ := hex.DecodeString("EE26B0DD4AF7E749AA1A8EE3C10AE9923F618980772E473F8819A5D4940E0DB27AC185F8A0E1D5F84F88BC887FD67B143732C304CC5FA9AD8E6F57F50028A8FF")
	if !bytes.Equal(ch[:], hs) {
		t.Errorf("sha512 hash wrong %x is not %x", ch, hs)
	}
}

func TestDbToFileAndDbFromFile(t *testing.T) {

	// schreiben
	err := DbToFile(writeTestFile, key, db)
	if err != nil {
		t.Error(err)
	}
	// lesen
	readdb, err := DbFromFile(writeTestFile, key)
	if err != nil {
		t.Error(err)
	}
	// vergleichen
	if !reflect.DeepEqual(readdb, db) {
		t.Error("DB not equal")
	}

}

func TestDbToEncGOBAndDbFromEncGOB(t *testing.T) {
	// db verschlüsseln
	nonce1, ciphertext1, err := dbToEncGOB(key, db)
	if err != nil {
		t.Errorf("db.toEncGOB error 1: %v", err)
	}

	nonce2, ciphertext2, err := dbToEncGOB(key, db)
	if err != nil {
		t.Errorf("db.toEncGOB error 2: %v", err)
	}

	if bytes.Equal(nonce1, nonce2) {
		t.Errorf("two nonce are equal: %v, %v", nonce1, nonce2)
	}

	if len(nonce1) != 12 {
		t.Errorf("the nonce (1) for GCM should be 12 bytes long: %d: %v", len(nonce1), nonce1)
	}

	if len(nonce2) != 12 {
		t.Errorf("the nonce (2) for GCM should be 12 bytes long: %d: %v", len(nonce2), nonce2)
	}

	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Errorf("ciphertext equal: %v, %v", ciphertext1, ciphertext2)
	}

	// db entschlüsseln
	newdb1, err := dbFromEncGOB(key, nonce1, ciphertext1)
	if err != nil {
		t.Errorf("dbFromEncGOB error 1: %v", err)
	}
	newdb2, err := dbFromEncGOB(key, nonce2, ciphertext2)
	if err != nil {
		t.Errorf("dbFromEncGOB error 2: %v", err)
	}

	if !reflect.DeepEqual(newdb1, newdb2) || !reflect.DeepEqual(newdb1, db) {
		t.Errorf("struct not equal\n%v\n%v\n%v", newdb1, newdb2, db)
	}

}

func TestCalcChunkSize(t *testing.T) {
	var test int64

	test = 0
	if x := CalcChunkSize(0, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #1: (%d)", x)
	}
	if x := CalcChunkSize(1, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #2: (%d)", x)
	}

	test = 17
	if x := CalcChunkSize(0, test); x != test {
		t.Errorf("TestCalcChunkSize Test #3: (%d)", x)
	}
	if x := CalcChunkSize(1, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #4: (%d)", x)
	}
	if x := CalcChunkSize(2, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #5: (%d)", x)
	}

	test = CHUNKSIZE*3 + 99
	if x := CalcChunkSize(0, test); x != CHUNKSIZE {
		t.Errorf("TestCalcChunkSize Test #6: (%d)", x)
	}
	if x := CalcChunkSize(1, test); x != CHUNKSIZE {
		t.Errorf("TestCalcChunkSize Test #7: (%d)", x)
	}
	if x := CalcChunkSize(2, test); x != CHUNKSIZE {
		t.Errorf("TestCalcChunkSize Test #8: (%d)", x)
	}
	if x := CalcChunkSize(3, test); x != 99 {
		t.Errorf("TestCalcChunkSize Test #9: (%d)", x)
	}
	if x := CalcChunkSize(4, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #10: (%d)", x)
	}
	if x := CalcChunkSize(5, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #11: (%d)", x)
	}

	test = CHUNKSIZE
	if x := CalcChunkSize(0, test); x != test {
		t.Errorf("TestCalcChunkSize Test #12: (%d)", x)
	}
	if x := CalcChunkSize(1, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #13: (%d)", x)
	}
	if x := CalcChunkSize(3, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #14: (%d)", x)
	}

	test = CHUNKSIZE - 1
	if x := CalcChunkSize(0, test); x != test {
		t.Errorf("TestCalcChunkSize Test #15: (%d)", x)
	}
	if x := CalcChunkSize(1, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #16: (%d)", x)
	}
	if x := CalcChunkSize(3, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #17: (%d)", x)
	}

	test = CHUNKSIZE + 1
	if x := CalcChunkSize(0, test); x != CHUNKSIZE {
		t.Errorf("TestCalcChunkSize Test #18: (%d)", x)
	}
	if x := CalcChunkSize(1, test); x != 1 {
		t.Errorf("TestCalcChunkSize Test #19: (%d)", x)
	}
	if x := CalcChunkSize(3, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #20: (%d)", x)
	}
	if x := CalcChunkSize(4, test); x != 0 {
		t.Errorf("TestCalcChunkSize Test #21: (%d)", x)
	}
}

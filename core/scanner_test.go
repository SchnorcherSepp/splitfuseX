package core

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

var (
	emptyTestFile string
)

func init() {
	// create emptyTestFile
	emptyTestFile = filepath.Join(os.TempDir(), "scanner.test")
	fh, err := os.OpenFile(emptyTestFile, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}
	defer fh.Close()
}

func TestScanFolder(t *testing.T) {
	db = SfDb{}

	// scan local dir
	db, changed1, _, err1 := ScanFolder("./", db, false)
	// scan local dir (again)
	db, changed2, _, err2 := ScanFolder("./", db, false)
	// add a fake file and scan local dir (again)
	db["iAmAFakeFile.txt"] = SfFile{}
	db, changed3, _, err3 := ScanFolder("./", db, false)

	// check errors
	if err1 != nil || err2 != nil || err3 != nil {
		t.Errorf("scan error")
	}

	// check change flag
	if changed1 != true || changed2 != false || changed3 != true {
		t.Errorf("changed flag wrong")
	}
}

func TestScanFileTime(t *testing.T) {
	// leer.testfile
	ol, err := scanFile(emptyTestFile)
	if err != nil {
		panic(err)
	}
	if ol.Size != 0 {
		t.Errorf("leer.testfile size wrong")
	}
	if ol.Mtime < 1000 {
		t.Errorf("leer.testfile mtime wrong")
	}

	// test.keyfile
	ot, err := scanFile(testKeyFile)
	if err != nil {
		panic(err)
	}
	if ot.Size != 128 {
		t.Errorf("test.keyfile size wrong")
	}
	if ot.Mtime < 1000 {
		t.Errorf("test.keyfile mtime wrong")
	}

	// testfail.keyfile
	of, err := scanFile(failKeyFile)
	if err != nil {
		panic(err)
	}
	if of.Size != 145 {
		t.Errorf("testfail.keyfile size wrong: %d", of.Size)
	}
	if of.Mtime < 1000 {
		t.Errorf("testfail.keyfile mtime wrong")
	}
}

func TestScanFileHash(t *testing.T) {
	ol, err := scanFile(emptyTestFile)
	if err != nil {
		panic(err)
	}
	if len(ol.FileChunks) != 0 {
		t.Errorf("leer.testfile hash wrong")
	}

	ot, err := scanFile(testKeyFile)
	ht, _ := hex.DecodeString("DD5610DABC3B5C9BF4F567AAD68AABA0489DD5B9C6552C8C8B6AC4EC6DFA71430C827DD2675BA6760BB635C59964218A3F17F6B995932F5C47CFEF666761CE69")
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(ot.FileChunks[0][:], ht) {
		t.Errorf("test.keyfile hash wrong")
	}

	of, err := scanFile(failKeyFile)
	hf, _ := hex.DecodeString("49107437477e374fdda857778573ee0043790b739389885c63270686119e9219fee42f93d45921ea587d7741c9b9ae0e66f0f9c2def0355cbd7bdf532f0f548f")
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(of.FileChunks[0][:], hf) {
		t.Errorf("testfail.keyfile hash wrong: %x", of.FileChunks[0][:])
	}
}

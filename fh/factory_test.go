package fh

import (
	"encoding/base64"
	"os"
	"reflect"
	"testing"

	"splitfuseX/backbone/local"
)

// testet die preload funktion (const PreloadSize)
func TestNewFileHandler(t *testing.T) {

	// Dummy Client
	client := local.NewDiskClient(testFileList[0].folder)

	// TestNewFileHandler()
	id := base64.StdEncoding.EncodeToString([]byte(testFileList[0].name))
	fh, err := NewFileHandler(client, id, PreloadSize*2)
	if err != nil {
		t.Error(err)
	}
	defer fh.CloseAndClear()

	// TESTS  PreloadSize
	b, err := fh.Download(PreloadSize-1, 10)
	shouldFail(t, err, "PreloadSize: one byte to much")
	testBytes(t, b, testFileList[0].path, PreloadSize, 0, "no bytes")

	b, err = fh.Download(PreloadSize, 10)
	shouldPass(t, err, "PreloadSize: max preload")
	testBytes(t, b, testFileList[0].path, PreloadSize, 10, "read 10 bytes")
}

// --- helper -----------------------------------------------

func shouldFail(t *testing.T, err error, text string) {
	if err == nil {
		t.Errorf("test should fail: %s: %v", text, err)
	}
}

func shouldPass(t *testing.T, err error, text string) {
	if err != nil {
		t.Errorf("test should pass: %s: %v", text, err)
	}
}

func testBytes(t *testing.T, b []byte, path string, offset int64, size int, test string) {

	// open origin file
	f, err := os.Open(path)
	if err != nil {
		t.Error(err)
	}
	defer f.Close()

	// read origin bytes
	ob := make([]byte, size)
	f.Seek(offset, 0)
	n, err := f.Read(ob)
	if err != nil {
		t.Error(err)
	}
	ob = ob[:n]

	// test len
	if len(b) != len(ob) {
		t.Errorf("%s: test data have a wrong length: %d != %d", test, len(b), len(ob))
	}

	// compare bytes
	if len(b) == 0 {
		return
	}

	if !reflect.DeepEqual(b, ob) {
		t.Errorf("%s: bytes not equal: offset=%d, size=%d\n'%x'\n'%x'\n", test, offset, size, b, ob)
	}
}

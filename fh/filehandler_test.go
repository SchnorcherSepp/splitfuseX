package fh

import (
	"encoding/base64"
	"io"
	"math/rand"
	"os"
	"testing"

	"splitfuseX/backbone/local"
)

// Test const
func TestFileHandler_Const(t *testing.T) {

	// Dummy Client
	client := local.NewDiskClient(testFileList[0].folder)

	// TestNewFileHandler()
	fileId := base64.StdEncoding.EncodeToString([]byte(testFileList[0].name))
	fh, err := NewFileHandler(client, fileId, 0)
	if err != nil {
		t.Error(err)
	}
	defer fh.CloseAndClear()

	// TESTS

	// MaxCacheSize
	b, err := fh.Download(0, 10)
	shouldPass(t, err, "MaxCacheSize: start")
	testBytes(t, b, testFileList[0].path, 0, 10, "MaxCacheSize: start 10 bytes")

	b, err = fh.Download(0, MaxCacheSize*2+1000)
	shouldPass(t, err, "MaxCacheSize: read a big block (more then the cache)")
	testBytes(t, b, testFileList[0].path, 0, MaxCacheSize*2+1000, "MaxCacheSize: start 2*cache")

	b, err = fh.Download(0, 10)
	shouldFail(t, err, "MaxCacheSize: requested bytes not in the cache")
	testBytes(t, b, testFileList[0].path, 0, 0, "MaxCacheSize: error: no bytes read")

	// MaxForwardJump
	b, err = fh.Download(MaxForwardJump*2, 10)
	shouldFail(t, err, "MaxForwardJump error")
	testBytes(t, b, testFileList[0].path, MaxForwardJump*2, 0, "MaxForwardJump error")

	b, err = fh.Download(MaxForwardJump, 10)
	shouldPass(t, err, "MaxForwardJump ok")
	testBytes(t, b, testFileList[0].path, MaxForwardJump, 10, "MaxForwardJump ok")
}

// Test download
func TestFileHandler_Download(t *testing.T) {

	// Dummy Client
	client := local.NewDiskClient(testFileList[0].folder)

	// TestNewFileHandler()
	fileId := base64.StdEncoding.EncodeToString([]byte(testFileList[0].name))
	fh, err := NewFileHandler(client, fileId, 0)
	if err != nil {
		t.Error(err)
	}
	defer fh.CloseAndClear()

	// TESTS
	fileSize := testFileList[0].size
	for offset := 0; offset <= fileSize+1; offset += rand.Intn(17011) - 7000 {
		if offset < 0 {
			offset = 0
		}

		b, err := fh.Download(int64(offset), 3333)
		shouldPass(t, err, "download looptest 1")
		testBytes(t, b, testFileList[0].path, int64(offset), 3333, "download looptest 1")
	}

	for offset := 199995000; offset < fileSize; offset++ {
		b, err := fh.Download(int64(offset), 17011)
		shouldPass(t, err, "download looptest 2")
		testBytes(t, b, testFileList[0].path, int64(offset), 17011, "download looptest 2")
	}
}

// read all test files
func TestFileHandler_BigRead(t *testing.T) {
	client := local.NewDiskClient(testFileList[0].folder)

	// all test files
	for _, x := range testFileList {
		fileId := base64.StdEncoding.EncodeToString([]byte(x.name))

		// new fh
		fh, err := NewFileHandler(client, fileId, 0)
		if err != nil {
			t.Error(err)
		}
		defer fh.CloseAndClear() // Das ist absicht! Die FH internen Caches sollen in der Schleife erhalten bleiben!

		// new file (origin)
		f, err := os.Open(x.path)
		if err != nil {
			t.Error(err)
		}
		defer f.Close()

		// read test
		z := 99871
		buf := make([]byte, z)
		for i := 0; true; i++ {
			// read fh
			b1, err2 := fh.Download(int64((i+1)*z), 1333333)
			b2, err1 := fh.Download(int64(i*z), 100)
			if (err1 != nil && err1 != io.EOF) || (err2 != nil && err2 != io.EOF) {
				t.Error(err1)
				t.Error(err2)
			}

			// len check
			if len(b2) != 100 && err2 == nil {
				t.Errorf("len check failed")
			}

			// compare
			f.Read(buf)
			for pos, c := range b2 {
				if c != buf[pos] {
					t.Errorf("bytes not equal:\n%x\n%x\n", b2, buf)
				}
			}

			// exit
			if len(b1) == 0 && len(b2) == 0 {
				if i*z < x.size {
					t.Errorf("read uncomplide")
				}
				break
			}
		}
	}
}

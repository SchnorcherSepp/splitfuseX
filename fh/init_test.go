package fh

import (
	"fmt"
	"math/rand"
	"os"
	"path"
	"testing"

	"splitfuseX/core"
)

var testFileList []testFile

type testFile struct {
	path   string
	name   string
	folder string
	size   int
}

// initialisiert die Testumgebung für dieses packet
func init() {
	buildUnitTestFiles()
}

// TestInitFiles prüft ob die gobale Variable test files enthält und init() funktioniert hat
func TestInitFiles(t *testing.T) {
	if len(testFileList) < 31 {
		t.Error("fh test files init error")
	}
}

// baut die testfiles
func buildUnitTestFiles() string {

	// create test folder
	testFolder := path.Join(os.TempDir(), "unit_tests_files")
	os.Mkdir(testFolder, 0700)

	// create test files
	sizeList := []int{
		200000000, // main test file
		0,         // empty file
		1,
		10,
		4095,
		4096, // disk block size
		4097,
		65535,
		65536, // foo bar
		65537,
		MaxCacheSize - 1,
		MaxCacheSize,
		MaxCacheSize + 1,
		PreloadSize - 1,
		PreloadSize,
		PreloadSize + 1,
		MaxForwardJump - 1,
		MaxForwardJump,
		MaxForwardJump + 1,
		ReadBufferSize - 1,
		ReadBufferSize,
		ReadBufferSize + 1,
		131071,
		131072, // fuse default buffer size
		131073,
		core.BUFFERSIZE - 1,
		core.BUFFERSIZE,
		core.BUFFERSIZE + 1,
		core.CHUNKSIZE - 1,
		core.CHUNKSIZE,
		core.CHUNKSIZE + 1,
	}

	testFileList = make([]testFile, len(sizeList))
	for testIndex, size := range sizeList {
		name := fmt.Sprintf("test%d_%d.dat", testIndex, size)
		createTestFile(testFolder, name, size, int64(testIndex+1337))
		testFileList[testIndex] = testFile{size: size, path: path.Join(testFolder, name), name: name, folder: testFolder}
	}

	return testFolder
}

// createTestFile generiert sehr performant ein random file
func createTestFile(folderPath string, filename string, size int, seed int64) {
	p := path.Join(folderPath, filename)

	// existiernede Datei nicht überschreiben
	s, err := os.Stat(p)
	if err == nil && s.Size() == int64(size) {
		return
	}

	// create file
	f, err := os.Create(p)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// write bytes
	fs := 0
	rs := rand.NewSource(seed)
	buf := make([]byte, 32768)
	for {
		// random
		randomRead(rs, buf)
		if fs+len(buf) > size {
			buf = buf[:size-fs]
		}
		// write
		n, err := f.Write(buf)
		if err != nil {
			panic(err)
		}
		fs += n
		// exit
		if len(buf) < 1 {
			break
		}
	}
}

// randomRead ist eine Hilfsfunktion für createTestFile() und schreibt in den übergebenen Puffer zufällige Bytes
func randomRead(r rand.Source, p []byte) {
	todo := len(p)
	offset := 0
	for {
		val := int64(r.Int63())
		for i := 0; i < 8; i++ {
			p[offset] = byte(val)
			todo--
			if todo == 0 {
				return
			}
			offset++
			val >>= 8
		}
	}
}

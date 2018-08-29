package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"testing"

	"splitfuseX/core"
	"splitfuseX/fh"
)

var testFolderOrig = path.Join(os.TempDir(), "unit_test_fuse_orig")
var testFolderChunks = path.Join(os.TempDir(), "unit_test_fuse_chunks")
var testFolderMount = path.Join(os.TempDir(), "unit_test_fuse_mount")

var testKeyFile = path.Join(os.TempDir(), "unit_test_fuse_keyfile")
var testDbFile = path.Join(os.TempDir(), "unit_test_fuse_db")

// initialisiert die Testumgebung für dieses Packet
func init() {

	// create test folder
	os.Mkdir(testFolderOrig, 0700)
	os.Mkdir(testFolderChunks, 0700)
	os.Mkdir(testFolderMount, 0700)

	// create test files
	sizeList := []int{
		0,
		1,
		4095,
		4096, // disk block size
		4097,
		fh.MaxCacheSize,
		fh.MaxCacheSize + 9,
		fh.PreloadSize + 1234,
		2*fh.MaxForwardJump + 13,
		131071,
		131072, // fuse default buffer size
		131073,
		core.BUFFERSIZE - 1,
		core.BUFFERSIZE,
		3*core.BUFFERSIZE + 1,
		core.CHUNKSIZE - 1000,
		core.CHUNKSIZE + 33,
	}

	for testIndex, size := range sizeList {
		name := fmt.Sprintf("test%d_%d.dat", testIndex, size)
		createTestFile(testFolderOrig, name, size, int64(testIndex+1337))
	}

	// create keyfile
	keyFileData, _ := hex.DecodeString("60a47fe220af89723bebda9fb741b479e15b74c817df1326b26d807d086376f6f3fe03a457d8458168cdc89f09303fe570f51305b48180e7d9fc6ef3e6aa2796915d5ca065469277d7a7eb4983f6dbcd932180cb6115bf1334c725a72b9be480b35a30a821f38a9b44660bdf0baabdf6391ad67fa1b5484503751d9afe0d4cf0")
	err := ioutil.WriteFile(testKeyFile, keyFileData, 0600)
	if err != nil {
		panic(err)
	}
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

// ====  TESTS  =============================================

// in der funtion upload() wird auch scan() aufgerufen!
func TestScanAndUpload(t *testing.T) {

	// die db muss aktualisierbar sein
	// ändert sich die db nicht, dann macht UPLOAD nichts!
	os.Remove(testDbFile)

	// alle chunks löschen (sonst macht upload nichts)
	os.RemoveAll(testFolderChunks)
	os.Mkdir(testFolderChunks, 0700)

	// upload (da ist scan mit dabei)
	uploadFunc(testKeyFile, testDbFile, testFolderOrig, "local", testFolderChunks, "", "", false, "indexius.dbius")
}

/*
// TODO: Dieser FUSE Test ist sehr instabil!  auf script auslagern??
func TestLinuxMount(t *testing.T) {
	// nur unter linux
	if runtime.GOOS != "linux" {
		panic("mount test need linux")
	}

	// umount (falls notwendig)
	exec.Command("fusermount", "-u", testFolderMount).Run()

	// mount
	client := clientModule("local", testFolderChunks, "", "")
	fuseServer := fuse.MountNormal(client, "indexius.dbius", testKeyFile, testFolderMount, false, false)
	go fuseServer.Serve()

	// originale dateiliste
	list, err := ioutil.ReadDir(testFolderOrig)
	if err != nil {
		panic(err)
	}

	for _, f1 := range list {
		// path
		path1 := path.Join(testFolderOrig, f1.Name())
		path2 := path.Join(testFolderMount, f1.Name())

		// datei im fuse
		f2, err := os.Stat(path2)
		if err != nil {
			panic(err)
		}

		// tests
		if f1.Size() != f2.Size() {
			t.Errorf("size not equal: %s", f1.Name())
		}
		if f1.ModTime() != f2.ModTime() {
			t.Errorf("time not equal: %s", f1.Name())
		}

		// data
		fh1, err := os.Open(path1)
		if err != nil {
			panic(err)
		}

		fh2, err := os.Open(path2)
		if err != nil {
			panic(err)
		}

		buf1 := make([]byte, 10000)
		buf2 := make([]byte, 10000)
		for {
			n1, err1 := fh1.Read(buf1)
			n2, err2 := fh2.Read(buf2)
			if bytes.Equal(buf1[:n1], buf2[:n2]) {
				t.Errorf("bytes not equal: %s", f1.Name())
				break
			}
			if err1 != err2 {
				t.Errorf("errors not equal: %s", f1.Name())
			}
		}

		fh1.Close()
		fh2.Close()
	}

	// umount
	fuseServer.Unmount()
}
*/

package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"runtime"
	"testing"
	"time"

	"splitfuseX/core"
	"splitfuseX/fh"
	"splitfuseX/fuse"
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

func checkChunk(folder, md5Str, name string) {
	p := path.Join(folder, name)
	b, err := ioutil.ReadFile(p)
	if err != nil {
		panic(err)
	}

	hash := md5.New()
	hash.Write(b)
	md5Str2 := fmt.Sprintf("%x", hash.Sum([]byte{}))

	if md5Str != md5Str2 {
		panic(fmt.Sprintf("%s: %s != %s", p, md5Str, md5Str2))
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

	// chunks prüfen
	checkChunk(testFolderChunks, "52807d542214c74747d241d072f1a07d", "0e5654f5dad72e4a930782da5ed941d6a54c678d7e6008d38c839ab01227bf83d58fb6a168cd3d5b64965375f9dc6fce565eaefc8e955f5f12a6b140a8345afa")
	checkChunk(testFolderChunks, "d68442a8d2cade052324fc2aa5d7039c", "106d3d36b3895de399a2bd90a37395bbc5c13f4d0dfdb4781cebca0150900616b0053071c0803d349011ab2b24ae12119f0570acb375d39379824e7c20b9c7f7")
	checkChunk(testFolderChunks, "37f70674005c62dfcb8ad3d9cfd9dbe2", "17a8d2142b53429e7d010ef6825415bc0ed70a022f043af85ed7a0588ea348a2fcb953efb7e0851722c2fa10ab04d602395b832ed874e5410640439891df36b0")
	checkChunk(testFolderChunks, "23d886859c548e09397f352646d7abab", "20edd9fa457c2cba8463bd7f7dc6ac21afdf9673853eb2602100058d665adb80f4861ae88a651da4c145590efa8fc6e0df2d44a74a654e37eead48c503188b03")
	checkChunk(testFolderChunks, "eaf4d7e335c2936d90fe59af42bbfd65", "25e317288f83083a5e43b7499af7c9c90a44776a95a0ef40a2ce6b35e28f3bce1c09a6124bb2c84a6e62ebeb74ccb963699f67fad052421809d4026313939475")
	checkChunk(testFolderChunks, "21c569901ae692227d573ab309c8ef95", "3438889537b39cf53193196b77827659c97c04c5426d374f938fd32d690986791bb505d7f6768495b355efbfad5aa9c2df0f71153361105b565adcfbb7412e66")
	checkChunk(testFolderChunks, "926f7a7bfc0ed6fb53921a5f51729982", "3bdb2720b8f95bce3a11e1507a7f0cbf98acb66d4cfc2d286d3b642692043c432ba515f38b1f1bdb0bca6a6d261395c7b5f0f63e439c5fac6b9750e959801de4")
	checkChunk(testFolderChunks, "d89a8b9be7e3e2dfff46a034bfe2cd7c", "4a051bf722261a34b5a630be0527b53bad73070f9656668ddd18b7d733be763b5edbdd33e7f1f91f5b30b2a0f3309c1fa24a2196d21f228713f20fede2075228")
	checkChunk(testFolderChunks, "5db3702afb48b262a6abd0fcce0712e3", "4ba461f080588236f8a4d74b645052dc00aa0fc9902caa87feda54d5da4d8661471583b9c2cadfdf2ee6cbc318ea575958e63ab64eab72f4b4cc1b3a3b81b786")
	checkChunk(testFolderChunks, "eacbe62638367d92f1edd30cce7e4221", "6026d9479048c7b8bf6c5a2fb7790b36274c697ae06849e472d725bc98d4fbb7434712b9fb735f5e32b9c5945c7750ef8c0489c6bb17b461012764ce71e0c6cd")
	checkChunk(testFolderChunks, "1ca9cdb950f5c67231838f9177918af5", "61cde125084dc77ce45fb49bcf5ea3a9c9be0ad295d20ef6038ddf057c9db77d92f1385b298d908a5e9ebd9f24e661ea839cc710ee06a68e36f3a6c7eea15176")
	checkChunk(testFolderChunks, "0c7a5029dc9c82b3e4307de92d4732e8", "771d8d5f08750862d96318ae4dfb164c41660af11ae03a4f7a64b401103f93925ef1d9bc564747622632f525b03cf7d9401e8e1ed28b5734bb20885b36d4af7b")
	checkChunk(testFolderChunks, "7c4bfbf61155fa6c9e506dc35798d3e9", "9800d08b853a77dd6a702d89fd0c216f06799a8db1021083d4706d8c336f5e7bf3443a283b31a737e200a18e73e1b3869474aa8142206785c4d60006452e5567")
	checkChunk(testFolderChunks, "fc6386533cad56cac9c6740671adea9f", "ad0873e4601f920ea7507657e529a18b4ffc7f049864dc2c9c8ecf32be175d639f5d614e2b5e5982d3e120a320cc426bb7f23becb001d581bda0c77c9fe8a59a")
	checkChunk(testFolderChunks, "ea177f35671f35e7197139a0a22362ad", "c38f10dbe04caf39f5a32f9e17672eeed56e262fc8f054d25f63b8b77837c307d51596139cb450db0517b470df5af36e56457005ab7dd070b6e2b298f2f9d209")
	checkChunk(testFolderChunks, "a530819b6f491aaf4358fb5140a71ddf", "e0a1ccb7cafaef54bbd1fb1e2ee1bda8710b973d8265d5f307ac9c15e4251de90af1b5e082becde84b74b6e40c44760c0e3eae5e2d3a48ec6ddb817b4addd3c8")
	checkChunk(testFolderChunks, "9f465340b573299d1211f99c086da4f0", "f9b554868a388df9ed389ab7928f7408d89b8985505f0109a329e54649fe6d07ca23ad0b568decdddee117e182d95928f3ff365104440dfcec32d1fa863d2bb2")
}

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
	fuseServer := fuse.MountNormal(client, "indexius.dbius", testKeyFile, testFolderMount, false, true)
	go fuseServer.Serve()

	time.Sleep(5 * time.Second)

	// originale dateiliste
	list, err := ioutil.ReadDir(testFolderOrig)
	if err != nil {
		t.Error(err)
	}

	// TEST: READ ALL
	bigFilePath := ""
	for _, f1 := range list {
		// path
		path1 := path.Join(testFolderOrig, f1.Name())
		path2 := path.Join(testFolderMount, f1.Name())

		// datei im fuse
		f2, err := os.Stat(path2)
		if err != nil {
			t.Error(err)
		}

		// bigFilePath
		if f2.Size() > core.CHUNKSIZE {
			bigFilePath = path2
		}

		// tests
		if f1.Size() != f2.Size() {
			t.Errorf("size not equal: %s", f1.Name())
		}
		if f1.ModTime().Unix() != f2.ModTime().Unix() {
			t.Errorf("time not equal: %s: %d != %d", f1.Name(), f1.ModTime().Unix(), f2.ModTime().Unix())
		}

		// data
		data1, err1 := ioutil.ReadFile(path1)
		if err1 != nil {
			t.Error(err1)
		}
		data2, err2 := ioutil.ReadFile(path2)
		if err2 != nil {
			t.Error(err2)
		}

		// check
		if len(data1) != len(data2) {
			t.Error("wrong data size")
		}
		if len(data1) > 0 && !bytes.Equal(data1, data2) {
			t.Errorf("bytes not equal: %s: %x != %x", f1.Name(), data1, data2)
		}
	}

	// TEST: READ BIG FILE
	f, err := os.Open(bigFilePath)
	if err != nil {
		t.Error(err)
	}
	defer f.Close()

	// read A (ok)
	_, err = f.Seek(0, 0)
	if err != nil {
		t.Error(err)
	}
	_, err = f.Read(make([]byte, fh.ReadBufferSize+3))
	if err != nil {
		t.Error(err)
	}

	// read B (err1)
	_, err = f.Seek(2*fh.MaxForwardJump, 0)
	if err != nil {
		t.Error(err)
	}
	_, err = f.Read(make([]byte, fh.ReadBufferSize+3))
	if err != nil {
		t.Error(err)
	}

	// read C (err2)
	_, err = f.Seek(0, 0)
	if err != nil {
		t.Error(err)
	}
	_, err = f.Read(make([]byte, fh.ReadBufferSize+3))
	if err != nil {
		t.Error(err)
	}

	// read D (err3)
	_, err = f.Seek(2*fh.MaxForwardJump, 0)
	if err != nil {
		t.Error(err)
	}
	_, err = f.Read(make([]byte, fh.ReadBufferSize+3))
	if err != nil {
		t.Error(err)
	}

	// read E (END)
	_, err = f.Seek(0, 0)
	if err != nil {
		t.Error(err)
	}
	_, err = f.Read(make([]byte, fh.ReadBufferSize+3))
	if err == nil { // !!!!!!!!!!!!!!!!
		t.Error(".....  no error??   whyyyyy!!!")
	}

	// umount
	time.Sleep(3 * time.Second)
	fuseServer.Unmount()
}

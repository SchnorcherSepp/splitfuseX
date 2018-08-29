package core

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

var (
	hashSecret    []byte
	cryptSecret   []byte
	testChunkHash []byte
	chunkKey      []byte
	indexSecret   []byte
	testKeyFile   string
	failKeyFile   string
)

func init() {

	// keyfile data
	data, _ := hex.DecodeString("60a47fe220af89723bebda9fb741b479e15b74c817df1326b26d807d086376f6f3fe03a457d8458168cdc89f09303fe570f51305b48180e7d9fc6ef3e6aa2796915d5ca065469277d7a7eb4983f6dbcd932180cb6115bf1334c725a72b9be480b35a30a821f38a9b44660bdf0baabdf6391ad67fa1b5484503751d9afe0d4cf0")
	dataFail, _ := hex.DecodeString("60a47fe220af68cdc89f09303fe570f51305b48180e7d9fc6ef3e6aa2796915d5ca065469277d7a7eb4983f6dbcd932180cb6115bf1334c725a72b9be480b35a30a821f38a9b44660bdf0baabdf639b35a30a821f38a9b44660bdf0baabdf639b35a30a821f38a9b44660bdf0baabdf639b35a30a821f38a9b44660bdf0baabdf6391ad67fa1b5484503751d9afe0d4cf0")

	// file paths
	testKeyFile = path.Join(os.TempDir(), "test.keyfile")
	failKeyFile = path.Join(os.TempDir(), "testfail.keyfile")

	err := ioutil.WriteFile(testKeyFile, data, 0600)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(failKeyFile, dataFail, 0600)
	if err != nil {
		panic(err)
	}

	// init other stuff
	hashSecret, _ = hex.DecodeString("d25e1be922e922bfe6492218d42bf0f8f3753ce6de030a78cf38a7c47e4b5882999baffa6c40d790bde0b30ac675af5a2b60f1026bf30ffe50656f17a0a4d68e")
	cryptSecret, _ = hex.DecodeString("e4c91c0559eb3db0e4d1df7d3d5a394619758231c2fe07ea0d7de2f6f8802ea539c46609a8b574d1ac320ee0ff08cf9c93caa3e82e031fd6377c62ee2a0b8948")
	chunkKey, _ = hex.DecodeString("1f685083dcddadb70c3d9d93da8eabb42176a09e2784d5766c06302ef542d2db")
	testChunkHash = []byte("testparthash")
	indexSecret, _ = hex.DecodeString("de936cc4451729817a60b3b8d66921cf7e39760ee1f7b64c4b539aba7a83dbb1d93d58ce44a7da8bf6b1854ac1e45ce3c4915449fe51b5988a6686b59b73e28a")
}

// ================================================================================================================== //

// Testet die Ableitung des Chunk Key.
// Er wird aus dem cryptSecret und dem sha512 hash des klartextes gebildet
// dadurch ist sichergestellt, das jede Datei einen eigenen Schlüssel verwendet
func TestCalcChunkKey(t *testing.T) {
	// cryptSecret setzen
	k := KeyFile{
		cryptSecret: cryptSecret,
	}

	// Ableitungsfunktion aufrufen
	key := k.CalcChunkKey(testChunkHash)

	// Schlüssel vergleichen
	if !bytes.Equal(chunkKey, key) {
		t.Errorf("chunkKey %x is not %x", key, chunkKey)
	}
}

// Das keyfile muss genau 128 byte enthalten.
// Dieser Test prüft, ab das Laden scheitert, wenn es nicht so ist
func TestFailLoadKeyfile(t *testing.T) {

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The call LoadKeyfile with testfail.keyfile should fail.")
		}
	}()

	LoadKeyfile(failKeyFile)
}

// Lädt das keyfile und prüft die Ableitung der einzelnen Schlüssel
func TestLoadKeyfile(t *testing.T) {
	k := LoadKeyfile(testKeyFile)

	if !bytes.Equal(k.cryptSecret, cryptSecret) {
		t.Errorf("cryptSecret %x is not %x", k.cryptSecret, cryptSecret)
	}
	if !bytes.Equal(k.hashSecret, hashSecret) {
		t.Errorf("hashSecret %x is not %x", k.hashSecret, hashSecret)
	}
	if !bytes.Equal(k.indexSecret, indexSecret) {
		t.Errorf("indexSecret %x is not %x", k.indexSecret, indexSecret)
	}
}

// schreibt ein neues keyfile
// wenn irgend etwas nicht passt, dann wird sowieso panic geworfen
func TestNewRandomKeyfile(t *testing.T) {

	// datei löschen
	err := os.Remove(writeTestFile)
	if err != nil {
		t.Error(err)
	}

	// datei schreiben
	NewRandomKeyfile(writeTestFile)
}

// teste die verschlüsselung des chunkhashes (ist dann der dateiname)
func TestCalcChunkCryptHash(t *testing.T) {
	hashsecret := []byte("oijajfoiajfdoiajsdojassdfo")
	parthash := []byte("ich bin ein kleiner knuddeliger part")
	enchash, _ := hex.DecodeString("01a3a9314eb0357c3eb0fd8ddb88cd0c90423c38f2b9b0a808334999dce717d0b3cda79eab836433f8c4162f3270c5af10f0248d13b931978b0ddd48f207da07")

	k := KeyFile{hashSecret: hashsecret}
	b := k.CalcChunkName(parthash)

	if !bytes.Equal(b, enchash) {
		t.Errorf("TestCalcChunkCryptHash:\n%v\n%v", b, enchash)
	}
}

// verschlüsselt einen text und prüft ihn bei verschiedenen offsets
func TestCryptBytes(t *testing.T) {
	data := []byte("Das ist ein sehr langer und geheimer text den ich hier entschluessel will! Jajaja, so ist das. Geheim und geheimer und so ein Zeug! Penis!?= ENDE")
	encdata0, _ := hex.DecodeString("5a81c011433c79455bb7a3cbcdcc33e77dd25f6b859c876dd9c0a292476e05b4463e5ef33d88e49099291964936f2b824e92bfa9e135f943b50f63869940fcc4c2ca435147ab73c4c116ea40cc46ede6d93b8b5596d8a4b1471e55883874a6c25cbde345f0d77df47658e2c0661e43adbf6350eac073866e1b9b26248c0253a82d1d77504d2b2444cb89e1f9604f51d781")
	encdata1G, _ := hex.DecodeString("79db76e0a5ec269d4c8b20105592123aa8125d08e0355d7b4fa80cb4d83ec4aa575c1f8b2095926aaee5c173416aab638ca55ee281f183302601e0ce6e2f0b2e3bda2ca8c9d8ab8a895b07c6d02f3d3a4c3c2dc2e046173690cc8fe0d319e347ac28baae5aabd0f0f868ba004198912b1e458f28b5b7306bbefeb31820279eb7badc05ff84a4c87aa4b0eb8defcb691b51")
	key, _ := hex.DecodeString("8374fd0d213ab30f4eb6ae85d43dd4981234b566fff84cfb161e3500b709563e")

	// start offset 0
	for i := 0; i <= len(data)-50; i++ {
		work := make([]byte, len(encdata0))
		copy(work, encdata0)

		CryptBytes(work[i:], int64(i), key)

		if !bytes.Equal(work[i:], data[i:]) {
			t.Errorf("%s\n is not\n %s\n", work[i:], data[i:])
		}
	}

	// start offset 1000000000
	for i := 1000000000; i <= len(data)-50; i++ {
		work := make([]byte, len(encdata1G))
		copy(work, encdata1G)

		CryptBytes(work[i:], int64(i), key)

		if !bytes.Equal(work[i:], data[i:]) {
			t.Errorf("%s\n is not\n %s\n", work[i:], data[i:])
		}
	}

}

func TestCryptReader(t *testing.T) {
	// test data
	key, _ := hex.DecodeString("8374fd0d213ab30f4eb6ae85d43dd4981234b566fff84cfb161e3500b709563e")
	data := []byte("Das ist ein sehr langer und geheimer text den ich hier entschluessel will! Jajaja, so ist das. Geheim und geheimer und so ein Zeug! Penis!?= ENDE")
	dataEnc, _ := hex.DecodeString("5a81c011433c79455bb7a3cbcdcc33e77dd25f6b859c876dd9c0a292476e05b4463e5ef33d88e49099291964936f2b824e92bfa9e135f943b50f63869940fcc4c2ca435147ab73c4c116ea40cc46ede6d93b8b5596d8a4b1471e55883874a6c25cbde345f0d77df47658e2c0661e43adbf6350eac073866e1b9b26248c0253a82d1d77504d2b2444cb89e1f9604f51d781")

	// readers
	r := bytes.NewReader(data)
	cr := CryptReader(r, key)

	// test
	buf := make([]byte, 1)
	for pos, c := range dataEnc {
		_, err := cr.Read(buf)
		if err != nil {
			t.Error(err)
		}
		if c != buf[0] {
			t.Errorf("CryptReader error: %x != %x at pos %d", c, buf[0], pos)
		}
	}

}

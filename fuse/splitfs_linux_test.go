package fuse

import (
	"io/ioutil"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"splitfuseX/backbone/local"
	"splitfuseX/core"
)

// Prüft ob die CheckUpdate Funktion wie geplant funktioniert
//  x   0 ... Erfolgreich
//  x 401 ... Intervall noch nicht erreicht
//    402 ... Fehler beim Aktualisieren der FileList (ApiClient)
//  x 403 ... DBfile existiert nicht
//  x 404 ... DBfile unverändert (alles bleibt gleich)
//    405 ... Fehler beim Download der DB
//  x 406 ... Fehler beim Entschlüsseln der DB (MAC)
func TestCheckDbUpdate(t *testing.T) {

	testFolder := path.Join(os.TempDir(), "TestCheckDbUpdate")
	os.Mkdir(testFolder, 0700)
	dbPath := path.Join(testFolder, "index.db")

	// defekte db datei schreiben
	err := ioutil.WriteFile(dbPath, []byte("hallo error"), 0600)
	if err != nil {
		t.Error(err)
	}

	// test filesystem
	fs := SplitFs{
		debug:      false, // DEBUG: setze das auf true, wenn du mehr sehen willst (DEBUG)
		interval:   2,
		dbFileName: "index.db",
		keyFile:    core.KeyFile{},
		apiClient:  local.NewDiskClient(testFolder),
		mutex:      &sync.Mutex{},
	}

	// PHASE 0
	if s := fs.checkDbUpdate(); s != 406 { // defekte db
		t.Errorf("update test failed #0.1: status is %d", s)
	}
	time.Sleep(2100 * time.Millisecond) // 2100 ms

	// korrekte DB schreiben
	err = core.DbToFile(dbPath, fs.keyFile.DbKey(), core.SfDb{})
	if err != nil {
		panic(err)
	}

	// PHASE 1
	/*
		SOOOOOO!  WÄRE DAS DER google drive CLIENT, DANN WÄRE DAS HIER 402!
		ABER DA ZUM TESTEN DER local CLIENT BENUTZT WIRD, PASSIERT DAS NICHT!

			if s := fs.checkDbUpdate(); s != 402 { // filelist nicht initialisiert
				t.Errorf("update test failed #1.1: status is %d", s)
			}

			fs.apiClient.InitFileList() // INIT

			if s := fs.checkDbUpdate(); s != 401 { // zeit noch nicht um
				t.Errorf("update test failed #1.2: status is %d", s)
			}
			time.Sleep(2100 * 1000000) // 2100 ms
	*/

	// PHASE 2
	if s := fs.checkDbUpdate(); s != 0 { // korrekt geladen
		t.Errorf("update test failed #2.1: status is %d", s)
	}
	if s := fs.checkDbUpdate(); s != 401 { // zeit noch nicht um
		t.Errorf("update test failed #2.2: status is %d", s)
	}
	time.Sleep(2100 * time.Millisecond) // 2100 ms

	if s := fs.checkDbUpdate(); s != 404 { // db file unverändert
		t.Errorf("update test failed #2.3: status is %d", s)
	}
	if s := fs.checkDbUpdate(); s != 401 { // zeit noch nicht um
		t.Errorf("update test failed #2.4: status is %d", s)
	}
	time.Sleep(2100 * time.Millisecond) // 2100 ms

	// PHASE 3
	// korrekte DB schreiben (AKTUALISIEREN)
	err = core.DbToFile(dbPath, fs.keyFile.DbKey(), core.SfDb{})
	if err != nil {
		panic(err)
	}

	if s := fs.checkDbUpdate(); s != 0 { // db file aktualisiert
		t.Errorf("update test failed #3.3: status is %d", s)
	}
	if s := fs.checkDbUpdate(); s != 401 { // zeit noch nicht um
		t.Errorf("update test failed #3.4: status is %d", s)
	}
	time.Sleep(2100 * time.Millisecond) // 2100 ms

	// db löschen
	err = os.Remove(dbPath)
	if err != nil {
		t.Error(err)
	}
	err = fs.apiClient.UpdateFileList()
	if err != nil {
		t.Error(err)
	}

	// PHASE 4
	if s := fs.checkDbUpdate(); s != 403 { // Datei existiert nicht
		t.Errorf("update test failed #4.1: status is %d", s)
	}
	if s := fs.checkDbUpdate(); s != 401 { // zeit noch nicht um
		t.Errorf("update test failed #4.2: status is %d", s)
	}
}

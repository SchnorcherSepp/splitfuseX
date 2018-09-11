package fuse

import (
	"fmt"
	"sync"
	"time"

	"splitfuseX/backbone"
	"splitfuseX/core"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

// SplitFs ist ein pathfs und hier sind fast alle eigenen FUSE Funktionen gebunden.
type SplitFs struct {
	pathfs.FileSystem

	debug      bool            // zusätzliche Meldungen einblenden
	interval   int64           // update interval in Sekunden  (bei 0 wird der Defaultwert genommen)
	dbFileName string          // Der Name der Datenbank im ChunkFolder wie zB 'index.db' (siehe ApiClient.InitFileList())
	keyFile    core.KeyFile    // Keyfile mit allen Schlüsseln
	apiClient  backbone.Client // Verbindung zu Google Drive! ACHTUNG: .InitFileList() muss bereits passiert sein!!

	mutex        *sync.Mutex
	db           core.SfDb // Datenbank
	lastDbUpdate int64     // wann wurde zuletzt checkDbUpdate() ausgeführt (Unix Time)
	lastDbMtime  int64     // die mtime des zuletzt geladenen DB files (RFC 3339 date-time: 2018-08-03T12:03:30.407Z)
}

// Diese Funktion wird von openDir getriggert.
// Dabei stellt sie sicher, dass sie nur alle x sekunden einen Effekt hat
// return:
//     0 ... Erfolgreich
//   401 ... Intervall noch nicht erreicht
//   402 ... Fehler beim Aktualisieren der FileList (ApiClient)
//   403 ... DBfile existiert nicht
//   404 ... DBfile unverändert (alles bleibt gleich)
//   405 ... Fehler beim Download der DB
//   406 ... Fehler beim Entschlüsseln der DB (MAC)
func (fs *SplitFs) checkDbUpdate() int {
	// LOCK / UNLOCK
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	// check interval
	var interval int64 = 10 * 60 // 10 min
	if fs.interval > 0 {
		interval = fs.interval
	}

	// update nur alle 5 Minuten versuchen, egal ob erfolgreich oder nicht
	now := time.Now().Unix()
	thenPlus := fs.lastDbUpdate + interval
	if thenPlus > now {
		// nur alle x Sekunden erlauben
		return 401
	}
	fs.lastDbUpdate = now

	// Funktionsaufruf melden (debug=true)
	debug(fs.debug, LOGINFO, "checkDbUpdate(): update", nil)

	// Aktualisiere die Filelist
	// ACHTUNG: Die Initialisierung muss daher bereits vorher passieren! (siehe ApiClient.InitFileList())
	err := fs.apiClient.UpdateFileList()
	if err != nil {
		// Wie Fehler? Das ist eigentlich sehr schlecht! Das müsste immer gehen!
		debug(fs.debug, LOGERROR, "checkDbUpdate(): can't update FileList", err)
		return 402
	}

	// Neue DB suchen: Hat sich die Datei verändert?
	// Nur aktualisierte Dateien laden
	newestFile := &backbone.FileObject{}
	for _, file := range fs.apiClient.FileList() {
		if file.Name == fs.dbFileName {
			// betrachtete Datei hat den richtigen Namen
			if newestFile.ModifiedTime < file.ModifiedTime {
				// betrachtete Datei ist neuer als 'newestFile'
				newestFile = file
			}
		}
	}

	// wurde etwas gefunden?
	if newestFile.ModifiedTime <= 0 {
		// db file nicht da? ka. einfach abbrechen
		debug(fs.debug, LOGERROR, fmt.Sprintf("checkDbUpdate(): no db file found: '%s'", fs.dbFileName), nil)
		return 403
	}

	// ist das DBfile unverändert?
	if newestFile.ModifiedTime == fs.lastDbMtime {
		// Datei ist noch gleich
		debug(fs.debug, LOGINFO, fmt.Sprintf("checkDbUpdate(): file unchanged: '%d'", fs.lastDbMtime), nil)
		return 404
	}

	// download (OPEN)
	resp, err := fs.apiClient.Read(newestFile.Id, 0, 44222111) // 44222111 (ca 44mb) ist eine willkührliche Grenze für die DB
	if err != nil {
		// db konnte nicht geladen werden
		debug(fs.debug, LOGERROR, fmt.Sprintf("checkDbUpdate(): can't open db file: '%s'", fs.dbFileName), err)
		return 405
	}
	defer resp.Close() // CLOSE

	// lesen und entschlüsseln
	newdb, err := core.DbFromReader(resp, fs.keyFile.DbKey())
	if err != nil {
		// fehler beim Entschlüsseln der datei
		// eventuell wird die Datei gerade erst geschrieben
		debug(fs.debug, LOGERROR, fmt.Sprintf("checkDbUpdate(): can't decrypt db file: '%s'", fs.dbFileName), err)
		return 406
	}

	// neue DB setzen
	fs.db = newdb

	// ACHTUNG: Nachdem die DB gesetzt wurde, muss nun auch fs.lastDbMtime gespeichert werden
	// Vorher darf das nicht passieren, weil sonst die DB nicht geladen wird im Fehlerfall
	fs.lastDbMtime = newestFile.ModifiedTime

	// log schreiben (debug=true)
	debug(fs.debug, LOGINFO, fmt.Sprintf("checkDbUpdate(): OK: %s, %d, %s", fs.dbFileName, fs.lastDbMtime, newestFile.Id), nil)

	// bei Erfolg, 0 zurück geben
	return 0
}

// GetAttr gibt die File-Attribute für Einträge aus der DB zurück.
func (fs *SplitFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	// db update triggern
	fs.checkDbUpdate()

	// FIX: root
	if name == "" {
		name = "."
	}

	// Element in der DB suchen
	dbFile, ok := fs.db[name]
	if !ok {
		debug(fs.debug, LOGERROR, "GetAttr(): file/folder not found in DB: "+name, nil)
		return nil, fuse.ENOENT
	}

	// Attribute setzen
	ret := &fuse.Attr{}

	// Basis-Attribute setzen
	ret.Size = uint64(dbFile.Size)
	ret.Mtime = dbFile.Mtime
	ret.Ctime = dbFile.Mtime
	ret.Atime = dbFile.Mtime

	// Mode (Datei/Ordner)
	if dbFile.IsFile {
		ret.Mode = fuse.S_IFREG | 0644
		ret.Nlink = 1
	} else {
		ret.Mode = fuse.S_IFDIR | 0755
		ret.Nlink = uint32(len(dbFile.FolderContent))
	}

	return ret, fuse.OK
}

// OpenDir listet den Ordnerinhalt auf.
func (fs *SplitFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {

	// db update triggern
	fs.checkDbUpdate()

	// FIX: root
	if name == "" {
		name = "."
	}

	// Ordner in der DB suchen
	dbFile, ok := fs.db[name]
	if !ok {
		debug(fs.debug, LOGERROR, "OpenDir(): folder not found in DB: "+name, nil)
		return nil, fuse.ENOENT
	}

	// prüfen, ob es e ein Ordner ist
	if dbFile.IsFile {
		return nil, fuse.ENOTDIR
	}

	// enthaltene Elemente zurück geben
	l := len(dbFile.FolderContent)
	c = make([]fuse.DirEntry, 0, l)

	for _, v := range dbFile.FolderContent {
		// Sub-Element erzeugen
		tmp := fuse.DirEntry{Name: v.Name}
		// Mode setzen (Datei oder Ordner)
		// Nur das höchste Bit (eg. S_IFDIR) wird ausgewertet
		if v.IsFile {
			tmp.Mode = fuse.S_IFREG
		} else {
			tmp.Mode = fuse.S_IFDIR
		}
		// zu Liste hinzufügen
		c = append(c, tmp)
	}

	return c, fuse.OK
}

// Öffnet eine Datei und berechnet dabei alle Informationen, um auf die Chunks zuzugreifen.
func (fs *SplitFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {

	// Datei in der DB suchen
	dbFile, ok := fs.db[name]
	if !ok {
		debug(fs.debug, LOGERROR, "Open(): file not found in DB: "+name, nil)
		return nil, fuse.ENOENT
	}

	// prüfen, ob es e eine Datei ist
	if !dbFile.IsFile {
		return nil, fuse.ENOENT
	}

	// filelist mit den chunks erstellen
	filelist := fs.apiClient.FileList()

	// chunkkeys und fileids ermitteln
	l := len(dbFile.FileChunks)
	chunkKeys := make([][]byte, l)
	fileIds := make([]string, l)
	for i, chunkHash := range dbFile.FileChunks {

		// berechnungen
		chunkKeys[i] = fs.keyFile.CalcChunkKey(chunkHash[:])
		chunkName := fmt.Sprintf("%x", fs.keyFile.CalcChunkName(chunkHash[:]))
		chunkSize := core.CalcChunkSize(i, dbFile.Size)

		// fileId suchen
		for fileId, obj := range filelist {
			if obj.Name == chunkName && obj.Size == chunkSize {
				fileIds[i] = fileId
				break
			}
		}

		// keine fileId ?
		if fileIds[i] == "" {
			debug(fs.debug, LOGERROR, "Open(): can't find a fileId: "+name, nil)
			return nil, fuse.ENOENT
		}
	}

	// Datei zurückgeben
	return &SplitFile{
		File:      nodefs.NewDefaultFile(),
		debug:     fs.debug,
		dbFile:    dbFile,
		chunkKeys: chunkKeys,
		fileIds:   fileIds,
		apiClient: fs.apiClient,
		errRetrys: 3, // max. 3x darf der FH ungestraft einen Lesefehler verursachen
	}, fuse.OK
}

// Informationen für 'df -h'
func (fs *SplitFs) StatFs(name string) *fuse.StatfsOut {

	// Summe aller Dateien berechnen
	var sum uint64 = 0
	for _, v := range fs.db {
		sum += uint64(v.Size)
	}

	// Dingige Dinge
	var blocksize uint64 = 8192
	var total uint64 = 109951162777600 // 100 TiB
	var free = total - sum

	return &fuse.StatfsOut{
		Blocks:  total / blocksize,
		Bfree:   free / blocksize,
		Bavail:  free / blocksize,
		Bsize:   uint32(blocksize),
		NameLen: 255,
		Frsize:  uint32(blocksize),
	}
}

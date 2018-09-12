package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"splitfuseX/backbone"
	"splitfuseX/backbone/drive"
	"splitfuseX/backbone/local"
	"splitfuseX/core"
	"splitfuseX/fuse"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app   = kingpin.New(filepath.Base(os.Args[0]), "Ein Kommandozeilen-Tool zum Verwalten und Mounten von SplitFUSE")
	debug = app.Flag("debug", "Aktiviert den Debug-Mode für FUSE, SCAN oder UPLOAD").Bool()

	oauth       = app.Command("oauth", "Hilft bei der Erstellung aller Dateien für den Zugriff auf Google Drive")
	oauthClient = oauth.Flag("client", "Pfad zur client_secret Datei").Default("client_secret.json").String()
	oauthToken  = oauth.Flag("token", "Pfad zur Token Datei").Default("token.json").String()
	oauthWrite  = oauth.Flag("upload", "Soll ein schreibender Zugriff auf Google Drive erlaubt werden?").Bool()

	gen    = app.Command("newkey", "Erstellt ein neues Keyfile für SplitFuse")
	genKey = gen.Flag("key", "Pfad zum Keyfile (Datei darf noch NICHT existieren)").Default("splitfuse.key").String()

	scan    = app.Command("scan", "Scant einen Ordner und aktualisiert gegebebenfalls die DB")
	scanKey = scan.Flag("key", "Pfad zum Keyfile").Default("splitfuse.key").ExistingFile()
	scanDB  = scan.Flag("db", "Pfad zur DB (wird überschrieben)").Default("splitfuse.db").String()
	scanDir = scan.Flag("dir", "Pfad zum Ordner mit allen Klartext Dateien").Required().ExistingDir()

	upload       = app.Command("upload", "Lädt alle Chunks in den angegebenen Speicher. Die DB wird dabei aktualisiert und überschrieben!")
	uploadKey    = upload.Flag("key", "Pfad zum Keyfile").Default("splitfuse.key").ExistingFile()
	uploadDB     = upload.Flag("db", "Pfad zur DB").Default("splitfuse.db").ExistingFile()
	uploadDir    = upload.Flag("dir", "Pfad zum Ordner mit allen Klartext Dateien").Required().ExistingDir()
	uploadMod    = upload.Flag("module", "'drive' für Google Drive und 'local' für die lokale Festplatte").Required().String()
	uploadDest   = upload.Flag("dest", "Für 'drive' muss hier eine FolderID angegeben werden (es geht auch der Alias root). Für 'local' ist hier der Pfad zum Zielordner anzugeben.").Required().String()
	uploadClient = upload.Flag("client", "Pfad zur client_secret Datei (für 'drive')").Default("client_secret.json").String()
	uploadToken  = upload.Flag("token", "Pfad zur Token Datei (für 'drive')").Default("token.json").String()
	uploadDbName = upload.Flag("dbFileName", "Die DB wird unter dem angegebenen Namen bei den Chunks im Speicher abgelegt.").Default("index.db").String()
	uploadForce  = upload.Flag("force", "Zwingt zu einem SCAN und UPLOAD, auch wenn sich die DB nicht verändert hat. (Die DB wird dabei immer neu hochgeladen!)").Bool()

	clean       = app.Command("clean", "Löscht nicht mehr benötigte Chunks. Die DB muss vorher mit SCAN aktualisiert werden. (ACHTUNG: Datenverlust!)")
	cleanKey    = clean.Flag("key", "Pfad zum Keyfile").Default("splitfuse.key").ExistingFile()
	cleanDB     = clean.Flag("db", "Pfad zur DB").Default("splitfuse.db").ExistingFile()
	cleanMod    = clean.Flag("module", "'drive' für Google Drive und 'local' für die lokale Festplatte").Required().String()
	cleanDest   = clean.Flag("dest", "Für 'drive' muss hier eine FolderID angegeben werden (es geht auch der Alias root). Für 'local' ist hier der Pfad zum Zielordner anzugeben.").Required().String()
	cleanClient = clean.Flag("client", "Pfad zur client_secret Datei (für 'drive')").Default("client_secret.json").String()
	cleanToken  = clean.Flag("token", "Pfad zur Token Datei (für 'drive')").Default("token.json").String()

	normal       = app.Command("mount", "Mountet Klartext Dateien")
	normalMod    = normal.Flag("module", "'drive' für Google Drive und 'local' für die lokale Festplatte").Required().String()
	normalMount  = normal.Flag("dir", "Ordner, in dem die Klartext Dateien gemountet werden sollen").Required().ExistingDir()
	normalChunks = normal.Flag("chunks", "Die folderId des Chunk-Ordners oder sein Pfad").Default("root").String()
	normalDbName = normal.Flag("dbfileName", "Die DB wird unter dem angegebenen Namen bei den Chunks im Speicher regelmäßig eingelesen.").Default("index.db").String()
	normalClient = normal.Flag("client", "Pfad zur client_secret Datei (für 'drive')").Default("client_secret.json").String()
	normalToken  = normal.Flag("token", "Pfad zur Token Datei (für 'drive')").Default("token.json").String()
	normalKey    = normal.Flag("key", "Pfad zum Keyfile").Default("splitfuse.key").ExistingFile()
	normalCache  = normal.Flag("cache", "Puffert die FileList in einer Datei und beschleunigt den Start des FUSE. Ein leerer String deaktiviert diese Funktion!").Default("cache.dat").String()
)

func main() {
	app.Version("splitfuseX 3.14")
	app.UsageTemplate(kingpin.LongHelpTemplate)
	command := kingpin.MustParse(app.Parse(os.Args[1:]))

	switch command {
	case oauth.FullCommand(): //________________________________________________________________________________________
		// neuen token schreiben (Google Drive API)
		drive.CreateTokenFile(*oauthClient, *oauthToken, *oauthWrite)

	case gen.FullCommand(): //__________________________________________________________________________________________
		// neues keyfile schreiben (SplitFuse Verschlüsselung)
		core.NewRandomKeyfile(*genKey)

	case scan.FullCommand(): //_________________________________________________________________________________________
		// db aktualisieren
		scanFunc(*scanKey, *scanDB, *scanDir, *debug)

	case upload.FullCommand(): //_______________________________________________________________________________________
		// db aktualisieren und alles hochladen
		uploadFunc(*uploadKey, *uploadDB, *uploadDir, *uploadMod, *uploadDest, *uploadClient, *uploadToken, *debug, *uploadDbName)

	case clean.FullCommand(): //________________________________________________________________________________________
		// alte chunks im Speicher löschen
		cleanFunc(*cleanKey, *cleanDB, *cleanMod, *cleanDest, *cleanClient, *cleanToken)

	case normal.FullCommand(): //_______________________________________________________________________________________
		// FUSE MOUNT (Linux only)
		client := clientModule(*normalMod, *normalChunks, *normalClient, *normalToken, *normalCache)
		fuse.MountNormal(client, *normalDbName, *normalKey, *normalMount, *debug, false)
	}
}

//____________________________________________________________________________________________________________________//

// scanFunc liest einen Ordner ein und aktualisiert gegebenenfalls die DB
// Es wird true zurück gegeben, sollte es zu einer Änderung gekommen sein!
func scanFunc(keyFile, dbFile, dir string, debug bool) bool {

	// keyFile laden
	k := core.LoadKeyfile(keyFile)

	// alte DB laden
	oldDB, err := core.DbFromFile(dbFile, k.DbKey())
	if err != nil {
		panic(err)
	}

	// Ordner scannen
	newDB, changed, summary, err := core.ScanFolder(dir, oldDB, debug)
	if err != nil {
		panic(err)
	}

	// gibt es änderungen? -> DB überschreiben
	if changed {
		print("update DB: ")
		println(summary)
		err = core.DbToFile(dbFile, k.DbKey(), newDB)
		if err != nil {
			panic(err)
		}
	}

	// changed?
	return changed
}

// clientModule ist eine Hilfsfunktion die je nach 'module' eine andere Client Implementierung zurück gibt.
func clientModule(module, destination, apiClient, apiToken, cacheFile string) backbone.Client {
	switch module {
	case "drive":
		return drive.NewApiClient(apiClient, apiToken, cacheFile, destination)

	case "local":
		return local.NewDiskClient(destination)

	default:
		panic("unsupported module: use 'drive' or 'local'")
	}
}

// uploadFunc aktualisiert die DB mit scanFunc() und lädt dann neue Chunks in den Speicher.
// Die DB wird ebenfalls aktualisiert. Dabei werden zuerst alle DBs mit dem angegebenen Namen gelöscht und dann die neue DB gespeichert.
func uploadFunc(keyFile, dbFile, dir, module, destination, apiClient, apiToken string, debug bool, dbFileNameOnStorage string) {
	uploadCount := 0

	// DB AKTUALISIEREN
	changed := scanFunc(keyFile, dbFile, dir, debug)
	if !changed && !*uploadForce {
		return // NICHTS ANDERS, NICHTS ÄNDERN, NICHTS HOCHLADEN
	}

	// keyFile laden
	k := core.LoadKeyfile(keyFile)

	// DB laden
	db, err := core.DbFromFile(dbFile, k.DbKey())
	if err != nil {
		panic(err)
	}

	// client erstellen (drive oder local)
	client := clientModule(module, destination, apiClient, apiToken, "")

	// fileList initialisieren
	if debug {
		fmt.Printf("DEBUG: init FileList\n")
	}

	err = client.InitFileList()
	if err != nil {
		panic(err)
	}
	clientFileList := client.FileList()

	// search new stuff (welche chunks sind noch nicht am Speicher (drive oder local)
	if debug {
		fmt.Printf("DEBUG: search new stuff\n")
	}

	for origFilePath, dbFileObj := range db {

		// alle Dateien in der DB ...
		for chunkIndex, chunk := range dbFileObj.FileChunks {

			// ... die wiederum aus mehreren Chunks bestehen
			chunkFileName := fmt.Sprintf("%x", k.CalcChunkName(chunk[:]))
			chunkFileKey := k.CalcChunkKey(chunk[:])
			chunkFileSize := core.CalcChunkSize(chunkIndex, dbFileObj.Size)

			// in der FileList des Speichers suchen
			found := false
			for _, clientFileObj := range clientFileList {
				if clientFileObj.Size == chunkFileSize && clientFileObj.Name == chunkFileName {
					found = true
					break
				}
			}

			// ACTION: Chunk ist noch NICHT am Speicher (drive/local) -> HOCHLADEN
			if !found {
				// original Datei öffnen
				fh, err := os.Open(path.Join(dir, origFilePath))
				if err != nil {
					panic(err)
				}
				// zum chunk Anfang springen
				_, err = fh.Seek(int64(chunkIndex)*core.CHUNKSIZE, 0)
				if err != nil {
					panic(err)
				}
				// DEBUG
				if debug {
					fmt.Printf("DEBUG: upload %s ... ", chunkFileName)
				}
				// chunk (verschlüsselt) hochladen
				cryptReader := core.CryptReader(fh, chunkFileKey)
				_, err = client.Save(chunkFileName, cryptReader, chunkFileSize)
				if err != nil {
					panic(err)
				}
				uploadCount++
				// DEBUG
				if debug {
					fmt.Printf("OK (%d bytes)\n", chunkFileSize)
				}
				// fh schließen  (gäbe es eine panic, wäre das Programm sowieso beendet)
				fh.Close()
			}
		}
	}

	// report
	println(fmt.Sprintf("upload files: %d chunks", uploadCount)) // Dein Ernst? Ja, mein Ernst?

	// index.db hochladen (ganz am Ende)

	// fileList aktualisieren
	err = client.UpdateFileList()
	if err != nil {
		panic(err)
	}
	// alle alten index.db dateien löschen
	for _, fileObj := range client.FileList() {
		if fileObj.Name == dbFileNameOnStorage {
			err := client.Trash(fileObj.Id)
			if err != nil {
				panic(err)
			}
		}
	}
	// neuen index hochladen
	fh, err := os.Open(dbFile)
	if err != nil {
		panic(err)
	}
	defer fh.Close()
	_, err = client.Save(dbFileNameOnStorage, fh, 0)
	if err != nil {
		panic(err)
	}
}

// cleanFunc löscht alte chunks aus dem Speicher. Dabei muss die DB zuerst mit scanFunc() aktualisiert werden.
// Gelöscht werden nur Dateien die anhand des Dateinamens ein chunk sein können.
func cleanFunc(keyFile, dbFile, module, destination, apiClient, apiToken string) {

	// Warnung
	fmt.Printf("ATTENTION: This process will delete data!\n")
	ask4confirm()
	fmt.Printf("\n\n")

	// keyFile laden
	k := core.LoadKeyfile(keyFile)

	// DB laden
	db, err := core.DbFromFile(dbFile, k.DbKey())
	if err != nil {
		panic(err)
	}

	// client erstellen (drive oder local)
	client := clientModule(module, destination, apiClient, apiToken, "")

	// fileList initialisieren
	err = client.InitFileList()
	if err != nil {
		panic(err)
	}

	// reverse DB baun
	reverseDB := make(map[string]int64)
	for _, dbFileObj := range db {
		for chunkIndex, chunk := range dbFileObj.FileChunks {
			chunkFileName := fmt.Sprintf("%x", k.CalcChunkName(chunk[:]))
			chunkFileSize := core.CalcChunkSize(chunkIndex, dbFileObj.Size)
			reverseDB[chunkFileName] = chunkFileSize
		}
	}

	// alle chunks im Speicher durchgehen um nach alten chunks zu suchen
	clientFileList := client.FileList()
	removeList := make([]string, 0) // speichert alle zu löschenden fileIds
	var removeBytes int64 = 0
	var totalBytes int64 = 0
	for fileId, fileObj := range clientFileList {
		totalBytes += fileObj.Size

		// alle Einträge in der DB duch gehen und nach dem aktuell betrachteten chunk im Speicher suchen
		chunkFileSize, ok := reverseDB[fileObj.Name]
		if len(fileObj.Name) == 128 && (!ok || fileObj.Size != chunkFileSize) {
			// HUCH, der chunk ist NICHT in der Datenbank
			// oder die Größe passt nicht.
			// dann schreibe seine fileId auf die Löschliste
			removeList = append(removeList, fileId)
			// debug
			removeBytes += fileObj.Size
			fmt.Printf("OLD CHUNK: %s (%d bytes)\n", fileObj.Name, fileObj.Size)
		}
	}

	// Bericht und freigabe
	p := message.NewPrinter(language.German)

	fmt.Printf("--------------------------------------\n")
	fmt.Printf("total %d chunks with %s Byte\n", len(clientFileList), p.Sprintf("%d", totalBytes))
	fmt.Printf("remove %d chunks with %s Byte\n", len(removeList), p.Sprintf("%d", removeBytes))
	fmt.Printf("remaining %d chunks with %s Byte\n", len(clientFileList)-len(removeList), p.Sprintf("%d", totalBytes-removeBytes))
	ask4confirm()

	// LÖSCHEN
	for _, fileId := range removeList {
		err := client.Trash(fileId)
		if err != nil {
			panic(err)
		}
	}
}

// ask4confirm ist eine Hilfsfunktion die von Anwender ein y oder n erwartet.
// Bei n wird das Programm mit os.Exit() beendet
func ask4confirm() {
	var s string

	fmt.Printf("(y/N): ")
	_, err := fmt.Scan(&s)
	if err != nil {
		panic(err)
	}

	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	if s == "y" || s == "yes" {
		return
	}

	os.Exit(0)
}

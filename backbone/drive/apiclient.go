package drive

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"splitfuseX/backbone"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

const folderMimeType = "application/vnd.google-apps.folder"

// ApiClient wird mit NewApiClient() erzeugt und speichert intern einen *drive.Service.
// Es bietet die Basis-Methoden an um mit Google Drive zu arbeiten.
type ApiClient struct {
	api                  *drive.Service
	folderId             string
	cachePath            string
	apiQuerySize         int // default 1000
	fileList             map[string]*backbone.FileObject
	changeStartPageToken string
}

// Read gibt einen *http.Response auf die angeforderte Drive Datei zurück.
// Die fileId ist eine eindeutige ID auf eine Datei.
// Mit dem offset wird angegeben, am welchem Byte innerhalb der Datei begonnen werden soll.
// fileSize bestimmt bis wohin gelesen werden soll. Dabei ist das angegebene Byte immer inkludiert!
// und es wird beim Byte 0 der Datei mit dem zählen begonnen.  (Es ist auch egal, wenn hier zu viel angegeben wird!)
// ACHTUNG: Am Ende .Close() nicht vergessen!
func (client *ApiClient) Read(fileId string, offset int64, fileSize int64) (io.ReadCloser, error) {
	fileGetCall := client.api.Files.Get(fileId)
	fileGetCall.Header().Set("Range", fmt.Sprintf("bytes=%d-%d", offset, fileSize))

	resp, err := fileGetCall.Download()
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Trash verschiebt eine Datei in den Papierkorb.
func (client *ApiClient) Trash(fileId string) error {
	_, err := client.api.Files.Update(fileId, &drive.File{Trashed: true}).Do()
	return err
}

// Save lädt Daten in die Cloud hoch.
// Im Erfolgsfall wird die fileID der erstellten Datei zurück gegeben,
func (client *ApiClient) Save(fileName string, file io.Reader, maxRead int64) (string, error) {

	// root fix:  "You can use the alias root to refer to the root folder anywhere a file ID is provided"
	parentId := client.folderId
	if parentId == "" {
		parentId = "root"
	}

	// driveFile metadata
	driveFile := &drive.File{
		Name:     fileName,
		Parents:  []string{parentId},
		MimeType: "application/octet-stream",
	}

	// file upload
	if maxRead > 0 {
		file = io.LimitReader(file, maxRead)
	}
	driveFile, err := client.api.Files.Create(driveFile).Media(file).Do()

	// request error
	if err != nil {
		errMsg := fmt.Sprintf("%v", err)
		if strings.Contains(errMsg, "insufficientPermissions") {
			// wrong permissions
			return "", fmt.Errorf("upload error: wrong permissions: create a new oauth token with --upload flag: %v", err)
		} else {
			// other error
			return "", fmt.Errorf("upload error: %v", err)
		}
	}

	// ok
	return driveFile.Id, nil
}

// InitFileList aktualisiert den internen Speicher mit allen DATEIEN im angegebenen Ordner.
// Ordner (folderMimeType) sowie Unterordner und deren Inhalt werden komplett ignoriert!
func (client *ApiClient) InitFileList() error {

	// root fix:  "You can use the alias root to refer to the root folder anywhere a file ID is provided"
	if client.folderId == "root" || client.folderId == "" {
		// root is not the correct fileId! its only a symlink!
		rootId, err := client.api.Files.Get("root").Do()
		if err != nil {
			return err
		}
		client.folderId = rootId.Id
	}

	// CACHE ????
	if client.cachePath != "" {
		err := client.loadStatus()
		if err == nil {
			// juhu! -> fin
			return nil
		} else {
			// :*(
			fmt.Printf("%v\n", err)
		}
	}

	// config
	query := fmt.Sprintf("trashed = false and mimeType != '%s' and '%s' in parents", folderMimeType, client.folderId)
	fields := "nextPageToken, files(id, name, size, modifiedTime)"
	spaces := "drive"               // Supported values are 'drive', 'appDataFolder' and 'photos'.
	corpora := "user"               // The user corpus includes all files in "My Drive" and "Shared with me"
	pageSize := client.apiQuerySize // split big file lists in pages (default 1000)

	if pageSize < 1 {
		pageSize = 1000
	}

	// get a new StartPageToken to watch changes
	startPageTokenObj, err := client.api.Changes.GetStartPageToken().Do()
	if err != nil {
		return err
	}
	client.changeStartPageToken = startPageTokenObj.StartPageToken

	// get all relevant files
	newList := make(map[string]*backbone.FileObject)
	pageToken := ""
	for {
		// read a result page
		fileList, err := client.api.Files.List().Q(query).PageToken(pageToken).
			Spaces(spaces).Corpora(corpora).PageSize(int64(pageSize)).
			Fields(googleapi.Field(fields)).Do()

		// error handling
		if err != nil {
			return err
		}

		// add all results (files) to list
		for _, f := range fileList.Files {
			newList[f.Id] = &backbone.FileObject{
				Id:           f.Id,
				Name:         f.Name,
				Size:         f.Size,
				ModifiedTime: parseTime(f.ModifiedTime), // RFC 3339 date-time: 2018-08-03T12:03:30.407Z
			}
		}

		// break loop (no more pages)
		pageToken = fileList.NextPageToken
		if pageToken == "" {
			break
		}
	}

	// FIN: set new list
	client.fileList = newList
	return nil
}

// UpdateFileList aktualisiert die fileList mit einem DIFF seit dem letzten Ausführen dieser Methode.
// Das kann sehr viel effizienter sein als alle Dateien neu einzulesen.
// Zuerst muss jedoch InitFileList() mindestens einmal aufgeführt worden sein!
func (client *ApiClient) UpdateFileList() error {

	// first init the file list
	if client.changeStartPageToken == "" {
		return fmt.Errorf("can't get changes without StartPageToken: call InitFileList() first")
	}

	// config
	fields := "nextPageToken, newStartPageToken, changes(file(id, name, size, trashed, mimeType, parents, modifiedTime))"
	spaces := "drive"               // Supported values are 'drive', 'appDataFolder' and 'photos'.
	pageSize := client.apiQuerySize // split big file lists in pages (default 1000)

	if pageSize < 1 {
		pageSize = 1000
	}

	// the first page is the changeStartPageToken
	pageToken := client.changeStartPageToken

	// loop to get all changes
	for {
		// read a result pages
		changeList, err := client.api.Changes.List(pageToken).Spaces(spaces).PageSize(int64(pageSize)).Fields(googleapi.Field(fields)).Do()
		if err != nil {
			return err
		}

		// update fileList
		for _, change := range changeList.Changes {
			// is a file
			if change.File.MimeType != folderMimeType {
				// is in the watched folder
				for _, parent := range change.File.Parents {
					if parent == client.folderId {
						// add/update or remove?
						if change.File.Trashed {
							// remove file from list
							delete(client.fileList, change.File.Id)
						} else {
							// update or add file
							client.fileList[change.File.Id] = &backbone.FileObject{
								Id:           change.File.Id,
								Name:         change.File.Name,
								Size:         change.File.Size,
								ModifiedTime: parseTime(change.File.ModifiedTime), // RFC 3339 date-time: 2018-08-03T12:03:30.407Z
							}
						}
					}
				}
			}
		}

		// break loop (no more pages)
		pageToken = changeList.NextPageToken // NextPageToken for the next page
		if pageToken == "" {
			// no more pages
			// set the new NewStartPageToken for the next UpdateFileList() call
			client.changeStartPageToken = changeList.NewStartPageToken
			break
		}
	}

	// write cache file
	err := client.saveStatus()
	if err != nil {
		// :*(
		fmt.Printf("%v\n", err)
	}

	return nil
}

// FileList gibt alle Dateien in einem bestimmten Ordner zurück.
// Es werden keine Unterordner berücksichtigt!
// Diese Methode ist offline!
// Für aktuelle Date muss InitFileList() bzw. UpdateFileList() aufgerufen werden.
// Min. enthalten sind: id, name, size, modifiedTime
func (client *ApiClient) FileList() map[string]*backbone.FileObject {
	ret := make(map[string]*backbone.FileObject)

	for k, v := range client.fileList {
		ret[k] = &backbone.FileObject{
			Id:           v.Id,
			Name:         v.Name,
			Size:         v.Size,
			ModifiedTime: v.ModifiedTime,
		}
	}

	return ret
}

//--------------------------------------------------------------------------------------------------------------------//

type cacheClient struct {
	FileList             map[string]*backbone.FileObject
	ChangeStartPageToken string
	CacheSig             string
}

// saveStatus speichert die FileList in einer Datei um später mit loadStatus() die InitFileList() Funktion zu beschleunigen.
func (client *ApiClient) saveStatus() error {

	// leerer String deaktivert diese Funktion
	if client.cachePath == "" {
		return nil // ok
	}

	// Datei anlegen
	fh, err := os.Create(client.cachePath)
	if err != nil {
		return fmt.Errorf("can't create cache file: %v", err)
	}
	defer fh.Close()

	// calc cache signature
	sig, err := client.calcCacheSig()
	if err != nil {
		return fmt.Errorf("can't calc cache signature: %v", err)
	}

	// Objekt zum Serialisieren vorbereiten
	obj := &cacheClient{FileList: client.fileList, ChangeStartPageToken: client.changeStartPageToken, CacheSig: sig}

	// Daten schreiben
	if err := gob.NewEncoder(fh).Encode(obj); err != nil {
		return fmt.Errorf("can't write/encode cache file: %v", err)
	}

	return nil
}

// loadStatus lädt eine FileList aus einer Datei und stellt damit den Zustand zu einem Zeitpunkt wiederher.
// Der Status muss jedoch mit einem erfolgreichen UpdateFileList() validiert werden!
func (client *ApiClient) loadStatus() error {

	// leerer String deaktivert diese Funktion
	if client.cachePath == "" {
		return nil // ok
	}

	// cache exist check
	if _, err := os.Stat(client.cachePath); err != nil {
		return fmt.Errorf("no cache file found! first call?: %v", err)
	}

	// Datei öffnen
	fh, err := os.Open(client.cachePath)
	if err != nil {
		return fmt.Errorf("can't open cache file: %v", err)
	}
	defer fh.Close()

	// Daten lesen (cacheClient deserialisieren)
	var cacheClient cacheClient
	if err := gob.NewDecoder(fh).Decode(&cacheClient); err != nil {
		return fmt.Errorf("can't read/decode cache file: %v", err)
	}

	// check cache signature
	sig, err := client.calcCacheSig()
	if err != nil {
		return fmt.Errorf("can't calc cache signature: %v", err)
	}
	if cacheClient.CacheSig != sig {
		return fmt.Errorf("wrong cache signature: load='%s', calc='%s'", cacheClient.CacheSig, sig)
	}

	// einen neuen Client baun für den UpdateFileList() test
	newClient := &ApiClient{
		folderId:     client.folderId,
		apiQuerySize: client.apiQuerySize,
		cachePath:    client.cachePath,
		api:          client.api,

		changeStartPageToken: cacheClient.ChangeStartPageToken,
		fileList:             cacheClient.FileList,
	}

	err = newClient.UpdateFileList()
	if err != nil {
		return fmt.Errorf("cacheClient UpdateFileList call fail: %v", err)
	}

	// die aktuelle FileList in den aktiven client übertragen
	client.fileList = newClient.fileList
	client.changeStartPageToken = newClient.changeStartPageToken

	return nil
}

// calcCacheSig berechnet einen String um Caches fix einem oauth file und der rootFolderId zuzuordnen.
func (client *ApiClient) calcCacheSig() (string, error) {
	badSig := fmt.Sprintf("%d", time.Now().Unix())

	// get user id
	about, err := client.api.About.Get().Fields("user(permissionId)").Do()
	if err != nil {
		return badSig, err
	}

	folderId := client.folderId
	accountId := about.User.PermissionId

	// check accountId
	if len(accountId) < 3 {
		return badSig, fmt.Errorf("invalid PermissionId: %s", accountId)
	}

	return fmt.Sprintf("%s|%s", folderId, accountId), nil
}

// parseTime wandelt einen RFC 3339 date-time string in ein Time Objekt.
// Beispiel für einen input: 2018-08-03T12:03:30.407Z
func parseTime(t string) int64 {

	// parse
	ret := &time.Time{}
	err := ret.UnmarshalText([]byte(t))

	// error ??  Das ist blöd! Dürfte nicht passieren!
	if err != nil {
		fmt.Printf("WARNING: can't parse time '%s': %v", t, err)
		return time.Now().Unix() - 4730000000 // -150 Jahre
	}

	// OK
	return ret.Unix()
}

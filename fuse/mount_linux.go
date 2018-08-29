package fuse

import (
	"fmt"

	"splitfuseX/backbone"
	"splitfuseX/core"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

// MountNormal greift auf Chunks zu und mountet die Klartextdateien
func MountNormal(apiClient backbone.Client, dbFileName, keyFilePath, mountpoint string, debugFlag bool, test bool) *fuse.Server {

	// OPTIONEN
	opts := &fuse.MountOptions{
		FsName:         "splitfuseX", // erste Spalte bei 'df -hT'
		Name:           "splitfuseX", // zweite Spalte bei 'df -hT'
		MaxReadAhead:   131072,
		Debug:          debugFlag,
		AllowOther:     true,
		SingleThreaded: true,
	}

	// SplitFS erzeugen  (mit meinen Methoden)
	fs := &SplitFs{
		FileSystem: pathfs.NewDefaultFileSystem(),

		debug:      debugFlag,
		dbFileName: dbFileName,
		keyFile:    core.LoadKeyfile(keyFilePath),
		apiClient:  apiClient,
	}

	// Alle Dateien von google Drive laden
	debug(debugFlag, LOGINFO, "InitFileList()", nil)
	fs.apiClient.InitFileList()

	// checkDbUpdate lädt die DB von google drive herunter
	debug(debugFlag, LOGINFO, "load DB", nil)
	statusCode := fs.checkDbUpdate()
	if statusCode != 0 {
		panic(fmt.Errorf("checkDbUpdate() error %d", statusCode))
	}

	// Als Zwischenschicht, (dann ist alles ein wenig einfacher), kommt NewPathNodeFs zum Einsatz
	nfs := pathfs.NewPathNodeFs(fs, nil)

	// NewFileSystemConnector erzeugen
	fsconn := nodefs.NewFileSystemConnector(nfs.Root(), nil)

	// FUSE mit den Optionen mounten
	debug(debugFlag, LOGINFO, "start FUSE server (mount)", nil)
	server, err := fuse.NewServer(fsconn.RawFS(), mountpoint, opts)
	if err != nil {
		panic(err)
	}

	// loop (wartet auf EXIT)
	if !test {
		server.Serve()
	}

	return server
}

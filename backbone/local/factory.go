package local

import (
	"splitfuseX/backbone"
)

// NewDiskClient speichert die Daten in dem übergebenen Ordner
func NewDiskClient(path string) backbone.Client {
	var ret *DiskClient
	ret = &DiskClient{localFolder: path}
	return ret
}

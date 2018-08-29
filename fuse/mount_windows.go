package fuse

import "splitfuseX/backbone"

// dummy mount für windows
func MountNormal(apiClient backbone.Client, dbFileName, keyFilePath, mountpoint string, debug bool, test bool) {
	panic("fuse only work with linux")
}

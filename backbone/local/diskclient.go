package local

import (
	"encoding/base64"
	"io"
	"io/ioutil"
	"os"
	"path"

	"splitfuseX/backbone"
)

type DiskClient struct {
	localFolder string
	fileList    map[string]*backbone.FileObject
}

func (client *DiskClient) Read(fileId string, offset int64, fileSize int64) (io.ReadCloser, error) {
	// get path
	name, err := base64.StdEncoding.DecodeString(fileId)
	if err != nil {
		return nil, err
	}
	p := path.Join(client.localFolder, string(name))

	// open file
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}

	// offset
	_, err = f.Seek(offset, 0)
	if err != nil {
		return nil, err
	}

	// return
	return f, nil
}

func (client *DiskClient) Trash(fileId string) error {
	name, err := base64.StdEncoding.DecodeString(fileId)
	if err != nil {
		return err
	}
	p := path.Join(client.localFolder, string(name))

	return os.Remove(p)
}

func (client *DiskClient) Save(fileName string, file io.Reader, maxRead int64) (string, error) {
	// get path
	p := path.Join(client.localFolder, string(fileName))

	// create file
	writer, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer writer.Close()

	// write bytes
	if maxRead > 0 {
		file = io.LimitReader(file, maxRead)
	}
	_, err = io.Copy(writer, file)
	if err != nil {
		return "", err
	}

	// return
	id := base64.StdEncoding.EncodeToString([]byte(fileName))
	return id, nil
}

func (client *DiskClient) InitFileList() error {
	files, err := ioutil.ReadDir(client.localFolder)
	if err != nil {
		return err
	}

	list := make(map[string]*backbone.FileObject)
	for _, f := range files {
		if !f.IsDir() {
			id := base64.StdEncoding.EncodeToString([]byte(f.Name()))
			list[id] = &backbone.FileObject{
				Id:           id,
				Name:         f.Name(),
				Size:         f.Size(),
				ModifiedTime: f.ModTime(),
				Md5Checksum:  "", // not implemented
			}
		}
	}
	client.fileList = list

	return nil
}

func (client *DiskClient) UpdateFileList() error {
	// InitFileList
	return client.InitFileList()
}

func (client *DiskClient) FileList() map[string]*backbone.FileObject {
	return client.fileList
}

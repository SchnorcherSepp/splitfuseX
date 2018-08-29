package drive

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

// TESTS:
// - NewApiClient()
// - InitFileList()
// - Save()
// - Read()
// - UpdateFileList()
// - Trash()
// - FileList()
func TestApiClient(t *testing.T) {
	testFolder := "root"
	testName := "Unit_Test_upload"

	// --- TEST NewApiClient()
	// Wenn ein ApiClient bezogen werden kann, dann hat bereits alles funktioniert.
	tmp := NewApiClient("client_secret.json", "token.json", testFolder)
	if tmp == nil {
		t.Error("can't create an ApiClient")
	}

	// cast back
	var client *ApiClient
	client = tmp.(*ApiClient)

	// test query size
	client.apiQuerySize = 2

	// --- TEST InitFileList()
	err := client.InitFileList()
	if err != nil {
		t.Error(err)
	}
	list1 := len(client.FileList())

	// --- TEST save()
	upload := make([]map[string]string, 5)
	for i := range upload {
		randInt, _ := rand.Prime(rand.Reader, 32)
		text := fmt.Sprintf("Unit_Test_upload: %d", randInt)
		fileId, err := client.Save(testName, strings.NewReader(text), 0)
		if err != nil {
			t.Error(err)
		}
		// calc md5 hash from upload file (text)
		hash := md5.New()
		hash.Write([]byte(text))
		md5sum := hex.EncodeToString(hash.Sum(nil))
		// map
		upload[i] = make(map[string]string)
		upload[i]["fileId"] = fileId
		upload[i]["md5sum"] = md5sum
		upload[i]["text"] = text
	}

	// --- TEST read()
	for i := range upload {
		resp, err := client.Read(upload[i]["fileId"], 0, 20000000)
		if err != nil {
			t.Error(err)
		}

		b, err := ioutil.ReadAll(resp)
		if err != nil {
			t.Error(err)
		}

		resp.Close()

		if upload[i]["text"] != string(b) {
			t.Errorf("download not correct: write='%s', read='%s'", upload[i]["text"], b)
		}
	}

	// --- TEST UpdateFileList()
	err = client.UpdateFileList()
	if err != nil {
		t.Error(err)
	}
	list2 := len(client.FileList())

	// test FileList params
	count := 0
	for _, f := range client.FileList() {
		for i := range upload {
			if f.Id == upload[i]["fileId"] {
				count++
				err = nil
				if f.Name != testName {
					t.Errorf("wrong name: '%s' != '%s'", f.Name, testName)
				}
				if f.Size != int64(len(upload[i]["text"])) {
					t.Errorf("wrong size: %d != %d", f.Size, len(upload[i]["text"]))
				}
				if f.Md5Checksum != upload[i]["md5sum"] {
					t.Errorf("wrong md5Checksum: %s != %s", f.Md5Checksum, upload[i]["md5sum"])
				}
				timeDiff := time.Now().UnixNano() - f.ModifiedTime.UnixNano()
				if timeDiff < 10000000 || timeDiff > 11100000000 {
					t.Errorf("wrong modifiedTime: %v | %v", f.ModifiedTime, timeDiff)
				}
			}
		}
	}
	if count != 5 {
		t.Errorf("uncomplied filelist")
	}

	// --- TEST Trash()
	for i := range upload {
		err = client.Trash(upload[i]["fileId"])
		if err != nil {
			t.Error(err)
		}
	}

	// --- TEST UpdateFileList()
	err = client.UpdateFileList()
	if err != nil {
		t.Error(err)
	}
	list3 := len(client.FileList())

	if !(list1 == list2-5 && list1 == list3) {
		t.Errorf("file lists not correct: l1=%d, l2=%d, l3=%d", list1, list2, list3)
	}
}

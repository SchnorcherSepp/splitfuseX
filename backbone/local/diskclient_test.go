package local

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"path"
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
func TestNewDiskClient(t *testing.T) {
	// create test folder
	testFolder := path.Join(os.TempDir(), "unit_tests")
	os.Mkdir(testFolder, 0700)

	// --- TEST NewDiskClient()
	client := NewDiskClient(testFolder)

	// --- TEST InitFileList()
	err := client.InitFileList()
	if err != nil {
		t.Error(err)
	}
	list1 := len(client.FileList())

	// --- TEST save()
	bigData := make([]byte, 5000000)
	upload := make([]map[string]string, 5)
	for i := range upload {
		randInt, _ := rand.Prime(rand.Reader, 32)
		text := fmt.Sprintf("Unit_Test_upload: %d\n%x", randInt, bigData)
		testName := fmt.Sprintf("%d.txt", randInt)
		fileId, err := client.Save(testName, strings.NewReader(text), 0)
		if err != nil {
			t.Error(err)
		}
		// map
		upload[i] = make(map[string]string)
		upload[i]["testName"] = testName
		upload[i]["fileId"] = fileId
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
	time.Sleep(1 * time.Second)
	count := 0
	for _, f := range client.FileList() {
		for i := range upload {
			if f.Id == upload[i]["fileId"] {
				count++
				err = nil
				if f.Name != upload[i]["testName"] {
					t.Errorf("wrong name: '%s' != '%s'", f.Name, upload[i]["testName"])
				}
				if f.Size != int64(len(upload[i]["text"])) {
					t.Errorf("wrong size: %d != %d", f.Size, len(upload[i]["text"]))
				}
				timeDiff := time.Now().UnixNano() - f.ModifiedTime.UnixNano()
				if timeDiff < 900000000 || timeDiff > 10000000000 {
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

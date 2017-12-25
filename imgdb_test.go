package imgdb

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// =============================================
//                    Globals
// =============================================

var outDir = "testout"

var db *ImgDB

// =============================================
//                    Tests
// =============================================

func TestMain(m *testing.M) {
	// clear test area
	clearOutDir()
	db = New("sqlite3", "test.db")
	defer db.Close()
	retCode := m.Run()
	os.Exit(retCode)
}

//// Database Updates and Query Tests

func TestPrivateQueries(t *testing.T) {
	directory := getDirectory(db, outDir)
	if directory != nil {
		t.Errorf("failed to report missing directory %s", outDir)
	}

	// add directory
	mockDir := "mockDir"
	mockImg := ImageFile{
		Name:   "mockImg1",
		Format: "mock",
		Index:  "31",
	}
	db.Create(&Directory{Dirpath: mockDir, ImageFiles: []ImageFile{mockImg}})
	directory = getDirectory(db, mockDir)
	if directory == nil {
		t.Errorf("failed to find directory %s", outDir)
	} else {
		// check path
		if directory.Dirpath != mockDir {
			t.Errorf("expecting dirpath %s, got %s", mockDir, directory.Dirpath)
		}

		// check associations
		imgs := getAssocs(db, directory)
		db.Model(directory).Association("ImageFiles").Find(&imgs)
		if imgs[0].Name != mockImg.Name {
			t.Errorf("expected filename %s, got %s", imgs[0].Name, mockImg.Name)
		} else if imgs[0].Index != mockImg.Index {
			t.Errorf("expected index %s, got %s", imgs[0].Index, mockImg.Index)
		}
	}

	afterEach()
}

//// Public API Tests

func TestAddImg(t *testing.T) {
	file, err := os.Open("testsamples/testimg.jpg")
	if err != nil {
		panic(err)
	}

	rawdata, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	b := bytes.NewBuffer(rawdata)

	err = db.AddImg(outDir, "mockfile", *b)
	if err != nil {
		t.Fatal(err)
	}
	imgs := []ImageFile{}
	db.Find(&imgs, "name = ?", "mockfile")

	if len(imgs) != 1 {
		t.Errorf("expecting 1 inserted image, got %d", len(imgs))
	}
	_, err = os.Stat(filepath.Join(outDir, "mockfile.jpeg"))
	if err != nil && os.IsNotExist(err) {
		t.Errorf("image file (mockfile.jpeg) not found")
	}

	afterEach()
}

// =============================================
//                    Private
// =============================================

func clearOutDir() {
	dir, err := os.Open(outDir)
	if err != nil {
		return
	}
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return
	}
	for _, name := range names {
		os.RemoveAll(filepath.Join(outDir, name))
	}
}

func afterEach() {
	db.DropTableIfExists(&Directory{}, &ImageFile{})
	db.AutoMigrate(&Directory{}, &ImageFile{})
}

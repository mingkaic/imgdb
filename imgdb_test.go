package imgdb

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// =============================================
//                    Globals
// =============================================

const (
	outDir = "testout"
	dbFile = "test.db"
)

// =============================================
//                    Tests
// =============================================

func TestMain(m *testing.M) {
	retCode := m.Run()
	os.Exit(retCode)
}

//// Database Updates and Query Tests

func TestPrivateGetCluster(t *testing.T) {
	testWrap(func(db *ImgDB) {
		mockFeats := make([]float32, 512)
		// randomly generate cluster
		genRandFeat(mockFeats)
		cluster := getCluster(db, mockFeats[:])
		if cluster == nil {
			t.Errorf("failed to generate cluster for mock features %v", mockFeats)
		}
	})
}

func TestPrivateGetAssoc(t *testing.T) {
	testWrap(func(db *ImgDB) {
		mockFeats := make([]float32, 512)
		// randomly generate cluster
		genRandFeat(mockFeats)
		cluster := getCluster(db, mockFeats[:])
		mockImg := ImageFile{
			Name:   "mockImg1",
			Format: "mock",
			Index:  stringify(mockFeats[:]),
		}

		if cluster != nil {
			db.Model(cluster).
				Association("ImageFiles").
				Append(mockImg)
		}

		imgs := getAssocs(db, cluster)
		if len(imgs) != 1 {
			t.Errorf("expecting 1 image associated, got %d", len(imgs))
		} else {
			if imgs[0].Name != mockImg.Name {
				t.Errorf("expected filename %s, got %s", imgs[0].Name, mockImg.Name)
			} else if imgs[0].Index != mockImg.Index {
				t.Errorf("expected index %s, got %s", imgs[0].Index, mockImg.Index)
			}
		}
	})
}

//// Public API Tests

func TestAddImg(t *testing.T) {
	testWrap(func(db *ImgDB) {
		file, err := os.Open(filepath.Join("testimgs", "testimg.jpg"))
		if err != nil {
			panic(err)
		}

		rawdata, err := ioutil.ReadAll(file)
		if err != nil {
			panic(err)
		}
		b := bytes.NewBuffer(rawdata)

		fileLoc, err := db.AddImg("mockfile", *b)
		if err != nil {
			t.Fatal(err)
		}
		imgs := []ImageFile{}
		db.Find(&imgs, "name = ?", "mockfile")

		if len(imgs) != 1 {
			t.Errorf("expecting 1 inserted image, got %d", len(imgs))
		}
		_, err = os.Stat(fileLoc)
		if err != nil && os.IsNotExist(err) {
			t.Errorf("image file at specified path (%s) not found", fileLoc)
		}

		// todo: check for value correctness
	})
}

// =============================================
//                    Private
// =============================================

func testWrap(test func(*ImgDB)) {
	db := New("sqlite3", dbFile, outDir)
	defer db.Close()

	test(db)

	// after each
	os.Remove(dbFile)
	cleanDir(outDir)
}

func cleanDir(dirpath string) {
	dir, err := os.Open(dirpath)
	if err != nil {
		return
	}
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return
	}
	for _, name := range names {
		os.RemoveAll(filepath.Join(dirpath, name))
	}
	os.Remove(dirpath)
}

func genRandFeat(feats []float32) {
	// randomly generate cluster
	bits := make([]byte, len(feats)*4)
	io.ReadFull(rando, bits[:])
	buf := bytes.NewBuffer(bits[:])
	err := binary.Read(buf, binary.LittleEndian, feats)
	if err != nil {
		panic(err)
	}
}

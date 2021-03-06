package imgdb

import (
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
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
		clusterName := bitApproximation(mockFeats[:])
		cluster := getCluster(db, clusterName)
		if cluster != nil {
			t.Errorf("got unexpected cluster %s with features %v", clusterName, mockFeats)
		}
	})
}

func TestPrivateCreateCluster(t *testing.T) {
	testWrap(func(db *ImgDB) {
		mockFeats := make([]float32, 512)
		// randomly generate cluster
		genRandFeat(mockFeats)
		clusterName := bitApproximation(mockFeats[:])
		cluster := createCluster(db, clusterName)
		if cluster == nil {
			t.Errorf("failed to create cluster %s", clusterName)
		}

		foundClust := getCluster(db, clusterName)
		if cluster.ID != foundClust.ID {
			t.Errorf("created cluster ID=%d does not match found cluster ID=%d", cluster.ID, foundClust.ID)
		}
	})
}

func TestPrivateGetAssoc(t *testing.T) {
	testWrap(func(db *ImgDB) {
		mockFeats := make([]float32, 512)
		// randomly generate cluster
		genRandFeat(mockFeats)
		clusterName := bitApproximation(mockFeats[:])
		cluster := createCluster(db, clusterName)
		mockImg := ImageFile{
			Name:   "mockImg1",
			Format: "mock",
			Index:  stringify(mockFeats[:]),
		}

		if cluster == nil {
			t.Fatalf("failed to createCluster")
		}

		db.Model(cluster).
			Association("ImageFiles").
			Append(mockImg)
		imgs := getAssocs(db, cluster)
		if len(imgs) != 1 {
			t.Errorf("expecting 1 image associated, got %d", len(imgs))
		} else {
			if imgs[0].Name != mockImg.Name {
				t.Errorf("expected filename %s, got %s", imgs[0].Name, mockImg.Name)
			} else if !reflect.DeepEqual(imgs[0].Index, mockImg.Index) {
				t.Errorf("expected index %s, got %s", imgs[0].Index, mockImg.Index)
			}
		}
	})
}

//// Public API Tests

func TestAddImg(t *testing.T) {
	testWrap(func(db *ImgDB) {
		inputFileLoc := filepath.Join("testimgs", "testimg.jpg")
		file, err := os.Open(inputFileLoc)
		panicCheck(err)

		rawdata, err := ioutil.ReadAll(file)
		panicCheck(err)

		expectFileLoc := filepath.Join(outDir, "mockfile.jpeg")
		expectName := "mockfile"
		imgModel, err := db.AddImg(expectName, rawdata)
		if err != nil {
			t.Fatal(err)
		}
		imgs := []ImageFile{}
		db.Find(&imgs, "name = ?", "mockfile")

		if len(imgs) != 1 {
			t.Errorf("expecting 1 inserted image, got %d", len(imgs))
		}
		if imgModel.Name != expectName {
			t.Errorf("expected image name %s, got %s", expectName, imgModel.Name)
		}
		if imgModel.Format != "jpeg" {
			t.Errorf("expected image format jpeg, got %s", imgModel.Format)
		}
		_, err = os.Stat(expectFileLoc)
		if err != nil && os.IsNotExist(err) {
			t.Errorf("image file at specified path (%s) not found", expectFileLoc)
		} else {
			// check for value correctness
			file, err := os.Open(expectFileLoc)
			panicCheck(err)
			gotout, err := ioutil.ReadAll(file)
			panicCheck(err)
			if !reflect.DeepEqual(rawdata, gotout) {
				t.Errorf("file data different between %s, %s", inputFileLoc, expectFileLoc)
			}
		}
	})
}

// =============================================
//                    Private
// =============================================

func testWrap(test func(*ImgDB)) {
	db, err := New("sqlite3", dbFile, outDir)
	if err != nil {
		panic(err)
	}
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
	sum := 0
	// randomly generate cluster
	for i := range feats {
		feat := rand.Intn(1000)
		sum += feat
		feats[i] = float32(feat)
	}
	for i, feat := range feats {
		feats[i] = feat / float32(sum)
	}
}

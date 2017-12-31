//// file: imgdb.go

// Package imgdb ...
// Is a wrapper database management api
// for storing and grouping image
// and avoiding duplicates
package imgdb

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/mingkaic/imgdb/imgutil"
)

// =============================================
//                    Declarations
// =============================================

// ImgDB ...
// Is a wrapper gorm for images
type ImgDB struct {
	*gorm.DB
	MinW     uint
	MinH     uint
	basePath string
	mutex    sync.Mutex
}

//// Models

// Cluster ...
// Specifies a grouping of images
type Cluster struct {
	gorm.Model
	Name       string `gorm:"not null;unique"`
	ImageFiles []ImageFile
}

// ImageFile ...
// Specifies name information and image features
type ImageFile struct {
	gorm.Model
	Name      string `gorm:"not null;unique"`
	Format    string `gorm:"not null"`
	Index     string `gorm:"not null"`
	ClusterID int
}

// =============================================
//                    Globals
// =============================================

const (
	chiThresh = 1e-5
	minLimit  = 500
)

var rando = rand.Reader

// =============================================
//                    Public
// =============================================

// New ...
// Initializes and migrates relevant schemas
func New(dialect, source, filedir string) *ImgDB {
	db, err := gorm.Open(dialect, source)
	panicCheck(err)
	db.AutoMigrate(&Cluster{}, &ImageFile{})
	out := &ImgDB{
		DB:       db,
		MinW:     minLimit,
		MinH:     minLimit,
		basePath: filedir,
	}
	panicCheck(os.MkdirAll(filedir, 0755))
	return out
}

// AddImg ...
// Extracts image features as search index
// Save to file system then add in database
// Filters out images too small beyond a limit
// Index and logic inspired from https://tinyurl.com/yaup47bg
func (this *ImgDB) AddImg(name string, data []byte) (imgModel *ImageFile, err error) {
	img, format, err := image.Decode(bytes.NewBuffer(data))

	// size filter
	if err == nil {
		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()
		if uint(width) < this.MinW || uint(height) < this.MinH {
			err = fmt.Errorf("image too small: got <%d, %d> size", width, height)
			return
		}
	} else {
		return
	}

	// feature extraction
	features := imgutil.GenerateFeature(img, format)
	if features == nil {
		err = fmt.Errorf("failed to extract features for %s", name)
		return
	}

	filename := name + "." + format
	imgModel = &ImageFile{Name: name, Format: format, Index: stringify(features)}

	// ==== begin critical section ====
	err = func() error {
		this.mutex.Lock()
		defer this.mutex.Unlock()
		// asserts that gorm api calls are thread-safe
		cluster := getCluster(this, features)
		if cluster == nil {
			return fmt.Errorf("get cluster failed for features %v", features)
		}
		// similarity check
		// 1. check for duplicate features to avoid pollution
		imgFiles := getAssocs(this, cluster)
		for _, file := range imgFiles {
			// test similarity between new file and file
			sim := imgutil.ChiDist(features, featureParse(file.Index))
			if sim < chiThresh { // too similar beyond a threshold is marked as same
				return fmt.Errorf("%s similar to existing file %s", name, file.Name)
			}
		}
		// 2. check for same files and insert uuid to avoid dups
		if fileExists(this, imgModel.Name) {
			var r [8]byte // ~ 10 ^ -19 prob of dup assuming perfect randomness
			io.ReadFull(rando, r[:])
			var appendage [16]byte
			hex.Encode(appendage[:], r[:])
			imgModel.Name += string(appendage[:])
			filename = imgModel.Name + "." + format
		}
		// associate image model
		this.Model(cluster).Association("ImageFiles").Append(*imgModel)
		return nil
	}()
	if err != nil {
		return
	}
	// ==== end critical section ====

	// write to file (invariant: filename is unique)
	file, err := os.Create(filepath.Join(this.basePath, filename))
	if err != nil {
		return
	}
	defer file.Close()

	if _, err = file.Write(data); err != nil {
		return
	}

	return
}

// =============================================
//                    Private
// =============================================

func panicCheck(err error) {
	if err != nil {
		panic(err)
	}
}

//// Data Serialization Utility

// exact record of input float array
func stringify(arr []float32) string {
	buf := new(bytes.Buffer)
	panicCheck(binary.Write(buf, binary.LittleEndian, arr))
	return string(buf.Bytes())
}

func featureParse(str string) []float32 {
	out := make([]float32, len([]byte(str))/4)
	buf := bytes.NewBuffer([]byte(str))
	panicCheck(binary.Read(buf, binary.LittleEndian, out))
	return out
}

// approximate input array by rounding each float value to 1 or 0
// representing the entire array as a bit string, then encode as hex
// assumes that this array has values between 1 and 0
// otherwise we will most likely get arrays of F
func bitApproximation(arr []float32) string {
	n := len(arr)
	outN := n/8 + n%8
	b := make([]byte, outN)
	var accum byte
	for i, f := range arr {
		bi := uint(i % 8)
		if uint(f) > 0 {
			accum |= 1 << bi
		}
		if bi == 7 {
			b[i/8] = accum
			accum = 0
		}
	}
	return string(b)
}

//// Database Updates and Query

// create cluster if not found
func getCluster(db *ImgDB, features []float32) *Cluster {
	cluster := bitApproximation(features)
	if db == nil {
		panic("input db is nil")
	}
	dirs := []Cluster{}
	db.Find(&dirs, "name = ?", cluster)
	var result *Cluster
	if len(dirs) > 0 {
		result = &dirs[0]
	} else {
		// create
		result = &Cluster{Name: cluster}
		db.Create(result)
	}
	return result
}

func fileExists(db *ImgDB, name string) bool {
	files := []ImageFile{}
	db.Find(&files, "name = ?", name)
	return len(files) > 0
}

func getAssocs(db *ImgDB, cluster *Cluster) []ImageFile {
	if db == nil {
		panic("input db is nil")
	}
	if cluster == nil {
		panic("input cluster is nil")
	}
	imgs := []ImageFile{}
	db.Model(cluster).Association("ImageFiles").Find(&imgs)
	return imgs
}

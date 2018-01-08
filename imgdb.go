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
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
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
	mutex    sync.RWMutex
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
	Index     []byte `gorm:"not null"`
	Sources   []Source
	ClusterID int
}

// Source ...
// Specifies image link
type Source struct {
	gorm.Model
	Link        string `gorm:"not null;unique"`
	ImageFileID int
}

type DupFileError struct {
	existing string
	dupfile  string
}

// =============================================
//                    Globals
// =============================================

const (
	chiThresh = 5e-3
	minLimit  = 500
)

var rando = rand.Reader

// =============================================
//                    Public
// =============================================

// New ...
// Initializes and migrates relevant schemas
func New(dialect, source, filedir string) (out *ImgDB, err error) {
	db, err := gorm.Open(dialect, source)
	if err != nil {
		return
	}
	db.AutoMigrate(&Cluster{}, &ImageFile{}, &Source{})
	out = &ImgDB{
		DB:       db,
		MinW:     minLimit,
		MinH:     minLimit,
		basePath: filedir,
	}
	err = os.MkdirAll(filedir, 0755)
	return
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
	clusterName := bitApproximation(features)
	imgModel = &ImageFile{Name: name, Format: format, Index: stringify(features)}

	// ==== begin reading from db ====
	// asserts that gorm api calls are thread-safe
	this.mutex.RLock()
	cluster := getCluster(this, clusterName)
	if cluster != nil {
		// similarity check
		// 1. check for duplicate features to avoid pollution
		imgFiles := getAssocs(this, cluster)
		for _, file := range imgFiles {
			// test similarity between new file and file
			sim := imgutil.ChiDist(features, featureParse(file.Index))
			if sim < chiThresh { // too similar beyond a threshold is marked as same
				err = &DupFileError{file.Name + "." + file.Format, filename}
				return
			}
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
	this.mutex.RUnlock()
	// ==== end reading from db ====

	// ==== begin writing to db ====
	// associate image model
	this.mutex.Lock()
	if cluster == nil {
		cluster = createCluster(this, clusterName)
	}
	this.Model(cluster).Association("ImageFiles").Append(*imgModel)
	this.mutex.Unlock()
	// ==== end writing to db ====

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

// AddSource ...
// Associate a link to a imagefile
func (this *ImgDB) AddSource(imgModel *ImageFile, link string) {
	this.Model(imgModel).Association("Sources").Append(Source{Link: link})
}

// SourceExists ...
// Check if there is already an associated link in the database
func (this *ImgDB) SourceExists(link string) bool {
	sources := []Source{}
	this.Find(&sources, "link = ?", link)
	return len(sources) > 0
}

//// Error Member

// Error ...
// Implements error method for duplicate files
func (this DupFileError) Error() string {
	return fmt.Sprintf("existing %s, duplicate %s", this.existing, this.dupfile)
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
func stringify(arr []float32) []byte {
	buf := new(bytes.Buffer)
	panicCheck(binary.Write(buf, binary.LittleEndian, arr))
	return buf.Bytes()
}

func featureParse(feat []byte) []float32 {
	out := make([]float32, len(feat)/4)
	buf := bytes.NewBuffer(feat)
	panicCheck(binary.Read(buf, binary.LittleEndian, out))
	return out
}

// approximate input array by rounding each float value to 1 or 0
// representing the entire array as a bit string, then encode as hex
// assumes that this array has values between 1 and 0
// otherwise we will most likely get arrays of F
func bitApproximation(arr []float32) string {
	const nBitEncode = 6
	n := len(arr)
	outN := int(math.Ceil(float64(n) / nBitEncode))
	out := make([]byte, outN)
	thresh := float32(1) / float32(len(arr))
	accum := 0
	for i, f := range arr {
		bi := uint(i % nBitEncode)
		if f >= thresh {
			accum |= 1 << bi
		}
		if bi == nBitEncode-1 {
			out[i/nBitEncode] = b64Encode(accum)
			accum = 0
		}
	}
	if n%nBitEncode > 0 {
		out[outN-1] = b64Encode(accum)
		accum = 0
	}
	return string(out)
}

func b64Encode(i int) byte {
	if i == 62 {
		i = '-'
	} else if i == 63 {
		i = '_'
	} else if i < 10 {
		i = '0' + i
	} else if i < 36 {
		i = 'A' + (i - 10)
	} else {
		i = 'a' + (i - 36)
	}
	return byte(i)
}

//// Database Updates and Query

// create cluster if not found
func getCluster(db *ImgDB, clusterName string) *Cluster {
	if db == nil {
		panic("input db is nil")
	}
	clusters := []Cluster{}
	db.Find(&clusters, "name = ?", clusterName)
	var out *Cluster
	if len(clusters) > 0 {
		out = &clusters[0]
	}
	return out
}

func createCluster(db *ImgDB, clusterName string) *Cluster {
	if db == nil {
		panic("input db is nil")
	}
	out := &Cluster{Name: clusterName}
	db.Create(out)
	return out
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
	assoc := db.Model(cluster).Association("ImageFiles")
	assoc.Find(&imgs)
	return imgs
}

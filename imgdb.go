//// file: imgdb.go

// Package imgdb ...
// Is a wrapper database management api
// for storing and grouping image
// and avoiding duplicates
package imgdb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
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
	minW uint
	minH uint
}

//// Models

// Directory ...
// Specifies the directory path
type Directory struct {
	gorm.Model
	Dirpath    string `gorm:"not null;unique"`
	ImageFiles []ImageFile
}

// ImageFile ...
// Specifies name information and image features
type ImageFile struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Format      string `gorm:"not null"`
	Index       string `gorm:"not null"`
	DirectoryID int
}

// =============================================
//                    Globals
// =============================================

const (
	minLimit  = 500
	epsilon   = 1e-10
	chiThresh = 1e-5
)

// =============================================
//                    Public
// =============================================

// New ...
// Initializes and migrates relevant schemas
func New(dialect, source string) *ImgDB {
	var out *ImgDB = &ImgDB{minW: minLimit, minH: minLimit}
	var err error
	out.DB, err = gorm.Open(dialect, source)
	if err != nil {
		panic(err)
	}

	out.DB.AutoMigrate(&Directory{}, &ImageFile{})
	return out
}

// AddImg ...
// Extracts image features as search index
// Save to file system then add in database
// Filters out images too small beyond a limit
// Index and logic inspired from https://tinyurl.com/yaup47bg
func (this *ImgDB) AddImg(dir, name string, source bytes.Buffer) error {
	data := source.Bytes()
	img, format, err := image.Decode(&source)

	// size filter
	if err == nil {
		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()
		if uint(width) < this.minW || uint(height) < this.minH {
			return fmt.Errorf("image too small: got <%d, %d> size", width, height)
		}
	}

	// feature extraction
	features := generateFeature(img, format)
	if features == nil {
		return fmt.Errorf("failed to extract features for %s", name)
	}

	// serialize features
	featstr, err := stringify(features)
	if err != nil {
		return err
	}

	filename := name + "." + format
	imgModel := ImageFile{Name: name, Format: format, Index: featstr}

	// ==== begin critical section ====
	// todo: make threadsafe
	directory := getDirectory(this, dir)
	if directory == nil {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
		// create directory on db
		this.Create(&Directory{Dirpath: dir, ImageFiles: []ImageFile{imgModel}})
	} else {
		// similarity check
		// 1. check for duplicate features to avoid pollution
		imgFiles := getAssocs(this, directory) // todo: change to get clusters...
		// todo: optimize this search via clustering (to minimize search space)
		for _, file := range imgFiles {
			feat, err := featureParse(file.Index)
			if err != nil {
				return err
			}
			// test similarity between new file and file
			sim := chiDist(features, feat)
			if sim < chiThresh { // too similar beyond a threshold is marked as same
				fmt.Println("found similar files %s %s", file.Name, name+"."+format)
			}
		}
		// 2. check for same files and insert uuid to avoid dups
		if _, err := os.Stat(filepath.Join(dir, filename)); err == nil {
			imgModel.Name += uuid.New().String()
			filename = imgModel.Name + "." + format
		}
		// associate image model
		this.Model(directory).Association("ImageFiles").Append(imgModel)
	}
	// create image on db
	this.Create(imgModel)

	// write to file (invariant: filename is unique)
	file, err := os.Create(filepath.Join(dir, filename))
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err != nil {
		return err
	}
	file.Close()
	// ==== end critical section ====

	return nil
}

// =============================================
//                    Private
// =============================================

//// Data Serialization Utility

func stringify(arr []float32) (out string, err error) {
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.LittleEndian, arr)
	out = string(buf.Bytes())
	return
}

func featureParse(str string) (out []float32, err error) {
	out = make([]float32, len([]byte(str))/4)
	buf := bytes.NewBuffer([]byte(str))
	err = binary.Read(buf, binary.LittleEndian, out)
	return
}

//// Feature Extraction and Similarity Metric

func generateFeature(img image.Image, format string) []float32 {
	if format == "png" || format == "jpeg" {
		// extract color features and store on db
		histo := imgutil.New(8, 8, 8)
		histo.Describe(img)
		return histo.Feature
	}
	return nil
}

func chiDist(feat1, feat2 []float32) float64 {
	if len(feat1) != len(feat2) {
		return math.Inf(1)
	}
	var accum float64 = 0
	for i, f1 := range feat1 {
		num := f1 - feat2[i]
		den := f1 + feat2[i] + epsilon
		accum += float64(num * num / den)
	}
	return accum / 2
}

//// Database Updates and Query

func getDirectory(db *ImgDB, dir string) *Directory {
	if db == nil {
		panic("input db is nil")
	}
	dirs := []Directory{}
	db.Find(&dirs, "dirpath = ?", dir)
	var result *Directory
	if len(dirs) > 0 {
		result = &dirs[0]
	}
	return result
}

func getAssocs(db *ImgDB, dir *Directory) []ImageFile {
	if db == nil {
		panic("input db is nil")
	}
	if dir == nil {
		panic("input dir is nil")
	}
	imgs := []ImageFile{}
	db.Model(dir).Association("ImageFiles").Find(&imgs)
	return imgs
}

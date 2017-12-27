package imgutil

import (
	"image"
	"math"
)

// =============================================
//                    Declarations
// =============================================

type RGBHistogram struct {
	rBin, gBin, bBin uint32
	Feature          []float32
}

// =============================================
//                    Globals
// =============================================

const (
	epsilon         = 1e-10
	limit   float64 = 65535
)

// =============================================
//                    Public
// =============================================

// New ...
// Create a new RGBHistogram with # of bins matching input values
func New(r, g, b uint32) *RGBHistogram {
	nCount := r * g * b
	return &RGBHistogram{r, g, b, make([]float32, nCount)}
}

// Describe ...
// Builds the histogram from an image then normalize
func (this *RGBHistogram) Describe(img image.Image) {
	bounds := img.Bounds()
	dX := bounds.Dx()
	dY := bounds.Dy()
	nPix := dX * dY
	for x := 0; x < dX; x++ {
		for y := 0; y < dY; y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r /= uint32(math.Ceil(limit / float64(this.rBin)))
			g /= uint32(math.Ceil(limit / float64(this.gBin)))
			b /= uint32(math.Ceil(limit / float64(this.bBin)))
			this.Feature[r+g*this.rBin+b*this.rBin*this.gBin]++
		}
	}
	// normalize
	for i, v := range this.Feature {
		this.Feature[i] = v / float32(nPix)
	}
}

// Clear ...
// Clears all features from this histogram
func (this *RGBHistogram) Clear() {
	nBins := len(this.Feature)
	if nBins == 0 {
		return
	}
	this.Feature[0] = 0
	for i := 1; i < nBins; i *= 2 {
		copy(this.Feature[i:], this.Feature[:i])
	}
}

// GenerateFeature ...
// Grabs just the histogram values from input image
// And performs a format check
func GenerateFeature(img image.Image, format string) []float32 {
	if format == "png" || format == "jpeg" {
		// extract color features and store on db
		histo := New(8, 8, 8)
		histo.Describe(img)
		return histo.Feature
	}
	return nil
}

// ChiDist ...
// Measure the similarity between features
// Using chi squared distance metric
func ChiDist(feat1, feat2 []float32) float64 {
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

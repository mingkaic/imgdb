package imgutil

import (
	"image"
	"math"
)

type RGBHistogram struct {
	rBin, gBin, bBin uint32
	Feature    []float32
}

const limit float64 = 65535

func New(r, g, b uint32) *RGBHistogram {
	nCount := r * g * b
	return &RGBHistogram{r, g, b, make([]float32, nCount)}
}

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

			this.Feature[r*this.gBin*this.bBin+g*this.bBin+b]++
		}
	}
	for i, v := range this.Feature {
		this.Feature[i] = v / float32(nPix)
	}
}

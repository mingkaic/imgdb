package imgutil

import (
	"image/jpeg"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var colorMaps = map[string][]float32{
	"black":  {1, 0, 0, 0, 0, 0, 0, 0}, // (0, 0, 0)
	"red":    {0, 1, 0, 0, 0, 0, 0, 0}, // (1, 0, 0)
	"green":  {0, 0, 1, 0, 0, 0, 0, 0}, // (0, 1, 0)
	"yellow": {0, 0, 0, 1, 0, 0, 0, 0}, // (1, 1, 0)
	"blue":   {0, 0, 0, 0, 1, 0, 0, 0}, // (0, 0, 1)
	"purple": {0, 0, 0, 0, 0, 1, 0, 0}, // (1, 0, 1)
	"teal":   {0, 0, 0, 0, 0, 0, 1, 0}, // (0, 1, 1)
	"white":  {0, 0, 0, 0, 0, 0, 0, 1}, // (1, 1, 1)
}

var colorMaps8 = map[string][]float32{
	"black":  make([]float32, 512), // (0, 0, 0)
	"red":    make([]float32, 512), // (1, 0, 0)
	"green":  make([]float32, 512), // (0, 1, 0)
	"yellow": make([]float32, 512), // (1, 1, 0)
	"blue":   make([]float32, 512), // (0, 0, 1)
	"purple": make([]float32, 512), // (1, 0, 1)
	"teal":   make([]float32, 512), // (0, 1, 1)
	"white":  make([]float32, 512), // (1, 1, 1)
}

// =============================================
//                    Tests
// =============================================

func TestMain(m *testing.M) {
	setupExpectation()
	retCode := m.Run()
	os.Exit(retCode)
}

func TestHistogram(t *testing.T) {
	histo := New(2, 2, 2)
	for color, exFeats := range colorMaps {
		file, err := os.Open(filepath.Join("..", "testimgs", color+".jpg"))
		if err != nil {
			panic(err)
		}
		img, err := jpeg.Decode(file)
		if err != nil {
			panic(err)
		}
		histo.Describe(img)
		if !reflect.DeepEqual(exFeats, histo.Feature) {
			t.Errorf("expecting features %v, got %v", exFeats, histo.Feature)
		}
		histo.Clear()
	}
}

func TestGenerateFeature(t *testing.T) {
	nilFeat := GenerateFeature(nil, "bad format")
	if nilFeat != nil {
		t.Errorf("failed to notify bad format by returning nil features")
	}
	for color, exFeats := range colorMaps8 {
		file, err := os.Open(filepath.Join("..", "testimgs", color+".jpg"))
		if err != nil {
			panic(err)
		}
		img, err := jpeg.Decode(file)
		if err != nil {
			panic(err)
		}
		feat := GenerateFeature(img, "jpeg")
		if len(feat) != 512 {
			t.Errorf("expecting 512 features, got %d", len(feat))
		} else if !reflect.DeepEqual(exFeats, feat) {
			for i, ex := range exFeats {
				if ex != feat[i] {
					t.Errorf("color %s, expecting feature %f at index %d, got %f", color, ex, i, feat[i])
				}
			}
		}
	}
}

func TestChiDist(t *testing.T) {
	var gen = rand.New(rand.NewSource(time.Now().Unix()))
	for i := 0; i < 3000; i++ {
		sample, similar, opposite := randFeatures(gen)
		minDist := ChiDist(sample, similar)
		maxDist := ChiDist(sample, opposite)
		if minDist > 0.25 {
			t.Errorf("expect different feature less than 0.25, got %f", minDist)
		}
		if maxDist < 0.75 {
			t.Errorf("expect different feature greater than 0.75, got %f", maxDist)
		}
	}
}

// =============================================
//                    Private
// =============================================

func setupExpectation() {
	colorMaps8["black"][0] = 1    // (0, 0, 0)
	colorMaps8["red"][7] = 1      // (1, 0, 0)
	colorMaps8["green"][56] = 1   // (0, 1, 0)
	colorMaps8["yellow"][63] = 1  // (1, 1, 0)
	colorMaps8["blue"][448] = 1   // (0, 0, 1)
	colorMaps8["purple"][455] = 1 // (1, 0, 1)
	colorMaps8["teal"][504] = 1   // (0, 1, 1)
	colorMaps8["white"][511] = 1  // (1, 1, 1)
}

func randFeatures(rando *rand.Rand) (sample []float32, similar []float32, opposite []float32) {
	const nSample = 101
	mean := rando.Intn(nSample)
	stdev := 2 + rando.Intn(nSample/2)
	sample = make([]float32, nSample)
	similar = make([]float32, nSample)
	opposite = make([]float32, nSample)
	fillNormal(sample, mean, stdev)
	fillNormal(similar, mean, stdev-1)
	for i, s := range sample {
		opposite[i] = 1 - s
	}
	return
}

func fillNormal(sample []float32, mean, stdev int) {
	nSample := len(sample)
	var2 := float64(2 * stdev * stdev)
	var sum float32
	for i := 0; i < nSample; i++ {
		exnum := float64(i - mean)
		res := 1000 / float32(
			math.Sqrt(math.Pi*var2)*
				math.Exp(exnum*exnum/var2))
		sum += res
		sample[i] = res
	}
	for i, b := range sample {
		sample[i] = b / sum
	}
}

package fingerprint

import (
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/dsp/fourier"
)

// Spectrogram computes a magnitude spectrogram via Short-Time Fourier Transform.
// Returns a 2D slice [numFrames][numBins] of log-magnitude values.
// numBins = windowSize/2 + 1 (positive frequencies only).
func Spectrogram(samples []float64, windowSize, hopSize int) [][]float64 {
	numFrames := (len(samples) - windowSize) / hopSize + 1
	if numFrames <= 0 {
		return nil
	}
	numBins := windowSize/2 + 1

	hann := makeHannWindow(windowSize)
	fft := fourier.NewFFT(windowSize)
	windowed := make([]float64, windowSize)

	result := make([][]float64, numFrames)
	for i := 0; i < numFrames; i++ {
		offset := i * hopSize

		for j := 0; j < windowSize; j++ {
			windowed[j] = samples[offset+j] * hann[j]
		}

		coeffs := fft.Coefficients(nil, windowed)

		bins := make([]float64, numBins)
		for j := 0; j < numBins; j++ {
			mag := cmplx.Abs(coeffs[j])
			if mag > 0 {
				bins[j] = 20 * math.Log10(mag)
			}
		}
		result[i] = bins
	}

	return result
}

func makeHannWindow(size int) []float64 {
	w := make([]float64, size)
	for i := range w {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(size-1)))
	}
	return w
}

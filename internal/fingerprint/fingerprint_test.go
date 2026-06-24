package fingerprint

import (
	"math"
	"testing"
)

func TestSpectrogramBasic(t *testing.T) {
	sampleRate := 11025
	duration := 1.0
	freq := 440.0
	n := int(float64(sampleRate) * duration)
	samples := make([]float64, n)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate))
	}

	windowSize := 4096
	hopSize := 2048
	spec := Spectrogram(samples, windowSize, hopSize)

	expectedFrames := (n - windowSize) / hopSize + 1
	if len(spec) != expectedFrames {
		t.Errorf("expected %d frames, got %d", expectedFrames, len(spec))
	}

	expectedBins := windowSize/2 + 1
	if len(spec[0]) != expectedBins {
		t.Errorf("expected %d bins, got %d", expectedBins, len(spec[0]))
	}

	expectedBin := int(freq * float64(windowSize) / float64(sampleRate))
	peakBin := 0
	peakVal := math.Inf(-1)
	for b, v := range spec[0] {
		if v > peakVal {
			peakVal = v
			peakBin = b
		}
	}
	if intAbs(peakBin-expectedBin) > 2 {
		t.Errorf("expected peak near bin %d, got bin %d", expectedBin, peakBin)
	}
}

func intAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestFindPeaksFindsLocalMaxima(t *testing.T) {
	spec := make([][]float64, 10)
	for i := range spec {
		spec[i] = make([]float64, 10)
	}
	spec[5][5] = 50.0

	peaks := FindPeaks(spec, 3, 1.0)

	if len(peaks) != 1 {
		t.Fatalf("expected 1 peak, got %d", len(peaks))
	}
	if peaks[0].Frame != 5 || peaks[0].Bin != 5 {
		t.Errorf("expected peak at (5,5), got (%d,%d)", peaks[0].Frame, peaks[0].Bin)
	}
}

func TestGenerateHashesProducesHashes(t *testing.T) {
	peaks := []Peak{
		{Frame: 0, Bin: 100},
		{Frame: 10, Bin: 200},
		{Frame: 50, Bin: 300},
	}

	fp := GenerateHashes(peaks, 15, 200)
	if len(fp) == 0 {
		t.Fatal("expected non-empty fingerprint map")
	}

	totalEntries := 0
	for _, offsets := range fp {
		totalEntries += len(offsets)
	}
	if totalEntries < 2 {
		t.Errorf("expected at least 2 hash entries, got %d", totalEntries)
	}
}

func TestFingerprintEndToEnd(t *testing.T) {
	sampleRate := 11025
	n := sampleRate * 10
	samples := make([]float64, n)
	// Generate a richer signal: multiple tones that change over time,
	// simulating real audio with varying spectral content.
	freqs := []float64{220, 440, 880, 1200, 2000, 3500}
	for i := range samples {
		t1 := float64(i) / float64(sampleRate)
		for j, f := range freqs {
			// Each tone fades in/out at different rates to create spectral variation.
			amp := 0.3 * math.Sin(math.Pi*float64(j+1)*t1/5.0)
			if amp < 0 {
				amp = -amp
			}
			samples[i] += amp * math.Sin(2*math.Pi*f*t1)
		}
	}

	fp := Fingerprint(samples)
	if len(fp) == 0 {
		t.Fatal("expected non-empty fingerprints from multi-tone signal")
	}
}

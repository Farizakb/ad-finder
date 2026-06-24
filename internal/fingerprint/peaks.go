package fingerprint

// Peak represents a spectral peak at a specific time frame and frequency bin.
type Peak struct {
	Frame int
	Bin   int
}

// FindPeaks extracts local maxima from the spectrogram.
// A peak is a point greater than all neighbors within the given radius,
// and above minAmplitude.
func FindPeaks(spectrogram [][]float64, neighborhoodSize int, minAmplitude float64) []Peak {
	if len(spectrogram) == 0 {
		return nil
	}

	numFrames := len(spectrogram)
	numBins := len(spectrogram[0])
	var peaks []Peak

	for frame := 0; frame < numFrames; frame++ {
		for bin := 0; bin < numBins; bin++ {
			val := spectrogram[frame][bin]

			if val < minAmplitude {
				continue
			}

			isPeak := true

			fStart := frame - neighborhoodSize
			if fStart < 0 {
				fStart = 0
			}
			fEnd := frame + neighborhoodSize
			if fEnd >= numFrames {
				fEnd = numFrames - 1
			}
			bStart := bin - neighborhoodSize
			if bStart < 0 {
				bStart = 0
			}
			bEnd := bin + neighborhoodSize
			if bEnd >= numBins {
				bEnd = numBins - 1
			}

			for f := fStart; f <= fEnd && isPeak; f++ {
				for b := bStart; b <= bEnd; b++ {
					if f == frame && b == bin {
						continue
					}
					if spectrogram[f][b] >= val {
						isPeak = false
						break
					}
				}
			}

			if isPeak {
				peaks = append(peaks, Peak{Frame: frame, Bin: bin})
			}
		}
	}

	return peaks
}

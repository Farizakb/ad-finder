package fingerprint

import "sort"

// Peak represents a spectral peak at a specific time frame and frequency bin.
type Peak struct {
	Frame int
	Bin   int
}

const (
	// Number of frequency bands to split the spectrum into for relative peak selection.
	numBands = 6
	// Maximum peaks to keep per frame per band.
	peaksPerBand = 3
)

// FindPeaks extracts local maxima from the spectrogram using relative thresholding.
// Top-K loudest local maxima per frequency band per frame are kept, making peak
// density invariant to recording volume without an absolute amplitude floor.
func FindPeaks(spectrogram [][]float64, neighborhoodSize int) []Peak {
	if len(spectrogram) == 0 {
		return nil
	}

	numFrames := len(spectrogram)
	numBins := len(spectrogram[0])

	type scored struct {
		frame, bin int
		val        float64
	}
	var allMaxima []scored

	for frame := 0; frame < numFrames; frame++ {
		for bin := 0; bin < numBins; bin++ {
			val := spectrogram[frame][bin]

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
					if spectrogram[f][b] > val {
						isPeak = false
						break
					}
				}
			}

			if isPeak {
				allMaxima = append(allMaxima, scored{frame, bin, val})
			}
		}
	}

	// Step 2: keep only top-K peaks per frequency band per frame.
	bandSize := numBins / numBands
	if bandSize < 1 {
		bandSize = 1
	}

	type groupKey struct {
		frame, band int
	}
	groups := make(map[groupKey][]scored)
	for _, m := range allMaxima {
		band := m.bin / bandSize
		if band >= numBands {
			band = numBands - 1
		}
		key := groupKey{m.frame, band}
		groups[key] = append(groups[key], m)
	}

	var peaks []Peak
	for _, members := range groups {
		sort.Slice(members, func(i, j int) bool {
			return members[i].val > members[j].val
		})
		limit := peaksPerBand
		if limit > len(members) {
			limit = len(members)
		}
		for _, m := range members[:limit] {
			peaks = append(peaks, Peak{Frame: m.frame, Bin: m.bin})
		}
	}

	// Sort by frame for consistent hash generation order.
	sort.Slice(peaks, func(i, j int) bool {
		if peaks[i].Frame != peaks[j].Frame {
			return peaks[i].Frame < peaks[j].Frame
		}
		return peaks[i].Bin < peaks[j].Bin
	})

	return peaks
}

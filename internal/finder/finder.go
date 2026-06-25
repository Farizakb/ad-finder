package finder

import (
	"fmt"
	"math"
	"sort"

	"github.com/farizakb/ad-finder/internal/audio"
	"github.com/farizakb/ad-finder/internal/fingerprint"
)

const sampleRate = 11025

// Match represents a detected ad occurrence in a recording.
type Match struct {
	TimeSec     float64 // start time from beginning of the recording
	DurationSec float64 // ad duration in seconds
	Confidence  float64 // confidence score [0..1]
}

// FindAdvertInRecord detects all occurrences of an ad clip in a recording.
func FindAdvertInRecord(recordPath, advertPath string) ([]Match, error) {
	recSamples, err := audio.DecodeMono(recordPath, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("decode recording: %w", err)
	}

	advSamples, err := audio.DecodeMono(advertPath, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("decode advert: %w", err)
	}

	advDuration := float64(len(advSamples)) / float64(sampleRate)

	recFP := fingerprint.Fingerprint(recSamples)
	advFP := fingerprint.Fingerprint(advSamples)

	return matchFingerprints(recFP, advFP, advDuration)
}

// FindAdvertWithFingerprint matches a pre-computed recording fingerprint against an ad.
func FindAdvertWithFingerprint(recFP fingerprint.FingerprintMap, advertPath string) ([]Match, error) {
	advSamples, err := audio.DecodeMono(advertPath, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("decode advert: %w", err)
	}

	advDuration := float64(len(advSamples)) / float64(sampleRate)
	advFP := fingerprint.Fingerprint(advSamples)

	return matchFingerprints(recFP, advFP, advDuration)
}

// FingerprintRecord decodes and fingerprints a recording for reuse across multiple ads.
func FingerprintRecord(recordPath string) (fingerprint.FingerprintMap, error) {
	recSamples, err := audio.DecodeMono(recordPath, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("decode recording: %w", err)
	}
	return fingerprint.Fingerprint(recSamples), nil
}

func matchFingerprints(recFP, advFP fingerprint.FingerprintMap, advDuration float64) ([]Match, error) {
	totalAdvHashes := 0
	for _, offsets := range advFP {
		totalAdvHashes += len(offsets)
	}

	if totalAdvHashes == 0 {
		return nil, nil
	}

	// Build offset histogram: for each matching hash, compute
	// recording_frame - ad_frame. Real matches cluster at one offset.
	offsetHits := make(map[int]int)

	for hash, advOffsets := range advFP {
		recOffsets, ok := recFP[hash]
		if !ok {
			continue
		}
		for _, advOff := range advOffsets {
			for _, recOff := range recOffsets {
				offset := int(recOff) - int(advOff)
				offsetHits[offset]++
			}
		}
	}

	// Smooth with ±1 frame window for broadcast timing jitter.
	smoothed := make(map[int]int)
	for offset, hits := range offsetHits {
		sum := hits
		if v, ok := offsetHits[offset-1]; ok {
			sum += v
		}
		if v, ok := offsetHits[offset+1]; ok {
			sum += v
		}
		smoothed[offset] = sum
	}

	// Sort candidates by hit count descending.
	type candidate struct {
		offset int
		hits   int
	}
	var candidates []candidate
	for offset, hits := range smoothed {
		candidates = append(candidates, candidate{offset, hits})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].hits > candidates[j].hits
	})

	if len(candidates) == 0 {
		return nil, nil
	}

	// Compute median hits across all offsets for relative dominance check.
	median := candidates[len(candidates)/2].hits

	// Adaptive threshold: minimum absolute hits.
	minHits := totalAdvHashes / 20
	if minHits < 5 {
		minHits = 5
	}

	// Dominance ratio: a real match should be significantly above the noise floor.
	// Require the candidate to be at least 3x the median offset hit count.
	minDominance := 3.0

	secsPerFrame := float64(fingerprint.DefaultHopSize) / float64(sampleRate)

	var matches []Match
	for _, c := range candidates {
		if c.hits < minHits {
			break
		}

		// Relative dominance check: skip if not clearly above noise.
		if median > 0 && float64(c.hits)/float64(median) < minDominance {
			continue
		}

		timeSec := float64(c.offset) * secsPerFrame
		if timeSec < 0 {
			continue
		}

		// Confidence = peak-to-noise ratio, normalized.
		// Uses the ratio of this offset's hits to the median, scaled into [0,1].
		var confidence float64
		if median > 0 {
			ratio := float64(c.hits) / float64(median)
			confidence = 1.0 - 1.0/ratio
		} else {
			confidence = 1.0
		}
		if confidence > 1.0 {
			confidence = 1.0
		}

		// Merge nearby detections (within half the ad duration).
		merged := false
		for i := range matches {
			if math.Abs(matches[i].TimeSec-timeSec) < advDuration*0.5 {
				if confidence > matches[i].Confidence {
					matches[i].TimeSec = timeSec
					matches[i].Confidence = confidence
				}
				merged = true
				break
			}
		}

		if !merged {
			matches = append(matches, Match{
				TimeSec:     timeSec,
				DurationSec: advDuration,
				Confidence:  confidence,
			})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].TimeSec < matches[j].TimeSec
	})

	return matches, nil
}

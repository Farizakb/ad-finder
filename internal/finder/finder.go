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

	// For each matching hash, compute recording_frame - ad_frame = offset.
	// Real matches cluster at the same offset.
	offsetHits := make(map[int]int)

	totalAdvHashes := 0
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
		totalAdvHashes += len(advOffsets)
	}

	if totalAdvHashes == 0 {
		return nil, nil
	}

	// Sort candidates by hit count descending.
	type candidate struct {
		offset int
		hits   int
	}
	var candidates []candidate
	for offset, hits := range offsetHits {
		candidates = append(candidates, candidate{offset, hits})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].hits > candidates[j].hits
	})

	// Adaptive threshold: at least 5% of ad hashes, minimum 5 absolute.
	minHits := totalAdvHashes / 20
	if minHits < 5 {
		minHits = 5
	}

	secsPerFrame := float64(fingerprint.DefaultHopSize) / float64(sampleRate)

	var matches []Match
	for _, c := range candidates {
		if c.hits < minHits {
			break
		}

		timeSec := float64(c.offset) * secsPerFrame
		if timeSec < 0 {
			continue
		}

		confidence := float64(c.hits) / float64(totalAdvHashes)
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

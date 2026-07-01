package finder

import (
	"fmt"
	"os"
	"testing"

	"github.com/farizakb/ad-finder/internal/fingerprint"
)

func getTestPaths(t *testing.T) (recordWithAd, recordWithoutAd, advert string) {
	t.Helper()

	recordWithAd = os.Getenv("TEST_RECORD_WITH_AD")
	recordWithoutAd = os.Getenv("TEST_RECORD_WITHOUT_AD")
	advert = os.Getenv("TEST_ADVERT")

	if recordWithAd == "" || advert == "" {
		t.Skip("TEST_RECORD_WITH_AD and TEST_ADVERT must be set")
	}

	return
}

// TestFoundMultiple verifies that all occurrences of an ad are found.
func TestFoundMultiple(t *testing.T) {
	recordPath, _, advertPath := getTestPaths(t)

	matches, err := FindAdvertInRecord(recordPath, advertPath)
	if err != nil {
		t.Fatalf("FindAdvertInRecord failed: %v", err)
	}

	if len(matches) == 0 {
		t.Fatal("expected at least one match, got none")
	}

	t.Logf("Found %d match(es):", len(matches))
	for i, m := range matches {
		t.Logf("  Match %d: time=%.1fs, duration=%.1fs, confidence=%.4f",
			i+1, m.TimeSec, m.DurationSec, m.Confidence)
	}

	for i, m := range matches {
		if m.TimeSec < 0 {
			t.Errorf("match %d: negative timestamp %.1f", i, m.TimeSec)
		}
		if m.Confidence < 0 || m.Confidence > 1 {
			t.Errorf("match %d: confidence %.4f outside [0,1]", i, m.Confidence)
		}
		if m.DurationSec <= 0 {
			t.Errorf("match %d: non-positive duration %.1f", i, m.DurationSec)
		}
	}

	expectedCount := os.Getenv("TEST_AD_COUNT")
	if expectedCount != "" {
		var expected int
		fmt.Sscanf(expectedCount, "%d", &expected)
		if len(matches) != expected {
			t.Errorf("expected %d matches, got %d", expected, len(matches))
		}
	}
}

func TestMatchFingerprintsFindsInjectedAd(t *testing.T) {
	advFP := fingerprint.FingerprintMap{}
	recFP := fingerprint.FingerprintMap{}

	// 20 hashes all cluster at offset 500: ad-frame=0, rec-frame=500.
	for h := uint32(0); h < 20; h++ {
		advFP[h] = []uint32{0}
		recFP[h] = []uint32{500}
	}

	// 80 noise hashes each produce a unique single-hit offset, so the
	// median stays at 1 and the real peak (20 hits) is clearly dominant.
	for h := uint32(20); h < 100; h++ {
		advFP[h] = []uint32{h * 3}
		recFP[h] = []uint32{h * 7} // offset = h*7 - h*3 = h*4, all distinct
	}

	matches, err := matchFingerprints(recFP, advFP, 4.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	secsPerFrame := float64(fingerprint.DefaultHopSize) / float64(11025)
	expectedTime := 500.0 * secsPerFrame
	if diff := matches[0].TimeSec - expectedTime; diff < -0.1 || diff > 0.1 {
		t.Errorf("expected TimeSec ~%.2f, got %.2f", expectedTime, matches[0].TimeSec)
	}
	if matches[0].Confidence <= 0 || matches[0].Confidence > 1.0 {
		t.Errorf("confidence out of range: %f", matches[0].Confidence)
	}
}

func TestMatchFingerprintsEmptyRecording(t *testing.T) {
	advFP := fingerprint.FingerprintMap{1: []uint32{0}, 2: []uint32{10}}
	recFP := fingerprint.FingerprintMap{}

	matches, err := matchFingerprints(recFP, advFP, 4.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no matches against empty recording, got %d", len(matches))
	}
}

func TestMatchFingerprintsNoSharedHashes(t *testing.T) {
	advFP := fingerprint.FingerprintMap{1: []uint32{0}, 2: []uint32{5}}
	recFP := fingerprint.FingerprintMap{100: []uint32{0}, 200: []uint32{5}}

	matches, err := matchFingerprints(recFP, advFP, 4.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no matches with no shared hashes, got %d", len(matches))
	}
}

// TestNotFound verifies that a recording without the ad returns empty.
func TestNotFound(t *testing.T) {
	_, recordWithoutAdPath, advertPath := getTestPaths(t)

	if recordWithoutAdPath == "" {
		t.Skip("TEST_RECORD_WITHOUT_AD not set")
	}

	matches, err := FindAdvertInRecord(recordWithoutAdPath, advertPath)
	if err != nil {
		t.Fatalf("FindAdvertInRecord failed: %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("expected no matches, got %d:", len(matches))
		for i, m := range matches {
			t.Logf("  False positive %d: time=%.1fs, confidence=%.4f", i+1, m.TimeSec, m.Confidence)
		}
	}
}

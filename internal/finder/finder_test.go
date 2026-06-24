package finder

import (
	"fmt"
	"os"
	"testing"
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

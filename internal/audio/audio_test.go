package audio

import (
	"os"
	"testing"
)

func TestDecodeMono(t *testing.T) {
	testFile := os.Getenv("TEST_AUDIO_FILE")
	if testFile == "" {
		t.Skip("TEST_AUDIO_FILE not set, skipping audio decode test")
	}

	samples, err := DecodeMono(testFile, 11025)
	if err != nil {
		t.Fatalf("DecodeMono failed: %v", err)
	}

	if len(samples) == 0 {
		t.Fatal("DecodeMono returned empty samples")
	}

	if len(samples) < 1000 {
		t.Errorf("suspiciously few samples: %d", len(samples))
	}

	for i, s := range samples[:100] {
		if s < -1.1 || s > 1.1 {
			t.Errorf("sample[%d] = %f, outside [-1.1, 1.1]", i, s)
			break
		}
	}
}

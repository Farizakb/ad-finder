package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
)

// DecodeMono decodes an audio file to mono PCM samples at the given sample rate.
// Returns normalized float64 samples in [-1.0, 1.0].
// Requires ffmpeg on PATH.
func DecodeMono(path string, sampleRate int) ([]float64, error) {
	cmd := exec.Command("ffmpeg",
		"-i", path,
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-ac", "1",
		"-loglevel", "error",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	raw, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("read pcm: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ffmpeg exit: %w", err)
	}

	numSamples := len(raw) / 2
	samples := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		sample := int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
		samples[i] = float64(sample) / math.MaxInt16
	}

	return samples, nil
}

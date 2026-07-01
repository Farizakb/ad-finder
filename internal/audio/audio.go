package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"
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

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

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
		if msg := strings.TrimSpace(stderrBuf.String()); msg != "" {
			return nil, fmt.Errorf("ffmpeg: %w\n%s", err, msg)
		}
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}

	numSamples := len(raw) / 2
	samples := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		sample := int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
		samples[i] = float64(sample) / math.MaxInt16
	}

	return samples, nil
}

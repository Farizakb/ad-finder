# Ad Finder — Audio Fingerprint-based Ad Detection

Detects known advertisement clips within radio/TV recordings using Shazam-style audio fingerprinting.

## Algorithm

### Overview

The system uses **spectral peak-pair fingerprinting**, based on the algorithm described in Avery Wang's 2003 paper *"An Industrial-Strength Audio Search Algorithm"* (the Shazam paper).

### Pipeline

1. **Decode** — MP3 → mono PCM at 11025 Hz via ffmpeg
2. **Spectrogram** — STFT with 4096-sample Hann window, 50% overlap → log-magnitude frequency bins
3. **Peak extraction** — 2D local maximum filter (±20 bins/frames), minimum amplitude threshold → stable spectral landmarks
4. **Hash generation** — Each peak is paired with up to 15 nearby future peaks. Each pair is encoded as a 32-bit hash: `(freq1, freq2, delta_time)`. Stored with the peak's absolute time position.
5. **Matching** — For each hash in the ad that also appears in the recording, compute `time_recording - time_ad` = candidate offset. Build a histogram of offsets. Peaks in the histogram (many hashes agreeing on the same offset) indicate real matches.
6. **Scoring** — Confidence = matching hashes at offset / total ad hashes. Nearby detections are merged.

### Parameters

| Parameter | Value | Rationale |
|---|---|---|
| Sample rate | 11025 Hz | Captures 0–5512 Hz, sufficient for audio identity. 4× less data than 44.1 kHz |
| FFT window | 4096 samples (371ms) | Good frequency resolution (2.7 Hz/bin), power-of-2 for fast FFT |
| Hop size | 2048 (50% overlap) | Standard overlap, prevents missing boundary events |
| Peak neighborhood | ±20 | Controls peak density; Dejavu reference default |
| Fan-out | 15 | Pairs per anchor peak; balances uniqueness vs. computation |
| Target zone | 200 frames | ~37s lookahead for pair targets |
| Min amplitude | 10.0 dB | Filters silence/noise |

## Alternatives Considered

1. **Cross-correlation (time domain)**: Slide the ad waveform across the recording and compute correlation at each offset. Extremely accurate but O(n×m) per pair — billions of operations per pair. Impractical at 60,000 pairs/run.

2. **Chromaprint / AcoustID**: Open-source fingerprinting library designed for "is this the same song?" matching, not "where does this clip appear within a longer recording?" Would require sliding-window application, negating its speed advantage.

3. **Neural audio embeddings** (e.g., CLAP, VGGish): Extract learned embeddings and compare via cosine similarity. Requires a trained model, heavy dependency, and doesn't natively give ±2s timestamp precision without windowed inference — overkill for exact-match detection.

## Known Limitations

- **Requires ffmpeg** on PATH for audio decoding
- **Memory usage**: A 61-minute recording at 11025 Hz ≈ 320MB of PCM data, plus spectrogram and hash tables. Batch mode with many workers can use several GB. Control via `--workers` flag.
- **Highly distorted audio**: Handles MP3 compression and moderate noise well, but extreme time-stretching, pitch-shifting, or heavy dynamic compression may reduce recall.
- **Very short ads** (< 3 seconds): Produce few fingerprint hashes, increasing false negative risk.
- **Near-duplicate ads**: If two ads share a long identical segment, both may trigger at the same offset.

## Usage

### Single file

```bash
./finder --record path/to/record.mp3 --advert path/to/ad.mp3 --output text
```

### Batch mode

```bash
./finder \
  --records-dir path/to/records/ \
  --adverts-dir path/to/adverts/ \
  --workers 8 \
  --output json
```

## Building

```bash
go build -o finder ./cmd/finder/
```

## Testing

```bash
# Unit tests (no audio files needed):
go test ./internal/fingerprint/ -v

# Integration tests (requires audio files):
TEST_RECORD_WITH_AD=testdata/record.mp3 \
TEST_ADVERT=testdata/ad.mp3 \
TEST_AD_COUNT=3 \
TEST_RECORD_WITHOUT_AD=testdata/other.mp3 \
go test ./internal/finder/ -v -timeout 300s
```

## Project Structure

```
cmd/finder/main.go              — CLI: flag parsing, output formatting, batch mode
internal/audio/audio.go         — MP3→PCM decoding via ffmpeg
internal/fingerprint/spectro.go — STFT spectrogram (Hann window + FFT)
internal/fingerprint/peaks.go   — Spectral peak extraction (2D local maxima)
internal/fingerprint/hash.go    — Peak pairing + 32-bit hash generation
internal/fingerprint/fingerprint.go — Public API wiring the pipeline
internal/finder/finder.go       — Hash matching, offset histogram, scoring
internal/finder/finder_test.go  — Integration tests with real audio
```

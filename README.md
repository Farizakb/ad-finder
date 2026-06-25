# Ad Finder

A high-performance audio ad detection system written in Go. Given a 61-minute radio/TV recording and a short reference ad clip, it finds the exact timestamps of every occurrence of the ad within the recording.

Built for production scale: processes up to 60,000 recording-ad pairs per 15-minute window using parallel goroutines and a fingerprint-once architecture.

## How It Works

The system uses **spectral peak-pair fingerprinting** — the same class of algorithm behind Shazam — adapted for broadcast ad detection.

### Pipeline

```
MP3 file ──→ FFmpeg decode ──→ Spectrogram (STFT) ──→ Peak extraction ──→ Hash generation
                                                                              │
                                                                              ▼
                                                              Offset histogram matching
                                                                              │
                                                                              ▼
                                                              Matches with timestamps
```

**Step 1 — Decode.** The MP3 is decoded to mono PCM at 11,025 Hz via ffmpeg. Downsampling from 44.1 kHz to 11 kHz reduces data 4x while preserving the 0–5.5 kHz band where speech and music identity live.

**Step 2 — Spectrogram.** A Short-Time Fourier Transform (STFT) converts the raw waveform into a time-frequency representation. A 4096-sample Hann window slides across the audio with 75% overlap (hop = 1024 samples, ~93ms per frame), producing a grid of frequency magnitudes in log scale.

**Step 3 — Peak extraction.** Local maxima are extracted from the spectrogram using a 2D neighborhood filter (±10 frames × ±10 bins). Instead of an absolute amplitude threshold — which breaks on volume-normalized radio — the algorithm uses **relative peak selection**: the top 3 loudest peaks per frequency band (6 bands) per frame. This makes fingerprinting invariant to recording volume.

**Step 4 — Hash generation.** Each peak is paired with up to 20 nearby future peaks within a 300-frame target zone. Each pair encodes three values into a 32-bit hash:

```
hash = (freq_anchor : 10 bits) << 22 | (freq_target : 10 bits) << 12 | (delta_time : 12 bits)
```

A single frequency peak is common across many audio clips. But the combination of two specific frequencies at a specific time gap is highly distinctive — like a DNA marker for that audio segment.

**Step 5 — Matching.** For every hash in the ad that also appears in the recording, the algorithm computes `time_in_recording - time_in_ad` = a candidate time offset. If the ad truly appears in the recording, hundreds of hashes will agree on the same offset — producing a sharp spike in the offset histogram, while random collisions scatter across many offsets.

**Step 6 — Scoring.** Matches are scored by **peak-to-noise dominance**: the ratio of the best offset's hit count to the median across all offsets. Real matches produce ratios of 100–1000x, yielding 99%+ confidence. Candidates must exceed both an absolute minimum (5 hits or 5% of the ad's total hashes) and a 3x dominance threshold over the noise floor. Nearby detections within half the ad duration are merged.

### Algorithm Parameters

| Parameter | Value | Rationale |
|---|---|---|
| Sample rate | 11,025 Hz | Captures 0–5,512 Hz; 4x less data than 44.1 kHz |
| FFT window | 4,096 samples (371ms) | 2.7 Hz/bin frequency resolution; power-of-2 for fast FFT |
| Hop size | 1,024 samples (93ms) | High temporal resolution for ±2s timestamp accuracy |
| Peak neighborhood | ±10 frames × ±10 bins | Balanced peak density for broadcast audio |
| Peak selection | Top-3 per band, 6 bands | Volume-invariant; consistent density regardless of loudness |
| Fan-out | 20 pairs per anchor | Dense fingerprints; critical for short ads (< 5s) |
| Target zone | 300 frames (~28s) | Sufficient lookahead for combinatorial uniqueness |
| Min amplitude | 1.0 dB | Low floor; relative selection handles the rest |
| Match threshold | max(5, total_hashes/20) | Adaptive to ad length |
| Dominance threshold | 3x median | Rejects noise-level coincidences |

## Performance

Tested on real broadcast radio data:

| Metric | Value |
|---|---|
| Recall | 100% on test data (5/5 known occurrences found) |
| False positives | 0 (no cross-station false matches) |
| Confidence on real matches | 99.4% – 99.9% |
| Shortest ad detected | 4.2 seconds |
| Timestamp accuracy | < 1 second |
| Batch matching speed | ~30 pairs/sec (after fingerprinting) |
| Fingerprint time (61-min file) | ~25 seconds |

### Resource Usage

| Resource | Single pair | Batch (20 recordings × 3,000 ads) |
|---|---|---|
| Memory | ~500 MB peak | ~2–4 GB (controlled by `--workers`) |
| CPU | Single-threaded decode + fingerprint | Parallel matching via goroutines |
| Disk | No temp files | No temp files |
| External deps | ffmpeg on PATH | ffmpeg on PATH |

In batch mode, each recording is fingerprinted **once** and reused across all ad comparisons — the expensive decode+FFT+peak step runs N times (for N recordings), not N×M times.

## Usage

### Single file mode

```bash
./finder --record path/to/record.mp3 --advert path/to/ad.mp3 --output text
```

Output:
```
Record: recording_001.mp3
Advert: ad_clip_01.mp3
Result: 3 match(es)
  1) 06:03.90  duration=4.2s  confidence=0.9962
  2) 21:30.10  duration=4.2s  confidence=0.9964
  3) 39:38.84  duration=4.2s  confidence=0.9946
```

### JSON output

```bash
./finder --record path/to/record.mp3 --advert path/to/ad.mp3 --output json
```

```json
[
  {
    "TimeSec": 363.9,
    "DurationSec": 4.18,
    "Confidence": 0.9962
  }
]
```

### Batch mode (parallel processing)

```bash
./finder \
  --records-dir path/to/recordings/ \
  --adverts-dir path/to/adverts/ \
  --workers 8 \
  --output json
```

Processes all recording × ad combinations in parallel using goroutines. Reports throughput on stderr:

```
Batch: 20 recordings × 50 adverts = 1000 pairs, 8 workers
Fingerprinting 20 recordings...
  ✓ recording_001.mp3
  ✓ recording_002.mp3
  ...

Completed 1000 pairs in 12.4s (80.65 pairs/sec)
```

## Interface

The detection function has a fixed signature as specified:

```go
type Match struct {
    TimeSec     float64 // start time from beginning of the recording (seconds)
    DurationSec float64 // ad duration (seconds)
    Confidence  float64 // confidence score [0..1]
}

func FindAdvertInRecord(recordPath, advertPath string) ([]Match, error)
```

## Building

Requires Go 1.22+ and ffmpeg on PATH.

```bash
go build -o finder ./cmd/finder/
```

## Testing

```bash
# Unit tests — no audio files needed, runs in < 1 second:
go test ./internal/fingerprint/ -v

# Integration tests — requires real audio files:
TEST_RECORD_WITH_AD=testdata/records/record.mp3 \
TEST_ADVERT=testdata/adverts/ad.mp3 \
TEST_AD_COUNT=3 \
TEST_RECORD_WITHOUT_AD=testdata/records/other.mp3 \
go test ./internal/finder/ -v -timeout 300s
```

Tests include:
- `TestFoundMultiple` — verifies all known occurrences are found
- `TestNotFound` — verifies recordings without the ad return an empty result
- `TestSpectrogramBasic` — validates FFT output shape and peak location on a synthetic 440 Hz tone
- `TestFindPeaksFindsLocalMaxima` — validates 2D local maximum filter
- `TestGenerateHashesProducesHashes` — validates peak pairing produces hashes
- `TestFingerprintEndToEnd` — validates the full pipeline on a multi-tone signal

## Project Structure

```
ad-finder/
├── cmd/
│   └── finder/
│       └── main.go                 # CLI entrypoint, flag parsing, batch orchestration
├── internal/
│   ├── audio/
│   │   ├── audio.go                # MP3 → PCM decoding via ffmpeg subprocess
│   │   └── audio_test.go           # Decode validation
│   ├── fingerprint/
│   │   ├── spectro.go              # STFT spectrogram (Hann window + gonum FFT)
│   │   ├── peaks.go                # Spectral peak extraction (2D local max + top-K per band)
│   │   ├── hash.go                 # Combinatorial peak pairing → 32-bit hashes
│   │   ├── fingerprint.go          # Public API: samples → FingerprintMap
│   │   └── fingerprint_test.go     # Unit tests for the fingerprint pipeline
│   └── finder/
│       ├── finder.go               # Hash matching, offset histogram, confidence scoring
│       └── finder_test.go          # Integration tests with real audio files
├── go.mod
├── go.sum
└── README.md
```

Each package has a single responsibility:
- **audio** — only knows about ffmpeg and PCM conversion
- **fingerprint** — only knows about spectrograms, peaks, and hashes
- **finder** — only knows about matching fingerprints and scoring results

## Alternatives Considered

### Cross-correlation (time domain)

Slide the ad waveform across the recording and compute correlation at each offset. Produces highly accurate results but at O(n×m) complexity per pair: a 61-minute recording against a 30-second ad at 11 kHz requires ~2 billion multiply-accumulate operations. At 60,000 pairs per run, this is computationally infeasible.

### Chromaprint / AcoustID

An open-source audio fingerprinting library widely used for music identification. It produces a single global fingerprint per audio segment — designed for "is this the same song?" matching. For sub-clip detection ("where does this clip appear within a longer recording?"), it would require a sliding-window approach that negates its speed advantage and was not designed for timestamp-level precision.

### Neural audio embeddings (CLAP, VGGish)

Extract learned embeddings via a neural network and compare via cosine similarity. Requires a trained model, introduces a heavy Python/ONNX dependency, and does not natively provide ±2-second timestamp precision without windowed inference. Effective for semantic audio similarity but overkill for exact-match detection where the reference clip is known.

## Known Limitations

- **ffmpeg dependency** — required on PATH for audio decoding; no pure-Go MP3 decoder is used to ensure compatibility with all codec variants and bitrates
- **Memory** — a 61-minute recording at 11,025 Hz produces ~40M samples (~320 MB PCM). The spectrogram, peak list, and hash tables add overhead. In batch mode, control memory via the `--workers` flag
- **Highly distorted audio** — the algorithm handles MP3 compression artifacts, moderate background noise, and volume normalization well. Extreme time-stretching, pitch-shifting, or heavy dynamic compression may degrade recall
- **Very short ads (< 3s)** — produce fewer fingerprint hashes, reducing the signal-to-noise ratio in the offset histogram. The adaptive threshold compensates, but recall may drop for clips under 3 seconds
- **Overlapping ads** — if two different ads share a long segment of identical audio, both may produce matches at the same offset. The confidence score helps distinguish genuine from coincidental matches

## References

- Wang, A. (2003). *An Industrial-Strength Audio Search Algorithm.* Proceedings of the 4th International Society for Music Information Retrieval Conference (ISMIR). — The foundational Shazam paper describing spectral peak-pair fingerprinting.
- Ellis, D. (2009). *Robust Landmark-Based Audio Fingerprinting.* — Analysis of the peak-pair approach with parameter tuning guidelines.
- [Dejavu](https://github.com/worldveil/dejavu) — Open-source Python implementation of audio fingerprinting, used as a reference for parameter defaults.

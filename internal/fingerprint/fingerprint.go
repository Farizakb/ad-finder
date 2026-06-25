package fingerprint

// FingerprintMap maps a 32-bit hash to the list of frame offsets where it occurs.
type FingerprintMap map[uint32][]uint32

const (
	DefaultWindowSize       = 4096
	DefaultHopSize          = 1024
	DefaultNeighborhoodSize = 10
	DefaultMinAmplitude     = 1.0
	DefaultFanOut           = 20
	DefaultTargetZone       = 300
)

// Fingerprint computes the full fingerprint of an audio signal.
// samples should be mono PCM at 11025 Hz.
func Fingerprint(samples []float64) FingerprintMap {
	return FingerprintWithParams(samples,
		DefaultWindowSize, DefaultHopSize,
		DefaultNeighborhoodSize, DefaultMinAmplitude,
		DefaultFanOut, DefaultTargetZone,
	)
}

// FingerprintWithParams computes fingerprints with custom parameters.
func FingerprintWithParams(samples []float64, windowSize, hopSize, neighborhoodSize int, minAmplitude float64, fanOut, targetZone int) FingerprintMap {
	spec := Spectrogram(samples, windowSize, hopSize)
	peaks := FindPeaks(spec, neighborhoodSize, minAmplitude)
	return GenerateHashes(peaks, fanOut, targetZone)
}

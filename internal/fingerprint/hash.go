package fingerprint

// GenerateHashes creates combinatorial fingerprint hashes from peak pairs.
// Each anchor peak pairs with up to fanOut target peaks within targetZoneFrames.
//
// Hash packing (32 bits):
//
//	bits [31:21] = anchor frequency bin (11 bits, covers 0–2047)
//	bits [20:10] = target frequency bin (11 bits, covers 0–2047)
//	bits [9:0]   = delta time in frames (10 bits, covers 0–1023)
func GenerateHashes(peaks []Peak, fanOut int, targetZoneFrames int) FingerprintMap {
	fp := make(FingerprintMap)

	for i, anchor := range peaks {
		paired := 0

		for j := i + 1; j < len(peaks) && paired < fanOut; j++ {
			target := peaks[j]

			dt := target.Frame - anchor.Frame
			if dt <= 0 {
				continue
			}
			if dt > targetZoneFrames {
				break
			}

			f1 := uint32(anchor.Bin) & 0x7FF
			f2 := uint32(target.Bin) & 0x7FF
			d := uint32(dt) & 0x3FF

			hash := (f1 << 21) | (f2 << 10) | d
			timeOffset := uint32(anchor.Frame)

			fp[hash] = append(fp[hash], timeOffset)
			paired++
		}
	}

	return fp
}

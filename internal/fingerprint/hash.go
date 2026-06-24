package fingerprint

// GenerateHashes creates combinatorial fingerprint hashes from peak pairs.
// Each anchor peak pairs with up to fanOut target peaks within targetZoneFrames.
//
// Hash packing (32 bits):
//
//	bits [20:29] = anchor frequency (10 bits)
//	bits [10:19] = target frequency (10 bits)
//	bits [0:11]  = delta time in frames (12 bits)
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

			f1 := uint32(anchor.Bin) & 0x3FF
			f2 := uint32(target.Bin) & 0x3FF
			d := uint32(dt) & 0xFFF

			hash := (f1 << 20) | (f2 << 10) | d
			timeOffset := uint32(anchor.Frame)

			fp[hash] = append(fp[hash], timeOffset)
			paired++
		}
	}

	return fp
}

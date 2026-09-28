package ifpoll

import "time"

// computeRate derives a bits-per-second rate from two octet-counter
// samples, handling both counter wraparound and a device reboot (counter
// reset) -- ported from the standard RRDtool/Cacti-style heuristic:
//
//   - counter went forward (new >= prev): plain delta.
//   - counter went backward and the previous value was in the top 10% of
//     its width's range: treat it as a genuine wraparound and compute the
//     delta across the rollover.
//   - counter went backward from anywhere else: the device almost
//     certainly rebooted and the counter reset near zero. There's no
//     meaningful delta for this cycle -- returning ok=false tells the
//     caller to skip the rate rather than report a bogus (huge or
//     negative) spike. The next cycle resumes normal delta math from the
//     new baseline.
func computeRate(prevValue uint64, prevAt time.Time, newValue uint64, newAt time.Time, width int) (ratebps float64, ok bool) {
	if prevAt.IsZero() {
		return 0, false
	}
	dt := newAt.Sub(prevAt).Seconds()
	if dt <= 0 {
		return 0, false
	}

	var maxVal uint64
	if width == 32 {
		maxVal = 1<<32 - 1
	} else {
		maxVal = 1<<64 - 1
	}

	var delta uint64
	if newValue >= prevValue {
		delta = newValue - prevValue
	} else {
		threshold := maxVal - maxVal/10
		if prevValue >= threshold {
			// Wrapped: distance from prev to the top of the range, plus
			// the new value counted up from zero.
			delta = (maxVal - prevValue) + newValue + 1
		} else {
			return 0, false // reboot / counter reset -- no rate this cycle
		}
	}

	return float64(delta) * 8 / dt, true
}

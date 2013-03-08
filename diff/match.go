
package diffmatchpatch

import (
	"math"
	"strings"
)

// MatchMain locates the best instance of 'pattern' in 'text' near 'loc'.
// Returns -1 if no match found.
func (dmp *DiffMatchPatch) MatchMain(text string, pattern string, loc int) int {
	// Check for null inputs not needed since null can't be passed in C#.

	loc = int(math.Max(0, math.Min(float64(loc), float64(len(text)))))
	if text == pattern {
		// Shortcut (potentially not guaranteed by the algorithm)
		return 0
	} else if len(text) == 0 {
		// Nothing to match.
		return -1
	} else if loc+len(pattern) <= len(text) && text[loc:loc+len(pattern)] == pattern {
		// Perfect match at the perfect spot!  (Includes case of null pattern)
		return loc
	}
	// Do a fuzzy compare.
	return dmp.MatchBitap(text, pattern, loc)
}

// MatchBitap locates the best instance of 'pattern' in 'text' near 'loc' using the
// Bitap algorithm.  Returns -1 if no match found.
func (dmp *DiffMatchPatch) MatchBitap(text string, pattern string, loc int) int {
	// Initialise the alphabet.
	s := dmp.MatchAlphabet(pattern)

	// Highest score beyond which we give up.
	var score_threshold float64 = dmp.MatchThreshold
	// Is there a nearby exact match? (speedup)
	best_loc := strings.Index(text, pattern)
	if best_loc != -1 {
		score_threshold = math.Min(dmp.matchBitapScore(0, best_loc, loc,
			pattern), score_threshold)
		// What about in the other direction? (speedup)
		best_loc = strings.LastIndex(text, pattern)
		if best_loc != -1 {
			score_threshold = math.Min(dmp.matchBitapScore(0, best_loc, loc,
				pattern), score_threshold)
		}
	}

	// Initialise the bit arrays.
	matchmask := 1 << uint((len(pattern) - 1))
	best_loc = -1

	var bin_min, bin_mid int
	bin_max := len(pattern) + len(text)
	last_rd := []int{}
	for d := 0; d < len(pattern); d++ {
		// Scan for the best match; each iteration allows for one more error.
		// Run a binary search to determine how far from 'loc' we can stray at
		// this error level.
		bin_min = 0
		bin_mid = bin_max
		for bin_min < bin_mid {
			if dmp.matchBitapScore(d, loc+bin_mid, loc, pattern) <= score_threshold {
				bin_min = bin_mid
			} else {
				bin_max = bin_mid
			}
			bin_mid = (bin_max-bin_min)/2 + bin_min
		}
		// Use the result from this iteration as the maximum for the next.
		bin_max = bin_mid
		start := int(math.Max(1, float64(loc-bin_mid+1)))
		finish := int(math.Min(float64(loc+bin_mid), float64(len(text))) + float64(len(pattern)))

		rd := make([]int, finish+2)
		rd[finish+1] = (1 << uint(d)) - 1

		for j := finish; j >= start; j-- {
			var charMatch int
			if len(text) <= j-1 {
				// Out of range.
				charMatch = 0
			} else if _, ok := s[text[j-1]]; !ok {
				charMatch = 0
			} else {
				charMatch = s[text[j-1]]
			}

			if d == 0 {
				// First pass: exact match.
				rd[j] = ((rd[j+1] << 1) | 1) & charMatch
			} else {
				// Subsequent passes: fuzzy match.
				rd[j] = ((rd[j+1]<<1)|1)&charMatch | (((last_rd[j+1] | last_rd[j]) << 1) | 1) | last_rd[j+1]
			}
			if (rd[j] & matchmask) != 0 {
				score := dmp.matchBitapScore(d, j-1, loc, pattern)
				// This match will almost certainly be better than any existing
				// match.  But check anyway.
				if score <= score_threshold {
					// Told you so.
					score_threshold = score
					best_loc = j - 1
					if best_loc > loc {
						// When passing loc, don't exceed our current distance from loc.
						start = int(math.Max(1, float64(2*loc-best_loc)))
					} else {
						// Already passed loc, downhill from here on in.
						break
					}
				}
			}
		}
		if dmp.matchBitapScore(d+1, loc, loc, pattern) > score_threshold {
			// No hope for a (better) match at greater error levels.
			break
		}
		last_rd = rd
	}
	return best_loc
}

// matchBitapScore computes and returns the score for a match with e errors and x location.
func (dmp *DiffMatchPatch) matchBitapScore(e, x, loc int, pattern string) float64 {
	var accuracy float64 = float64(e) / float64(len(pattern))
	proximity := math.Abs(float64(loc - x))
	if dmp.MatchDistance == 0 {
		// Dodge divide by zero error.
		if proximity == 0 {
			return accuracy
		} else {
			return 1.0
		}
	}
	return accuracy + (proximity / float64(dmp.MatchDistance))
}

// MatchAlphabet initialises the alphabet for the Bitap algorithm.
func (dmp *DiffMatchPatch) MatchAlphabet(pattern string) map[byte]int {
	s := map[byte]int{}
	char_pattern := []byte(pattern)
	for _, c := range char_pattern {
		_, ok := s[c]
		if !ok {
			s[c] = 0
		}
	}
	i := 0

	for _, c := range char_pattern {
		value := s[c] | int(uint(1)<<uint((len(pattern)-i-1)))
		s[c] = value
		i++
	}
	return s
}


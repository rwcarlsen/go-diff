
package diffmatchpatch

import (
	"math"
	"strings"
	"reflect"
	"bytes"
	"strconv"
	"regexp"
	"errors"
	"net/url"
)

// Patch represents one patch operation.
type Patch struct {
	diffs   []Diff
	start1  int
	start2  int
	length1 int
	length2 int
}

// String emulates GNU diff's format.
// Header: @@ -382,8 +481,9 @@
// Indicies are printed as 1-based, not 0-based.
func (patch *Patch) String() string {
	var coords1, coords2 string
	start1 := int64(patch.start1)
	start2 := int64(patch.start2)

	if patch.length1 == 0 {
		coords1 = strconv.FormatInt(start1, 10) + ",0"
	} else if patch.length1 == 1 {
		coords1 = strconv.FormatInt(start1+1, 10)
	} else {
		coords1 = strconv.FormatInt(start1+1, 10) + "," + strconv.FormatInt(int64(patch.length1), 10)
	}

	if patch.length2 == 0 {
		coords2 = strconv.FormatInt(start2, 10) + ",0"
	} else if patch.length2 == 1 {
		coords2 = strconv.FormatInt(start2+1, 10)
	} else {
		coords2 = strconv.FormatInt(start2+1, 10) + "," + strconv.FormatInt(int64(patch.length2), 10)
	}

	var text bytes.Buffer
	text.WriteString("@@ -" + coords1 + " +" + coords2 + " @@\n")

	// Escape the body of the patch with %xx notation.
	for _, aDiff := range patch.diffs {
		switch aDiff.Type {
		case DiffInsert:
			text.WriteString("+")
			break
		case DiffDelete:
			text.WriteString("-")
			break
		case DiffEqual:
			text.WriteString(" ")
			break
		}

		text.WriteString(strings.Replace(url.QueryEscape(aDiff.Text), "+", " ", -1))
		text.WriteString("\n")
	}

	return unescaper.Replace(text.String())
}

// PatchAddContext increases the context until it is unique,
// but doesn't let the pattern expand beyond MatchMaxBits.
func (dmp *DiffMatchPatch) PatchAddContext(patch Patch, text string) Patch {
	if len(text) == 0 {
		return patch
	}

	pattern := text[patch.start2 : patch.start2+patch.length1]
	padding := 0

	// Look for the first and last matches of pattern in text.  If two
	// different matches are found, increase the pattern length.
	for strings.Index(text, pattern) != strings.LastIndex(text, pattern) &&
		len(pattern) < dmp.MatchMaxBits-dmp.PatchMargin-dmp.PatchMargin {
		padding += dmp.PatchMargin
		maxStart := int(math.Max(0, float64(patch.start2-padding)))
		minEnd := int(math.Min(float64(len(text)), float64(patch.start2+patch.length1+padding)))
		pattern = text[maxStart:minEnd]
	}
	// Add one chunk for good luck.
	padding += dmp.PatchMargin

	// Add the prefix.
	prefix := text[int(math.Max(0, float64(patch.start2-padding))):int(patch.start2)]
	if len(prefix) != 0 {
		patch.diffs = append([]Diff{Diff{DiffEqual, prefix}}, patch.diffs...)
	}
	// Add the suffix.
	suffix := text[patch.start2+patch.length1 : int(math.Min(float64(len(text)), float64(patch.start2+patch.length1+padding)))]
	if len(suffix) != 0 {
		patch.diffs = append(patch.diffs, Diff{DiffEqual, suffix})
	}

	// Roll back the start points.
	patch.start1 -= len(prefix)
	patch.start2 -= len(prefix)
	// Extend the lengths.
	patch.length1 += len(prefix) + len(suffix)
	patch.length2 += len(prefix) + len(suffix)

	return patch
}

func (dmp *DiffMatchPatch) PatchMake(opt ...interface{}) []Patch {
	if len(opt) == 1 {
		diffs, _ := opt[0].([]Diff)
		text1 := dmp.DiffText1(diffs)
		return dmp.PatchMake(text1, diffs)
	} else if len(opt) == 2 {
		text1 := opt[0].(string)
		kind := reflect.TypeOf(opt[1]).Name()
		if kind == "string" {
			text2 := opt[1].(string)
			diffs := dmp.DiffMain(text1, text2, true)
			if len(diffs) > 2 {
				diffs = dmp.DiffCleanupSemantic(diffs)
				diffs = dmp.DiffCleanupEfficiency(diffs)
			}
			return dmp.PatchMake(text1, diffs)
		} else if kind == "Diff" {
			return dmp.patchMake2(text1, opt[1].([]Diff))
		}
	} else if len(opt) == 3 {
		return dmp.PatchMake(opt[0], opt[2])
	}
	return []Patch{}
}

// Compute a list of patches to turn text1 into text2.
// text2 is not provided, diffs are the delta between text1 and text2.
func (dmp *DiffMatchPatch) patchMake2(text1 string, diffs []Diff) []Patch {
	// Check for null inputs not needed since null can't be passed in C#.
	patches := []Patch{}
	if len(diffs) == 0 {
		return patches // Get rid of the null case.
	}

	patch := Patch{}
	char_count1 := 0 // Number of characters into the text1 string.
	char_count2 := 0 // Number of characters into the text2 string.
	// Start with text1 (prepatch_text) and apply the diffs until we arrive at
	// text2 (postpatch_text). We recreate the patches one by one to determine
	// context info.
	prepatch_text := text1
	postpatch_text := text1

	for _, aDiff := range diffs {
		if len(patch.diffs) == 0 && aDiff.Type != DiffEqual {
			// A new patch starts here.
			patch.start1 = char_count1
			patch.start2 = char_count2
		}

		switch aDiff.Type {
		case DiffInsert:
			patch.diffs = append(patch.diffs, aDiff)
			patch.length2 += len(aDiff.Text)
			postpatch_text = postpatch_text[0:char_count2] +
				aDiff.Text + postpatch_text[char_count2:]
			break
		case DiffDelete:
			patch.length1 += len(aDiff.Text)
			patch.diffs = append(patch.diffs, aDiff)
			postpatch_text = postpatch_text[0:char_count2] + postpatch_text[char_count2+len(aDiff.Text):]
			break
		case DiffEqual:
			if len(aDiff.Text) <= 2*dmp.PatchMargin &&
				len(patch.diffs) != 0 && aDiff != diffs[len(diffs)-1] {
				// Small equality inside a patch.
				patch.diffs = append(patch.diffs, aDiff)
				patch.length1 += len(aDiff.Text)
				patch.length2 += len(aDiff.Text)
			}

			if len(aDiff.Text) >= 2*dmp.PatchMargin {
				// Time for a new patch.
				if len(patch.diffs) != 0 {
					dmp.PatchAddContext(patch, prepatch_text)
					patches = append(patches, patch)
					patch = Patch{}
					// Unlike Unidiff, our patch lists have a rolling context.
					// http://code.google.com/p/google-diff-match-patch/wiki/Unidiff
					// Update prepatch text & pos to reflect the application of the
					// just completed patch.
					prepatch_text = postpatch_text
					char_count1 = char_count2
				}
			}
			break
		}

		// Update the current character count.
		if aDiff.Type != DiffInsert {
			char_count1 += len(aDiff.Text)
		}
		if aDiff.Type != DiffDelete {
			char_count2 += len(aDiff.Text)
		}
	}
	// Pick up the leftover patch if not empty.
	if len(patch.diffs) != 0 {
		dmp.PatchAddContext(patch, prepatch_text)
		patches = append(patches, patch)
	}

	return patches
}

// PatchDeepCopy returns an array that is identical to a
// given an array of patches.
func (dmp *DiffMatchPatch) PatchDeepCopy(patches []Patch) []Patch {
	patchesCopy := []Patch{}
	for _, aPatch := range patches {
		patchCopy := Patch{}
		for _, aDiff := range aPatch.diffs {
			patchCopy.diffs = append(patchCopy.diffs, Diff{
				aDiff.Type,
				aDiff.Text,
			})
		}
		patchCopy.start1 = aPatch.start1
		patchCopy.start2 = aPatch.start2
		patchCopy.length1 = aPatch.length1
		patchCopy.length2 = aPatch.length2
		patchesCopy = append(patchesCopy, patchCopy)
	}
	return patchesCopy
}

// PatchApply merges a set of patches onto the text.  Returns a patched text, as well
// as an array of true/false values indicating which patches were applied.
func (dmp *DiffMatchPatch) PatchApply(patches []Patch, text string) (string, []bool) {
	if len(patches) == 0 {
		return text, []bool{}
	}

	// Deep copy the patches so that no changes are made to originals.
	patches = dmp.PatchDeepCopy(patches)

	nullPadding := dmp.PatchAddPadding(patches)
	text = nullPadding + text + nullPadding
	dmp.PatchSplitMax(patches)

	x := 0
	// delta keeps track of the offset between the expected and actual
	// location of the previous patch.  If there are patches expected at
	// positions 10 and 20, but the first patch was found at 12, delta is 2
	// and the second patch has an effective expected position of 22.
	delta := 0
	results := []bool{}
	for _, aPatch := range patches {
		expected_loc := aPatch.start2 + delta
		text1 := dmp.DiffText1(aPatch.diffs)
		var start_loc int
		end_loc := -1
		if len(text1) > dmp.MatchMaxBits {
			// PatchSplitMax will only provide an oversized pattern
			// in the case of a monster delete.
			start_loc = dmp.MatchMain(text, text1[0:dmp.MatchMaxBits], expected_loc)
			if start_loc != -1 {
				end_loc = dmp.MatchMain(text,
					text1[len(text1)-dmp.MatchMaxBits:], expected_loc+len(text1)-dmp.MatchMaxBits)
				if end_loc == -1 || start_loc >= end_loc {
					// Can't find valid trailing context.  Drop this patch.
					start_loc = -1
				}
			}
		} else {
			start_loc = dmp.MatchMain(text, text1, expected_loc)
		}
		if start_loc == -1 {
			// No match found.  :(
			results[x] = false
			// Subtract the delta for this failed patch from subsequent patches.
			delta -= aPatch.length2 - aPatch.length1
		} else {
			// Found a match.  :)
			results[x] = true
			delta = start_loc - expected_loc
			var text2 string
			if end_loc == -1 {
				text2 = text[start_loc:int(math.Min(float64(start_loc+len(text1)), float64(len(text))))]
			} else {
				text2 = text[start_loc:int(math.Min(float64(end_loc+dmp.MatchMaxBits), float64(len(text))))]
			}
			if text1 == text2 {
				// Perfect match, just shove the Replacement text in.
				text = text[0:start_loc] + dmp.DiffText2(aPatch.diffs) + text[start_loc+len(text1):]
			} else {
				// Imperfect match.  Run a diff to get a framework of equivalent
				// indices.
				diffs := dmp.DiffMain(text1, text2, false)
				if len(text1) > dmp.MatchMaxBits && float64(dmp.DiffLevenshtein(diffs)/len(text1)) > dmp.PatchDeleteThreshold {
					// The end points match, but the content is unacceptably bad.
					results[x] = false
				} else {
					diffs = dmp.DiffCleanupSemanticLossless(diffs)
					index1 := 0
					for _, aDiff := range aPatch.diffs {
						if aDiff.Type != DiffEqual {
							index2 := dmp.DiffXIndex(diffs, index1)
							if aDiff.Type == DiffInsert {
								// Insertion
								text = text[0:start_loc+index2] + aDiff.Text + text[start_loc+index2:]
							} else if aDiff.Type == DiffDelete {
								// Deletion
								start_index := start_loc + index2
								text = text[0:start_index] +
									text[start_index+dmp.DiffXIndex(diffs, index1+len(aDiff.Text))-index2:]
							}
						}
						if aDiff.Type != DiffDelete {
							index1 += len(aDiff.Text)
						}
					}
				}
			}
		}
		x++
	}
	// Strip the padding off.
	text = text[len(nullPadding) : len(nullPadding)+(len(text)-2*len(nullPadding))]
	return text, results
}

// PatchAddPadding adds some padding on text start and end so that edges can match something.
// Intended to be called only from within patch_apply.
func (dmp *DiffMatchPatch) PatchAddPadding(patches []Patch) string {
	paddingLength := dmp.PatchMargin
	nullPadding := ""
	for x := 1; x <= paddingLength; x++ {
		nullPadding += strconv.FormatInt(int64(x), 10)
	}

	// Bump all the patches forward.
	for _, aPatch := range patches {
		aPatch.start1 += paddingLength
		aPatch.start2 += paddingLength
	}

	// Add some padding on start of first diff.
	patch := patches[0]
	diffs := patch.diffs
	if len(diffs) == 0 || diffs[0].Type != DiffEqual {
		// Add nullPadding equality.
		diffs = append(diffs, Diff{DiffEqual, nullPadding})
		patch.start1 -= paddingLength // Should be 0.
		patch.start2 -= paddingLength // Should be 0.
		patch.length1 += paddingLength
		patch.length2 += paddingLength
	} else if paddingLength > len(diffs[0].Text) {
		// Grow first equality.
		firstDiff := diffs[0]
		extraLength := paddingLength - len(firstDiff.Text)
		firstDiff.Text = nullPadding[len(firstDiff.Text):] + firstDiff.Text
		patch.start1 -= extraLength
		patch.start2 -= extraLength
		patch.length1 += extraLength
		patch.length2 += extraLength
	}

	// Add some padding on end of last diff.
	patch = patches[len(patches)-1]
	diffs = patch.diffs
	if len(diffs) == 0 || diffs[len(diffs)-1].Type != DiffEqual {
		// Add nullPadding equality.
		diffs = append(diffs, Diff{DiffEqual, nullPadding})
		patch.length1 += paddingLength
		patch.length2 += paddingLength
	} else if paddingLength > len(diffs[len(diffs)-1].Text) {
		// Grow last equality.
		lastDiff := diffs[len(diffs)-1]
		extraLength := paddingLength - len(lastDiff.Text)
		lastDiff.Text += nullPadding[0:extraLength]
		patch.length1 += extraLength
		patch.length2 += extraLength
	}

	return nullPadding
}

// PatchSplitMax looks through the patches and breaks up any which are longer than the
// maximum limit of the match algorithm.
// Intended to be called only from within patch_apply.
func (dmp *DiffMatchPatch) PatchSplitMax(patches []Patch) {
	patch_size := dmp.MatchMaxBits
	for x := 0; x < len(patches); x++ {
		if patches[x].length1 <= patch_size {
			continue
		}
		bigpatch := patches[x]
		// Remove the big old patch.
		x = x - 1
		patches = splice_patch(patches, x, 1)
		start1 := bigpatch.start1
		start2 := bigpatch.start2
		precontext := ""
		for len(bigpatch.diffs) != 0 {
			// Create one of several smaller patches.
			patch := Patch{}
			empty := true
			patch.start1 = start1 - len(precontext)
			patch.start2 = start2 - len(precontext)
			if len(precontext) != 0 {
				patch.length1 = len(precontext)
				patch.length2 = len(precontext)
				patch.diffs = append(patch.diffs, Diff{DiffEqual, precontext})
			}
			for len(bigpatch.diffs) != 0 && patch.length1 < patch_size-dmp.PatchMargin {
				diff_type := bigpatch.diffs[0].Type
				diff_text := bigpatch.diffs[0].Text
				if diff_type == DiffInsert {
					// Insertions are harmless.
					patch.length2 += len(diff_text)
					start2 += len(diff_text)
					patch.diffs = append(patch.diffs, bigpatch.diffs[0])
					bigpatch.diffs = append(bigpatch.diffs[:0], bigpatch.diffs[0:]...)
					empty = false
				} else if diff_type == DiffDelete && len(patch.diffs) == 1 && patch.diffs[0].Type == DiffEqual && len(diff_text) > 2*patch_size {
					// This is a large deletion.  Let it pass in one chunk.
					patch.length1 += len(diff_text)
					start1 += len(diff_text)
					empty = false
					patch.diffs = append(patch.diffs, Diff{diff_type, diff_text})
					bigpatch.diffs = append(bigpatch.diffs[:0], bigpatch.diffs[0:]...)
				} else {
					// Deletion or equality.  Only take as much as we can stomach.
					diff_text = diff_text[0:int(math.Min(float64(len(diff_text)),
						float64(patch_size-patch.length1-dmp.PatchMargin)))]

					patch.length1 += len(diff_text)
					start1 += len(diff_text)
					if diff_type == DiffEqual {
						patch.length2 += len(diff_text)
						start2 += len(diff_text)
					} else {
						empty = false
					}
					patch.diffs = append(patch.diffs, Diff{diff_type, diff_text})
					if diff_text == bigpatch.diffs[0].Text {
						bigpatch.diffs = append(bigpatch.diffs[:0], bigpatch.diffs[0:]...)
					} else {
						bigpatch.diffs[0].Text =
							bigpatch.diffs[0].Text[len(diff_text):]
					}
				}
			}
			// Compute the head context for the next patch.
			precontext = dmp.DiffText2(patch.diffs)
			precontext = precontext[int(math.Max(0, float64(len(precontext)-dmp.PatchMargin))):]

			postcontext := ""
			// Append the end context for this patch.
			if len(dmp.DiffText1(bigpatch.diffs)) > dmp.PatchMargin {
				postcontext = dmp.DiffText1(bigpatch.diffs)[0:dmp.PatchMargin]
			} else {
				postcontext = dmp.DiffText1(bigpatch.diffs)
			}

			if len(postcontext) != 0 {
				patch.length1 += len(postcontext)
				patch.length2 += len(postcontext)
				if len(patch.diffs) != 0 && patch.diffs[len(patch.diffs)-1].Type == DiffEqual {
					patch.diffs[len(patch.diffs)-1].Text += postcontext
				} else {
					patch.diffs = append(patch.diffs, Diff{DiffEqual, postcontext})
				}
			}
			if !empty {
				x = x + 1
				splice_patch(patches, x, 0, patch)
			}
		}
	}
}

// PatchToText takes a list of patches and returns a textual representation.
func (dmp *DiffMatchPatch) PatchToText(patches []Patch) string {
	var text bytes.Buffer
	for _, aPatch := range patches {
		text.WriteString(aPatch.String())
	}
	return text.String()
}

// PatchFromText parses a textual representation of patches and returns a List of Patch
// objects.
func (dmp *DiffMatchPatch) PatchFromText(textline string) ([]Patch, error) {
	patches := []Patch{}
	if len(textline) == 0 {
		return patches, nil
	}
	text := strings.Split(textline, "\n")
	textPointer := 0
	patchHeader := regexp.MustCompile("^@@ -(\\d+),?(\\d*) \\+(\\d+),?(\\d*) @@$")

	var patch Patch
	var sign uint8
	var line string
	for textPointer < len(text) {

		if !patchHeader.MatchString(text[textPointer]) {
			return patches, errors.New("Invalid patch string: " + text[textPointer])
		}

		patch = Patch{}
		m := patchHeader.FindStringSubmatch(text[textPointer])

		patch.start1, _ = strconv.Atoi(m[1])
		if len(m[2]) == 0 {
			patch.start1--
			patch.length1 = 1
		} else if m[2] == "0" {
			patch.length1 = 0
		} else {
			patch.start1--
			patch.length1, _ = strconv.Atoi(m[2])
		}

		patch.start2, _ = strconv.Atoi(m[3])

		if len(m[4]) == 0 {
			patch.start2--
			patch.length2 = 1
		} else if m[4] == "0" {
			patch.length2 = 0
		} else {
			patch.start2--
			patch.length2, _ = strconv.Atoi(m[4])
		}
		textPointer++

		for textPointer < len(text) {
			if len(text[textPointer]) > 0 {
				sign = text[textPointer][0]
			} else {
				textPointer++
				continue
			}

			line = text[textPointer][1:]
			line = strings.Replace(line, "+", "%2b", -1)
			line, _ = url.QueryUnescape(line)
			if sign == '-' {
				// Deletion.
				patch.diffs = append(patch.diffs, Diff{DiffDelete, line})
			} else if sign == '+' {
				// Insertion.
				patch.diffs = append(patch.diffs, Diff{DiffInsert, line})
			} else if sign == ' ' {
				// Minor equality.
				patch.diffs = append(patch.diffs, Diff{DiffEqual, line})
			} else if sign == '@' {
				// Start of next patch.
				break
			} else {
				// WTF?
				return patches, errors.New("Invalid patch mode '" + string(sign) + "' in: " + string(line))
			}
			textPointer++
		}

		patches = append(patches, patch)
	}
	return patches, nil
}

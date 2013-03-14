
package diffmatchpatch

import (
	"testing"

	"github.com/stretchrcom/testify/assert"
)

func Test_ToChars(t *testing.T) {

	vals1 := []int{1, 2, 3, 4, 5}
	vals2 := []int{1, 2, 4, 3, 5}

	s1, s2, key := ToChars(vals1, vals2)

	for i, r := range s1 {
		assert.Equal(t, key[r], vals1[i])
		if r == []rune(s2)[i] {
			assert.Equal(t, vals1[i], vals2[i])
		}
	}
	for i, r := range s2 {
		assert.Equal(t, key[r], vals2[i])
	}
}

func Test_FromChars(t *testing.T) {

	vals1 := []int{1, 2, 3, 4, 5}
	vals2 := []int{1, 2, 4, 3, 5}

	s1, s2, key := ToChars(vals1, vals2)

	dmp := New()

	diffs := dmp.DiffMain(s1, s2, true)
	diffVals := []int{}
	ops := FromChars(diffs, &diffVals, key, nil)

	expOps := []int8{
		DiffEqual,
		DiffEqual,
		DiffDelete,
		DiffEqual,
		DiffInsert,
		DiffEqual,
	}
	expVals := []int{1, 2, 3, 4, 3, 5}

	assert.Equal(t, expOps, ops)
	assert.Equal(t, expVals, diffVals)
}

func Test_LinesToChars(t *testing.T) {
	// Convert lines down to characters.
	tmpVector := Key{"", "alpha\n", "beta\n"}

	result0, result1, result2 := LinesToChars("alpha\nbeta\nalpha\n", "beta\nalpha\nbeta\n")
	assert.Equal(t, "\u0001\u0002\u0001", result0)
	assert.Equal(t, "\u0002\u0001\u0002", result1)
	assert.Equal(t, tmpVector, result2)

	tmpVector = Key{"", "alpha\r\n", "beta\r\n", "\r\n"}
	result0, result1, result2 = LinesToChars("", "alpha\r\nbeta\r\n\r\n\r\n")
	assert.Equal(t, "", result0, "")
	assert.Equal(t, "\u0001\u0002\u0003\u0003", result1, "")
	assert.Equal(t, tmpVector, result2)

	tmpVector = Key{"", "a", "b"}
	result0, result1, result2 = LinesToChars("a", "b")
	assert.Equal(t, "\u0001", result0, "")
	assert.Equal(t, "\u0002", result1, "")
	assert.Equal(t, tmpVector, result2)

	// More than 256 to reveal any 8-bit limitations.
	/*
	n := 300
	tmpVector = Key{}
	lineList := []rune{}
	charList := []rune{}

	for x := 1; x < n+1; x++ {
	    tmpVector = append(tmpVector, string(x)+"\n")
	    lineList = append(lineList, rune(x), '\n')
	    charList = append(charList, rune(x))
	}
	assert.Equal(t, n, len(tmpVector), "")

	lines := string(lineList)
	chars := string(charList)
	assert.Equal(t, n, utf8.RuneCountInString(chars), "")
	tmpVector = append(tmpVector, "")

	result0, result1, result2 = LinesToChars(lines, "")

	assert.Equal(t, chars, result0)
	assert.Equal(t, "", result1, "")
	assert.Equal(t, tmpVector, result2)
	*/
}

func Test_CharsToLines(t *testing.T) {
	// Convert chars up to lines.
	diffs := []Diff{
		Diff{DiffEqual, "\u0001\u0002\u0001"},
		Diff{DiffInsert, "\u0002\u0001\u0002"}}

	tmpVector := Key{"", "alpha\n", "beta\n"}
	actual := LinesFromChars(diffs, tmpVector)
	assertDiffEqual(t, []Diff{
		Diff{DiffEqual, "alpha\nbeta\nalpha\n"},
		Diff{DiffInsert, "beta\nalpha\nbeta\n"}}, actual)

	// More than 256 to reveal any 8-bit limitations.
	n := 257
	tmpVector = Key{}
	lineList := []rune{}
	charList := []rune{}

	for x := 1; x <= n; x++ {
		tmpVector = append(tmpVector, string(x)+"\n")
		lineList = append(lineList, rune(x), '\n')
		charList = append(charList, rune(x))
	}

	assert.Equal(t, n, len(tmpVector))
	assert.Equal(t, n, len(charList))

	tmpVector = append(Key{""}, tmpVector...)
	diffs = []Diff{Diff{DiffDelete, string(charList)}}
	actual = LinesFromChars(diffs, tmpVector)
	assertDiffEqual(t, []Diff{
		Diff{DiffDelete, string(lineList)}}, actual)
}


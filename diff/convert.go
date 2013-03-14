
package diffmatchpatch

import (
	"reflect"
	"bytes"
	"strings"
)

type Key []interface{}

// ToChars converts each element in slice1 and slice2 into a rune in s1 and
// s2.  k maps each rune back to its original value and can be used to
// rebuild/recover slice elements from one or more runes.
//
// Equality must be defined for the slice element types.
func ToChars(slice1, slice2 interface{}) (s1, s2 string, k Key) {
	m := map[interface{}]rune{"": 0}
	k = Key{""}
	nextInt := 1

	s1, k, nextInt = makeString(m, k, slice1, nextInt)
	s2, k, nextInt = makeString(m, k, slice2, nextInt)

	return s1, s2, k
}

func makeString(m map[interface{}]rune, k Key, slice interface{}, nextInt int) (s string, key Key, next int) {
	var buf bytes.Buffer
	v := reflect.ValueOf(slice)

	for i := 0; i < v.Len(); i++ {
		item := v.Index(i).Interface()
		r, ok := m[item]
		if !ok {
			r = rune(nextInt)
			k = append(k, item)
			m[item] = r
			nextInt++
		}

		buf.WriteString(string(r))
	}
	return buf.String(), k, nextInt
}

// FromChars populates hydrated with original diff vals prior to ToChars
// conversion and returns the corresponding list of diff operations for
// convenience.  combine is a function combines one or more
// values into a single value of the same type.  It is used to consolidate
// multiple consecutive diffs of the same operation.  If combining is not
// possible for the variable type, pass nil.
func FromChars(diffs []Diff, hydrated interface{}, k Key, combine func(v ...interface{})interface{}) (ops []int8) {
	if t := reflect.TypeOf(hydrated).Kind(); t != reflect.Ptr {
		panic("invalid type for hydrated: " + string(t))
	}
	t := reflect.ValueOf(hydrated).Elem().Type()
	slice := reflect.MakeSlice(t, 0, len(diffs))
	ops = make([]int8, 0, len(diffs))
	for _, d := range diffs {
		items := []interface{}{}
		for _, r := range d.Text {
			items = append(items, k[int(r)])
		}
		if combine != nil {
			ops = append(ops, d.Type)
			val := combine(items...)
			slice = reflect.Append(slice, reflect.ValueOf(val))
		} else {
			for _, item := range items {
				ops = append(ops, d.Type)
				slice = reflect.Append(slice, reflect.ValueOf(item))
			}
		}
	}

	h := reflect.ValueOf(hydrated)
	reflect.Indirect(h).Set(slice)
	return ops
}

// LinesToChars split two texts into a list of strings.  Reduces the texts to a string of
// hashes where each Unicode character represents one line.
func LinesToChars(text1, text2 string) (string, string, Key) {
	s1 := strings.SplitAfter(text1, "\n")
	s2 := strings.SplitAfter(text2, "\n")
	if end := len(s1)-1; len(s1[end]) == 0 {
		s1 = s1[:end]
	}
	if end := len(s2)-1; len(s2[end]) == 0 {
		s2 = s2[:end]
	}
	return ToChars(s1, s2)
}

// LinesFromChars rehydrates the text in a diff from a string of line hashes to real lines of
// text.
func LinesFromChars(diffs []Diff, k Key) []Diff {
	lines := []string{}
	FromChars(diffs, &lines, k, combineString)
	hydrated := make([]Diff, len(diffs))
	for i, d := range diffs {
		d.Text = lines[i]
		hydrated[i] = d
	}
	return hydrated
}

func combineString(vals ...interface{}) interface{} {
	var buf bytes.Buffer
	for _, v := range vals {
		buf.WriteString(v.(string))
	}
	return buf.String()
}


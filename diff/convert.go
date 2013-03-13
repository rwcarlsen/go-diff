
package diffmatchpatch

import (
	"reflect"
	"bytes"
)

type Key map[rune]interface{}

// ToRunes converts each element in slice1 and slice2 into a rune in s1 and
// s2.  k maps each rune back to its original value and can be used to
// hydrate or recover slice elements from one or more runes.
func ToRunes(slice1, slice2 interface{}) (s1, s2 string, k Key) {
	var buf1, buf2 bytes.Buffer
	m := map[interface{}]rune{}
	k = Key{}

	v1 := reflect.ValueOf(slice1)
	v2 := reflect.ValueOf(slice2)

	nextInt := 0
	for i := 0; i < v1.Len(); i++ {
		item := v1.Index(i).Interface()
		r, ok := m[item]
		if !ok {
			r = rune(nextInt)
			k[r] = item
			m[item] = r
			nextInt++
		}

		buf1.WriteString(string(r))
	}

	for i := 0; i < v2.Len(); i++ {
		item := v2.Index(i).Interface()
		r, ok := m[item]
		if !ok {
			r = rune(nextInt)
			k[r] = item
			m[item] = r
			nextInt++
		}

		buf2.WriteString(string(r))
	}
	return buf1.String(), buf2.String(), k
}

func Hydrate(diffs []Diff, hydrated interface{}, k Key, combine func(v ...interface{})interface{}) (ops []int8) {
	if t := reflect.TypeOf(hydrated).Kind(); t != reflect.Ptr {
		panic("invalid type for hydrated: " + string(t))
	}
	t := reflect.TypeOf(reflect.ValueOf(hydrated).Elem())
	slice := reflect.MakeSlice(t, 0, len(diffs))
	ops = make([]int8, 0, len(diffs))
	for _, d := range diffs {
		items := []interface{}{}
		for _, r := range d.Text {
			items = append(items, k[r])
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



package diffmatchpatch

import (
	"fmt"
	"testing"

	"github.com/stretchrcom/testify/assert"
)

func Test_ToRunes(t *testing.T) {

	vals1 := []int{1, 2, 3, 4, 5}
	vals2 := []int{1, 2, 4, 3, 5}

	s1, s2, key := ToRunes(vals1, vals2)

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

func Test_Hydrate(t *testing.T) {

	vals1 := []int{1, 2, 3, 4, 5}
	vals2 := []int{1, 2, 4, 3, 5}

	s1, s2, key := ToRunes(vals1, vals2)

	dmp := New()

	diffs := dmp.DiffMain(s1, s2, true)
	converted := []int{}
	ops := Hydrate(diffs, &converted, key, nil)

	for i, op := range ops {
		fmt.Printf("op=%v, val=%v", op, converted[i])
	}
}

// Public Domain (-) 2010-present, The Web4 Authors.
// See the Web4 UNLICENSE file for details.

package retry

import (
	"testing"
	"time"
)

func TestIterator(t *testing.T) {
	retry := MustBuild(Policy{
		BackoffFactor: 1.5,
		MinInterval:   time.Millisecond,
		MaxInterval:   3 * time.Millisecond,
		MaxIterations: 10,
		TotalLimit:    10 * time.Millisecond,
	})
	count := 0
	it := retry.Iter()
	for it.Next() {
		count++
	}
	it = retry.Iter()
	for it.Next() {
		count++
	}
	want := len(retry) * 2
	if count != want {
		t.Fatalf("unexpected retry count: got %d, want %d", count, want)
	}
}

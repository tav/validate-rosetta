// Public Domain (-) 2010-present, The Web4 Authors.
// See the Web4 UNLICENSE file for details.

// Package retry provides support for retry with backoff.
package retry

import (
	"fmt"
	"time"
)

// Default is the default retry Handler. It keeps trying up to 5 times without
// any delays.
var Default = MustBuild(Policy{
	MaxIterations: 5,
})

// Never is a retry Handler that will never retry, i.e. will only run once.
var Never = MustBuild(Policy{
	MaxIterations: 1,
})

// Handler encapsulates a retry policy. Each element specifies the time interval
// before the next call. The first interval is always zero, so as to not cause
// any delays before the very first attempt.
type Handler []time.Duration

// Iter returns an Iterator for the retry Handler.
func (h Handler) Iter() Iterator {
	return Iterator{h}
}

// Iterator forms the core API of the retry mechanism. Callers should call Next
// in a for loop, and exit the loop on success.
type Iterator struct {
	h Handler
}

// Next advances the Iterator by one.
func (i *Iterator) Next() bool {
	if len(i.h) == 0 {
		return false
	}
	d := i.h[0]
	i.h = i.h[1:]
	if d == 0 {
		return true
	}
	time.Sleep(d)
	return true
}

// Policy specifies the constraints for creating a retry Handler.
type Policy struct {
	// BackoffFactor defines the backoff between each retry iteration. If
	// specified, this value must be greater than or equal to 1.0, otherwise it
	// defaults to 1.0. For exponential backoff, set this to 2.0.
	BackoffFactor float64
	// DisableJitter turns off the automatic addition of jitter into the retry
	// intervals.
	DisableJitter bool
	// MaxInterval defines the maximum interval duration. If specified, this
	// must be greater than or equal to the MinInterval value.
	MaxInterval time.Duration
	// MaxIterations defines the maxinum number of iterations for a retry
	// Handler. At least one of MaxIterations and TotalLimit must be specified.
	MaxIterations uint
	// MinInterval defines the starting interval duration. If specified, this
	// must be greater than or equal to zero.
	MinInterval time.Duration
	// TotalLimit defines the total limit for the various intervals of a retry
	// Handler. At least one of MaxIterations and TotalLimit must be specified.
	TotalLimit time.Duration
}

// Build creates a retry Handler from the given Policy.
func Build(p Policy) (Handler, error) {
	if p.MaxIterations == 0 && p.TotalLimit == 0 {
		return Handler{}, fmt.Errorf(
			"retry: cannot have both MaxIterations and TotalLimit unspecified",
		)
	}
	if p.BackoffFactor == 0 {
		p.BackoffFactor = 1.0
	} else if p.BackoffFactor < 1.0 {
		return Handler{}, fmt.Errorf(
			"retry: BackoffFactor must be greater than or equal to 1.0, not %v",
			p.BackoffFactor,
		)
	}
	if p.MaxInterval < p.MinInterval {
		return Handler{}, fmt.Errorf(
			"retry: MaxInterval (%s) must be greater than or equal to MinInterval (%s)",
			p.MaxInterval, p.MinInterval,
		)
	}
	if p.MinInterval < 0 {
		return Handler{}, fmt.Errorf(
			"retry: MinInterval must be greater than or equal to zero: %s", p.MinInterval,
		)
	}
	if p.TotalLimit != 0 && p.TotalLimit < 0 {
		return Handler{}, fmt.Errorf(
			"retry: TotalLimit cannot be negative: %s", p.TotalLimit,
		)
	}
	h := Handler{0}
	ival := p.MinInterval
	total := time.Duration(0)
	for {
		if p.MaxIterations > 0 && uint(len(h)) == p.MaxIterations {
			break
		}
		if len(h) == 1 {
			total = ival
		} else {
			ival = time.Duration(float64(ival) * p.BackoffFactor)
			if ival > p.MaxInterval {
				ival = p.MaxInterval
			}
			total += ival
		}
		if p.TotalLimit > 0 && total > p.TotalLimit {
			break
		}
		h = append(h, ival)
	}
	return h, nil
}

// MustBuild creates a retry Handler from the given Policy. It panics if an
// invalid Policy causes an error.
func MustBuild(p Policy) Handler {
	h, err := Build(p)
	if err != nil {
		panic(err)
	}
	return h
}

// Copyright 2021 Coinbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package retry

import (
	"testing"
	"time"

	"github.com/cenkalti/backoff"
)

var (
	result int
)

// Copied from rosetta-sdk-go/fetcher.
type Backoff struct {
	backoff  backoff.BackOff
	attempts int
}

func BenchmarkNew(b *testing.B) {
	count := 0
	retry := MustBuild(Policy{
		MaxInterval:   time.Nanosecond,
		MinInterval:   time.Nanosecond,
		MaxIterations: 6,
		TotalLimit:    time.Minute,
	})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		it := retry.Iter()
		for it.Next() {
			count++
		}
	}
	result = count
}

func BenchmarkOld(b *testing.B) {
	count := 0
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		backoffRetries := backoffRetries(time.Minute, 5)
		count = 0
		for {
			count++
			if !tryAgain(backoffRetries) {
				break
			}
		}
	}
	result = count
}

// Adapted from rosetta-sdk-go/fetcher.
func backoffRetries(maxElapsedTime time.Duration, maxRetries uint64) *Backoff {
	exponentialBackoff := &backoff.ExponentialBackOff{
		Clock:           backoff.SystemClock,
		InitialInterval: time.Nanosecond,
		MaxElapsedTime:  maxElapsedTime,
		MaxInterval:     time.Nanosecond,
		Multiplier:      1,
	}
	exponentialBackoff.Reset()
	return &Backoff{backoff: backoff.WithMaxRetries(exponentialBackoff, maxRetries)}
}

// Adapted from rosetta-sdk-go/fetcher.
func tryAgain(thisBackoff *Backoff) bool {
	nextBackoff := thisBackoff.backoff.NextBackOff()
	if nextBackoff == backoff.Stop {
		return false
	}
	thisBackoff.attempts++
	time.Sleep(nextBackoff)
	return true
}

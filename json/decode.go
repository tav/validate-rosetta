// Adapted from https://github.com/goccy/go-json
//
// MIT License
//
// Copyright (c) 2020 Masaaki Goshima
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package json

import (
	"io"
)

// Decoder provides support for decoding JSON data.
//
// To use, first use one of the ResetFrom* methods to set the data to decode,
// and then pass the Decoder as a parameter into an API value's DecodeJSON
// method.
type Decoder struct {
	buf    []byte
	cursor int
	start  int
}

// ResetFromBytes will reset the Decoder's buffer and copy the given data into
// it.
func (d *Decoder) ResetFromBytes(data []byte) {
	l := len(data)
	if cap(d.buf) < (l + 1) {
		d.buf = make([]byte, l+1)
	} else {
		d.buf = d.buf[:l+1]
	}
	copy(d.buf, data)
	d.buf[l] = 0
	d.cursor = 0
}

// ResetFromReadCloser will reset the Decoder's buffer, and attempt to fill it by
// reading everything from the given Reader. The Reader will be closed when the
// method exits.
//
// This method has been adapted from Go's io.ReadAll function.
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the Go LICENSE file.
func (d *Decoder) ResetFromReadCloser(r io.ReadCloser) error {
	b := d.buf[:0]
	for {
		if len(b) == cap(b) {
			// Add more capacity (let append pick how much).
			b = append(b, 0)[:len(b)]
		}
		n, err := r.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err == io.EOF {
				// NOTE(tav): We append a null byte at the end, so as to make
				// decoding JSON faster.
				b = append(b, 0)
				err = nil
			}
			r.Close()
			d.buf = b
			d.cursor = 0
			return err
		}
	}
}

// NewDecoder instantiates a fresh Decoder.
func NewDecoder() *Decoder {
	return &Decoder{
		buf: make([]byte, 0, 1024),
	}
}

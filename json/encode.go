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

// Package json provides support for encoding/decoding JSON data.
package json

import (
	"math"
	"math/bits"
	"reflect"
	"strconv"
	"unicode/utf8"
	"unsafe"
)

const (
	hex = "0123456789abcdef"
	lsb = 0x0101010101010101
	msb = 0x8080808080808080
)

// "00010203...96979899" cast to []uint16
var intBELookup = [100]uint16{
	0x3030, 0x3031, 0x3032, 0x3033, 0x3034, 0x3035, 0x3036, 0x3037, 0x3038, 0x3039,
	0x3130, 0x3131, 0x3132, 0x3133, 0x3134, 0x3135, 0x3136, 0x3137, 0x3138, 0x3139,
	0x3230, 0x3231, 0x3232, 0x3233, 0x3234, 0x3235, 0x3236, 0x3237, 0x3238, 0x3239,
	0x3330, 0x3331, 0x3332, 0x3333, 0x3334, 0x3335, 0x3336, 0x3337, 0x3338, 0x3339,
	0x3430, 0x3431, 0x3432, 0x3433, 0x3434, 0x3435, 0x3436, 0x3437, 0x3438, 0x3439,
	0x3530, 0x3531, 0x3532, 0x3533, 0x3534, 0x3535, 0x3536, 0x3537, 0x3538, 0x3539,
	0x3630, 0x3631, 0x3632, 0x3633, 0x3634, 0x3635, 0x3636, 0x3637, 0x3638, 0x3639,
	0x3730, 0x3731, 0x3732, 0x3733, 0x3734, 0x3735, 0x3736, 0x3737, 0x3738, 0x3739,
	0x3830, 0x3831, 0x3832, 0x3833, 0x3834, 0x3835, 0x3836, 0x3837, 0x3838, 0x3839,
	0x3930, 0x3931, 0x3932, 0x3933, 0x3934, 0x3935, 0x3936, 0x3937, 0x3938, 0x3939,
}

var intLELookup = [100]uint16{
	0x3030, 0x3130, 0x3230, 0x3330, 0x3430, 0x3530, 0x3630, 0x3730, 0x3830, 0x3930,
	0x3031, 0x3131, 0x3231, 0x3331, 0x3431, 0x3531, 0x3631, 0x3731, 0x3831, 0x3931,
	0x3032, 0x3132, 0x3232, 0x3332, 0x3432, 0x3532, 0x3632, 0x3732, 0x3832, 0x3932,
	0x3033, 0x3133, 0x3233, 0x3333, 0x3433, 0x3533, 0x3633, 0x3733, 0x3833, 0x3933,
	0x3034, 0x3134, 0x3234, 0x3334, 0x3434, 0x3534, 0x3634, 0x3734, 0x3834, 0x3934,
	0x3035, 0x3135, 0x3235, 0x3335, 0x3435, 0x3535, 0x3635, 0x3735, 0x3835, 0x3935,
	0x3036, 0x3136, 0x3236, 0x3336, 0x3436, 0x3536, 0x3636, 0x3736, 0x3836, 0x3936,
	0x3037, 0x3137, 0x3237, 0x3337, 0x3437, 0x3537, 0x3637, 0x3737, 0x3837, 0x3937,
	0x3038, 0x3138, 0x3238, 0x3338, 0x3438, 0x3538, 0x3638, 0x3738, 0x3838, 0x3938,
	0x3039, 0x3139, 0x3239, 0x3339, 0x3439, 0x3539, 0x3639, 0x3739, 0x3839, 0x3939,
}

var intLookup [100]uint16

var needEscape = [256]bool{
	'"':  true,
	'\\': true,
	0x00: true,
	0x01: true,
	0x02: true,
	0x03: true,
	0x04: true,
	0x05: true,
	0x06: true,
	0x07: true,
	0x08: true,
	0x09: true,
	0x0a: true,
	0x0b: true,
	0x0c: true,
	0x0d: true,
	0x0e: true,
	0x0f: true,
	0x10: true,
	0x11: true,
	0x12: true,
	0x13: true,
	0x14: true,
	0x15: true,
	0x16: true,
	0x17: true,
	0x18: true,
	0x19: true,
	0x1a: true,
	0x1b: true,
	0x1c: true,
	0x1d: true,
	0x1e: true,
	0x1f: true,
	/* 0x20 - 0x7f */
	0x80: true,
	0x81: true,
	0x82: true,
	0x83: true,
	0x84: true,
	0x85: true,
	0x86: true,
	0x87: true,
	0x88: true,
	0x89: true,
	0x8a: true,
	0x8b: true,
	0x8c: true,
	0x8d: true,
	0x8e: true,
	0x8f: true,
	0x90: true,
	0x91: true,
	0x92: true,
	0x93: true,
	0x94: true,
	0x95: true,
	0x96: true,
	0x97: true,
	0x98: true,
	0x99: true,
	0x9a: true,
	0x9b: true,
	0x9c: true,
	0x9d: true,
	0x9e: true,
	0x9f: true,
	0xa0: true,
	0xa1: true,
	0xa2: true,
	0xa3: true,
	0xa4: true,
	0xa5: true,
	0xa6: true,
	0xa7: true,
	0xa8: true,
	0xa9: true,
	0xaa: true,
	0xab: true,
	0xac: true,
	0xad: true,
	0xae: true,
	0xaf: true,
	0xb0: true,
	0xb1: true,
	0xb2: true,
	0xb3: true,
	0xb4: true,
	0xb5: true,
	0xb6: true,
	0xb7: true,
	0xb8: true,
	0xb9: true,
	0xba: true,
	0xbb: true,
	0xbc: true,
	0xbd: true,
	0xbe: true,
	0xbf: true,
	0xc0: true,
	0xc1: true,
	0xc2: true,
	0xc3: true,
	0xc4: true,
	0xc5: true,
	0xc6: true,
	0xc7: true,
	0xc8: true,
	0xc9: true,
	0xca: true,
	0xcb: true,
	0xcc: true,
	0xcd: true,
	0xce: true,
	0xcf: true,
	0xd0: true,
	0xd1: true,
	0xd2: true,
	0xd3: true,
	0xd4: true,
	0xd5: true,
	0xd6: true,
	0xd7: true,
	0xd8: true,
	0xd9: true,
	0xda: true,
	0xdb: true,
	0xdc: true,
	0xdd: true,
	0xde: true,
	0xdf: true,
	0xe0: true,
	0xe1: true,
	0xe2: true,
	0xe3: true,
	0xe4: true,
	0xe5: true,
	0xe6: true,
	0xe7: true,
	0xe8: true,
	0xe9: true,
	0xea: true,
	0xeb: true,
	0xec: true,
	0xed: true,
	0xee: true,
	0xef: true,
	0xf0: true,
	0xf1: true,
	0xf2: true,
	0xf3: true,
	0xf4: true,
	0xf5: true,
	0xf6: true,
	0xf7: true,
	0xf8: true,
	0xf9: true,
	0xfa: true,
	0xfb: true,
	0xfc: true,
	0xfd: true,
	0xfe: true,
	0xff: true,
}

// AppendBool appends the given bool value as JSON.
func AppendBool(buf []byte, v bool) []byte {
	if v {
		return append(buf, "true"...)
	}
	return append(buf, "false"...)
}

// AppendFloat appends the given float value as JSON.
func AppendFloat(buf []byte, v float64) []byte {
	abs := math.Abs(v)
	fmt := byte('f')
	if abs != 0 {
		if abs < 1e-6 || abs >= 1e21 {
			fmt = 'e'
		}
	}
	return strconv.AppendFloat(buf, v, fmt, -1, 64)
}

// AppendHexBytes appends the given byte slice as a hex-encoded JSON string.
func AppendHexBytes(buf []byte, v []byte) []byte {
	buf = append(buf, `"`...)
	for _, c := range v {
		buf = append(buf, hex[c>>4], hex[c&0x0f])
	}
	return append(buf, `"`...)
}

// AppendKey appends the given value as an encoded JSON key. The key must be
// comprised of printable ASCII characters only, i.e. not need any escaping.
func AppendKey(buf []byte, k string) []byte {
	buf = append(buf, `"`...)
	buf = append(buf, k...)
	return append(buf, `":`...)
}

// AppendInt appends the given signed integer as JSON.
func AppendInt(buf []byte, n int64) []byte {
	negative := false
	if n < 0 {
		negative = true
		n = -n
	} else {
		if n < 10 {
			return append(buf, byte(n+'0'))
		} else if n < 100 {
			u := intLELookup[n]
			return append(buf, byte(u), byte(u>>8))
		}
	}

	var b [22]byte
	u := (*[11]uint16)(unsafe.Pointer(&b))
	i := 11

	for n >= 100 {
		j := n % 100
		n /= 100
		i--
		u[i] = intLookup[j]
	}

	i--
	u[i] = intLookup[n]

	i *= 2 // convert to byte index
	if n < 10 {
		i++ // remove leading zero
	}

	if negative {
		i--
		b[i] = '-'
	}
	return append(buf, b[i:]...)
}

// AppendNull appends a null JSON value.
func AppendNull(buf []byte) []byte {
	return append(buf, "null"...)
}

// AppendString appends the given string as JSON.
func AppendString(buf []byte, s string) []byte {
	valLen := len(s)
	if valLen == 0 {
		return append(buf, `""`...)
	}
	buf = append(buf, `"`...)
	var escapeIdx int
	if valLen >= 8 {
		if escapeIdx = escapeIndex(s); escapeIdx < 0 {
			return append(append(buf, s...), `"`...)
		}
	}

	i := 0
	j := escapeIdx
	for j < valLen {
		c := s[j]

		if c >= 0x20 && c <= 0x7f && c != '\\' && c != '"' {
			// Fast path: most of the time, printable ascii characters are used.
			j++
			continue
		}

		switch c {
		case '\\', '"':
			buf = append(buf, s[i:j]...)
			buf = append(buf, '\\', c)
			i = j + 1
			j = j + 1
			continue

		case '\n':
			buf = append(buf, s[i:j]...)
			buf = append(buf, '\\', 'n')
			i = j + 1
			j = j + 1
			continue

		case '\r':
			buf = append(buf, s[i:j]...)
			buf = append(buf, '\\', 'r')
			i = j + 1
			j = j + 1
			continue

		case '\t':
			buf = append(buf, s[i:j]...)
			buf = append(buf, '\\', 't')
			i = j + 1
			j = j + 1
			continue

		case '<', '>', '&':
			buf = append(buf, s[i:j]...)
			buf = append(buf, `\u00`...)
			buf = append(buf, hex[c>>4], hex[c&0x0f])
			i = j + 1
			j = j + 1
			continue
		}

		// This encodes bytes < 0x20 except for \t, \n and \r.
		if c < 0x20 {
			buf = append(buf, s[i:j]...)
			buf = append(buf, `\u00`...)
			buf = append(buf, hex[c>>4], hex[c&0x0f])
			i = j + 1
			j = j + 1
			continue
		}

		r, size := utf8.DecodeRuneInString(s[j:])

		if r == utf8.RuneError && size == 1 {
			buf = append(buf, s[i:j]...)
			buf = append(buf, `\ufffd`...)
			i = j + size
			j = j + size
			continue
		}

		switch r {
		case '\u2028', '\u2029':
			// U+2028 is LINE SEPARATOR.
			// U+2029 is PARAGRAPH SEPARATOR.
			// They are both technically valid characters in JSON strings,
			// but don't work in JSONP, which has to be evaluated as JavaScript,
			// and can lead to security holes there. It is valid JSON to
			// escape them, so we do so unconditionally.
			// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
			buf = append(buf, s[i:j]...)
			buf = append(buf, `\u202`...)
			buf = append(buf, hex[r&0x0f])
			i = j + size
			j = j + size
			continue
		}

		j += size
	}

	return append(append(buf, s[i:]...), `"`...)
}

// AppendUint appends the given unsigned integer as JSON.
func AppendUint(buf []byte, n uint64) []byte {
	if n < 10 {
		return append(buf, byte(n+'0'))
	} else if n < 100 {
		u := intLELookup[n]
		return append(buf, byte(u), byte(u>>8))
	}

	var b [22]byte
	u := (*[11]uint16)(unsafe.Pointer(&b))
	i := 11

	for n >= 100 {
		j := n % 100
		n /= 100
		i--
		u[i] = intLookup[j]
	}

	i--
	u[i] = intLookup[n]

	i *= 2 // convert to byte index
	if n < 10 {
		i++ // remove leading zero
	}
	return append(buf, b[i:]...)
}

// below return a mask that can be used to determine if any of the bytes in `n`
// are below `b`. If a byte's MSB is set in the mask then that byte was below
// `b`. The result is only valid if `b`, and each byte in `n`, is below 0x80.
func below(n uint64, b byte) uint64 {
	return n - expand(b)
}

// contains returns a mask that can be used to determine if any of the bytes in
// `n` are equal to `b`. If a byte's MSB is set in the mask then that byte is
// equal to `b`. The result is only valid if `b`, and each byte in `n`, is below
// 0x80.
func contains(n uint64, b byte) uint64 {
	return (n ^ expand(b)) - lsb
}

// escapeIndex finds the index of the first char in `s` that requires escaping.
// A char requires escaping if it's outside of the range of [0x20, 0x7f] or if
// it includes a double quote or backslash. If no chars in `s` require escaping,
// the return value is -1.
func escapeIndex(s string) int {
	chunks := stringToUint64Slice(s)
	for _, n := range chunks {
		// Combine masks before checking for the MSB of each byte. We include
		// `n` in the mask to check whether any of the *input* byte MSBs were
		// set (i.e. the byte was outside the ASCII range).
		mask := n | below(n, 0x20) | contains(n, '"') | contains(n, '\\')
		if (mask & msb) != 0 {
			return bits.TrailingZeros64(mask&msb) / 8
		}
	}
	valLen := len(s)
	for i := len(chunks) * 8; i < valLen; i++ {
		if needEscape[s[i]] {
			return i
		}
	}
	return -1
}

// expand puts the specified byte into each of the 8 bytes of a uint64.
func expand(b byte) uint64 {
	return lsb * uint64(b)
}

//nolint:govet
func stringToUint64Slice(s string) []uint64 {
	return *(*[]uint64)(unsafe.Pointer(&reflect.SliceHeader{
		Cap:  len(s) / 8,
		Data: ((*reflect.StringHeader)(unsafe.Pointer(&s))).Data,
		Len:  len(s) / 8,
	}))
}

func init() {
	var b [2]byte
	*(*uint16)(unsafe.Pointer(&b)) = uint16(0xabcd)

	switch b[0] {
	case 0xcd: // LE
		intLookup = intLELookup
	case 0xab: // BE
		intLookup = intBELookup
	default:
		panic("json: could not determine endianness")
	}
}

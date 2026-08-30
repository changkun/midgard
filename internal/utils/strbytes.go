// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package utils

import "unsafe"

// StringToBytes converts string to byte slice without a memory allocation.
//
// The returned slice aliases the string's backing array. Writing to it is
// undefined behavior.
func StringToBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// BytesToString converts byte slice to string without a memory allocation.
//
// The returned string aliases b. Mutating b after the conversion changes
// the string's contents, which is undefined behavior.
func BytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

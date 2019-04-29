// Copyright (c) 2019 IoTeX
// This program is free software: you can redistribute it and/or modify it under the terms of the
// GNU General Public License as published by the Free Software Foundation, either version 3 of
// the License, or (at your option) any later version.
// This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; 
// without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See
// the GNU General Public License for more details.
// You should have received a copy of the GNU General Public License along with this program. If
// not, see <http://www.gnu.org/licenses/>.

package util

import (
	"encoding/binary"
)

func Uint64ToBytes(u uint64) []byte {
	retval := make([]byte, 8)
	binary.LittleEndian.PutUint64(retval, u)

	return retval
}

func BytesToUint64(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b)
}

func CopyBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)

	return c
}

// Copyright 2018 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package fdlimit

import "errors"

// Raise tries to maximize the file descriptor allowance of this process
// to the maximum hard-limit allowed by the OS.
func Raise(max uint64) error {
	// This method is NOP by design:
	//  * Linux/Darwin counterparts need to manually increase per process limits
	//  * On Windows Go uses the CreateFile API, which is limited to 16K files, non
	//    changeable from within a running process
	// This way we can always "request" raising the limits, which will either have
	// or not have effect based on the platform we're running on.
	if max > 16384 {
		return errors.New("file descriptor limit (16384) reached")
	}
	return nil
}

// Current retrieves the number of file descriptors allowed to be opened by this
// process.
func Current() (int, error) {
	// Please see Raise for the reason why we use hard coded 16K as the limit
	return 16384, nil
}

// Maximum retrieves the maximum number of file descriptors this process is
// allowed to request for itself.
func Maximum() (int, error) {
	return Current()
}

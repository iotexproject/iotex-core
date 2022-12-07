// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/pkg/errors"
)

// Errors
var (
	ErrInvalidCanName = errors.New("invalid candidate name")
)

// IsValidCandidateName check if a candidate name string is valid.
func IsValidCandidateName(s string) bool {
	if len(s) == 0 || len(s) > 12 {
		return false
	}
	for _, c := range s {
		if !(('a' <= c && c <= 'z') || ('0' <= c && c <= '9')) {
			return false
		}
	}
	return true
}

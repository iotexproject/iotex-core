// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import "github.com/pkg/errors"

func ExemptError(chainID uint32, height uint64, act Action, err error) bool {
	if chainID == 1 {
		switch act.(type) {
		// TODO: add transfer legacy address exemption here
		case *CandidateUpdate:
			if height == 21910804 && errors.Cause(err) == ErrInvalidCanName {
				return true
			}
		default:
			return false
		}
	}
	return false
}

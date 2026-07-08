// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// init() in candidate_deactivate.go panics if confirmCandidateDeactivation is
// missing from the staking ABI; this asserts the method ID is non-zero so a
// future refactor that drops the JSON entry will fail loudly.
func TestCandidateDeactivate_ConfirmMethodLoaded(t *testing.T) {
	r := require.New(t)
	r.NotEmpty(confirmCandidateDeactivationMethod.ID)
	r.NotEqual(requestCandidateDeactivationMethod.ID, confirmCandidateDeactivationMethod.ID)
	r.NotEqual(cancelCandidateDeactivationMethod.ID, confirmCandidateDeactivationMethod.ID)
}

// EthData(op=Confirm) used to silently produce empty bytes because the method
// ID was the zero value. With the method loaded we expect a real 4-byte
// selector that round-trips through the ABI binary parser.
func TestCandidateDeactivate_ConfirmEthDataRoundTrip(t *testing.T) {
	r := require.New(t)
	cd := &CandidateDeactivate{op: CandidateDeactivateOpConfirm}
	data, err := cd.EthData()
	r.NoError(err)
	r.True(len(data) >= 4, "EthData should at least contain the 4-byte selector")
	r.True(bytes.Equal(data[:4], confirmCandidateDeactivationMethod.ID))

	cd2, err := NewCandidateDeactivateFromABIBinary(data)
	r.NoError(err)
	r.Equal(CandidateDeactivateOp(CandidateDeactivateOpConfirm), cd2.op)
}

// Same round trip for the Request op so this file documents both ABI selectors.
func TestCandidateDeactivate_RequestEthDataRoundTrip(t *testing.T) {
	r := require.New(t)
	cd := &CandidateDeactivate{op: CandidateDeactivateOpRequest}
	data, err := cd.EthData()
	r.NoError(err)
	r.True(bytes.Equal(data[:4], requestCandidateDeactivationMethod.ID))

	cd2, err := NewCandidateDeactivateFromABIBinary(data)
	r.NoError(err)
	r.Equal(CandidateDeactivateOp(CandidateDeactivateOpRequest), cd2.op)
}

// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package systemcontracts

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestCandidateStorage(t *testing.T) {
	r := require.New(t)
	outputStr := `0x000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000ddfc506136fb7c050cc2e9511eccd81b15e742600000000000000000000000000000000000000000004ec87c8a45ecae2f659220000000000000000000000000ddfc506136fb7c050cc2e9511eccd81b15e7426626f743100000000000000000000000000000000000000000000000000000000`
	output, err := hex.DecodeString(outputStr[2:])
	r.NoError(err, "failed to decode output hex string")

	parsedABI, err := abi.JSON(strings.NewReader(CandidateStorageABI))
	r.NoError(err, "failed to parse ABI")

	var candidates []SolidityCandidate
	err = parsedABI.UnpackIntoInterface(&candidates, "getCandidateList", output)
	r.NoError(err, "failed to unpack candidates from output")
	for _, candidate := range candidates {
		t.Logf("Candidate Address: %s, Votes: %s, Reward Address: %s, Name: %s",
			candidate.CandidateAddress.Hex(),
			candidate.Votes.String(),
			candidate.RewardAddress.Hex(),
			string(candidate.CanName[:]),
		)
	}
	r.Equal(1, len(candidates), "expected one candidate")
}

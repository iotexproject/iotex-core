// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
)

type (
	typedCandidateRegister interface {
		OwnerAddress() address.Address
		OperatorAddress() address.Address
		RewardAddress() address.Address
		Amount() *big.Int
		Duration() uint32
		AutoStake() bool
		CandidateType() uint32
		// ExtraData returns the customised data for the candidate
		ExtraData() []byte
	}

	// CandidateType is the type of candidate
	CandidateType uint32

	// TypedCandidate is the struct of a candidate with type
	TypedCandidate struct {
		Owner              address.Address
		Operator           address.Address
		Reward             address.Address
		Type               CandidateType
		SelfStakeBucketIdx uint64
		// extraData is the customised data for the candidate
		extraData []byte
	}
)

func candidateExtra[T interface{ Deserilized([]byte) error }](c *TypedCandidate, extra T) error {
	return extra.Deserilized(c.extraData)
}

// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestCheckActionAddress(t *testing.T) {
	r := require.New(t)
	ok := identityset.Address(0).String()

	cases := []struct {
		field string
		build func(addr string) (Action, error)
	}{
		{"Transfer.recipient", func(a string) (Action, error) {
			return NewTransfer(big.NewInt(0), a, nil), nil
		}},
		{"Execution.contract", func(a string) (Action, error) {
			return NewExecution(a, big.NewInt(0), nil), nil
		}},
		{"TransferStake.voterAddress", func(a string) (Action, error) {
			return NewTransferStake(a, 1, nil)
		}},
		{"CandidateTransferOwnership.newOwner", func(a string) (Action, error) {
			return NewCandidateTransferOwnership(a, nil)
		}},
		{"CandidateRegister.operatorAddress", func(a string) (Action, error) {
			return NewCandidateRegister("testcand1", a, ok, ok, "100", 1, false, nil)
		}},
		{"CandidateRegister.rewardAddress", func(a string) (Action, error) {
			return NewCandidateRegister("testcand1", ok, a, ok, "100", 1, false, nil)
		}},
		{"CandidateRegister.ownerAddress", func(a string) (Action, error) {
			return NewCandidateRegister("testcand1", ok, ok, a, "100", 1, false, nil)
		}},
		{"CandidateUpdate.operatorAddress", func(a string) (Action, error) {
			return NewCandidateUpdate("testcand1", a, ok)
		}},
		{"CandidateUpdate.rewardAddress", func(a string) (Action, error) {
			return NewCandidateUpdate("testcand1", ok, a)
		}},
		{"ClaimFromRewardingFund.address", func(a string) (Action, error) {
			addr, err := address.FromString(a)
			if err != nil {
				return nil, err
			}
			return NewClaimFromRewardingFund(big.NewInt(1), addr, nil), nil
		}},
	}

	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			r := require.New(t)
			// a regular address is accepted
			act, err := tc.build(ok)
			r.NoError(err)
			r.NoError(CheckActionAddress(act))
			for _, sp := range []string{address.StakingBucketPoolAddr, address.RewardingPoolAddr} {
				act, err := tc.build(sp)
				r.NoError(err)
				r.ErrorIs(CheckActionAddress(act), ErrAddress, "%s = %s", tc.field, sp)
				// SanityCheck also runs during block validation and is left
				// unchanged, so the special address is rejected on admission only
				if sc, isSC := act.(interface{ SanityCheck() error }); isSC {
					r.NoError(sc.SanityCheck())
				}
			}
		})
	}

	t.Run("optional fields", func(t *testing.T) {
		// absent optional addresses pass
		r.NoError(CheckActionAddress(NewExecution(EmptyAddress, big.NewInt(0), nil)))
		r.NoError(CheckActionAddress(NewClaimFromRewardingFund(big.NewInt(1), nil, nil)))
		cu, err := NewCandidateUpdate("testcand1", "", "")
		r.NoError(err)
		r.NoError(CheckActionAddress(cu))
		cr, err := NewCandidateRegister("testcand1", ok, ok, "", "100", 1, false, nil)
		r.NoError(err)
		r.NoError(CheckActionAddress(cr))
	})
	t.Run("malformed transfer recipient", func(t *testing.T) {
		r.Error(CheckActionAddress(NewTransfer(big.NewInt(0), "io1notvalid", nil)))
	})
}

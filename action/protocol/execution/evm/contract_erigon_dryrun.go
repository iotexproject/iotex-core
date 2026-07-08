// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	libcommon "github.com/erigontech/erigon-lib/common"
	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
)

// contractErigonDryrun wraps contractErigon for the estimateGas / eth_call
// simulate path only. It deliberately mirrors the mainline-MPT contract cache
// pollution bug (see the fix in the same commit series adding the `missing`
// map to mptrie contract.go) so that eth_estimateGas returns a gas value
// large enough to cover the overcharge that the block-producing MPT path
// still incurs before the correctness fix is deployed behind a fork gate.
//
// Concretely, the MPT bug behaves like this for a storage slot K whose true
// pre-tx value is 0:
//
//	first SLOAD(K)         → trie ErrNotExist, committed[K] not populated
//	SSTORE(K, V1)          → trie[K]=V1, committed[K] still absent
//	later SLOAD(K)         → trie succeeds, committed[K] populated with V1
//	later GetCommittedState→ returns V1 (post-mutation), not 0
//
// Under EIP-2200 the last step misclassifies subsequent SSTOREs on K as
// SSTORE_RESET (2900 gas) instead of dirty in-place (100 gas), overcharging
// 2800 gas per hit. We simulate that shape in Erigon by, whenever the true
// prestate is zero, returning intra.GetState (current, post-mutation) from
// GetCommittedState. Non-zero prestates behave normally.
type contractErigonDryrun struct {
	*contractErigon
}

func newContractErigonDryrun(addr hash.Hash160, account *state.Account, intra *erigonstate.IntraBlockState, sr protocol.StateReader) (Contract, error) {
	inner, err := newContractErigon(addr, account, intra, sr)
	if err != nil {
		return nil, err
	}
	return &contractErigonDryrun{contractErigon: inner.(*contractErigon)}, nil
}

// GetCommittedState mirrors the mainline MPT prestate-absent pollution.
// If the true tx-start value is zero we deliberately return the current
// (potentially post-mutation) live value so the EIP-2200 dynamic gas
// calculator classifies the write the same way it will on the still-buggy
// MPT block-producing path.
func (c *contractErigonDryrun) GetCommittedState(key hash.Hash256) ([]byte, error) {
	committed, err := c.contractErigon.GetCommittedState(key)
	if err != nil {
		return nil, err
	}
	if isAllZero(committed) {
		// Mirror the polluted-cache observation: return the live value from
		// IntraBlockState (which reflects any SSTOREs written earlier in the
		// same tx). Before any SSTORE lands, GetState is also zero, so this
		// is a no-op; only after a write does it start returning the bug's
		// post-mutation value.
		k := libcommon.Hash(key)
		v := uint256.NewInt(0)
		c.intra.GetState(libcommon.Address(c.addr), &k, v)
		h := hash.BytesToHash256(v.Bytes())
		return h[:], nil
	}
	return committed, nil
}

// Snapshot preserves the dryrun wrapper across EVM snapshots.
func (c *contractErigonDryrun) Snapshot() Contract {
	inner := c.contractErigon.Snapshot().(*contractErigon)
	return &contractErigonDryrun{contractErigon: inner}
}

func isAllZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

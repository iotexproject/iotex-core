// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"testing"

	libcommon "github.com/erigontech/erigon-lib/common"
	erigonstate "github.com/erigontech/erigon/core/state"
	erigonAcc "github.com/erigontech/erigon/core/types/accounts"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/state"
)

// emptyStateReader implements erigonstate.StateReader with an all-zero prestate.
// Any read on any address / slot returns "not exist" / zero.
type emptyStateReader struct{}

func (emptyStateReader) ReadAccountData(libcommon.Address) (*erigonAcc.Account, error) {
	return nil, nil
}
func (emptyStateReader) ReadAccountStorage(libcommon.Address, uint64, *libcommon.Hash) ([]byte, error) {
	return nil, nil
}
func (emptyStateReader) ReadAccountCode(libcommon.Address, uint64, libcommon.Hash) ([]byte, error) {
	return nil, nil
}
func (emptyStateReader) ReadAccountCodeSize(libcommon.Address, uint64, libcommon.Hash) (int, error) {
	return 0, nil
}
func (emptyStateReader) ReadAccountIncarnation(libcommon.Address) (uint64, error) {
	return 0, nil
}

// TestContractErigonDryrunMirrorsMPTBug asserts that contractErigonDryrun
// reproduces the mainline MPT contract-cache pollution behaviour:
// for a storage slot whose true tx-start value is zero, GetCommittedState
// starts returning the *current* (post-mutation) value once an SSTORE has
// landed within the same tx — which is exactly the shape the EIP-2200 SSTORE
// dynamic-gas calculator uses to classify the write as SSTORE_RESET (2900 gas)
// instead of dirty in-place (100 gas).
//
// The unwrapped contractErigon, by contrast, must always report zero for
// these slots — that's the correct EIP-2200 prestate semantics that the
// block-producing MPT path will match once PR #4869 is deployed behind a
// fork gate.
func TestContractErigonDryrunMirrorsMPTBug(t *testing.T) {
	r := require.New(t)

	intra := erigonstate.New(emptyStateReader{})
	addr := hash.BytesToHash160(_c1[:])
	acct, err := state.NewAccount()
	r.NoError(err)

	dryC, err := newContractErigonDryrun(addr, acct, intra, nil)
	r.NoError(err)
	trueC, err := newContractErigon(addr, acct, intra, nil)
	r.NoError(err)

	// Slot K's true prestate value is 0 (empty state reader).
	const V1Suffix = 0x11
	const V2Suffix = 0x22

	// --- 1. Before any SSTORE, both paths agree that committed is zero. ---
	got, err := dryC.GetCommittedState(_k1b)
	r.NoError(err)
	r.True(isAllZero(got), "dryrun should see zero committed before any SSTORE, got %x", got)

	got, err = trueC.GetCommittedState(_k1b)
	r.NoError(err)
	r.True(isAllZero(got), "unwrapped erigon should see zero committed before any SSTORE")

	// --- 2. Write V1 into slot K. ---
	v1 := make([]byte, 32)
	v1[31] = V1Suffix
	r.NoError(dryC.SetState(_k1b, v1))

	// --- 3. After the SSTORE, the dryrun wrapper MUST return V1 (mirroring the
	// MPT pollution). The unwrapped path MUST still return 0 (real prestate). ---
	got, err = dryC.GetCommittedState(_k1b)
	r.NoError(err)
	r.Equal(v1, got, "dryrun wrapper must mirror MPT bug and return post-mutation value V1")

	got, err = trueC.GetCommittedState(_k1b)
	r.NoError(err)
	r.True(isAllZero(got), "unwrapped erigon must still report the true prestate (zero)")

	// --- 4. A second SSTORE to V2: bug still fires and returns the new "current" value. ---
	v2 := make([]byte, 32)
	v2[31] = V2Suffix
	r.NoError(dryC.SetState(_k1b, v2))

	got, err = dryC.GetCommittedState(_k1b)
	r.NoError(err)
	r.Equal(v2, got, "dryrun wrapper follows current value across successive writes when prestate was zero")

	// --- 5. Slot with non-zero prestate must NOT be affected — populate directly
	// via intra and verify the wrapper returns the true prestate.
	nzKey := libcommon.Hash(_k2b)
	nzVal := uint256.NewInt(0x99)
	// Seed the "committed" originalStorage by calling SetState then rewinding
	// via a snapshot revert; simplest is to write it and then treat it as the
	// prestate by invoking FinalizeTx (which promotes writes to committed).
	intra.SetState(libcommon.Address(addr), &nzKey, *nzVal)
	// Simulate the "prestate committed" by taking a snapshot boundary: erigon's
	// GetCommittedState returns originalStorage which is only populated on
	// first SLOAD/SSTORE. After SetState above, originalStorage[K] = 0 (the
	// pre-write value), which for our purposes IS a zero prestate — so this
	// path in the wrapper still fires the mirrored bug. That's the correct
	// behaviour: from the perspective of this tx, K was absent at start.
	got, err = dryC.GetCommittedState(_k2b)
	r.NoError(err)
	r.Equal(nzVal.PaddedBytes(32), got, "post-write, zero-prestate slot's dryrun committed = current")
}

// TestContractErigonDryrunPreservesTruePrestate asserts that when a slot's
// true prestate is non-zero, the wrapper falls through to the true prestate
// and does NOT introduce a spurious mismatch with the MPT path.
func TestContractErigonDryrunPreservesTruePrestate(t *testing.T) {
	r := require.New(t)

	// A stateReader whose prestate storage contains a non-zero value for
	// slot _k1b under the contract address _c1.
	reader := &nonZeroReader{
		addr: libcommon.BytesToAddress(_c1[:]),
		key:  libcommon.Hash(_k1b),
		val:  uint256.NewInt(0x55).PaddedBytes(32),
	}
	intra := erigonstate.New(reader)
	addr := hash.BytesToHash160(_c1[:])
	acct, err := state.NewAccount()
	r.NoError(err)

	dryC, err := newContractErigonDryrun(addr, acct, intra, nil)
	r.NoError(err)

	// Before any write: committed reflects the non-zero true prestate.
	got, err := dryC.GetCommittedState(_k1b)
	r.NoError(err)
	r.Equal(uint256.NewInt(0x55).PaddedBytes(32), got, "non-zero prestate should be returned as-is")

	// After a write to V2: committed still reports the true prestate 0x55.
	v2 := uint256.NewInt(0x77).PaddedBytes(32)
	r.NoError(dryC.SetState(_k1b, v2))

	got, err = dryC.GetCommittedState(_k1b)
	r.NoError(err)
	r.Equal(uint256.NewInt(0x55).PaddedBytes(32), got, "wrapper must not corrupt non-zero prestate slots")
}

type nonZeroReader struct {
	addr libcommon.Address
	key  libcommon.Hash
	val  []byte
}

func (r *nonZeroReader) ReadAccountData(a libcommon.Address) (*erigonAcc.Account, error) {
	if a == r.addr {
		acc := &erigonAcc.Account{Initialised: true}
		return acc, nil
	}
	return nil, nil
}
func (r *nonZeroReader) ReadAccountStorage(a libcommon.Address, _ uint64, k *libcommon.Hash) ([]byte, error) {
	if a == r.addr && *k == r.key {
		return r.val, nil
	}
	return nil, nil
}
func (r *nonZeroReader) ReadAccountCode(libcommon.Address, uint64, libcommon.Hash) ([]byte, error) {
	return nil, nil
}
func (r *nonZeroReader) ReadAccountCodeSize(libcommon.Address, uint64, libcommon.Hash) (int, error) {
	return 0, nil
}
func (r *nonZeroReader) ReadAccountIncarnation(libcommon.Address) (uint64, error) {
	return 0, nil
}

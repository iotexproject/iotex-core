// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestCreateContract(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	defer testutil.CleanupPath(testTriePath)

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := batch.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			val, err := cb.Get("state", cfg.Key)
			if err != nil {
				return 0, state.ErrStateNotExist
			}
			return 0, state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			ss, err := state.Serialize(account)
			if err != nil {
				return 0, err
			}
			cb.Put("state", cfg.Key, ss, "failed to put state")
			return 0, nil
		}).AnyTimes()

	addr := identityset.Address(28)
	_, err = accountutil.LoadOrCreateAccount(sm, addr)
	require.NoError(err)
	stateDB, err := NewStateDBAdapter(sm, 0, hash.ZeroHash256, NotFixTopicCopyBugOption())
	require.NoError(err)

	contract := addr.Bytes()
	var evmContract common.Address
	copy(evmContract[:], contract[:])
	stateDB.SetCode(evmContract, _bytecode)
	// contract exist
	codeHash := stateDB.GetCodeHash(evmContract)
	var emptyEVMHash common.Hash
	require.NotEqual(emptyEVMHash, codeHash)
	v := stateDB.GetCode(evmContract)
	require.Equal(_bytecode, v)
	// non-existing contract
	addr1 := hash.Hash160b([]byte("random"))
	var evmAddr1 common.Address
	copy(evmAddr1[:], addr1[:])
	h := stateDB.GetCodeHash(evmAddr1)
	require.Equal(emptyEVMHash, h)
	require.Nil(stateDB.GetCode(evmAddr1))
	require.NoError(stateDB.CommitContracts())
	stateDB.clear()
	// reload same contract
	contract1, err := accountutil.LoadOrCreateAccount(sm, addr)
	require.NoError(err)
	require.Equal(codeHash[:], contract1.CodeHash)
}

func TestLoadStoreCommit(t *testing.T) {
	require := require.New(t)

	testLoadStoreCommit := func(t *testing.T, enableAsync bool) {
		ctrl := gomock.NewController(t)
		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		acct := &state.Account{}
		cntr1, err := newContract(hash.BytesToHash160(_c1[:]), acct, sm, enableAsync, true)
		require.NoError(err)

		tests := []cntrTest{
			{
				cntr1,
				[]code{
					{_c1, []byte("2nd contract creation")},
				},
				[]set{
					{_k1b, _v1b[:], nil},
					{_k2b, _v2b[:], nil},
				},
			},
			{
				cntr1,
				[]code{
					{_c2, _bytecode},
				},
				[]set{
					{_k1b, _v4b[:], nil},
					{_k2b, _v3b[:], nil},
					{_k3b, _v2b[:], nil},
					{_k4b, _v1b[:], nil},
				},
			},
			{
				cntr1,
				nil,
				[]set{
					{_k1b, _v2b[:], nil},
					{_k2b, _v1b[:], nil},
					{_k3b, _v4b[:], nil},
					{_k4b, nil, nil},
				},
			},
		}

		for i, test := range tests {
			c := test.contract
			// set code
			for _, e := range test.codes {
				c.SetCode(hash.Hash256b(e.v), e.v)
			}
			// set states
			for _, e := range test.states {
				require.NoError(c.SetState(e.k, e.v))
				if i > 0 {
					// committed state == value of previous test's SetState()
					committed := tests[i-1].states
					for _, e := range committed {
						v, err := c.GetCommittedState(e.k)
						require.NoError(err)
						require.Equal(e.v, v)
					}
				}
				v, err := c.GetState(e.k)
				require.NoError(err)
				require.Equal(e.v, v)
			}
			require.NoError(c.Commit())
		}

		checks := []cntrTest{
			{
				cntr1,
				[]code{
					{_c1, _bytecode},
				},
				[]set{
					{_k1b, _v2b[:], nil},
					{_k2b, _v1b[:], nil},
					{_k3b, _v4b[:], nil},
					{_k4b, nil, nil},
				},
			},
		}

		for _, test := range checks {
			c := test.contract
			// check code
			for _, e := range test.codes {
				v, err := c.GetCode()
				require.NoError(err)
				require.Equal(e.v, v)
				chash := hash.Hash256b(e.v)
				require.Equal(chash[:], c.SelfState().CodeHash)
				require.NotEqual(hash.ZeroHash256, hash.BytesToHash256(chash[:]))
			}
			// check states
			for _, e := range test.states {
				v, err := c.GetState(e.k)
				require.Equal(e.v, v)
				if err != nil {
					require.Equal(e.cause, errors.Cause(err))
				}
			}
		}
	}

	t.Run("contract load/store with stateDB, sync mode", func(t *testing.T) {
		testLoadStoreCommit(t, false)
	})
	t.Run("contract load/store with stateDB, async mode", func(t *testing.T) {
		testLoadStoreCommit(t, true)
	})

}

func TestSnapshot(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	testfunc := func(enableAsync bool) {
		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		s, err := state.NewAccount()
		require.NoError(err)
		require.NoError(s.AddBalance(big.NewInt(5)))
		_c1, err := newContract(
			hash.BytesToHash160(identityset.Address(28).Bytes()),
			s,
			sm,
			enableAsync,
			true,
		)
		require.NoError(err)
		require.NoError(_c1.SetState(_k2b, _v2[:]))
		_c2 := _c1.Snapshot()
		require.NoError(_c1.SelfState().AddBalance(big.NewInt(7)))
		require.NoError(_c1.SetState(_k1b, _v1[:]))
		require.Equal(big.NewInt(12), _c1.SelfState().Balance)
		require.Equal(big.NewInt(5), _c2.SelfState().Balance)
	}
	t.Run("sync mode", func(t *testing.T) {
		testfunc(false)
	})
	t.Run("async mode", func(t *testing.T) {
		testfunc(true)
	})
}

// TestGetCommittedStateAbsentKey guards two aspects of the prestate-absent
// storage-slot behavior around the CorrectPrestateForAbsentKeys fork gate:
//
//   - post-fork (trackAbsent=true, this PR's fix): GetCommittedState must
//     report the true prestate (ErrNotExist / zero) for a slot that was
//     absent at tx start, even after an intra-tx SSTORE has landed on it.
//   - pre-fork (trackAbsent=false): the historical buggy behavior is
//     preserved so catch-up from mainnet state prior to the fork height
//     replays byte-identically. GetCommittedState returns the post-mutation
//     value written earlier in the same tx.
//
// Repro pattern (mainnet block 48,900,885, tx 0xfe792b0c...):
//  1. SLOAD  K  (K absent → trie.ErrNotExist propagated up)
//  2. SSTORE K, V1  (writes V1 to the live trie)
//  3. SLOAD  K  (trie now returns V1 without error)
//  4. GetCommittedState(K)
//
// Under the pre-fork code path, step 4 returns V1 (post-mutation), causing
// EIP-2200 SSTORE gas to be misclassified as SSTORE_RESET on the next write
// and burning ~2800 extra gas per SSTORE — enough to OOG-revert the ioID
// device registration flow.
func TestGetCommittedStateAbsentKey(t *testing.T) {
	require := require.New(t)
	run := func(t *testing.T, enableAsync, trackAbsent bool) {
		ctrl := gomock.NewController(t)
		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		acct, err := state.NewAccount()
		require.NoError(err)
		c, err := newContract(hash.BytesToHash160(_c1[:]), acct, sm, enableAsync, trackAbsent)
		require.NoError(err)

		// 1. First SLOAD on an absent key surfaces trie.ErrNotExist.
		v, err := c.GetState(_k1b)
		require.True(errors.Cause(err) == trie.ErrNotExist)
		require.Empty(v)

		// 2. Write V1 to the same key.
		require.NoError(c.SetState(_k1b, _v1b[:]))

		// 3. Second SLOAD now succeeds and returns the just-written value.
		v, err = c.GetState(_k1b)
		require.NoError(err)
		require.Equal(_v1b[:], v)

		// 4. GetCommittedState behaviour depends on the fork gate.
		v, err = c.GetCommittedState(_k1b)
		if trackAbsent {
			require.True(errors.Cause(err) == trie.ErrNotExist,
				"post-fork: expected trie.ErrNotExist for prestate-absent key, got err=%v v=%x", err, v)
			require.Empty(v)
		} else {
			require.NoError(err, "pre-fork: GetCommittedState must succeed and return polluted value")
			require.Equal(_v1b[:], v, "pre-fork: expected post-mutation V1 (bug-preserving), got %x", v)
		}
	}
	t.Run("post-fork sync", func(t *testing.T) { run(t, false, true) })
	t.Run("post-fork async", func(t *testing.T) { run(t, true, true) })
	t.Run("pre-fork sync (bug preserved)", func(t *testing.T) { run(t, false, false) })
	t.Run("pre-fork async (bug preserved)", func(t *testing.T) { run(t, true, false) })
}

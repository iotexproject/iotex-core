// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestCreateContract(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
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
	_, err = accountutil.LoadOrCreateAccount(sm, addr.String())
	require.NoError(err)
	stateDB := NewStateDBAdapter(sm, 0, !cfg.Genesis.IsAleutian(0), cfg.Genesis.IsGreenland(0), hash.ZeroHash256)
	contract := addr.Bytes()
	var evmContract common.Address
	copy(evmContract[:], contract[:])
	stateDB.SetCode(evmContract, bytecode)
	// contract exist
	codeHash := stateDB.GetCodeHash(evmContract)
	var emptyEVMHash common.Hash
	require.NotEqual(emptyEVMHash, codeHash)
	v := stateDB.GetCode(evmContract)
	require.Equal(bytecode, v)
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
	contract1, err := accountutil.LoadOrCreateAccount(sm, addr.String())
	require.NoError(err)
	require.Equal(codeHash[:], contract1.CodeHash)
}

func TestLoadStoreCommit(t *testing.T) {
	require := require.New(t)

	testLoadStoreCommit := func(cfg config.Config, t *testing.T, enableAsync bool) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		cntr1, err := newContract(hash.BytesToHash160(c1[:]), &state.Account{}, sm, enableAsync)
		require.NoError(err)

		tests := []cntrTest{
			{
				cntr1,
				[]code{
					{c1, []byte("2nd contract creation")},
				},
				[]set{
					{k1b, v1b[:], nil},
					{k2b, v2b[:], nil},
				},
			},
			{
				cntr1,
				[]code{
					{c2, bytecode},
				},
				[]set{
					{k1b, v4b[:], nil},
					{k2b, v3b[:], nil},
					{k3b, v2b[:], nil},
					{k4b, v1b[:], nil},
				},
			},
			{
				cntr1,
				nil,
				[]set{
					{k1b, v2b[:], nil},
					{k2b, v1b[:], nil},
					{k3b, v4b[:], nil},
					{k4b, nil, nil},
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
					{c1, bytecode},
				},
				[]set{
					{k1b, v2b[:], nil},
					{k2b, v1b[:], nil},
					{k3b, v4b[:], nil},
					{k4b, nil, nil},
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

	cfg := config.Default
	t.Run("contract load/store with stateDB, sync mode", func(t *testing.T) {
		testTriePath, err := testutil.PathOfTempFile("trie")
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, testTriePath)
		}()

		cfg.Chain.TrieDBPath = testTriePath
		testLoadStoreCommit(cfg, t, false)
	})
	t.Run("contract load/store with stateDB, async mode", func(t *testing.T) {
		testTriePath, err := testutil.PathOfTempFile("trie")
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, testTriePath)
		}()

		cfg := config.Default
		cfg.Chain.TrieDBPath = testTriePath
		testLoadStoreCommit(cfg, t, true)
	})

	t.Run("contract load/store with trie, sync mode", func(t *testing.T) {
		testTriePath2, err := testutil.PathOfTempFile("trie")
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, testTriePath2)
		}()
		cfg.Chain.EnableTrielessStateDB = false
		cfg.Chain.TrieDBPath = testTriePath2
		testLoadStoreCommit(cfg, t, false)
	})
	t.Run("contract load/store with trie, async mode", func(t *testing.T) {
		testTriePath2, err := testutil.PathOfTempFile("trie")
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, testTriePath2)
		}()
		cfg.Chain.EnableTrielessStateDB = false
		cfg.Chain.TrieDBPath = testTriePath2
		testLoadStoreCommit(cfg, t, true)
	})
}

func TestSnapshot(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	testfunc := func(enableAsync bool) {
		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		s := &state.Account{
			Balance: big.NewInt(5),
		}
		c1, err := newContract(
			hash.BytesToHash160(identityset.Address(28).Bytes()),
			s,
			sm,
			enableAsync,
		)
		require.NoError(err)
		require.NoError(c1.SetState(k2b, v2[:]))
		c2 := c1.Snapshot()
		require.NoError(c1.SelfState().AddBalance(big.NewInt(7)))
		require.NoError(c1.SetState(k1b, v1[:]))
		require.Equal(big.NewInt(12), c1.SelfState().Balance)
		require.Equal(big.NewInt(5), c2.SelfState().Balance)
	}
	t.Run("sync mode", func(t *testing.T) {
		testfunc(false)
	})
	t.Run("async mode", func(t *testing.T) {
		testfunc(true)
	})
}

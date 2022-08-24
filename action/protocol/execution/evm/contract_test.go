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
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/testutil"
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
		cntr1, err := newContract(hash.BytesToHash160(_c1[:]), acct, sm, enableAsync)
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

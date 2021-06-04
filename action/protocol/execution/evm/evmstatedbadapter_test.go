// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

func initMockStateManager(ctrl *gomock.Controller) (*mock_chainmanager.MockStateManager, error) {
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := batch.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			ns := "state"
			if cfg.Namespace != "" {
				ns = cfg.Namespace
			}
			val, err := cb.Get(ns, cfg.Key)
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
			ns := "state"
			if cfg.Namespace != "" {
				ns = cfg.Namespace
			}
			cb.Put(ns, cfg.Key, ss, "failed to put state")
			return 0, nil
		}).AnyTimes()
	sm.EXPECT().DelState(gomock.Any()).DoAndReturn(
		func(s interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			ns := "state"
			if cfg.Namespace != "" {
				ns = cfg.Namespace
			}
			cb.Delete(ns, cfg.Key, "failed to delete state")
			return 0, nil
		}).AnyTimes()
	sm.EXPECT().Snapshot().DoAndReturn(
		func() int {
			return cb.Snapshot()
		}).AnyTimes()
	sm.EXPECT().Revert(gomock.Any()).DoAndReturn(
		func(snapshot int) error {
			return cb.Revert(snapshot)
		}).AnyTimes()
	return sm, nil
}

func TestAddBalance(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB := NewStateDBAdapter(sm, 1, true, false, hash.ZeroHash256)
	addAmount := big.NewInt(40000)
	stateDB.AddBalance(addr, addAmount)
	amount := stateDB.GetBalance(addr)
	require.Equal(0, amount.Cmp(addAmount))
	stateDB.AddBalance(addr, addAmount)
	amount = stateDB.GetBalance(addr)
	require.Equal(0, amount.Cmp(big.NewInt(80000)))
}

func TestRefundAPIs(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	stateDB := NewStateDBAdapter(sm, 1, true, false, hash.ZeroHash256)
	require.Zero(stateDB.GetRefund())
	refund := uint64(1024)
	stateDB.AddRefund(refund)
	require.Equal(refund, stateDB.GetRefund())
}

func TestEmptyAndCode(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB := NewStateDBAdapter(sm, 1, true, false, hash.ZeroHash256)
	require.True(stateDB.Empty(addr))
	stateDB.CreateAccount(addr)
	require.True(stateDB.Empty(addr))
	stateDB.SetCode(addr, []byte("0123456789"))
	require.True(bytes.Equal(stateDB.GetCode(addr), []byte("0123456789")))
	require.False(stateDB.Empty(addr))
}

func TestForEachStorage(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB := NewStateDBAdapter(sm, 1, true, false, hash.ZeroHash256)
	stateDB.CreateAccount(addr)
	kvs := map[common.Hash]common.Hash{
		common.HexToHash("0123456701234567012345670123456701234567012345670123456701234560"): common.HexToHash("0123456701234567012345670123456701234567012345670123456701234560"),
		common.HexToHash("0123456701234567012345670123456701234567012345670123456701234561"): common.HexToHash("0123456701234567012345670123456701234567012345670123456701234561"),
		common.HexToHash("0123456701234567012345670123456701234567012345670123456701234562"): common.HexToHash("0123456701234567012345670123456701234567012345670123456701234562"),
		common.HexToHash("0123456701234567012345670123456701234567012345670123456701234563"): common.HexToHash("0123456701234567012345670123456701234567012345670123456701234563"),
		common.HexToHash("2345670123456701234567012345670123456701234567012345670123456301"): common.HexToHash("2345670123456701234567012345670123456701234567012345670123456301"),
		common.HexToHash("4567012345670123456701234567012345670123456701234567012345630123"): common.HexToHash("4567012345670123456701234567012345670123456701234567012345630123"),
		common.HexToHash("6701234567012345670123456701234567012345670123456701234563012345"): common.HexToHash("6701234567012345670123456701234567012345670123456701234563012345"),
		common.HexToHash("0123456701234567012345670123456701234567012345670123456301234567"): common.HexToHash("0123456701234567012345670123456701234567012345670123456301234567"),
		common.HexToHash("ab45670123456701234567012345670123456701234567012345630123456701"): common.HexToHash("ab45670123456701234567012345670123456701234567012345630123456701"),
		common.HexToHash("cd67012345670123456701234567012345670123456701234563012345670123"): common.HexToHash("cd67012345670123456701234567012345670123456701234563012345670123"),
		common.HexToHash("ef01234567012345670123456701234567012345670123456301234567012345"): common.HexToHash("ef01234567012345670123456701234567012345670123456301234567012345"),
	}
	for k, v := range kvs {
		stateDB.SetState(addr, k, v)
	}
	require.NoError(
		stateDB.ForEachStorage(addr, func(k common.Hash, v common.Hash) bool {
			require.Equal(k, v)
			delete(kvs, k)
			return true
		}),
	)
	require.Equal(0, len(kvs))
}

func TestNonce(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB := NewStateDBAdapter(sm, 1, true, false, hash.ZeroHash256)
	require.Equal(uint64(0), stateDB.GetNonce(addr))
	stateDB.SetNonce(addr, 1)
	require.Equal(uint64(1), stateDB.GetNonce(addr))
}

func TestSnapshotRevertAndCommit(t *testing.T) {
	testSnapshotAndRevert := func(cfg config.Config, t *testing.T) {
		require := require.New(t)
		ctrl := gomock.NewController(t)

		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		stateDB := NewStateDBAdapter(sm, 1, true, false, hash.ZeroHash256)
		tests := []stateDBTest{
			{
				[]bal{
					{addr1, big.NewInt(40000)},
				},
				[]code{
					{c1, bytecode},
				},
				[]evmSet{
					{c1, k1, v1},
					{c1, k2, v2},
					{c3, k3, v4},
				},
				[]sui{
					{c2, false, false},
					{C4, false, false},
				},
				[]image{
					{common.BytesToHash(v1[:]), []byte("cat")},
					{common.BytesToHash(v2[:]), []byte("dog")},
				},
				[]access{
					{c1, []common.Hash{k1, k2}, []common.Hash{k3, k4}, false},
				},
			},
			{
				[]bal{
					{addr1, big.NewInt(40000)},
				},
				[]code{
					{c2, bytecode},
				},
				[]evmSet{
					{c1, k1, v3},
					{c1, k2, v4},
					{c2, k3, v3},
					{c2, k4, v4},
				},
				[]sui{
					{c1, true, true},
					{c3, true, true},
				},
				[]image{
					{common.BytesToHash(v3[:]), []byte("hen")},
				},
				[]access{
					{c1, []common.Hash{k3, k4}, nil, true},
					{c2, []common.Hash{k1, k3}, []common.Hash{k2, k4}, false},
				},
			},
			{
				nil,
				nil,
				[]evmSet{
					{c2, k3, v1},
					{c2, k4, v2},
				},
				[]sui{
					{addr1, true, true},
				},
				[]image{
					{common.BytesToHash(v4[:]), []byte("fox")},
				},
				[]access{
					{c2, []common.Hash{k2, k4}, nil, true},
				},
			},
		}

		for i, test := range tests {
			// add balance
			for _, e := range test.balance {
				stateDB.AddBalance(e.addr, e.v)
			}
			// set code
			for _, e := range test.codes {
				stateDB.SetCode(e.addr, e.v)
				v := stateDB.GetCode(e.addr)
				require.Equal(e.v, v)
			}
			// set states
			for _, e := range test.states {
				stateDB.SetState(e.addr, e.k, e.v)
			}
			// set suicide
			for _, e := range test.suicide {
				require.Equal(e.suicide, stateDB.Suicide(e.addr))
				require.Equal(e.exist, stateDB.Exist(e.addr))
			}
			// set preimage
			for _, e := range test.preimage {
				stateDB.AddPreimage(e.hash, e.v)
			}
			// set access list
			for _, e := range test.accessList {
				require.Equal(e.exist, stateDB.AddressInAccessList(e.addr))
				for _, slot := range e.slots {
					aOk, sOk := stateDB.SlotInAccessList(e.addr, slot)
					require.Equal(e.exist, aOk)
					require.False(sOk)
					stateDB.AddSlotToAccessList(e.addr, slot)
					e.exist = true
					aOk, sOk = stateDB.SlotInAccessList(e.addr, slot)
					require.True(aOk)
					require.True(sOk)
				}
				for _, slot := range e.nx {
					aOk, sOk := stateDB.SlotInAccessList(e.addr, slot)
					require.True(aOk)
					require.False(sOk)
				}
			}
			require.Equal(i, stateDB.Snapshot())
		}

		reverts := []stateDBTest{
			{
				[]bal{
					{addr1, big.NewInt(0)},
				},
				[]code{},
				[]evmSet{
					{c1, k1, v3},
					{c1, k2, v4},
					{c2, k3, v1},
					{c2, k4, v2},
				},
				[]sui{
					{c1, true, true},
					{c3, true, true},
					{c2, false, true},
					{C4, false, false},
					{addr1, true, true},
				},
				[]image{
					{common.BytesToHash(v1[:]), []byte("cat")},
					{common.BytesToHash(v2[:]), []byte("dog")},
					{common.BytesToHash(v3[:]), []byte("hen")},
					{common.BytesToHash(v4[:]), []byte("fox")},
				},
				[]access{
					{c1, []common.Hash{k1, k2, k3, k4}, nil, true},
					{c2, []common.Hash{k1, k2, k3, k4}, nil, true},
				},
			},
			{
				[]bal{
					{addr1, big.NewInt(80000)},
				},
				[]code{},
				tests[1].states,
				[]sui{
					{c1, true, true},
					{c3, true, true},
					{c2, false, true},
					{C4, false, false},
					{addr1, false, true},
				},
				[]image{
					{common.BytesToHash(v1[:]), []byte("cat")},
					{common.BytesToHash(v2[:]), []byte("dog")},
					{common.BytesToHash(v3[:]), []byte("hen")},
					{common.BytesToHash(v4[:]), []byte(nil)},
				},
				[]access{
					{c1, []common.Hash{k1, k2, k3, k4}, nil, true},
					{c2, []common.Hash{k1, k3}, []common.Hash{k2, k4}, true},
				},
			},
			{
				[]bal{
					{addr1, big.NewInt(40000)},
				},
				[]code{},
				[]evmSet{
					{c1, k1, v1},
					{c1, k2, v2},
					{c3, k3, v4},
				},
				[]sui{
					{c1, false, true},
					{c3, false, true},
					{c2, false, false},
					{C4, false, false},
					{addr1, false, true},
				},
				[]image{
					{common.BytesToHash(v1[:]), []byte("cat")},
					{common.BytesToHash(v2[:]), []byte("dog")},
					{common.BytesToHash(v3[:]), []byte(nil)},
					{common.BytesToHash(v4[:]), []byte(nil)},
				},
				[]access{
					{c1, []common.Hash{k1, k2}, []common.Hash{k3, k4}, true},
					{c2, nil, []common.Hash{k1, k2, k3, k4}, false},
				},
			},
		}

		// test revert
		for i, test := range reverts {
			stateDB.RevertToSnapshot(len(reverts) - 1 - i)

			// test balance
			for _, e := range test.balance {
				amount := stateDB.GetBalance(e.addr)
				require.Equal(e.v, amount)
			}
			// test states
			for _, e := range test.states {
				require.Equal(e.v, stateDB.GetState(e.addr, e.k))
			}
			// test suicide/exist
			for _, e := range test.suicide {
				require.Equal(e.suicide, stateDB.HasSuicided(e.addr))
				require.Equal(e.exist, stateDB.Exist(e.addr))
			}
			// test preimage
			for _, e := range test.preimage {
				v := stateDB.preimages[e.hash]
				require.Equal(e.v, []byte(v))
			}
			// test access list
			for _, e := range test.accessList {
				require.Equal(e.exist, stateDB.AddressInAccessList(e.addr))
				for _, slot := range e.slots {
					aOk, sOk := stateDB.SlotInAccessList(e.addr, slot)
					require.Equal(e.exist, aOk)
					require.True(sOk)
				}
				for _, slot := range e.nx {
					aOk, sOk := stateDB.SlotInAccessList(e.addr, slot)
					require.Equal(e.exist, aOk)
					require.False(sOk)
				}
			}
		}

		// commit snapshot 0's state
		require.NoError(stateDB.CommitContracts())
		stateDB.clear()
		//[TODO] need e2etest to verify state factory commit/re-open (whether result from state/balance/suicide/exist is same)
	}

	t.Run("contract snapshot/revert/commit with in memery DB", func(t *testing.T) {
		cfg := config.Default
		testSnapshotAndRevert(cfg, t)
	})
}

func TestGetCommittedState(t *testing.T) {
	t.Run("committed state with in mem DB", func(t *testing.T) {
		require := require.New(t)
		ctrl := gomock.NewController(t)

		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		stateDB := NewStateDBAdapter(sm, 1, true, false, hash.ZeroHash256)

		stateDB.SetState(c1, k1, v1)
		// k2 does not exist
		require.Equal(common.Hash{}, stateDB.GetCommittedState(c1, common.BytesToHash(k2[:])))
		require.Equal(v1, stateDB.GetState(c1, k1))
		require.Equal(common.Hash{}, stateDB.GetCommittedState(c1, common.BytesToHash(k2[:])))

		// commit (k1, v1)
		require.NoError(stateDB.CommitContracts())
		stateDB.clear()

		require.Equal(v1, stateDB.GetState(c1, k1))
		require.Equal(common.BytesToHash(v1[:]), stateDB.GetCommittedState(c1, common.BytesToHash(k1[:])))
		stateDB.SetState(c1, k1, v2)
		require.Equal(common.BytesToHash(v1[:]), stateDB.GetCommittedState(c1, common.BytesToHash(k1[:])))
		require.Equal(v2, stateDB.GetState(c1, k1))
		require.Equal(common.BytesToHash(v1[:]), stateDB.GetCommittedState(c1, common.BytesToHash(k1[:])))
	})
}

func TestGetBalanceOnError(t *testing.T) {
	ctrl := gomock.NewController(t)

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	errs := []error{
		state.ErrStateNotExist,
		errors.New("other error"),
	}
	for _, err := range errs {
		sm.EXPECT().State(gomock.Any(), gomock.Any()).Return(uint64(0), err).Times(1)
		addr := common.HexToAddress("test address")
		stateDB := NewStateDBAdapter(sm, 1, true, false, hash.ZeroHash256)
		amount := stateDB.GetBalance(addr)
		assert.Equal(t, big.NewInt(0), amount)
	}
}

func TestPreimage(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	stateDB := NewStateDBAdapter(sm, 1, true, false, hash.ZeroHash256)

	stateDB.AddPreimage(common.BytesToHash(v1[:]), []byte("cat"))
	stateDB.AddPreimage(common.BytesToHash(v2[:]), []byte("dog"))
	stateDB.AddPreimage(common.BytesToHash(v3[:]), []byte("hen"))
	// this won't overwrite preimage of v1
	stateDB.AddPreimage(common.BytesToHash(v1[:]), []byte("fox"))
	require.NoError(stateDB.CommitContracts())
	stateDB.clear()
	var k SerializableBytes
	_, err = stateDB.sm.State(&k, protocol.NamespaceOption(PreimageKVNameSpace), protocol.KeyOption(v1[:]))
	require.NoError(err)
	require.Equal([]byte("cat"), []byte(k))
	_, err = stateDB.sm.State(&k, protocol.NamespaceOption(PreimageKVNameSpace), protocol.KeyOption(v2[:]))
	require.NoError(err)
	require.Equal([]byte("dog"), []byte(k))
	_, err = stateDB.sm.State(&k, protocol.NamespaceOption(PreimageKVNameSpace), protocol.KeyOption(v3[:]))
	require.NoError(err)
	require.Equal([]byte("hen"), []byte(k))
}

func TestSortMap(t *testing.T) {
	uniqueSlice := func(slice []string) bool {
		for _, v := range slice[1:] {
			if v != slice[0] {
				return false
			}
		}
		return true
	}

	testFunc := func(t *testing.T, sm *mock_chainmanager.MockStateManager, opts ...StateDBAdapterOption) bool {
		stateDB := NewStateDBAdapter(sm, 1, true, false, hash.ZeroHash256, opts...)
		size := 10

		for i := 0; i < size; i++ {
			addr := common.HexToAddress(identityset.Address(i).Hex())
			stateDB.SetCode(addr, []byte("0123456789"))
			stateDB.SetState(addr, k1, k2)
		}
		sn := stateDB.Snapshot()
		caches := []string{}
		for i := 0; i < size; i++ {
			stateDB.RevertToSnapshot(sn)
			s := ""
			if stateDB.sortCachedContracts {
				for _, addr := range stateDB.cachedContractAddrs() {
					c := stateDB.cachedContract[addr]
					s += string(c.SelfState().Root[:])
				}
			} else {
				for _, c := range stateDB.cachedContract {
					s += string(c.SelfState().Root[:])
				}
			}

			caches = append(caches, s)
		}
		return uniqueSlice(caches)
	}
	require := require.New(t)

	ctrl := gomock.NewController(t)
	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	t.Run("before fix sort map", func(t *testing.T) {
		require.False(testFunc(t, sm))
	})

	t.Run("after fix sort map", func(t *testing.T) {
		require.True(testFunc(t, sm, SortCachedContractsOption()))
	})
}

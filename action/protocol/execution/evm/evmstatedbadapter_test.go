// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
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
		func(opts ...protocol.StateOption) (uint64, error) {
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
	sm.EXPECT().Snapshot().DoAndReturn(cb.Snapshot).AnyTimes()
	sm.EXPECT().Revert(gomock.Any()).DoAndReturn(cb.RevertSnapshot).AnyTimes()
	return sm, nil
}

func TestAddBalance(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB, err := NewStateDBAdapter(
		sm,
		1,
		hash.ZeroHash256,
		NotFixTopicCopyBugOption(),
		FixSnapshotOrderOption(),
	)
	require.NoError(err)
	addAmount := big.NewInt(40000)
	stateDB.AddBalance(addr, uint256.MustFromBig(addAmount))
	require.Equal(addAmount, stateDB.lastAddBalanceAmount)
	beneficiary, _ := address.FromBytes(addr[:])
	require.Equal(beneficiary.String(), stateDB.lastAddBalanceAddr)
	amount := stateDB.GetBalance(addr)
	require.Equal(amount.ToBig(), addAmount)
	stateDB.AddBalance(addr, uint256.MustFromBig(addAmount))
	amount = stateDB.GetBalance(addr)
	require.Equal(amount, uint256.NewInt(80000))
	stateDB.AddBalance(addr, common.U2560)
	require.Zero(len(stateDB.lastAddBalanceAmount.Bytes()))
}

func TestRefundAPIs(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	stateDB, err := NewStateDBAdapter(
		sm,
		1,
		hash.ZeroHash256,
		NotFixTopicCopyBugOption(),
		FixSnapshotOrderOption(),
	)
	require.NoError(err)
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
	stateDB, err := NewStateDBAdapter(
		sm,
		1,
		hash.ZeroHash256,
		NotFixTopicCopyBugOption(),
		FixSnapshotOrderOption(),
	)
	require.NoError(err)
	require.True(stateDB.Empty(addr))
	stateDB.CreateAccount(addr)
	require.True(stateDB.Empty(addr))
	stateDB.SetCode(addr, []byte("0123456789"))
	require.True(bytes.Equal(stateDB.GetCode(addr), []byte("0123456789")))
	require.False(stateDB.Empty(addr))
}

var kvs = map[common.Hash]common.Hash{
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

func TestForEachStorage(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB, err := NewStateDBAdapter(
		sm,
		1,
		hash.ZeroHash256,
		NotFixTopicCopyBugOption(),
		FixSnapshotOrderOption(),
	)
	require.NoError(err)
	stateDB.CreateAccount(addr)
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

func TestReadContractStorage(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB, err := NewStateDBAdapter(
		sm,
		1,
		hash.ZeroHash256,
		AsyncContractTrieOption(),
		FixSnapshotOrderOption(),
	)
	require.NoError(err)
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
	stateDB.CommitContracts()

	ctx := protocol.WithBlockchainCtx(protocol.WithFeatureCtx(protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), genesis.TestDefault()),
		protocol.BlockCtx{BlockHeight: genesis.TestDefault().MidwayBlockHeight})),
		protocol.BlockchainCtx{})
	for k, v := range kvs {
		b, err := ReadContractStorage(ctx, sm, addr, k[:])
		require.NoError(err)
		require.Equal(v[:], b)
	}
}

func TestNonce(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	t.Run("legacy nonce account with confirmed nonce", func(t *testing.T) {
		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		opt := []StateDBAdapterOption{
			NotFixTopicCopyBugOption(),
			FixSnapshotOrderOption(),
			LegacyNonceAccountOption(),
			UseConfirmedNonceOption(),
		}
		stateDB, err := NewStateDBAdapter(sm, 1, hash.ZeroHash256, opt...)
		require.NoError(err)
		require.Equal(uint64(0), stateDB.GetNonce(addr))
		stateDB.SetNonce(addr, 1)
		require.Equal(uint64(1), stateDB.GetNonce(addr))
	})
	t.Run("legacy nonce account with pending nonce", func(t *testing.T) {
		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		opt := []StateDBAdapterOption{
			NotFixTopicCopyBugOption(),
			FixSnapshotOrderOption(),
			LegacyNonceAccountOption(),
		}
		stateDB, err := NewStateDBAdapter(sm, 1, hash.ZeroHash256, opt...)
		require.NoError(err)
		require.Equal(uint64(1), stateDB.GetNonce(addr))
		stateDB.SetNonce(addr, 2)
		require.Equal(uint64(2), stateDB.GetNonce(addr))
	})
	t.Run("zero nonce account with confirmed nonce", func(t *testing.T) {
		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		opt := []StateDBAdapterOption{
			NotFixTopicCopyBugOption(),
			FixSnapshotOrderOption(),
			UseConfirmedNonceOption(),
		}
		_, err = NewStateDBAdapter(sm, 1, hash.ZeroHash256, opt...)
		require.Error(err)
	})
	t.Run("zero nonce account with pending nonce", func(t *testing.T) {
		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		opt := []StateDBAdapterOption{
			NotFixTopicCopyBugOption(),
			FixSnapshotOrderOption(),
		}
		stateDB, err := NewStateDBAdapter(sm, 1, hash.ZeroHash256, opt...)
		require.NoError(err)
		require.Equal(uint64(0), stateDB.GetNonce(addr))
		stateDB.SetNonce(addr, 1)
		require.Equal(uint64(1), stateDB.GetNonce(addr))
	})
	t.Run("legacy fresh nonce account with pending nonce", func(t *testing.T) {
		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		opt := []StateDBAdapterOption{
			NotFixTopicCopyBugOption(),
			FixSnapshotOrderOption(),
			ZeroNonceForFreshAccountOption(),
		}
		stateDB, err := NewStateDBAdapter(sm, 1, hash.ZeroHash256, opt...)
		require.NoError(err)
		require.Equal(uint64(0), stateDB.GetNonce(addr))
		stateDB.SetNonce(addr, 1)
		require.Equal(uint64(1), stateDB.GetNonce(addr))
	})
}

var tests = []stateDBTest{
	{
		[]bal{
			{_addr1, big.NewInt(40000)},
		},
		[]code{
			{_c1, _bytecode},
		},
		[]evmSet{
			{_c1, _k1, _v1},
			{_c1, _k2, _v2},
			{_c3, _k3, _v4},
		},
		15000,
		[]sui{
			{nil, _c4, _c2, false, false},
			{nil, _c2, _c4, false, false},
		},
		[]image{
			{common.BytesToHash(_v1[:]), []byte("cat")},
			{common.BytesToHash(_v2[:]), []byte("dog")},
		},
		[]access{
			{_c1, []common.Hash{_k1, _k2}, []common.Hash{_k3, _k4}, false},
		},
		[]transient{
			{_c1, _k1, _v1},
		},
		[]*types.Log{
			newTestLog(_c3), newTestLog(_c2), newTestLog(_c1),
		},
		3, 0, 1,
		"io1q87zge3ngux0v2hz49tdy85dfqwr560pj9mk7r", "io1q87zge3ngux0v2hz49tdy85dfqwr560pj9mk7r", "",
	},
	{
		[]bal{
			{_addr1, big.NewInt(40000)},
		},
		[]code{
			{_c2, _bytecode},
		},
		[]evmSet{
			{_c1, _k1, _v3},
			{_c1, _k2, _v4},
			{_c2, _k3, _v3},
			{_c2, _k4, _v4},
		},
		2000,
		[]sui{
			{nil, _c4, _c1, true, true},
			{big.NewInt(1000), _c2, _c3, true, true},
		},
		[]image{
			{common.BytesToHash(_v3[:]), []byte("hen")},
		},
		[]access{
			{_c1, []common.Hash{_k3, _k4}, nil, true},
			{_c2, []common.Hash{_k1, _k3}, []common.Hash{_k2, _k4}, false},
		},
		[]transient{
			{_c2, _k2, _v2},
		},
		[]*types.Log{
			newTestLog(_c4),
		},
		4, 1, 2,
		"io1zg0qrlpyvc68pnmz4c4f2mfc6jqu8f57jjy09q",
		"io1j4kjr6x5s8p6dyqlcfrxxdrsea32u2hpvpl5us",
		"io1x3cv7c4w922k6wx5s8p6d8sjrcqlcfrxhkn5xe",
	},
	{
		nil,
		nil,
		[]evmSet{
			{_c2, _k3, _v1},
			{_c2, _k4, _v2},
		},
		15000,
		[]sui{
			{big.NewInt(0), _c1, _addr1, true, true},
		},
		[]image{
			{common.BytesToHash(_v4[:]), []byte("fox")},
		},
		[]access{
			{_c2, []common.Hash{_k2, _k4}, nil, true},
		},
		[]transient{
			{_c3, _k3, _v3},
		},
		[]*types.Log{
			newTestLog(_c1), newTestLog(_c2),
		},
		6, 2, 3,
		"io1x3cv7c4w922k6wx5s8p6d8sjrcqlcfrxhkn5xe",
		"io1q2hz49tdy85dfqwr560pge3ngux0vf0vmhanad",
		"io1q87zge3ngux0v2hz49tdy85dfqwr560pj9mk7r",
	},
}

func TestSnapshotRevertAndCommit(t *testing.T) {
	testSnapshotAndRevert := func(t *testing.T, async, fixSnapshot, revertLog bool) {
		require := require.New(t)
		ctrl := gomock.NewController(t)

		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		opt := []StateDBAdapterOption{
			NotFixTopicCopyBugOption(),
			SuicideTxLogMismatchPanicOption(),
			EnableCancunEVMOption(),
		}
		if async {
			opt = append(opt, AsyncContractTrieOption())
		}
		if fixSnapshot {
			opt = append(opt, FixSnapshotOrderOption())
		}
		if revertLog {
			opt = append(opt, RevertLogOption())
		}
		stateDB, err := NewStateDBAdapter(sm, 1, hash.ZeroHash256, opt...)
		require.NoError(err)

		for i, test := range tests {
			// add balance
			for _, e := range test.balance {
				stateDB.AddBalance(e.addr, uint256.MustFromBig(e.v))
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
			// set refund
			stateDB.refund = test.refund
			// set SelfDestruct
			for _, e := range test.selfDestruct {
				if e.amount != nil {
					stateDB.AddBalance(e.addr, uint256.MustFromBig(e.amount))
				}
				stateDB.AddBalance(e.beneficiary, stateDB.GetBalance(e.addr)) // simulate transfer to beneficiary inside Suicide()
				stateDB.SelfDestruct(e.addr)
				require.Equal(e.exist, stateDB.Exist(e.addr))
				require.Zero(new(uint256.Int).Cmp(stateDB.GetBalance(e.addr)))
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
			//set transient storage
			for _, e := range test.transient {
				stateDB.SetTransientState(e.addr, e.k, e.v)
			}
			// set logs and txLogs
			for _, l := range test.logs {
				stateDB.AddLog(l)
			}
			require.Equal(test.logSize, len(stateDB.logs))
			require.Equal(test.txLogSize, len(stateDB.transactionLogs))
			require.Equal(test.transientSize, len(stateDB.transientStorage))
			require.Equal(test.logAddr, stateDB.logs[test.logSize-1].Address)
			if test.txLogSize > 0 {
				require.Equal(test.txSender, stateDB.transactionLogs[test.txLogSize-1].Sender)
				require.Equal(test.txReceiver, stateDB.transactionLogs[test.txLogSize-1].Recipient)
			}
			require.Equal(i, stateDB.Snapshot())
		}

		reverts := []stateDBTest{
			{
				[]bal{
					{_addr1, big.NewInt(0)},
				},
				[]code{},
				[]evmSet{
					{_c1, _k1, _v3},
					{_c1, _k2, _v4},
					{_c2, _k3, _v1},
					{_c2, _k4, _v2},
				},
				tests[2].refund,
				[]sui{
					{nil, common.Address{}, _c1, true, true},
					{nil, common.Address{}, _c3, true, true},
					{nil, common.Address{}, _c2, false, true},
					{nil, common.Address{}, _c4, false, false},
					{nil, common.Address{}, _addr1, true, true},
				},
				[]image{
					{common.BytesToHash(_v1[:]), []byte("cat")},
					{common.BytesToHash(_v2[:]), []byte("dog")},
					{common.BytesToHash(_v3[:]), []byte("hen")},
					{common.BytesToHash(_v4[:]), []byte("fox")},
				},
				[]access{
					{_c1, []common.Hash{_k1, _k2, _k3, _k4}, nil, true},
					{_c2, []common.Hash{_k1, _k2, _k3, _k4}, nil, true},
				},
				[]transient{
					{_c3, _k3, _v3},
				},
				nil,
				6, 2, 3,
				"io1x3cv7c4w922k6wx5s8p6d8sjrcqlcfrxhkn5xe",
				"io1q2hz49tdy85dfqwr560pge3ngux0vf0vmhanad",
				"io1q87zge3ngux0v2hz49tdy85dfqwr560pj9mk7r",
			},
			{
				[]bal{
					{_addr1, big.NewInt(80000)},
				},
				[]code{},
				tests[1].states,
				tests[1].refund,
				[]sui{
					{nil, common.Address{}, _c1, true, true},
					{nil, common.Address{}, _c3, true, true},
					{nil, common.Address{}, _c2, false, true},
					{nil, common.Address{}, _c4, false, false},
					{nil, common.Address{}, _addr1, false, true},
				},
				[]image{
					{common.BytesToHash(_v1[:]), []byte("cat")},
					{common.BytesToHash(_v2[:]), []byte("dog")},
					{common.BytesToHash(_v3[:]), []byte("hen")},
					{common.BytesToHash(_v4[:]), []byte(nil)},
				},
				[]access{
					{_c1, []common.Hash{_k1, _k2, _k3, _k4}, nil, true},
					{_c2, []common.Hash{_k1, _k3}, []common.Hash{_k2, _k4}, true},
				},
				[]transient{
					{_c2, _k2, _v2},
				},
				nil,
				4, 1, 2,
				"io1zg0qrlpyvc68pnmz4c4f2mfc6jqu8f57jjy09q",
				"io1j4kjr6x5s8p6dyqlcfrxxdrsea32u2hpvpl5us",
				"io1x3cv7c4w922k6wx5s8p6d8sjrcqlcfrxhkn5xe",
			},
			{
				[]bal{
					{_addr1, big.NewInt(40000)},
				},
				[]code{},
				[]evmSet{
					{_c1, _k1, _v1},
					{_c1, _k2, _v2},
					{_c3, _k3, _v4},
				},
				tests[0].refund,
				[]sui{
					{nil, common.Address{}, _c1, false, true},
					{nil, common.Address{}, _c3, false, true},
					{nil, common.Address{}, _c2, false, false},
					{nil, common.Address{}, _c4, false, false},
					{nil, common.Address{}, _addr1, false, true},
				},
				[]image{
					{common.BytesToHash(_v1[:]), []byte("cat")},
					{common.BytesToHash(_v2[:]), []byte("dog")},
					{common.BytesToHash(_v3[:]), []byte(nil)},
					{common.BytesToHash(_v4[:]), []byte(nil)},
				},
				[]access{
					{_c1, []common.Hash{_k1, _k2}, []common.Hash{_k3, _k4}, true},
					{_c2, nil, []common.Hash{_k1, _k2, _k3, _k4}, false},
				},
				[]transient{
					{_c1, _k1, _v1},
				},
				nil,
				3, 0, 1,
				"io1q87zge3ngux0v2hz49tdy85dfqwr560pj9mk7r",
				"io1q87zge3ngux0v2hz49tdy85dfqwr560pj9mk7r",
				"",
			},
		}

		// test revert
		for i, test := range reverts {
			stateDB.RevertToSnapshot(len(reverts) - 1 - i)
			// test balance
			for _, e := range test.balance {
				amount := stateDB.GetBalance(e.addr)
				require.Equal(uint256.MustFromBig(e.v), amount)
			}
			if async && !fixSnapshot {
				// test preimage
				for _, e := range reverts[0].preimage {
					v := stateDB.preimages[e.hash]
					require.Equal(e.v, []byte(v))
				}
			} else {
				// test states
				for _, e := range test.states {
					require.Equal(e.v, stateDB.GetState(e.addr, e.k))
				}
				// test refund
				require.Equal(test.refund, stateDB.refund)
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
				//test transient storage
				for _, e := range test.transient {
					require.Equal(e.v, stateDB.GetTransientState(e.addr, e.k))
				}
			}
			// test SelfDestruct/exist
			for _, e := range test.selfDestruct {
				require.Equal(e.selfDestruct, stateDB.HasSelfDestructed(e.addr))
				require.Equal(e.exist, stateDB.Exist(e.addr))
			}
			// test logs
			if revertLog {
				require.Equal(test.logSize, len(stateDB.logs))
				require.Equal(test.txLogSize, len(stateDB.transactionLogs))
				require.Equal(test.logAddr, stateDB.logs[test.logSize-1].Address)
				if test.txLogSize > 0 {
					require.Equal(test.txSender, stateDB.transactionLogs[test.txLogSize-1].Sender)
					require.Equal(test.txReceiver, stateDB.transactionLogs[test.txLogSize-1].Recipient)
				}
			} else {
				require.Equal(6, len(stateDB.logs))
				require.Equal(2, len(stateDB.transactionLogs))
				require.Equal("io1x3cv7c4w922k6wx5s8p6d8sjrcqlcfrxhkn5xe", stateDB.logs[5].Address)
				require.Equal("io1q2hz49tdy85dfqwr560pge3ngux0vf0vmhanad", stateDB.transactionLogs[1].Sender)
				require.Equal("io1q87zge3ngux0v2hz49tdy85dfqwr560pj9mk7r", stateDB.transactionLogs[1].Recipient)
			}
		}

		// snapshot after revert
		require.Equal(1, stateDB.Snapshot())
		if fixSnapshot {
			require.Equal(1, len(stateDB.contractSnapshot))
			require.Equal(1, len(stateDB.selfDestructedSnapshot))
			require.Equal(1, len(stateDB.preimageSnapshot))
			require.Equal(1, len(stateDB.accessListSnapshot))
			require.Equal(1, len(stateDB.transientStorageSnapshot))
			require.Equal(1, len(stateDB.refundSnapshot))
		} else {
			require.Equal(3, len(stateDB.contractSnapshot))
			require.Equal(3, len(stateDB.selfDestructedSnapshot))
			require.Equal(3, len(stateDB.preimageSnapshot))
			// refund fix, accessList, and transient storage are introduced after fixSnapshot
			// so their snapshot are always properly cleared
			require.Zero(len(stateDB.accessListSnapshot))
			require.Zero(len(stateDB.transientStorageSnapshot))
			require.Zero(len(stateDB.refundSnapshot))
		}
		// commit snapshot 0's state
		require.NoError(stateDB.CommitContracts())
		stateDB.clear()
		//[TODO] need e2etest to verify state factory commit/re-open (whether result from state/balance/SelfDestruct/exist is same)
	}

	t.Run("contract snapshot/revert/commit", func(t *testing.T) {
		testSnapshotAndRevert(t, false, true, false)
	})
	t.Run("contract snapshot/revert/commit w/o bug fix and revert log", func(t *testing.T) {
		testSnapshotAndRevert(t, false, false, true)
	})
	t.Run("contract snapshot/revert/commit with async trie and revert log", func(t *testing.T) {
		testSnapshotAndRevert(t, true, true, true)
	})
	t.Run("contract snapshot/revert/commit with async trie and w/o bug fix", func(t *testing.T) {
		testSnapshotAndRevert(t, true, false, false)
	})
}

func TestClearSnapshots(t *testing.T) {
	testClearSnapshots := func(t *testing.T, async, fixSnapshotOrder, panicUnrecoverableError bool) {
		require := require.New(t)
		ctrl := gomock.NewController(t)

		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		opts := []StateDBAdapterOption{
			NotFixTopicCopyBugOption(),
			RevertLogOption(),
		}
		if async {
			opts = append(opts, AsyncContractTrieOption())
		}
		if fixSnapshotOrder {
			opts = append(opts, FixSnapshotOrderOption())
		}
		if panicUnrecoverableError {
			opts = append(opts, PanicUnrecoverableErrorOption())
		}
		stateDB, err := NewStateDBAdapter(sm, 1, hash.ZeroHash256, opts...)
		require.NoError(err)

		for i, test := range tests {
			// add balance
			for _, e := range test.balance {
				stateDB.AddBalance(e.addr, uint256.MustFromBig(e.v))
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
			// set SelfDestruct
			for _, e := range test.selfDestruct {
				stateDB.SelfDestruct(e.addr)
				require.Equal(e.exist, stateDB.Exist(e.addr))
			}
			// set preimage
			for _, e := range test.preimage {
				stateDB.AddPreimage(e.hash, e.v)
			}
			require.Equal(i, stateDB.Snapshot())
		}

		// revert to snapshot 1
		stateDB.RevertToSnapshot(1)
		require.Equal(1, len(stateDB.logsSnapshot))

		if stateDB.fixSnapshotOrder {
			// snapshot 1, 2 cleared, only 0 left in map
			require.Equal(1, len(stateDB.selfDestructedSnapshot))
			require.Equal(1, len(stateDB.contractSnapshot))
			require.Equal(1, len(stateDB.preimageSnapshot))
			require.Equal(2, stateDB.Snapshot())
			// now there are 2 snapshots: 0 and the newly added one
			require.Equal(2, len(stateDB.selfDestructedSnapshot))
			require.Equal(2, len(stateDB.contractSnapshot))
			require.Equal(2, len(stateDB.preimageSnapshot))
			require.Equal(2, len(stateDB.logsSnapshot))
		} else {
			// snapshot not cleared
			require.Equal(3, len(stateDB.selfDestructedSnapshot))
			require.Equal(3, len(stateDB.contractSnapshot))
			require.Equal(3, len(stateDB.preimageSnapshot))
			require.Equal(2, stateDB.Snapshot())
			// still 3 old snapshots
			require.Equal(3, len(stateDB.selfDestructedSnapshot))
			require.Equal(3, len(stateDB.contractSnapshot))
			require.Equal(3, len(stateDB.preimageSnapshot))
			// log snapshot added after fixSnapshotOrder, so it is cleared and 1 remains
			require.Equal(1, len(stateDB.logsSnapshot))
		}
		if panicUnrecoverableError {
			require.Panics(func() { stateDB.RevertToSnapshot(1) })
		} else {
			stateDB.RevertToSnapshot(1)
		}
	}
	t.Run("contract w/o clear snapshots", func(t *testing.T) {
		testClearSnapshots(t, false, false, false)
	})
	t.Run("contract with clear snapshots", func(t *testing.T) {
		testClearSnapshots(t, false, true, false)
	})
	t.Run("contract with clear snapshots and panic on duplicate revert", func(t *testing.T) {
		testClearSnapshots(t, false, true, true)
	})
}

func TestGetCommittedState(t *testing.T) {
	t.Run("committed state with in mem DB", func(t *testing.T) {
		require := require.New(t)
		ctrl := gomock.NewController(t)

		sm, err := initMockStateManager(ctrl)
		require.NoError(err)
		stateDB, err := NewStateDBAdapter(
			sm,
			1,
			hash.ZeroHash256,
			NotFixTopicCopyBugOption(),
			FixSnapshotOrderOption(),
		)
		require.NoError(err)

		stateDB.SetState(_c1, _k1, _v1)
		// _k2 does not exist
		require.Equal(common.Hash{}, stateDB.GetCommittedState(_c1, common.BytesToHash(_k2[:])))
		require.Equal(_v1, stateDB.GetState(_c1, _k1))
		require.Equal(common.Hash{}, stateDB.GetCommittedState(_c1, common.BytesToHash(_k2[:])))

		// commit (_k1, _v1)
		require.NoError(stateDB.CommitContracts())
		stateDB.clear()

		require.Equal(_v1, stateDB.GetState(_c1, _k1))
		require.Equal(common.BytesToHash(_v1[:]), stateDB.GetCommittedState(_c1, common.BytesToHash(_k1[:])))
		stateDB.SetState(_c1, _k1, _v2)
		require.Equal(common.BytesToHash(_v1[:]), stateDB.GetCommittedState(_c1, common.BytesToHash(_k1[:])))
		require.Equal(_v2, stateDB.GetState(_c1, _k1))
		require.Equal(common.BytesToHash(_v1[:]), stateDB.GetCommittedState(_c1, common.BytesToHash(_k1[:])))
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
		stateDB, err := NewStateDBAdapter(
			sm,
			1,
			hash.ZeroHash256,
			NotFixTopicCopyBugOption(),
			FixSnapshotOrderOption(),
		)
		assert.NoError(t, err)
		amount := stateDB.GetBalance(addr)
		assert.Equal(t, common.U2560, amount)
	}
}

func TestPreimage(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	stateDB, err := NewStateDBAdapter(
		sm,
		1,
		hash.ZeroHash256,
		NotFixTopicCopyBugOption(),
		FixSnapshotOrderOption(),
	)
	require.NoError(err)

	stateDB.AddPreimage(common.BytesToHash(_v1[:]), []byte("cat"))
	stateDB.AddPreimage(common.BytesToHash(_v2[:]), []byte("dog"))
	stateDB.AddPreimage(common.BytesToHash(_v3[:]), []byte("hen"))
	// this won't overwrite preimage of _v1
	stateDB.AddPreimage(common.BytesToHash(_v1[:]), []byte("fox"))
	require.NoError(stateDB.CommitContracts())
	stateDB.clear()
	var k protocol.SerializableBytes
	_, err = stateDB.sm.State(&k, protocol.NamespaceOption(PreimageKVNameSpace), protocol.KeyOption(_v1[:]))
	require.NoError(err)
	require.Equal([]byte("cat"), []byte(k))
	_, err = stateDB.sm.State(&k, protocol.NamespaceOption(PreimageKVNameSpace), protocol.KeyOption(_v2[:]))
	require.NoError(err)
	require.Equal([]byte("dog"), []byte(k))
	_, err = stateDB.sm.State(&k, protocol.NamespaceOption(PreimageKVNameSpace), protocol.KeyOption(_v3[:]))
	require.NoError(err)
	require.Equal([]byte("hen"), []byte(k))
}

func TestSortMap(t *testing.T) {
	require := require.New(t)
	uniqueSlice := func(slice []string) bool {
		for _, v := range slice[1:] {
			if v != slice[0] {
				return false
			}
		}
		return true
	}

	testFunc := func(t *testing.T, sm *mock_chainmanager.MockStateManager, opts ...StateDBAdapterOption) bool {
		opts = append(opts,
			NotFixTopicCopyBugOption(),
			FixSnapshotOrderOption(),
		)
		stateDB, err := NewStateDBAdapter(sm, 1, hash.ZeroHash256, opts...)
		require.NoError(err)
		size := 10

		for i := 0; i < size; i++ {
			addr := common.HexToAddress(identityset.Address(i).Hex())
			stateDB.SetCode(addr, []byte("0123456789"))
			stateDB.SetState(addr, _k1, _k2)
		}
		sn := stateDB.Snapshot()
		caches := []string{}
		for i := 0; i < size; i++ {
			stateDB.RevertToSnapshot(sn)
			s := ""
			if stateDB.disableSortCachedContracts {
				for _, c := range stateDB.cachedContract {
					s += string(c.SelfState().Root[:])
				}
			} else {
				for _, addr := range stateDB.cachedContractAddrs() {
					c := stateDB.cachedContract[addr]
					s += string(c.SelfState().Root[:])
				}
			}

			caches = append(caches, s)
		}
		return uniqueSlice(caches)
	}

	ctrl := gomock.NewController(t)
	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	t.Run("before fix sort map", func(t *testing.T) {
		require.False(testFunc(t, sm, DisableSortCachedContractsOption()))
	})

	t.Run("after fix sort map", func(t *testing.T) {
		require.True(testFunc(t, sm))
	})
}

func TestStateDBTransientStorage(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm, err := initMockStateManager(ctrl)
	require.NoError(err)
	var opts []StateDBAdapterOption
	opts = append(opts,
		NotFixTopicCopyBugOption(),
		FixSnapshotOrderOption(),
		EnableCancunEVMOption(),
	)
	state, err := NewStateDBAdapter(sm, 1, hash.ZeroHash256, opts...)
	require.NoError(err)
	var (
		addr0 = common.Address{}
		addr1 = common.HexToAddress("1234567890")
		k1    = common.Hash{}
		k2    = common.HexToHash("34567890ab")
		v1    = common.HexToHash("567890abcd")
		v2    = common.HexToHash("7890abcdef")
	)
	tests := []struct {
		addr     common.Address
		key, val common.Hash
	}{
		{addr0, k1, v1},
		{addr0, k2, v2},
		{addr1, k1, v2},
		{addr1, k2, v1},
	}
	for _, test := range tests {
		addr := test.addr
		key := test.key
		value := test.val
		sn := state.Snapshot()
		state.SetTransientState(addr, key, value)
		require.Equal(value, state.GetTransientState(addr, key))

		// revert the transient state being set and then check that the
		// value is now the empty hash
		state.RevertToSnapshot(sn)
		require.Equal(common.Hash{}, state.GetTransientState(addr, key))

		state.SetTransientState(addr, key, value)
		require.Equal(value, state.GetTransientState(addr, key))
	}

}

func TestSelfdestruct6780(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm, err := initMockStateManager(ctrl)
	r.NoError(err)
	var opts []StateDBAdapterOption
	opts = append(opts,
		FixSnapshotOrderOption(),
		EnableCancunEVMOption(),
	)
	state := MustNoErrorV(NewStateDBAdapter(sm, 1, hash.ZeroHash256, opts...))
	r.NoError(err)
	_, err = accountutil.LoadOrCreateAccount(state.sm, _c4)
	state.AddBalance(_c4, uint256.NewInt(100))
	r.NoError(state.CommitContracts())
	state.clear()
	acc := MustNoErrorV(accountutil.LoadOrCreateAccount(state.sm, _c4))
	r.Equal(big.NewInt(100), acc.Balance)
	r.False(acc.IsContract())
	r.NoError(err)
	addrs := []common.Address{_c1, _c2, _c3, _c4}
	for _, addr := range addrs {
		if addr == _c4 {
			state.SetCode(addr, []byte{1, 2})
		} else {
			state.CreateAccount(addr)
		}
		state.Selfdestruct6780(addr)
		r.True(state.Exist(addr))
		state.Snapshot()
	}
	r.NoError(state.CommitContracts())
	for _, addr := range addrs[:3] {
		r.False(state.Exist(addr))
	}
	r.True(state.Exist(_c4)) // pre-existing account gets overwritten, but not deleted
	acc = MustNoErrorV(accountutil.LoadOrCreateAccount(state.sm, _c4))
	r.Equal(big.NewInt(100), acc.Balance)
	r.True(acc.IsContract())
	state.RevertToSnapshot(3)
	r.Equal(createdAccount{
		_c1: struct{}{}, _c2: struct{}{}, _c3: struct{}{}}, state.createdAccount)
	state.RevertToSnapshot(2)
	r.Equal(createdAccount{
		_c1: struct{}{}, _c2: struct{}{}, _c3: struct{}{}}, state.createdAccount)
	state.RevertToSnapshot(1)
	r.Equal(createdAccount{
		_c1: struct{}{}, _c2: struct{}{}}, state.createdAccount)
	state.RevertToSnapshot(0)
	r.Equal(createdAccount{
		_c1: struct{}{}}, state.createdAccount)
}

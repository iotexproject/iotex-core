// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// sstoreOp is one storage access done by the test contract.
type sstoreOp struct {
	load  bool // SLOAD the slot (and POP the value) instead of SSTORE
	slot  byte
	value byte
}

func sload(slot byte) sstoreOp         { return sstoreOp{load: true, slot: slot} }
func sstore(slot, value byte) sstoreOp { return sstoreOp{slot: slot, value: value} }

func assembleStorageOps(ops []sstoreOp) []byte {
	var code []byte
	for _, op := range ops {
		if op.load {
			code = append(code, 0x60, op.slot, 0x54, 0x50) // PUSH1 slot SLOAD POP
		} else {
			code = append(code, 0x60, op.value, 0x60, op.slot, 0x55) // PUSH1 value PUSH1 slot SSTORE
		}
	}
	return append(code, 0x00) // STOP
}

// initCode stores the prestate in the constructor and returns runtime as the
// contract code.
func initCode(prestate []sstoreOp, runtime []byte) []byte {
	code := assembleStorageOps(prestate)
	code = code[:len(code)-1] // drop STOP
	const tailLen = 11
	offset := len(code) + tailLen
	code = append(code,
		0x60, byte(len(runtime)), // PUSH1 len
		0x80,               // DUP1
		0x60, byte(offset), // PUSH1 offset
		0x60, 0x00, // PUSH1 0
		0x39,       // CODECOPY
		0x60, 0x00, // PUSH1 0
		0xf3, // RETURN
	)
	return append(code, runtime...)
}

// TestSStoreGasPinned pins the receipt gas and the resulting storage of SSTORE
// sequences at the heights where IoTeX switched SSTORE metering (legacy,
// EIP-2200, EIP-2929/3529) and around Zanzibar, which turned on
// CorrectPrestateForAbsentKeys. The expected values were produced with the
// go-ethereum v1.15.11 based fork (iotexproject/go-ethereum a8ad6229e289),
// before the SSTORE gas functions switched to GetStateAndCommittedState, so
// the test guards that the dependency bump keeps gas and state unchanged.
//
// Slot 1 holds 1 before the call; slot 0 is absent from the storage trie.
func TestSStoreGasPinned(t *testing.T) {
	g := genesis.TestDefault()
	// TestDefault does not schedule Zanzibar, use the MainNet height
	g.ZanzibarBlockHeight = 53533081
	heights := []struct {
		name   string
		height uint64
	}{
		{"Bering", g.BeringBlockHeight},
		{"Greenland", g.GreenlandBlockHeight},
		{"Iceland", g.IcelandBlockHeight},
		{"Okhotsk", g.OkhotskBlockHeight},
		{"Vanuatu", g.VanuatuBlockHeight},
		{"Yap", g.YapBlockHeight},
		{"Zanzibar-1", g.ZanzibarBlockHeight - 1},
		{"Zanzibar", g.ZanzibarBlockHeight},
	}
	scenarios := []struct {
		name string
		ops  []sstoreOp
	}{
		{"absent 0->0", []sstoreOp{sstore(0, 0)}},
		{"absent 0->1", []sstoreOp{sstore(0, 1)}},
		{"existing 1->1", []sstoreOp{sstore(1, 1)}},
		{"existing 1->2", []sstoreOp{sstore(1, 2)}},
		{"existing 1->0", []sstoreOp{sstore(1, 0)}},
		{"absent 0->1->2", []sstoreOp{sstore(0, 1), sstore(0, 2)}},
		{"absent 0->1->0", []sstoreOp{sstore(0, 1), sstore(0, 0)}},
		{"absent 0->1->1->0", []sstoreOp{sstore(0, 1), sstore(0, 1), sstore(0, 0)}},
		{"existing 1->2->1", []sstoreOp{sstore(1, 2), sstore(1, 1)}},
		{"existing 1->0->0->1", []sstoreOp{sstore(1, 0), sstore(1, 0), sstore(1, 1)}},
		{"absent load 0->1 load ->2", []sstoreOp{sload(0), sstore(0, 1), sload(0), sstore(0, 2)}},
		{"absent load 0->1 load ->0", []sstoreOp{sload(0), sstore(0, 1), sload(0), sstore(0, 0)}},
		{"absent load 0->0 load ->1", []sstoreOp{sload(0), sstore(0, 0), sload(0), sstore(0, 1)}},
		{"existing load 1->2 load ->1", []sstoreOp{sload(1), sstore(1, 2), sload(1), sstore(1, 1)}},
	}

	var got []string
	for _, h := range heights {
		for _, sc := range scenarios {
			gas, slot0, slot1 := runSStoreScenario(t, g, h.height, sc.ops)
			got = append(got, fmt.Sprintf("%s/%s: gas=%d slot0=%d slot1=%d", h.name, sc.name, gas, slot0, slot1))
		}
	}
	require.Equal(t, strings.Split(strings.TrimSpace(_sstoreGasPinned), "\n"), got, "got:\n%s", strings.Join(got, "\n"))
}

// execEnv runs executions through ExecuteContract at a fixed height on top of
// a mocked state manager, with a funded caller.
type execEnv struct {
	t      *testing.T
	ctx    context.Context
	sm     protocol.StateManager
	caller address.Address
	height uint64
	nonce  uint64
}

func newExecEnv(t *testing.T, g genesis.Genesis, height uint64) *execEnv {
	require := require.New(t)
	sm, err := initMockStateManager(gomock.NewController(t))
	require.NoError(err)
	getBlockTime := func(h uint64) (time.Time, error) {
		return time.Unix(g.Timestamp, 0).Add(time.Duration(h) * 5 * time.Second), nil
	}
	ts, err := getBlockTime(height)
	require.NoError(err)
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    height,
		BlockTimeStamp: ts,
		GasLimit:       g.BlockGasLimitByHeight(height),
		Producer:       identityset.Address(27),
	})
	ctx = protocol.WithBlockchainCtx(protocol.WithFeatureCtx(ctx), protocol.BlockchainCtx{
		ChainID:      1,
		EvmNetworkID: 4689,
		GetBlockHash: func(uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
		GetBlockTime: getBlockTime,
	})
	ctx = WithHelperCtx(ctx, HelperContext{
		GetBlockHash: func(uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
		GetBlockTime: getBlockTime,
		DepositGasFunc: func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
			return nil, nil
		},
	})
	env := &execEnv{t: t, ctx: ctx, sm: sm, caller: identityset.Address(28), height: height}
	stateDB := env.stateDB()
	stateDB.AddBalance(common.BytesToAddress(env.caller.Bytes()), uint256.MustFromBig(unit.ConvertIotxToRau(1000)), 0)
	require.NoError(stateDB.CommitContracts())
	return env
}

// stateDB returns a fresh adapter over the committed state
func (env *execEnv) stateDB() *StateDBAdapter {
	stateDB, err := prepareStateDBAdapter(protocol.WithActionCtx(env.ctx, protocol.ActionCtx{Caller: env.caller}), env.sm)
	require.NoError(env.t, err)
	return stateDB
}

// exec runs one successful execution and returns its receipt
func (env *execEnv) exec(to string, data []byte) *action.Receipt {
	require := require.New(env.t)
	env.nonce++
	elp := (&action.EnvelopeBuilder{}).SetNonce(env.nonce).SetGasPrice(big.NewInt(0)).
		SetGasLimit(1_000_000).SetAction(action.NewExecution(to, big.NewInt(0), data)).Build()
	intrinsic, err := elp.IntrinsicGas()
	require.NoError(err)
	actx := protocol.WithActionCtx(env.ctx, protocol.ActionCtx{
		Caller:       env.caller,
		ActionHash:   hash.Hash256b(append([]byte{byte(env.nonce)}, data...)),
		Nonce:        env.nonce,
		IntrinsicGas: intrinsic,
	})
	_, receipt, err := ExecuteContract(actx, env.sm, elp)
	require.NoError(err)
	require.EqualValues(1, receipt.Status, "status at height %d", env.height)
	return receipt
}

func runSStoreScenario(t *testing.T, g genesis.Genesis, height uint64, ops []sstoreOp) (uint64, byte, byte) {
	env := newExecEnv(t, g, height)
	// deploy the contract with slot 1 = 1, then run the ops in a new tx
	deploy := env.exec(action.EmptyAddress, initCode([]sstoreOp{sstore(1, 1)}, assembleStorageOps(ops)))
	receipt := env.exec(deploy.ContractAddress, nil)

	contract, err := address.FromString(deploy.ContractAddress)
	require.NoError(t, err)
	addr := common.BytesToAddress(contract.Bytes())
	stateDB := env.stateDB()
	slot0 := stateDB.GetState(addr, common.Hash{})
	slot1 := stateDB.GetState(addr, common.BigToHash(big.NewInt(1)))
	return receipt.GasConsumed, slot0[31], slot1[31]
}

// TestCreateEmptyRuntimeCode pins that a contract creation whose initcode
// returns no code still sets the code (and code hash) of the new account.
// Upstream go-ethereum v1.16.6 stopped calling SetCode for empty code
// (go-ethereum #32916); IoTeX's fork keeps the call because skipping it would
// change the account state of such creations in historical blocks.
func TestCreateEmptyRuntimeCode(t *testing.T) {
	require := require.New(t)
	g := genesis.TestDefault()
	for _, height := range []uint64{g.BeringBlockHeight, g.IcelandBlockHeight, g.OkhotskBlockHeight, g.VanuatuBlockHeight, g.YapBlockHeight} {
		env := newExecEnv(t, g, height)
		receipt := env.exec(action.EmptyAddress, []byte{0x00}) // initcode: STOP
		contract, err := address.FromString(receipt.ContractAddress)
		require.NoError(err)
		acct, err := accountutil.LoadAccountByHash160(env.sm, hash.BytesToHash160(contract.Bytes()))
		require.NoError(err)
		require.True(acct.IsContract(), "height %d", height)
		require.Equal(hash.Hash256b(nil), hash.BytesToHash256(acct.CodeHash), "height %d", height)
		require.Empty(env.stateDB().GetCode(common.BytesToAddress(contract.Bytes())))
	}
}

// _sstoreGasPinned was generated by running this test against
// iotexproject/go-ethereum a8ad6229e289 (iotex-core 8c01855b9).
const _sstoreGasPinned = `
Bering/absent 0->0: gas=15006 slot0=0 slot1=1
Bering/absent 0->1: gas=30006 slot0=1 slot1=1
Bering/existing 1->1: gas=15006 slot0=0 slot1=1
Bering/existing 1->2: gas=15006 slot0=0 slot1=2
Bering/existing 1->0: gas=7503 slot0=0 slot1=0
Bering/absent 0->1->2: gas=35012 slot0=2 slot1=1
Bering/absent 0->1->0: gas=20012 slot0=0 slot1=1
Bering/absent 0->1->1->0: gas=25018 slot0=0 slot1=1
Bering/existing 1->2->1: gas=20012 slot0=0 slot1=1
Bering/existing 1->0->0->1: gas=25018 slot0=0 slot1=1
Bering/absent load 0->1 load ->2: gas=35422 slot0=2 slot1=1
Bering/absent load 0->1 load ->0: gas=20422 slot0=0 slot1=1
Bering/absent load 0->0 load ->1: gas=35422 slot0=1 slot1=1
Bering/existing load 1->2 load ->1: gas=20422 slot0=0 slot1=1
Greenland/absent 0->0: gas=15006 slot0=0 slot1=1
Greenland/absent 0->1: gas=30006 slot0=1 slot1=1
Greenland/existing 1->1: gas=15006 slot0=0 slot1=1
Greenland/existing 1->2: gas=15006 slot0=0 slot1=2
Greenland/existing 1->0: gas=7503 slot0=0 slot1=0
Greenland/absent 0->1->2: gas=35012 slot0=2 slot1=1
Greenland/absent 0->1->0: gas=20012 slot0=0 slot1=1
Greenland/absent 0->1->1->0: gas=25018 slot0=0 slot1=1
Greenland/existing 1->2->1: gas=20012 slot0=0 slot1=1
Greenland/existing 1->0->0->1: gas=25018 slot0=0 slot1=1
Greenland/absent load 0->1 load ->2: gas=35422 slot0=2 slot1=1
Greenland/absent load 0->1 load ->0: gas=20422 slot0=0 slot1=1
Greenland/absent load 0->0 load ->1: gas=35422 slot0=1 slot1=1
Greenland/existing load 1->2 load ->1: gas=20422 slot0=0 slot1=1
Iceland/absent 0->0: gas=10806 slot0=0 slot1=1
Iceland/absent 0->1: gas=30006 slot0=1 slot1=1
Iceland/existing 1->1: gas=10806 slot0=0 slot1=1
Iceland/existing 1->2: gas=15006 slot0=0 slot1=2
Iceland/existing 1->0: gas=7503 slot0=0 slot1=0
Iceland/absent 0->1->2: gas=35012 slot0=2 slot1=1
Iceland/absent 0->1->0: gas=20012 slot0=0 slot1=1
Iceland/absent 0->1->1->0: gas=20818 slot0=0 slot1=1
Iceland/existing 1->2->1: gas=11612 slot0=0 slot1=1
Iceland/existing 1->0->0->1: gas=12418 slot0=0 slot1=1
Iceland/absent load 0->1 load ->2: gas=36622 slot0=2 slot1=1
Iceland/absent load 0->1 load ->0: gas=21622 slot0=0 slot1=1
Iceland/absent load 0->0 load ->1: gas=32422 slot0=1 slot1=1
Iceland/existing load 1->2 load ->1: gas=13222 slot0=0 slot1=1
Okhotsk/absent 0->0: gas=12206 slot0=0 slot1=1
Okhotsk/absent 0->1: gas=32106 slot0=1 slot1=1
Okhotsk/existing 1->1: gas=12206 slot0=0 slot1=1
Okhotsk/existing 1->2: gas=15006 slot0=0 slot1=2
Okhotsk/existing 1->0: gas=12005 slot0=0 slot1=0
Okhotsk/absent 0->1->2: gas=35012 slot0=2 slot1=1
Okhotsk/absent 0->1->0: gas=30212 slot0=0 slot1=1
Okhotsk/absent 0->1->1->0: gas=30318 slot0=0 slot1=1
Okhotsk/existing 1->2->1: gas=12312 slot0=0 slot1=1
Okhotsk/existing 1->0->0->1: gas=12418 slot0=0 slot1=1
Okhotsk/absent load 0->1 load ->2: gas=35122 slot0=2 slot1=1
Okhotsk/absent load 0->1 load ->0: gas=30322 slot0=0 slot1=1
Okhotsk/absent load 0->0 load ->1: gas=32322 slot0=1 slot1=1
Okhotsk/existing load 1->2 load ->1: gas=12422 slot0=0 slot1=1
Vanuatu/absent 0->0: gas=12206 slot0=0 slot1=1
Vanuatu/absent 0->1: gas=32106 slot0=1 slot1=1
Vanuatu/existing 1->1: gas=12206 slot0=0 slot1=1
Vanuatu/existing 1->2: gas=15006 slot0=0 slot1=2
Vanuatu/existing 1->0: gas=12005 slot0=0 slot1=0
Vanuatu/absent 0->1->2: gas=35012 slot0=2 slot1=1
Vanuatu/absent 0->1->0: gas=30212 slot0=0 slot1=1
Vanuatu/absent 0->1->1->0: gas=30318 slot0=0 slot1=1
Vanuatu/existing 1->2->1: gas=12312 slot0=0 slot1=1
Vanuatu/existing 1->0->0->1: gas=12418 slot0=0 slot1=1
Vanuatu/absent load 0->1 load ->2: gas=35122 slot0=2 slot1=1
Vanuatu/absent load 0->1 load ->0: gas=30322 slot0=0 slot1=1
Vanuatu/absent load 0->0 load ->1: gas=32322 slot0=1 slot1=1
Vanuatu/existing load 1->2 load ->1: gas=12422 slot0=0 slot1=1
Yap/absent 0->0: gas=12206 slot0=0 slot1=1
Yap/absent 0->1: gas=32106 slot0=1 slot1=1
Yap/existing 1->1: gas=12206 slot0=0 slot1=1
Yap/existing 1->2: gas=15006 slot0=0 slot1=2
Yap/existing 1->0: gas=12005 slot0=0 slot1=0
Yap/absent 0->1->2: gas=35012 slot0=2 slot1=1
Yap/absent 0->1->0: gas=30212 slot0=0 slot1=1
Yap/absent 0->1->1->0: gas=30318 slot0=0 slot1=1
Yap/existing 1->2->1: gas=12312 slot0=0 slot1=1
Yap/existing 1->0->0->1: gas=12418 slot0=0 slot1=1
Yap/absent load 0->1 load ->2: gas=35122 slot0=2 slot1=1
Yap/absent load 0->1 load ->0: gas=30322 slot0=0 slot1=1
Yap/absent load 0->0 load ->1: gas=32322 slot0=1 slot1=1
Yap/existing load 1->2 load ->1: gas=12422 slot0=0 slot1=1
Zanzibar-1/absent 0->0: gas=12206 slot0=0 slot1=1
Zanzibar-1/absent 0->1: gas=32106 slot0=1 slot1=1
Zanzibar-1/existing 1->1: gas=12206 slot0=0 slot1=1
Zanzibar-1/existing 1->2: gas=15006 slot0=0 slot1=2
Zanzibar-1/existing 1->0: gas=12005 slot0=0 slot1=0
Zanzibar-1/absent 0->1->2: gas=35012 slot0=2 slot1=1
Zanzibar-1/absent 0->1->0: gas=30212 slot0=0 slot1=1
Zanzibar-1/absent 0->1->1->0: gas=30318 slot0=0 slot1=1
Zanzibar-1/existing 1->2->1: gas=12312 slot0=0 slot1=1
Zanzibar-1/existing 1->0->0->1: gas=12418 slot0=0 slot1=1
Zanzibar-1/absent load 0->1 load ->2: gas=35122 slot0=2 slot1=1
Zanzibar-1/absent load 0->1 load ->0: gas=30322 slot0=0 slot1=1
Zanzibar-1/absent load 0->0 load ->1: gas=32322 slot0=1 slot1=1
Zanzibar-1/existing load 1->2 load ->1: gas=12422 slot0=0 slot1=1
Zanzibar/absent 0->0: gas=12206 slot0=0 slot1=1
Zanzibar/absent 0->1: gas=32106 slot0=1 slot1=1
Zanzibar/existing 1->1: gas=12206 slot0=0 slot1=1
Zanzibar/existing 1->2: gas=15006 slot0=0 slot1=2
Zanzibar/existing 1->0: gas=12005 slot0=0 slot1=0
Zanzibar/absent 0->1->2: gas=32212 slot0=2 slot1=1
Zanzibar/absent 0->1->0: gas=25770 slot0=0 slot1=1
Zanzibar/absent 0->1->1->0: gas=25855 slot0=0 slot1=1
Zanzibar/existing 1->2->1: gas=12312 slot0=0 slot1=1
Zanzibar/existing 1->0->0->1: gas=12418 slot0=0 slot1=1
Zanzibar/absent load 0->1 load ->2: gas=32322 slot0=2 slot1=1
Zanzibar/absent load 0->1 load ->0: gas=25858 slot0=0 slot1=1
Zanzibar/absent load 0->0 load ->1: gas=32322 slot0=1 slot1=1
Zanzibar/existing load 1->2 load ->1: gas=12422 slot0=0 slot1=1
`

// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
)

// newTransferStateDB builds a StateDBAdapter backed by the in-memory mock state
// manager, using the same option set that governs the production in-contract
// transfer path (RevertLog so that forged/legit tx-logs can be snapshot/revert
// tested).
func newTransferStateDB(t *testing.T) *StateDBAdapter {
	t.Helper()
	ctrl := gomock.NewController(t)
	sm, err := initMockStateManager(ctrl)
	require.NoError(t, err)
	stateDB, err := NewStateDBAdapter(
		sm,
		1,
		hash.ZeroHash256,
		NotFixTopicCopyBugOption(),
		FixSnapshotOrderOption(),
		RevertLogOption(),
	)
	require.NoError(t, err)
	return stateDB
}

// TestCanTransfer exercises the balance guard used by the EVM before it will
// move value. An off-by-one here would let a contract spend funds it does not
// have, so both the exact-balance boundary and the over-draft case are asserted.
func TestCanTransfer(t *testing.T) {
	require := require.New(t)
	stateDB := newTransferStateDB(t)

	stateDB.AddBalance(_addr1, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)

	require.True(CanTransfer(stateDB, _addr1, uint256.NewInt(0)))
	require.True(CanTransfer(stateDB, _addr1, uint256.NewInt(999)))
	// exact balance is transferable (Cmp >= 0)
	require.True(CanTransfer(stateDB, _addr1, uint256.NewInt(1000)))
	// one wei over balance is not
	require.False(CanTransfer(stateDB, _addr1, uint256.NewInt(1001)))
	// an account with no balance cannot transfer a positive amount
	require.False(CanTransfer(stateDB, _c1, uint256.NewInt(1)))
	require.True(CanTransfer(stateDB, _c1, uint256.NewInt(0)))
}

// TestMakeTransferRecordsTransferLog is the positive-path security assertion for
// the in-contract transfer machinery: a value move MUST (a) debit the sender,
// (b) credit the recipient by the same amount, and (c) emit exactly one
// IN_CONTRACT_TRANSFER transaction log describing that move — no more, no less.
func TestMakeTransferRecordsTransferLog(t *testing.T) {
	require := require.New(t)
	stateDB := newTransferStateDB(t)

	stateDB.AddBalance(_addr1, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)
	require.Zero(len(stateDB.TransactionLogs()))

	MakeTransfer(stateDB, _addr1, _c1, uint256.NewInt(300))

	// balances moved by exactly the transferred amount
	require.Equal(uint256.NewInt(700), stateDB.GetBalance(_addr1))
	require.Equal(uint256.NewInt(300), stateDB.GetBalance(_c1))

	// exactly one tx-log, and it is an in-contract transfer with correct fields
	txLogs := stateDB.TransactionLogs()
	require.Len(txLogs, 1)
	require.Equal(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, txLogs[0].Type)
	fromAddr, _ := address.FromBytes(_addr1[:])
	toAddr, _ := address.FromBytes(_c1[:])
	require.Equal(fromAddr.String(), txLogs[0].Sender)
	require.Equal(toAddr.String(), txLogs[0].Recipient)
	require.Equal(big.NewInt(300), txLogs[0].Amount)

	// the reserved-topic log must NOT surface as a normal (forgeable) event log
	require.Zero(len(stateDB.Logs()))

	// a second transfer appends a second tx-log (they accumulate, not overwrite)
	MakeTransfer(stateDB, _addr1, _c2, uint256.NewInt(100))
	txLogs = stateDB.TransactionLogs()
	require.Len(txLogs, 2)
	require.Equal(uint256.NewInt(600), stateDB.GetBalance(_addr1))
	require.Equal(uint256.NewInt(100), stateDB.GetBalance(_c2))
	// the first tx-log is untouched, the second describes the new _addr1 -> _c2 move
	require.Equal(fromAddr.String(), txLogs[0].Sender)
	require.Equal(toAddr.String(), txLogs[0].Recipient)
	require.Equal(big.NewInt(300), txLogs[0].Amount)
	toAddr2, _ := address.FromBytes(_c2[:])
	require.Equal(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, txLogs[1].Type)
	require.Equal(fromAddr.String(), txLogs[1].Sender)
	require.Equal(toAddr2.String(), txLogs[1].Recipient)
	require.Equal(big.NewInt(100), txLogs[1].Amount)
}

// TestMakeTransferZeroAmount is the adversarial counterpart: a zero-value
// transfer must NOT fabricate a transaction log (a phantom log would be a
// forgery) and must leave balances untouched.
func TestMakeTransferZeroAmount(t *testing.T) {
	require := require.New(t)
	stateDB := newTransferStateDB(t)

	stateDB.AddBalance(_addr1, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)

	MakeTransfer(stateDB, _addr1, _c1, uint256.NewInt(0))

	require.Equal(uint256.NewInt(1000), stateDB.GetBalance(_addr1))
	require.Equal(uint256.NewInt(0), stateDB.GetBalance(_c1))
	// zero-amount reserved-topic log is dropped: no tx-log, no event log
	require.Zero(len(stateDB.TransactionLogs()))
	require.Zero(len(stateDB.Logs()))
}

// TestSecurityDeposit covers the pre-execution gas escrow, which is the last
// balance check before the EVM runs. The three branches are consensus-critical:
// gas-limit rejection, insufficient-funds rejection, and the successful debit.
func TestSecurityDeposit(t *testing.T) {
	origin := common.BytesToAddress(_addr1[:])
	newParamsFor := func(gas uint64, gasPrice *big.Int) *Params {
		return &Params{
			txCtx: vm.TxContext{Origin: origin, GasPrice: gasPrice},
			nonce: 1,
			gas:   gas,
		}
	}

	t.Run("gas limit exceeded", func(t *testing.T) {
		require := require.New(t)
		stateDB := newTransferStateDB(t)
		ps := newParamsFor(100, big.NewInt(1))
		// block gas limit (10) below the action's declared gas (100)
		err := securityDeposit(ps, stateDB, 10)
		require.ErrorIs(err, action.ErrGasLimit)
	})

	t.Run("insufficient funds", func(t *testing.T) {
		require := require.New(t)
		stateDB := newTransferStateDB(t)
		// origin has only 50, needs gas(100)*price(1) = 100
		stateDB.AddBalance(origin, uint256.NewInt(50), tracing.BalanceChangeUnspecified)
		ps := newParamsFor(100, big.NewInt(1))
		err := securityDeposit(ps, stateDB, 1000)
		require.ErrorIs(err, action.ErrInsufficientFunds)
		// balance untouched on failure
		require.Equal(uint256.NewInt(50), stateDB.GetBalance(origin))
	})

	t.Run("success debits gas escrow", func(t *testing.T) {
		require := require.New(t)
		stateDB := newTransferStateDB(t)
		stateDB.AddBalance(origin, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)
		ps := newParamsFor(100, big.NewInt(2))
		err := securityDeposit(ps, stateDB, 1000)
		require.NoError(err)
		// escrow = gas(100) * price(2) = 200 subtracted
		require.Equal(uint256.NewInt(800), stateDB.GetBalance(origin))
	})
}

// TestIntrinsicGas checks the intrinsic-gas accounting including the overflow
// guard. Oversized data that would overflow the gas computation must be rejected
// rather than wrapping around to a small value (a classic underflow exploit).
func TestIntrinsicGas(t *testing.T) {
	require := require.New(t)

	// baseline: empty payload, no lists
	base, err := intrinsicGas(0, nil, nil)
	require.NoError(err)
	require.Equal(action.ExecutionBaseIntrinsicGas, base)

	// data bytes are charged per byte
	withData, err := intrinsicGas(10, nil, nil)
	require.NoError(err)
	require.Equal(action.ExecutionBaseIntrinsicGas+10*action.ExecutionDataGas, withData)

	// access list entries and storage keys add gas
	al := types.AccessList{
		{Address: common.Address{}, StorageKeys: []common.Hash{{}, {}}},
	}
	withAL, err := intrinsicGas(0, al, nil)
	require.NoError(err)
	require.Equal(
		action.ExecutionBaseIntrinsicGas+action.TxAccessListAddressGas+2*action.TxAccessListStorageKeyGas,
		withAL,
	)

	// authorization list entries add per-tuple gas
	withAuth, err := intrinsicGas(0, nil, []types.SetCodeAuthorization{{}})
	require.NoError(err)
	require.Equal(action.ExecutionBaseIntrinsicGas+action.CallNewAccountGas, withAuth)

	// overflow guard: an enormous size must error, not silently overflow
	_, err = intrinsicGas(math.MaxUint64, nil, nil)
	require.ErrorIs(err, action.ErrInsufficientFunds)
}

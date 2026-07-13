// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// reservedTopic returns the reserved IN_CONTRACT_TRANSFER topic exactly as
// MakeTransfer emits it.
func reservedTopic() common.Hash {
	return common.BytesToHash(_inContractTransfer[:])
}

// addrTopic left-pads a 20-byte address into a 32-byte topic, matching how the
// EVM encodes indexed address topics (address occupies the low 20 bytes).
func addrTopic(a common.Address) common.Hash {
	return common.BytesToHash(a[:])
}

func newAdapter(t *testing.T, opts ...StateDBAdapterOption) *StateDBAdapter {
	t.Helper()
	ctrl := gomock.NewController(t)
	sm, err := initMockStateManager(ctrl)
	require.NoError(t, err)
	base := []StateDBAdapterOption{
		NotFixTopicCopyBugOption(),
		FixSnapshotOrderOption(),
	}
	stateDB, err := NewStateDBAdapter(sm, 1, hash.ZeroHash256, append(base, opts...)...)
	require.NoError(t, err)
	return stateDB
}

// TestAddLogReservedTopicInvariants covers the AddLog reserved-topic path — the
// exact code the in-contract-transfer forgery abused.
//
// IMPORTANT (forward-compat with the forgery fix, PR #4868): whether a contract-
// emitted reserved-topic LOG becomes a TransactionLog is the precise behavior the
// fix changes (master: it does — the vulnerability; fixed: it is dropped). That
// assertion is owned by the fix's own regression test (TestInContractTransferLogNoForgery),
// so it is deliberately NOT pinned here. Instead this test locks the invariants
// that must hold in BOTH versions, and all tx-log correctness is asserted through
// the legitimate MakeTransfer path (see TestMakeTransfer* / TestTransferVsForgedLog).
func TestAddLogReservedTopicInvariants(t *testing.T) {
	from := _c1
	to := _c2

	t.Run("reserved topic is never surfaced as an EVM event log", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t)
		// a reserved-topic log — regardless of amount — must never enter
		// stateDB.logs (receipt.Logs / receipt root must be unaffected)
		for _, data := range [][]byte{nil, big.NewInt(0).Bytes(), big.NewInt(777).Bytes()} {
			stateDB.AddLog(&types.Log{
				Address: _c3, // non-zero: as if emitted by a contract
				Topics:  []common.Hash{reservedTopic(), addrTopic(from), addrTopic(to)},
				Data:    data,
			})
		}
		require.Zero(len(stateDB.Logs()))
	})

	t.Run("reserved topic never moves balances", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t)
		stateDB.AddBalance(from, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)
		// a log is not a transfer: emitting the reserved topic must not debit
		// or credit any account (the semantic heart of the forgery)
		stateDB.AddLog(&types.Log{
			Topics: []common.Hash{reservedTopic(), addrTopic(from), addrTopic(to)},
			Data:   big.NewInt(500).Bytes(),
		})
		require.Equal(uint256.NewInt(1000), stateDB.GetBalance(from))
		require.True(stateDB.GetBalance(to).IsZero())
	})

	t.Run("reserved topic with wrong topic count panics", func(t *testing.T) {
		require := require.New(t)
		for _, topics := range [][]common.Hash{
			{reservedTopic()},                  // 1 topic
			{reservedTopic(), addrTopic(from)}, // 2 topics
			{reservedTopic(), addrTopic(from), addrTopic(to), addrTopic(from)}, // 4 topics
		} {
			stateDB := newAdapter(t)
			log := &types.Log{Topics: topics, Data: big.NewInt(1).Bytes()}
			require.Panics(func() { stateDB.AddLog(log) })
		}
	})

	t.Run("non-reserved topic becomes a normal event log", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t)
		otherTopic := common.HexToHash("0xdeadbeef")
		stateDB.AddLog(&types.Log{
			Address: _c3,
			Topics:  []common.Hash{otherTopic, addrTopic(from)},
			Data:    []byte("payload"),
		})
		// recorded as an event log, NOT a transfer tx-log
		require.Zero(len(stateDB.TransactionLogs()))
		logs := stateDB.Logs()
		require.Len(logs, 1)
		c3Io, _ := address.FromBytes(_c3[:])
		require.Equal(c3Io.String(), logs[0].Address)
		require.Equal([]byte("payload"), logs[0].Data)
		require.Len(logs[0].Topics, 2)
		require.Equal(stateDB.blockHeight, logs[0].BlockHeight)
	})

	t.Run("non-reserved single topic with transfer-shaped data stays a normal log", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t)
		// an attacker-crafted log that looks like a transfer but whose first
		// topic is not the reserved one must never become a tx-log
		stateDB.AddLog(&types.Log{
			Address: _c1,
			Topics:  []common.Hash{addrTopic(from), addrTopic(to)},
			Data:    big.NewInt(999).Bytes(),
		})
		require.Zero(len(stateDB.TransactionLogs()))
		require.Len(stateDB.Logs(), 1)
	})
}

// TestAddLogReservedTopicBecomesRegularLogAfterUpgrade pins the post-upgrade
// counterpart of the reserved-topic invariants above: once
// FixInContractTransferTopicOption is active, AddLog no longer special-cases
// the reserved topic at all — regardless of topic count (including the
// well-formed 3-topic shape that collides with AddInContractTransferLog's own
// record), the log is recorded as an ordinary event log instead of being
// dropped or panicking. It is still never a tx-log and never moves a balance.
func TestAddLogReservedTopicBecomesRegularLogAfterUpgrade(t *testing.T) {
	require := require.New(t)
	from, to := _c1, _c2
	for _, topics := range [][]common.Hash{
		{reservedTopic()},                                                  // 1 topic
		{reservedTopic(), addrTopic(from)},                                 // 2 topics
		{reservedTopic(), addrTopic(from), addrTopic(to)},                  // 3 topics (well-formed shape)
		{reservedTopic(), addrTopic(from), addrTopic(to), addrTopic(from)}, // 4 topics
	} {
		stateDB := newAdapter(t, FixInContractTransferTopicOption())
		stateDB.AddBalance(from, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)
		log := &types.Log{Address: _c3, Topics: topics, Data: big.NewInt(1).Bytes()}
		require.NotPanics(func() { stateDB.AddLog(log) })
		require.Len(stateDB.Logs(), 1) // recorded as a regular event log
		gotTopics := stateDB.Logs()[0].Topics
		require.Len(gotTopics, len(topics))
		for i, tp := range topics {
			require.Equal(tp.Bytes(), gotTopics[i][:]) // untouched, not reinterpreted
		}
		require.Zero(len(stateDB.TransactionLogs()))                  // never a tx-log
		require.Equal(uint256.NewInt(1000), stateDB.GetBalance(from)) // no balance moved
		require.True(stateDB.GetBalance(to).IsZero())
	}
}

// TestTransferVsForgedLog is the adversarial contrast codex asked for: a forged
// reserved-topic EVM log and a genuine MakeTransfer of the SAME from/to/amount
// must diverge. The forged log moves no balance; the real transfer both moves
// balance and records exactly one IN_CONTRACT_TRANSFER tx-log with matching
// fields. This encodes "a log is not a transfer" without pinning the (fix-
// dependent) tx-log outcome of the forged direct-AddLog call.
func TestTransferVsForgedLog(t *testing.T) {
	require := require.New(t)
	from, to := _c1, _c2
	amount := uint256.NewInt(250)

	// forged path: a contract emits the reserved topic directly
	forged := newAdapter(t)
	forged.AddBalance(from, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)
	forged.AddLog(&types.Log{
		Address: from,
		Topics:  []common.Hash{reservedTopic(), addrTopic(from), addrTopic(to)},
		Data:    amount.Bytes(),
	})
	// no balance moved, and nothing landed in receipt.Logs
	require.Equal(uint256.NewInt(1000), forged.GetBalance(from))
	require.True(forged.GetBalance(to).IsZero())
	require.Zero(len(forged.Logs()))

	// real path: the node's MakeTransfer
	real := newAdapter(t, RevertLogOption())
	real.AddBalance(from, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)
	MakeTransfer(real, from, to, amount)
	require.Equal(uint256.NewInt(750), real.GetBalance(from))
	require.Equal(amount, real.GetBalance(to))
	txLogs := real.TransactionLogs()
	require.Len(txLogs, 1)
	fromIo, _ := address.FromBytes(from[:])
	toIo, _ := address.FromBytes(to[:])
	require.Equal(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, txLogs[0].Type)
	require.Equal(fromIo.String(), txLogs[0].Sender)
	require.Equal(toIo.String(), txLogs[0].Recipient)
	require.Equal(amount.ToBig(), txLogs[0].Amount)
	require.Zero(len(real.Logs())) // a native transfer is not an EVM event log
}

// TestTransferSnapshotRevert exercises the security-relevant atomic path: a real
// in-contract transfer (MakeTransfer) that moves balances AND records a tx-log,
// then a revert that must roll BOTH back together — a half-revert (log kept,
// balance restored, or vice-versa) would corrupt the transfer-log ledger. It
// also asserts a concurrently-added contract event log reverts, and that the
// legacy no-RevertLog path leaves logs in place.
func TestTransferSnapshotRevert(t *testing.T) {
	from, to := _c1, _c2

	t.Run("revert log enabled rolls back balances and tx-logs atomically", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t, RevertLogOption())
		stateDB.AddBalance(from, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)

		sn := stateDB.Snapshot()
		// a contract event log plus a genuine native transfer inside the snapshot
		stateDB.AddLog(&types.Log{Address: _c3, Topics: []common.Hash{common.HexToHash("0x01")}})
		MakeTransfer(stateDB, from, to, uint256.NewInt(400))
		require.Len(stateDB.Logs(), 1)
		require.Len(stateDB.TransactionLogs(), 1)
		require.Equal(uint256.NewInt(600), stateDB.GetBalance(from))
		require.Equal(uint256.NewInt(400), stateDB.GetBalance(to))

		stateDB.RevertToSnapshot(sn)
		// everything the snapshot enclosed is undone together
		require.Zero(len(stateDB.Logs()))
		require.Zero(len(stateDB.TransactionLogs()))
		require.Equal(uint256.NewInt(1000), stateDB.GetBalance(from))
		require.True(stateDB.GetBalance(to).IsZero())
	})

	t.Run("without revert log, logs persist across revert", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t) // no RevertLogOption
		stateDB.AddBalance(from, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)
		sn := stateDB.Snapshot()
		stateDB.AddLog(&types.Log{Address: _c3, Topics: []common.Hash{common.HexToHash("0x01")}})
		MakeTransfer(stateDB, from, to, uint256.NewInt(400))
		require.Len(stateDB.Logs(), 1)
		require.Len(stateDB.TransactionLogs(), 1)
		stateDB.RevertToSnapshot(sn)
		// legacy behavior: balances revert (state manager) but logs are not
		require.Equal(uint256.NewInt(1000), stateDB.GetBalance(from))
		require.True(stateDB.GetBalance(to).IsZero())
		require.Len(stateDB.Logs(), 1)
		require.Len(stateDB.TransactionLogs(), 1)
	})
}

// TestSubBalanceAdapter covers SubBalance branches, including the zero-amount
// short-circuits (which must not touch state) and the insufficient-balance error
// path (which must record an error rather than silently underflow).
func TestSubBalanceAdapter(t *testing.T) {
	t.Run("zero amount on existing account returns balance, no change", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t)
		stateDB.AddBalance(_addr1, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)
		got := stateDB.SubBalance(_addr1, uint256.NewInt(0), tracing.BalanceChangeUnspecified)
		require.Equal(uint256.NewInt(1000), &got)
		require.Equal(uint256.NewInt(1000), stateDB.GetBalance(_addr1))
		require.NoError(stateDB.Error())
	})

	t.Run("zero amount on non-existent account returns zero", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t)
		got := stateDB.SubBalance(_c4, uint256.NewInt(0), tracing.BalanceChangeUnspecified)
		require.True(got.IsZero())
		require.NoError(stateDB.Error())
	})

	t.Run("positive amount debits balance", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t)
		stateDB.AddBalance(_addr1, uint256.NewInt(1000), tracing.BalanceChangeUnspecified)
		got := stateDB.SubBalance(_addr1, uint256.NewInt(400), tracing.BalanceChangeUnspecified)
		require.Equal(uint256.NewInt(600), &got)
		require.Equal(uint256.NewInt(600), stateDB.GetBalance(_addr1))
	})

	t.Run("over-draft records an error and does not go negative", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t)
		stateDB.AddBalance(_addr1, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
		stateDB.SubBalance(_addr1, uint256.NewInt(500), tracing.BalanceChangeUnspecified)
		require.Error(stateDB.Error())
		// balance unchanged (the failed sub was not persisted)
		require.Equal(uint256.NewInt(100), stateDB.GetBalance(_addr1))
	})
}

// TestSelfDestructTransferLogForgeryGuard exercises the SELFDESTRUCT transfer-log
// derivation, which was hardened to derive the log from the actual balance
// movement. With SuicideTxLogMismatchPanic active:
//   - a matching last-AddBalance produces a correct IN_CONTRACT_TRANSFER tx-log
//   - a mismatch (forged/inconsistent movement) must panic rather than emit a
//     fabricated log.
func TestSelfDestructTransferLogForgeryGuard(t *testing.T) {
	beneficiary := _c2

	t.Run("matching balance movement emits correct transfer log", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t, SuicideTxLogMismatchPanicOption())
		// contract holds 100
		stateDB.AddBalance(_c1, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
		// EVM transfers the contract balance to the beneficiary just before
		// SelfDestruct; this sets lastAddBalance{Addr,Amount}
		stateDB.AddBalance(beneficiary, stateDB.GetBalance(_c1), tracing.BalanceChangeUnspecified)
		stateDB.SelfDestruct(_c1)

		require.True(stateDB.HasSelfDestructed(_c1))
		require.True(stateDB.GetBalance(_c1).IsZero())
		txLogs := stateDB.TransactionLogs()
		require.Len(txLogs, 1)
		fromIo, _ := address.FromBytes(_c1[:])
		toIo, _ := address.FromBytes(beneficiary[:])
		require.Equal(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, txLogs[0].Type)
		require.Equal(fromIo.String(), txLogs[0].Sender)
		require.Equal(toIo.String(), txLogs[0].Recipient)
		require.Equal(big.NewInt(100), txLogs[0].Amount)
	})

	t.Run("mismatched balance movement panics", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t, SuicideTxLogMismatchPanicOption())
		// contract holds 100
		stateDB.AddBalance(_c1, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
		// but the last AddBalance credited a DIFFERENT amount (70) — the tx-log
		// would misrepresent the true 100 movement, so this must panic
		stateDB.AddBalance(beneficiary, uint256.NewInt(70), tracing.BalanceChangeUnspecified)
		require.Panics(func() { stateDB.SelfDestruct(_c1) })
	})

	t.Run("zero-balance selfdestruct emits no transfer log", func(t *testing.T) {
		require := require.New(t)
		stateDB := newAdapter(t, SuicideTxLogMismatchPanicOption())
		// account exists but holds nothing; lastAddBalanceAmount stays 0 so the
		// amount-match holds (0==0) but the >0 gate suppresses the log
		stateDB.CreateAccount(_c1)
		stateDB.SelfDestruct(_c1)
		require.True(stateDB.HasSelfDestructed(_c1))
		require.Zero(len(stateDB.TransactionLogs()))
	})
}

// TestAccessListAndPrepare covers the access-list warming performed by Prepare
// (EIP-2929/2930/3651) plus AccessedSlots and SubRefund — machinery that gates
// how much a transfer/call costs and which addresses are pre-warmed.
func TestAccessListAndPrepare(t *testing.T) {
	require := require.New(t)
	stateDB := newAdapter(t)

	sender := _addr1
	coinbase := _c4
	dst := _c1
	precompile := _c2
	slotAddr := _c3
	slot := _k1

	rules := params.Rules{IsBerlin: true, IsShanghai: true}
	list := types.AccessList{
		{Address: slotAddr, StorageKeys: []common.Hash{slot}},
	}
	stateDB.Prepare(rules, sender, coinbase, &dst, []common.Address{precompile}, list)

	require.True(stateDB.AddressInAccessList(sender))
	require.True(stateDB.AddressInAccessList(dst))
	require.True(stateDB.AddressInAccessList(precompile))
	require.True(stateDB.AddressInAccessList(coinbase)) // EIP-3651 warm coinbase
	require.True(stateDB.AddressInAccessList(slotAddr))
	aOk, sOk := stateDB.SlotInAccessList(slotAddr, slot)
	require.True(aOk)
	require.True(sOk)

	// AccessedSlots reflects the (address -> slots) that were touched
	accessed := stateDB.AccessedSlots()
	require.Contains(accessed, slotAddr)
	require.Contains(accessed[slotAddr], slot)

	// Prepare on non-Berlin rules is a no-op
	fresh := newAdapter(t)
	fresh.Prepare(params.Rules{}, sender, coinbase, &dst, nil, nil)
	require.False(fresh.AddressInAccessList(sender))
}

// TestRefundBounds covers AddRefund/SubRefund including the panic guard against
// refunding more gas than has been accumulated (an underflow that would hand
// back gas the tx never paid for).
func TestRefundBounds(t *testing.T) {
	require := require.New(t)
	stateDB := newAdapter(t)

	require.Zero(stateDB.GetRefund())
	stateDB.AddRefund(100)
	stateDB.AddRefund(50)
	require.Equal(uint64(150), stateDB.GetRefund())
	stateDB.SubRefund(120)
	require.Equal(uint64(30), stateDB.GetRefund())
	// subtracting more than the counter holds must panic
	require.Panics(func() { stateDB.SubRefund(31) })
}

// TestAdapterGetters gives real assertions to the small accessor methods that
// feed execution decisions (code size, storage root, new-account detection).
func TestAdapterGetters(t *testing.T) {
	require := require.New(t)
	stateDB := newAdapter(t)

	// fresh account is new and has empty code + zero storage root
	require.True(stateDB.IsNewAccount(_c1))
	require.Zero(stateDB.GetCodeSize(_c1))
	require.Equal(common.Hash{}, stateDB.GetStorageRoot(_c1))

	// after setting code, code size reflects it
	stateDB.SetCode(_c1, []byte("0123456789"))
	require.Equal(len("0123456789"), stateDB.GetCodeSize(_c1))

	// Error is nil on a clean run
	require.NoError(stateDB.Error())
}

// TestFinaliseAndNilCaches guards the trivial no-op / nil-returning methods so a
// future refactor that starts returning non-nil (which the EVM would misuse)
// is caught.
func TestFinaliseAndNilCaches(t *testing.T) {
	require := require.New(t)
	stateDB := newAdapter(t)
	require.NotPanics(func() { stateDB.Finalise(true) })
	require.Nil(stateDB.PointCache())
	require.Nil(stateDB.Witness())
	require.Nil(stateDB.AccessEvents())
}

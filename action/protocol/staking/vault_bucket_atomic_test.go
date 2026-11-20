// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
	"time"
)

// failVaultWrite injects a backend failure after accounts, vault and allocator
// have been written. Snapshot and Revert still use the real working set.
type failVaultWrite struct{ protocol.StateManager }

func (s failVaultWrite) PutState(v interface{}, opts ...protocol.StateOption) (uint64, error) {
	if _, ok := v.(*staking.BucketGuardV2Record); ok {
		return 0, errors.New("injected vault guard write failure")
	}
	return s.StateManager.PutState(v, opts...)
}

func TestVaultBucketAtomicRollback(t *testing.T) {
	e := newStakingEnvWith(t, 1, func(g *genesis.Genesis) {
		g.ToBeEnabledBlockHeight = 1
		testutil.NormalizeGenesisHeights(&g.Blockchain)
	})
	key, err := signer.NewKeyFromSeeds(pqcrypto.AlgSLHDSA192s, bytes.Repeat([]byte{0x71}, 72))
	require.NoError(t, err)
	m := &bucketguard.VaultAction{Method: "vaultCreate", Amount: unit.ConvertIotxToRau(500), OwnerTxNonce: 1, Guard: bucketguard.NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: key.PublicKey().Bytes()}}
	m.Guard.AcceptSig, err = key.Sign([]byte(bucketguard.VaultAcceptContext), bucketguard.EncodeVaultAccept(bucketguard.NewDomain(0, e.g.Hash()), 0, 1, 1, _staker.Bytes(), bucketguard.UnbindDelayBlocks, m.Guard.AlgID, m.Guard.PublicKey, [64]byte{}))
	require.NoError(t, err)
	a, err := action.NewVaultAction(m)
	require.NoError(t, err)
	elp := envelope(1).SetGasLimit(4_000_000).SetAction(a).Build()
	gas, err := elp.IntrinsicGas()
	require.NoError(t, err)
	ctx := e.actionCtx(_staker, 1, gas, time.Unix(e.g.Timestamp, 0))
	pre := protocol.MustGetBlockCtx(ctx)
	pre.BlockHeight = 0
	preCtx := protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, pre))
	require.Error(t, e.p.Validate(preCtx, elp, e.ws), "fork-1 rejects new envelope")
	_, _, err = e.p.ReadState(preCtx, e.ws, []byte("vaultBucket"), make([]byte, 32))
	require.Error(t, err)
	require.NoError(t, e.p.Validate(ctx, elp, e.ws), "fork accepts new envelope")
	before := e.account(t, _staker).Clone()
	_, err = e.p.Handle(ctx, elp, failVaultWrite{e.ws})
	require.ErrorContains(t, err, "injected vault guard write failure")
	require.Equal(t, before, e.account(t, _staker), "accounts and nonce roll back on backend failure")
	_, err = staking.ReadVaultBucket(e.ws, 0)
	require.ErrorIs(t, err, state.ErrStateNotExist)
	var meta staking.VaultMetaRecord
	_, err = e.ws.State(&meta, protocol.NamespaceOption(state.StakingNamespace), protocol.KeyOption([]byte{12}))
	require.ErrorIs(t, err, state.ErrStateNotExist)
	// The same signature and nonce succeeds after removing the fault.
	r, err := e.p.Handle(ctx, elp, e.ws)
	require.NoError(t, err)
	require.NotNil(t, r)
	require.EqualValues(t, 1, r.Status)
}

// TestVaultBucketExpiryWorkingSet checks exact-height finalization using the
// production Handler and a real undo log. It does not mine millions of blocks.
func TestVaultBucketExpiryWorkingSet(t *testing.T) {
	e := newStakingEnvWith(t, 1, func(g *genesis.Genesis) {
		g.ToBeEnabledBlockHeight = 1
		testutil.NormalizeGenesisHeights(&g.Blockchain)
	})
	key, err := signer.NewKeyFromSeeds(pqcrypto.AlgSLHDSA192s, bytes.Repeat([]byte{0x72}, 72))
	require.NoError(t, err)
	m := &bucketguard.VaultAction{Method: "vaultCreate", Amount: unit.ConvertIotxToRau(500), OwnerTxNonce: 1, Guard: bucketguard.NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: key.PublicKey().Bytes()}}
	m.Guard.AcceptSig, err = key.Sign([]byte(bucketguard.VaultAcceptContext), bucketguard.EncodeVaultAccept(bucketguard.NewDomain(0, e.g.Hash()), 0, 1, 1, _staker.Bytes(), bucketguard.UnbindDelayBlocks, m.Guard.AlgID, m.Guard.PublicKey, [64]byte{}))
	require.NoError(t, err)
	nonce := uint64(0)
	run := func(height uint64, m *bucketguard.VaultAction, success bool) {
		nonce++
		a, err := action.NewVaultAction(m)
		require.NoError(t, err)
		elp := envelope(nonce).SetGasLimit(4_000_000).SetAction(a).Build()
		gas, err := elp.IntrinsicGas()
		require.NoError(t, err)
		ctx := e.actionCtx(_staker, nonce, gas, time.Unix(e.g.Timestamp, 0))
		bc := protocol.MustGetBlockCtx(ctx)
		bc.BlockHeight = height
		ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, bc))
		r, err := e.p.Handle(ctx, elp, e.ws)
		require.NoError(t, err)
		require.NotNil(t, r)
		require.Equal(t, success, r.Status == 1)
	}
	run(1, m, true)
	run(2, &bucketguard.VaultAction{Method: "vaultInitiateExit"}, true)
	g, err := staking.ReadVaultGuard(e.ws, 0)
	require.NoError(t, err)
	expiry := g.Pending.ExpiresAt
	run(expiry-1, &bucketguard.VaultAction{Method: "vaultFinalizeExit"}, false)
	before := e.account(t, _staker).Balance
	run(expiry, &bucketguard.VaultAction{Method: "vaultFinalizeExit"}, true)
	finalAction, err := action.NewVaultAction(&bucketguard.VaultAction{Method: "vaultFinalizeExit"})
	require.NoError(t, err)
	gas, err := finalAction.IntrinsicGas()
	require.NoError(t, err)
	fee := new(big.Int).Mul(new(big.Int).SetUint64(gas), _testGasPrice)
	require.Equal(t, new(big.Int).Sub(new(big.Int).Add(before, m.Amount), fee), e.account(t, _staker).Balance)
	_, err = staking.ReadVaultBucket(e.ws, 0)
	require.ErrorIs(t, err, state.ErrStateNotExist)
	g, err = staking.ReadVaultGuard(e.ws, 0)
	require.NoError(t, err)
	require.Empty(t, g.PublicKey)
	run(expiry+1, &bucketguard.VaultAction{Method: "vaultFinalizeExit"}, false)
}

// Account state is persisted in a real working set and actions use Handle,
// including receipt classification and rollback on backend failure.
func TestVaultRotateOwnerAccountState(t *testing.T) {
	e := newStakingEnvWith(t, 1, func(g *genesis.Genesis) {
		g.ToBeEnabledBlockHeight = 1
		testutil.NormalizeGenesisHeights(&g.Blockchain)
	})
	key, err := signer.NewKeyFromSeeds(pqcrypto.AlgSLHDSA192s, bytes.Repeat([]byte{0x73}, 72))
	require.NoError(t, err)
	domain := bucketguard.NewDomain(0, e.g.Hash())
	create := &bucketguard.VaultAction{Method: "vaultCreate", Amount: unit.ConvertIotxToRau(500), OwnerTxNonce: 1, Guard: bucketguard.NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: key.PublicKey().Bytes()}}
	create.Guard.AcceptSig, err = key.Sign([]byte(bucketguard.VaultAcceptContext), bucketguard.EncodeVaultAccept(domain, 0, 1, 1, _staker.Bytes(), bucketguard.UnbindDelayBlocks, create.Guard.AlgID, create.Guard.PublicKey, [64]byte{}))
	require.NoError(t, err)
	run := func(m *bucketguard.VaultAction, nonce uint64, sm protocol.StateManager) (*action.Receipt, error) {
		a, err := action.NewVaultAction(m)
		require.NoError(t, err)
		elp := envelope(nonce).SetGasLimit(4_000_000).SetAction(a).Build()
		gas, err := elp.IntrinsicGas()
		require.NoError(t, err)
		ctx := e.actionCtx(_staker, nonce, gas, time.Unix(e.g.Timestamp, 0))
		block := protocol.MustGetBlockCtx(ctx)
		block.BlockHeight = 2
		ctx = protocol.WithBlockCtx(ctx, block)
		bc := protocol.MustGetBlockchainCtx(ctx)
		bc.GetBlockHash = func(uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil }
		ctx = protocol.WithBlockchainCtx(ctx, bc)
		return e.p.Handle(ctx, elp, sm)
	}
	receipt, err := run(create, 1, e.ws)
	require.NoError(t, err)
	require.EqualValues(t, 1, receipt.Status)
	before, err := staking.ReadVaultBucket(e.ws, 0)
	require.NoError(t, err)
	guard, err := staking.ReadVaultGuard(e.ws, 0)
	require.NoError(t, err)
	contract := identityset.Address(25)
	acct, err := accountutil.LoadAccount(e.ws, contract)
	require.NoError(t, err)
	acct.CodeHash = bytes.Repeat([]byte{0xc0}, 32)
	require.NoError(t, accountutil.StoreAccount(e.ws, contract, acct))
	rotate := func(to address.Address) *bucketguard.VaultAction {
		m := &bucketguard.VaultAction{Method: "vaultRotateOwner", Call: bucketguard.VaultCall{Op: bucketguard.VaultOpRotateOwner, Generation: guard.Generation, GuardNonce: guard.GuardNonce, Amount: new(big.Int), Deadline: 10, AnchorHeight: 1, AnchorHash: make([]byte, 32), NewOwner: to.Bytes()}}
		m.Call.Sig, err = key.Sign([]byte(bucketguard.VaultCallContext), bucketguard.EncodeVaultCall(domain, before.Owner, guard.KeyID(), &m.Call))
		require.NoError(t, err)
		return m
	}
	pool := bucketguard.VaultPool()
	poolAddr, err := address.FromBytes(pool[:])
	require.NoError(t, err)
	poolBefore := e.account(t, poolAddr).Clone()
	receipt, err = run(rotate(contract), 2, e.ws)
	require.NoError(t, err)
	require.NotEqualValues(t, 1, receipt.Status)
	after, err := staking.ReadVaultBucket(e.ws, 0)
	require.NoError(t, err)
	require.Equal(t, before, after, "principal and owner must remain unchanged")
	afterGuard, err := staking.ReadVaultGuard(e.ws, 0)
	require.NoError(t, err)
	require.Equal(t, guard, afterGuard, "generation and guard nonce must remain unchanged")
	require.Equal(t, poolBefore, e.account(t, poolAddr))
	eoa, err := address.FromBytes(bytes.Repeat([]byte{0xa5}, 20))
	require.NoError(t, err)
	_, err = accountutil.Recorded(e.ws, eoa)
	require.ErrorIs(t, err, state.ErrStateNotExist)
	fault := errors.New("new owner account read failed")
	receipt, err = run(rotate(eoa), 3, failNewOwnerRead{StateManager: e.ws, owner: eoa, err: fault})
	require.ErrorIs(t, err, fault)
	require.Nil(t, receipt, "backend failure must abort, not produce a failed receipt")
	after, err = staking.ReadVaultBucket(e.ws, 0)
	require.NoError(t, err)
	require.Equal(t, before, after)
	afterGuard, err = staking.ReadVaultGuard(e.ws, 0)
	require.NoError(t, err)
	require.Equal(t, guard, afterGuard)
	receipt, err = run(rotate(eoa), 3, e.ws)
	require.NoError(t, err)
	require.EqualValues(t, 1, receipt.Status, "missing account is a legal EOA")
	after, err = staking.ReadVaultBucket(e.ws, 0)
	require.NoError(t, err)
	require.Equal(t, eoa.Bytes(), after.Owner)
	require.Equal(t, before.Amount, after.Amount)
}

type failNewOwnerRead struct {
	protocol.StateManager
	owner address.Address
	err   error
}

func (s failNewOwnerRead) State(v interface{}, opts ...protocol.StateOption) (uint64, error) {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return 0, err
	}
	if _, ok := v.(*state.Account); ok && bytes.Equal(cfg.Key, s.owner.Bytes()) {
		return 0, s.err
	}
	return s.StateManager.State(v, opts...)
}

func TestVaultClassBCleanupOnWithdraw(t *testing.T) {
	e := newStakingEnvWith(t, 1, func(g *genesis.Genesis) {
		g.ToBeEnabledBlockHeight = 1
		testutil.NormalizeGenesisHeights(&g.Blockchain)
	})
	index, withdrawTime := e.setupBucket(t)
	tombstone := &staking.BucketGuardV2Record{BucketGuardV2: bucketguard.BucketGuardV2{BucketClass: bucketguard.ClassBStaking, Generation: 3, GuardNonce: 9, NextRequestID: 2, BoundHeight: 1}}
	_, err := e.ws.PutState(tombstone, protocol.NamespaceOption(state.StakingNamespace), protocol.KeyOption(binary.BigEndian.AppendUint64([]byte{13}, index)))
	require.NoError(t, err)
	vaultTombstone := &staking.BucketGuardV2Record{BucketGuardV2: bucketguard.BucketGuardV2{BucketClass: bucketguard.ClassAVault, Generation: 4, GuardNonce: 7, NextRequestID: 1, BoundHeight: 1}}
	_, err = e.ws.PutState(vaultTombstone, protocol.NamespaceOption(state.StakingNamespace), protocol.KeyOption(binary.BigEndian.AppendUint64([]byte{11}, index)))
	require.NoError(t, err)
	receipt := e.run(t, _staker, 3, envelope(3).SetAction(action.NewWithdrawStake(index, nil)).Build(), withdrawTime)
	require.EqualValues(t, 1, receipt.Status)
	_, err = staking.ReadStakingGuardV2(e.ws, index)
	require.ErrorIs(t, err, state.ErrStateNotExist)
	got, err := staking.ReadVaultGuard(e.ws, index)
	require.NoError(t, err)
	require.Equal(t, vaultTombstone, got)
}

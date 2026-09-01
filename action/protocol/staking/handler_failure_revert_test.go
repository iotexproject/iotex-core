// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// The tests in this file are the only ones in the package that drive the
// staking handlers against a real working set instead of
// testdb.NewMockStateManager. The mock keeps no undo log -- its Snapshot()
// returns a constant and testdb.AllowRevert stubs Revert() to a no-op -- so a
// unit-level version of these cases would pass whether or not the state
// manager is actually rolled back. Only a working set reverts, which is why
// this file lives in the external test package and builds a state factory.

package staking_test

import (
	"context"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

var (
	// candidate owner / operator / reward address, and the account that opens
	// the ordinary (non self-stake) bucket the withdraw acts on.
	_candOwner = identityset.Address(27)
	_staker    = identityset.Address(28)

	_selfStakeAmount = unit.ConvertIotxToRau(1200000)
	_stakeAmount     = unit.ConvertIotxToRau(100)
	_testGasPrice    = big.NewInt(unit.Qev)
)

// stakingEnv drives staking actions through Protocol.Handle against a working
// set produced by a real state factory.
type stakingEnv struct {
	p  *staking.Protocol
	ws protocol.StateManagerWithCloser
	g  genesis.Genesis
}

// testDepositGas mirrors the package's own test deposit function: it takes the
// fee out of the caller's balance so a settled receipt is observable as a
// balance change, without pulling the rewarding protocol into the fixture.
func testDepositGas(ctx context.Context, sm protocol.StateManager, gasFee *big.Int, _ ...protocol.DepositOption) ([]*action.TransactionLog, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	acc, err := accountutil.LoadAccount(sm, actionCtx.Caller)
	if err != nil {
		return nil, err
	}
	if err := acc.SubBalance(gasFee); err != nil {
		return nil, err
	}
	return nil, accountutil.StoreAccount(sm, actionCtx.Caller, acc)
}

// newStakingEnv boots a state factory whose genesis turns the corrected
// rollback on at toBeEnabledHeight. math.MaxUint64 leaves it off, which is what
// mainnet runs today.
func newStakingEnv(t *testing.T, toBeEnabledHeight uint64) *stakingEnv {
	r := require.New(t)

	g := genesis.TestDefault()
	g.ToBeEnabledBlockHeight = toBeEnabledHeight
	for _, addr := range []address.Address{_candOwner, _staker} {
		g.InitBalanceMap[addr.String()] = unit.ConvertIotxToRau(100000000).String()
	}

	registry := protocol.NewRegistry()
	r.NoError(account.NewProtocol(testDepositGas).Register(registry))
	p, err := staking.NewProtocol(
		staking.HelperCtx{
			DepositGas:    testDepositGas,
			BlockInterval: func(uint64) time.Duration { return 5 * time.Second },
		},
		&staking.BuilderConfig{
			Staking:                       g.Staking,
			PersistStakingPatchBlock:      math.MaxUint64,
			SkipContractStakingViewHeight: math.MaxUint64,
			Revise: staking.ReviseConfig{
				VoteWeight: g.Staking.VoteWeightCalConsts,
			},
		},
		nil, nil, nil, nil,
	)
	r.NoError(err)
	r.NoError(p.Register(registry))

	// A real KV store, not db.NewMemKVStore(): the candidate center is built
	// with a range scan and the in-memory store does not implement Filter().
	dbPath, err := testutil.PathOfTempFile("staking-failed-receipt")
	r.NoError(err)
	t.Cleanup(func() { testutil.CleanupPath(dbPath) })

	cfg := factory.DefaultConfig
	cfg.Genesis = g
	cfg.Chain.TrieDBPath = dbPath
	cfg.Chain.TrieDBPatchFile = ""
	kv, err := db.CreateKVStoreWithCache(db.DefaultConfig, dbPath, cfg.Chain.StateDBCacheSize)
	r.NoError(err)
	sdb, err := factory.NewStateDB(cfg, kv, factory.RegistryStateDBOption(registry))
	r.NoError(err)

	base := protocol.WithRegistry(genesis.WithGenesisContext(context.Background(), g), registry)
	startCtx := protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(
		protocol.WithBlockCtx(base, protocol.BlockCtx{})))
	r.NoError(sdb.Start(startCtx))
	t.Cleanup(func() { r.NoError(sdb.Stop(startCtx)) })

	// The working set builds its protocol views lazily from the context handed
	// to it, so that context needs the same block height the actions run at.
	wsCtx := protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(
		protocol.WithBlockCtx(base, protocol.BlockCtx{BlockHeight: 1})))
	ws, err := sdb.WorkingSet(wsCtx)
	r.NoError(err)
	t.Cleanup(func() { ws.Close() })

	return &stakingEnv{p: p, ws: ws, g: g}
}

// actionCtx builds the per-action context Handle expects.
func (e *stakingEnv) actionCtx(caller address.Address, nonce uint64, intrinsicGas uint64, blkTime time.Time) context.Context {
	ctx := protocol.WithRegistry(genesis.WithGenesisContext(context.Background(), e.g), protocol.NewRegistry())
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:       caller,
		Nonce:        nonce,
		ActionHash:   hash.Hash256b([]byte{byte(nonce), caller.Bytes()[0]}),
		GasPrice:     _testGasPrice,
		IntrinsicGas: intrinsicGas,
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    1,
		BlockTimeStamp: blkTime,
		GasLimit:       10000000,
	})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{Height: 0}})
	return protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
}

// envelope starts an envelope carrying the fixture's gas settings. SetAction
// takes an unexported interface, so the concrete action has to be supplied at
// the call site rather than passed around as action.Action.
func envelope(nonce uint64) *action.EnvelopeBuilder {
	return (&action.EnvelopeBuilder{}).SetNonce(nonce).SetGasLimit(1000000).SetGasPrice(_testGasPrice)
}

// run sends one staking action through Handle and returns its receipt.
func (e *stakingEnv) run(t *testing.T, caller address.Address, nonce uint64, elp action.Envelope, blkTime time.Time) *action.Receipt {
	r := require.New(t)
	intrinsicGas, err := elp.IntrinsicGas()
	r.NoError(err)
	receipt, err := e.p.Handle(e.actionCtx(caller, nonce, intrinsicGas, blkTime), elp, e.ws)
	r.NoError(err)
	r.NotNil(receipt)
	return receipt
}

func (e *stakingEnv) csm(t *testing.T) staking.CandidateStateManager {
	csm, err := staking.NewCandidateStateManagerWithContext(e.actionCtx(_staker, 1, 0, time.Unix(e.g.Timestamp, 0)), e.ws)
	require.NoError(t, err)
	return csm
}

func (e *stakingEnv) account(t *testing.T, addr address.Address) *state.Account {
	acc, err := accountutil.LoadAccount(e.ws, addr)
	require.NoError(t, err)
	return acc
}

// setupBucket registers a candidate, opens an ordinary bucket for _staker
// against it, and unstakes that bucket so it is ready to be withdrawn. It
// returns the bucket index and the block time at which the withdraw matures.
func (e *stakingEnv) setupBucket(t *testing.T) (uint64, time.Time) {
	r := require.New(t)
	genesisTime := time.Unix(e.g.Timestamp, 0)

	reg, err := action.NewCandidateRegister(
		"cand1", _candOwner.String(), _candOwner.String(), _candOwner.String(),
		_selfStakeAmount.String(), 0, false, nil)
	r.NoError(err)
	receipt := e.run(t, _candOwner, 1, envelope(1).SetAction(reg).Build(), genesisTime)
	r.EqualValues(iotextypes.ReceiptStatus_Success, receipt.Status)

	cs, err := action.NewCreateStake("cand1", _stakeAmount.String(), 0, false, nil)
	r.NoError(err)
	receipt = e.run(t, _staker, 1, envelope(1).SetAction(cs).Build(), genesisTime)
	r.EqualValues(iotextypes.ReceiptStatus_Success, receipt.Status)

	// bucket 0 is the candidate's self-stake, bucket 1 is _staker's.
	const bucketIdx = uint64(1)
	bucket, err := e.csm(t).NativeBucket(bucketIdx)
	r.NoError(err)
	r.Equal(_staker.String(), bucket.Owner.String())

	unstakeTime := genesisTime.Add(time.Hour)
	receipt = e.run(t, _staker, 2, envelope(2).SetAction(action.NewUnstake(bucketIdx, nil)).Build(), unstakeTime)
	r.EqualValues(iotextypes.ReceiptStatus_Success, receipt.Status)

	bucket, err = e.csm(t).NativeBucket(bucketIdx)
	r.NoError(err)
	r.False(bucket.UnstakeStartTime.IsZero())

	// Well past any withdraw waiting period the feature set may pick.
	return bucketIdx, unstakeTime.Add(30 * 24 * time.Hour)
}

// breakBucketPool drives the bucket pool's bucket count to zero while leaving
// the buckets themselves in place. Nothing a user can send produces this state;
// it is the shortest way to make a handler fail *after* it has already written,
// which is the shape this file is about. handleWithdrawStake deletes the bucket
// and its indexes first and only then credits the pool, and a pool with no
// buckets left to release rejects the credit with a failure receipt.
func (e *stakingEnv) breakBucketPool(t *testing.T, buckets int) {
	r := require.New(t)
	csm := e.csm(t)
	for i := 0; i < buckets; i++ {
		r.NoError(csm.CreditBucketPool(big.NewInt(0), true))
	}
	r.NoError(csm.Commit(e.actionCtx(_staker, 1, 0, time.Unix(e.g.Timestamp, 0))))
}

// TestWithdrawStakeFailureLeavesNoPartialState is the reproduction and the
// fix. Before the gate height a withdraw that settles a failure receipt still
// leaves the bucket deleted; after it, the bucket survives untouched while the
// gas charge and the nonce bump the receipt is supposed to keep still land.
func TestWithdrawStakeFailureLeavesNoPartialState(t *testing.T) {
	t.Run("pre-gate keeps today's behaviour", func(t *testing.T) {
		r := require.New(t)
		e := newStakingEnv(t, math.MaxUint64)
		bucketIdx, withdrawTime := e.setupBucket(t)
		e.breakBucketPool(t, 2)

		balanceBefore := new(big.Int).Set(e.account(t, _staker).Balance)
		nonceBefore := e.account(t, _staker).PendingNonce()

		receipt := e.run(t, _staker, 3, envelope(3).SetAction(action.NewWithdrawStake(bucketIdx, nil)).Build(), withdrawTime)
		r.EqualValues(iotextypes.ReceiptStatus_ErrWriteAccount, receipt.Status)

		// The write the handler made before it failed is still there: the
		// bucket the action reports not having touched is gone. Asserted, not
		// endorsed -- historical blocks have to replay to the same state root,
		// so this is what a node below the gate height must keep doing.
		_, err := e.csm(t).NativeBucket(bucketIdx)
		r.ErrorIs(err, state.ErrStateNotExist)

		// Gas and nonce, the two effects a failure receipt is meant to keep,
		// land here as well -- they are the baseline the post-gate case has to
		// reproduce.
		acc := e.account(t, _staker)
		gasFee := new(big.Int).Mul(_testGasPrice, new(big.Int).SetUint64(receipt.GasConsumed))
		r.Equal(new(big.Int).Sub(balanceBefore, gasFee), acc.Balance)
		r.Equal(nonceBefore+1, acc.PendingNonce())
	})

	t.Run("post-gate discards the partial write", func(t *testing.T) {
		r := require.New(t)
		e := newStakingEnv(t, 1)
		bucketIdx, withdrawTime := e.setupBucket(t)
		e.breakBucketPool(t, 2)

		before, err := e.csm(t).NativeBucket(bucketIdx)
		r.NoError(err)
		balanceBefore := new(big.Int).Set(e.account(t, _staker).Balance)
		nonceBefore := e.account(t, _staker).PendingNonce()

		receipt := e.run(t, _staker, 3, envelope(3).SetAction(action.NewWithdrawStake(bucketIdx, nil)).Build(), withdrawTime)
		r.EqualValues(iotextypes.ReceiptStatus_ErrWriteAccount, receipt.Status)

		// The bucket, its amount and its unstake time are exactly as they were.
		after, err := e.csm(t).NativeBucket(bucketIdx)
		r.NoError(err)
		r.Equal(before.StakedAmount, after.StakedAmount)
		r.Equal(before.UnstakeStartTime, after.UnstakeStartTime)
		r.Equal(before.Owner.String(), after.Owner.String())
		r.Equal(before.Candidate.String(), after.Candidate.String())

		// The staked amount was not handed back either -- only the gas came
		// out, and the nonce still advanced, so the failure receipt charges
		// its caller just as it does pre-gate.
		acc := e.account(t, _staker)
		gasFee := new(big.Int).Mul(_testGasPrice, new(big.Int).SetUint64(receipt.GasConsumed))
		r.Equal(new(big.Int).Sub(balanceBefore, gasFee), acc.Balance)
		r.Equal(nonceBefore+1, acc.PendingNonce())
	})
}

// TestWithdrawStakeSuccessUnaffected pins the success path: turning the gate on
// must not disturb an action that does not fail.
func TestWithdrawStakeSuccessUnaffected(t *testing.T) {
	for _, tc := range []struct {
		name        string
		toBeEnabled uint64
	}{
		{"pre-gate", math.MaxUint64},
		{"post-gate", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			e := newStakingEnv(t, tc.toBeEnabled)
			bucketIdx, withdrawTime := e.setupBucket(t)

			balanceBefore := new(big.Int).Set(e.account(t, _staker).Balance)
			receipt := e.run(t, _staker, 3, envelope(3).SetAction(action.NewWithdrawStake(bucketIdx, nil)).Build(), withdrawTime)
			r.EqualValues(iotextypes.ReceiptStatus_Success, receipt.Status)

			// A successful withdraw does delete the bucket, gate or no gate.
			_, err := e.csm(t).NativeBucket(bucketIdx)
			r.ErrorIs(err, state.ErrStateNotExist)

			gasFee := new(big.Int).Mul(_testGasPrice, new(big.Int).SetUint64(receipt.GasConsumed))
			expected := new(big.Int).Sub(new(big.Int).Add(balanceBefore, _stakeAmount), gasFee)
			r.Equal(expected, e.account(t, _staker).Balance)
		})
	}
}

// TestSnapshotCoversStakingView pins the assumption the rollback rests on: a
// state-manager snapshot covers the staking protocol view, not only the
// key-value store. Candidate records live in the candidate center and reach
// state only at commit, so a rollback that restored the store but not the view
// would leave a failed action's candidate changes standing.
//
// Both cases matter. A candidate already changed earlier in the same block sits
// in the center's pending change list; one that has been committed is served
// from the base. The revert has to put either back.
func TestSnapshotCoversStakingView(t *testing.T) {
	r := require.New(t)
	e := newStakingEnv(t, 1)
	e.setupBucket(t)
	commitCtx := e.actionCtx(_staker, 1, 0, time.Unix(e.g.Timestamp, 0))

	zeroOutVotes := func() *big.Int {
		votes := new(big.Int).Set(e.csm(t).GetByName("cand1").Votes)
		r.Positive(votes.Sign(), "fixture must leave the candidate with votes to drop")

		si := e.ws.Snapshot()
		csm := e.csm(t)
		cand := csm.GetByName("cand1")
		r.NoError(cand.SubVote(votes))
		r.NoError(csm.Upsert(cand))
		r.Zero(e.csm(t).GetByName("cand1").Votes.Sign(), "mutation must be visible before the revert")

		r.NoError(e.ws.Revert(si))
		return votes
	}

	// The candidate is still pending from setupBucket.
	votes := zeroOutVotes()
	r.Equal(votes, e.csm(t).GetByName("cand1").Votes)

	// And again once it has been committed to the center's base.
	r.NoError(e.csm(t).Commit(commitCtx))
	votes = zeroOutVotes()
	r.Equal(votes, e.csm(t).GetByName("cand1").Votes)
}

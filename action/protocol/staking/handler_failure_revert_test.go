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
	"crypto/sha256"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
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
	// owner of the candidate the proof-of-possession case registers
	_popOwner = identityset.Address(29)

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
func newStakingEnv(t *testing.T, gateHeight uint64) *stakingEnv {
	return newStakingEnvWith(t, gateHeight)
}

// newStakingEnvWith is newStakingEnv with room to adjust the genesis before the
// factory is built, for cases that need a feature the default heights leave off.
func newStakingEnvWith(t *testing.T, gateHeight uint64, tweaks ...func(*genesis.Genesis)) *stakingEnv {
	r := require.New(t)

	g := genesis.TestDefault()
	// The rollback rides Zanzibar Gamma. All three heights are set together
	// because a chain that has activated none of the family carries them
	// equal; leaving the earlier ones off would be a partial-family genesis,
	// and the register case below also needs EnforceBLSPoP, which Zanzibar
	// carries.
	g.ZanzibarBlockHeight = gateHeight
	g.ZanzibarBetaBlockHeight = gateHeight
	g.ZanzibarGammaBlockHeight = gateHeight
	for _, tweak := range tweaks {
		tweak(&g)
	}
	for _, addr := range []address.Address{_candOwner, _staker, _popOwner} {
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

// blsKey derives a deterministic BLS keypair so the case does not depend on
// randomness across runs.
func blsKey(t *testing.T, seed string) *crypto.BLS12381PrivateKey {
	t.Helper()
	h := sha256.Sum256([]byte(seed))
	sk, err := crypto.GenerateBLS12381PrivateKey(h[:])
	require.NoError(t, err)
	return sk
}

// TestCandidateRegisterPoPFailureLeavesNoBucket covers the ordering that makes
// this rollback matter beyond the withdraw case: handleCandidateRegister writes
// the self-stake bucket and its indexes and only then verifies the BLS
// proof-of-possession, and a rejected proof settles a failure receipt rather
// than an error. Without the rollback the bucket stays behind, owned by an
// address that has no candidate record but still counted against the bucket
// total.
//
// EnforceBLSPoP rides the same height as the corrected rollback, so there is no
// pre-gate half to compare against: below the gate the proof is not checked at
// all and the registration simply succeeds. The case is therefore post-gate
// only, and its teeth were confirmed by removing the rollback and watching the
// orphan bucket reappear.
func TestCandidateRegisterPoPFailureLeavesNoBucket(t *testing.T) {
	r := require.New(t)
	// XinguBlockHeight reaches the BLS register path; the CandidateBLSPublicKey
	// feature is gated on it and TestDefault leaves it out of range.
	e := newStakingEnvWith(t, 1, func(g *genesis.Genesis) { g.XinguBlockHeight = 0 })
	genesisTime := time.Unix(e.g.Timestamp, 0)

	// buckets 0 and 1 belong to the fixture candidate and _staker, so a
	// self-stake bucket written by the register below would land at index 2.
	e.setupBucket(t)
	const orphanIdx = uint64(2)
	_, err := e.csm(t).NativeBucket(orphanIdx)
	r.ErrorIs(err, state.ErrStateNotExist, "fixture must not have used the index yet")

	balanceBefore := new(big.Int).Set(e.account(t, _popOwner).Balance)
	nonceBefore := e.account(t, _popOwner).PendingNonce()

	// A pubkey with no proof at all: the shape the gate exists to reject.
	pk := blsKey(t, "register-pop-failure").PublicKey().Bytes()
	reg, err := action.NewCandidateRegisterWithBLS(
		"cand2", _popOwner.String(), _popOwner.String(), _popOwner.String(),
		_selfStakeAmount.String(), 0, false, pk, nil, nil)
	r.NoError(err)

	receipt := e.run(t, _popOwner, 1, envelope(1).SetAction(reg).Build(), genesisTime)
	r.EqualValues(iotextypes.ReceiptStatus_ErrUnauthorizedOperator, receipt.Status)

	// The self-stake bucket the handler wrote before it reached the proof is
	// gone, so nothing is left owned by a candidate that was never recorded.
	_, err = e.csm(t).NativeBucket(orphanIdx)
	r.ErrorIs(err, state.ErrStateNotExist)
	r.Nil(e.csm(t).GetByName("cand2"), "the rejected candidate must not be registered")
	r.Nil(e.csm(t).GetByOwner(_popOwner), "the rejected candidate must not be registered")

	// The self-stake was not taken either: only the gas left the account, and
	// the nonce still advanced, which is what a failure receipt owes its caller.
	acc := e.account(t, _popOwner)
	gasFee := new(big.Int).Mul(_testGasPrice, new(big.Int).SetUint64(receipt.GasConsumed))
	r.Equal(new(big.Int).Sub(balanceBefore, gasFee), acc.Balance)
	r.Equal(nonceBefore+1, acc.PendingNonce())
}

// runExpectingError sends one staking action through Handle and returns the
// error it reports instead of a receipt.
func (e *stakingEnv) runExpectingError(t *testing.T, caller address.Address, nonce uint64, elp action.Envelope, blkTime time.Time) error {
	r := require.New(t)
	intrinsicGas, err := elp.IntrinsicGas()
	r.NoError(err)
	receipt, err := e.p.Handle(e.actionCtx(caller, nonce, intrinsicGas, blkTime), elp, e.ws)
	r.Error(err)
	r.Nil(receipt)
	return err
}

// setupMigratableBucket opens an auto-staked, still-staked bucket for _staker,
// which is what validateStakeMigrate requires, and returns its index.
func (e *stakingEnv) setupMigratableBucket(t *testing.T) uint64 {
	r := require.New(t)
	genesisTime := time.Unix(e.g.Timestamp, 0)

	cs, err := action.NewCreateStake("cand1", _stakeAmount.String(), 91, true, nil)
	r.NoError(err)
	receipt := e.run(t, _staker, 3, envelope(3).SetAction(cs).Build(), genesisTime)
	r.EqualValues(iotextypes.ReceiptStatus_Success, receipt.Status)

	const bucketIdx = uint64(2)
	bucket, err := e.csm(t).NativeBucket(bucketIdx)
	r.NoError(err)
	r.True(bucket.AutoStake)
	r.Equal(_staker.String(), bucket.Owner.String())
	return bucketIdx
}

// TestStakeMigrateFailureRevertsNested covers the one handler that snapshots on
// its own: handleStakeMigrate withdraws the native bucket, then calls the
// staking contract, and reverts to its own inner snapshot if that call fails.
// The rollback added here takes an outer snapshot around the same action, so
// the two have to compose -- the inner revert must not leave the outer index
// unusable, and neither may leave the withdrawn bucket destroyed.
//
// The contract call is made to fail by handing the action a context whose
// registry carries no execution protocol, which is the shortest way to reach
// the failure branch after withdrawBucket has already written. That branch
// returns a plain error rather than a ReceiptError, so it is also the
// hard-error path: no receipt is settled, and nothing may survive the action.
func TestStakeMigrateFailureRevertsNested(t *testing.T) {
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
			e.setupBucket(t)
			bucketIdx := e.setupMigratableBucket(t)

			before, err := e.csm(t).NativeBucket(bucketIdx)
			r.NoError(err)
			votesBefore := new(big.Int).Set(e.csm(t).GetByName("cand1").Votes)
			balanceBefore := new(big.Int).Set(e.account(t, _staker).Balance)
			nonceBefore := e.account(t, _staker).PendingNonce()

			err = e.runExpectingError(t, _staker, 4,
				envelope(4).SetAction(action.NewMigrateStake(bucketIdx)).Build(),
				time.Unix(e.g.Timestamp, 0))
			r.ErrorContains(err, "execution protocol is not registered")

			// The bucket the migration withdrew before the contract call failed
			// is back, with its stake and its candidate intact.
			after, err := e.csm(t).NativeBucket(bucketIdx)
			r.NoError(err)
			r.Equal(before.StakedAmount, after.StakedAmount)
			r.Equal(before.Candidate.String(), after.Candidate.String())
			r.Equal(before.Owner.String(), after.Owner.String())
			r.True(after.AutoStake)

			// The candidate keeps the votes the withdraw had taken off it, and
			// a hard error settles no receipt, so neither gas nor nonce moved.
			r.Equal(votesBefore, e.csm(t).GetByName("cand1").Votes)
			acc := e.account(t, _staker)
			r.Equal(balanceBefore, acc.Balance)
			r.Equal(nonceBefore, acc.PendingNonce())
		})
	}
}

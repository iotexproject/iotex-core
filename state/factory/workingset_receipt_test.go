// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// poisonProtocol fails actions from one sender at a chosen stage, after
// writing to state, as a handler with a bad input would.
type poisonProtocol struct {
	bad     map[string]bool
	scratch address.Address
	stage   poisonStage
	cancel  context.CancelFunc
}

type poisonStage int

const (
	// failReceipt fails in HandleReceipt, after the action has run
	failReceipt poisonStage = iota
	// failHandle returns an error from Handle
	failHandle
	// panicHandle panics in Handle
	panicHandle
	// cancelHandle cancels the mint context and fails in Handle
	cancelHandle
	// expireHandle waits in Handle for the mint deadline to pass, then fails
	expireHandle
)

func (p *poisonProtocol) Handle(ctx context.Context, _ action.Envelope, sm protocol.StateManager) (*action.Receipt, error) {
	if p.stage == failReceipt || !p.isBad(ctx) {
		return nil, nil
	}
	if err := p.touchState(sm); err != nil {
		return nil, err
	}
	switch p.stage {
	case panicHandle:
		panic("injected panic while handling action")
	case cancelHandle:
		p.cancel()
	case expireHandle:
		<-ctx.Done()
	}
	return nil, errors.New("injected action handling failure")
}

func (p *poisonProtocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, uint64, error) {
	return nil, 0, nil
}

func (p *poisonProtocol) Register(r *protocol.Registry) error {
	return r.Register(p.Name(), p)
}

func (p *poisonProtocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(p.Name(), p)
}

func (p *poisonProtocol) Name() string { return "poison" }

func (p *poisonProtocol) HandleReceipt(ctx context.Context, _ action.Envelope, sm protocol.StateManager, _ *action.Receipt) error {
	if p.stage != failReceipt || !p.isBad(ctx) {
		return nil
	}
	if err := p.touchState(sm); err != nil {
		return err
	}
	return errors.New("injected receipt handling failure")
}

func (p *poisonProtocol) isBad(ctx context.Context) bool {
	return p.bad[protocol.MustGetActionCtx(ctx).Caller.String()]
}

func (p *poisonProtocol) touchState(sm protocol.StateManager) error {
	acct, err := accountutil.LoadOrCreateAccount(sm, p.scratch)
	if err != nil {
		return err
	}
	if err := acct.AddBalance(big.NewInt(1000)); err != nil {
		return err
	}
	return accountutil.StoreAccount(sm, p.scratch, acct)
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return m.GetCounter().GetValue()
}

func makeTransferFrom(t *testing.T, signer int, nonce uint64, gasPrice int64) *action.SealedEnvelope {
	tsf := action.NewTransfer(big.NewInt(1), identityset.Address(29).String(), nil)
	evlp := (&action.EnvelopeBuilder{}).
		SetAction(tsf).
		SetGasLimit(testutil.TestGasLimit).
		SetGasPrice(big.NewInt(gasPrice)).
		SetNonce(nonce).
		SetChainID(1).
		SetVersion(1).
		Build()
	selp, err := action.Sign(evlp, identityset.PrivateKey(signer))
	require.NoError(t, err)
	return selp
}

// TestWorkingSet_Mint_DropsFailingAction verifies that an action that fails
// while running is left out of the block, with its state changes reverted
// and its sender evicted, while the rest of the draft is kept and validates
// on another node. It also runs with a proposer that writes history to
// erigon, where a revert after the tx was finalized panics.
func TestWorkingSet_Mint_DropsFailingAction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stage  poisonStage
		reason string
	}{
		{"receipt", failReceipt, "run_failed"},
		{"handle", failHandle, "run_failed"},
		{"panic", panicHandle, "panic"},
	} {
		for _, withHistory := range []bool{false, true} {
			store := "statedb"
			if withHistory {
				store = "erigon"
			}
			t.Run(tc.name+"/"+store, func(t *testing.T) {
				skips := _mintSkipMtc.WithLabelValues(tc.reason, "Transfer")
				before := counterValue(t, skips)
				testMintDropsFailingAction(t, tc.stage, withHistory)
				require.Equal(t, before+1, counterValue(t, skips))
			})
		}
	}
}

// TestWorkingSet_Mint_DropsOnlyTheFailingSender verifies that dropping a
// failing action leaves out only its sender, not the account that runs next.
func TestWorkingSet_Mint_DropsOnlyTheFailingSender(t *testing.T) {
	require := require.New(t)
	proposer, validator, ctx := newPoisonMintEnv(t, &poisonProtocol{stage: failHandle}, false, context.Background())
	// the poison action is priced higher, so it fails before the honest one runs
	ap, ok := newPoisonActPoolWithPrices(t, 1, 2, 1)
	blk, err := proposer.Mint(ctx, ap, identityset.PrivateKey(27))
	require.NoError(err)
	okHash, err := ok.Hash()
	require.NoError(err)
	require.Len(blk.Actions, 1)
	h, err := blk.Actions[0].Hash()
	require.NoError(err)
	require.Equal(okHash, h)
	require.NoError(validator.Validate(ctx, blk))
}

// TestWorkingSet_Mint_LimitsDropLogs verifies that a draft logs only the
// first few actions it drops for failing to run, and sums up the rest.
func TestWorkingSet_Mint_LimitsDropLogs(t *testing.T) {
	require := require.New(t)
	const senders = _mintDropLogLimit + 4
	poison := &poisonProtocol{stage: failHandle, bad: map[string]bool{}}
	pending := map[string][]*action.SealedEnvelope{}
	for i := 0; i < senders; i++ {
		addr := identityset.Address(i).String()
		poison.bad[addr] = true
		pending[addr] = []*action.SealedEnvelope{makeTransferFrom(t, i, 1, 1)}
	}
	proposer, _, ctx := newPoisonMintEnv(t, poison, false, context.Background())
	ap := mock_actpool.NewMockActPool(gomock.NewController(t))
	ap.EXPECT().BundlePool().Return(nil).Times(1)
	ap.EXPECT().PendingActionMap().Return(pending).Times(1)
	ap.EXPECT().DeleteAction(gomock.Any()).Times(senders)

	core, logs := observer.New(zap.InfoLevel)
	defer zap.ReplaceGlobals(zap.New(core))()
	blk, err := proposer.Mint(ctx, ap, identityset.PrivateKey(27))
	require.NoError(err)
	require.Empty(blk.Actions)
	require.Equal(_mintDropLogLimit, logs.FilterMessage("dropping action that failed during mint").Len())
	summary := logs.FilterMessage("dropped more actions that failed during mint than were logged").All()
	require.Len(summary, 1)
	require.EqualValues(senders, summary[0].ContextMap()["dropped"])
}

// TestWorkingSet_Mint_StopsWhenCancelled verifies that an action failing
// because the mint context is gone abandons the draft without evicting the
// sender, who did nothing wrong.
func TestWorkingSet_Mint_StopsWhenCancelled(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proposer, _, ctx := newPoisonMintEnv(t, &poisonProtocol{stage: cancelHandle, cancel: cancel}, false, ctx)
	ap, _ := newPoisonActPool(t, 0)
	var err error
	require.NotPanics(func() {
		_, err = proposer.Mint(ctx, ap, identityset.PrivateKey(27))
	})
	require.ErrorContains(err, "mint cancelled while running an action")
}

// TestWorkingSet_Mint_SealsAtDeadline verifies that an action failing after
// the mint deadline passed is reverted and left out, without evicting its
// sender, and that the draft is sealed with the actions it already holds.
func TestWorkingSet_Mint_SealsAtDeadline(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	proposer, validator, ctx := newPoisonMintEnv(t, &poisonProtocol{stage: expireHandle}, false, ctx)
	ap, ok := newPoisonActPool(t, 0)
	skips := _mintSkipMtc.WithLabelValues("deadline", "Transfer")
	before := counterValue(t, skips)
	defer func() { require.Equal(before+1, counterValue(t, skips)) }()
	var (
		blk *block.Block
		err error
	)
	require.NotPanics(func() {
		blk, err = proposer.Mint(ctx, ap, identityset.PrivateKey(27))
	})
	require.NoError(err)
	okHash, err := ok.Hash()
	require.NoError(err)
	require.Len(blk.Actions, 1)
	h, err := blk.Actions[0].Hash()
	require.NoError(err)
	require.Equal(okHash, h)
	require.NoError(validator.Validate(context.WithoutCancel(ctx), blk))
}

func testMintDropsFailingAction(t *testing.T, stage poisonStage, withHistory bool) {
	require := require.New(t)
	proposer, validator, ctx := newPoisonMintEnv(t, &poisonProtocol{stage: stage}, withHistory, context.Background())
	ap, ok := newPoisonActPool(t, 1)

	var (
		blk *block.Block
		err error
	)
	require.NotPanics(func() {
		blk, err = proposer.Mint(ctx, ap, identityset.PrivateKey(27))
	})
	require.NoError(err)
	okHash, err := ok.Hash()
	require.NoError(err)
	var included []hash.Hash256
	for _, selp := range blk.Actions {
		h, err := selp.Hash()
		require.NoError(err)
		included = append(included, h)
	}
	require.Equal([]hash.Hash256{okHash}, included)

	// a validator re-executes the block from scratch: if the dropped action
	// had left any state behind on the proposer, the digests would differ
	require.NoError(validator.Validate(ctx, blk))
	// commit the draft on the proposer, which also writes its history to
	// erigon and releases the erigon store before the factory stops
	require.NoError(proposer.PutBlock(ctx, blk))
}

var (
	_poisonSender = identityset.Address(28)
	_honestSender = identityset.Address(26)
)

// newPoisonMintEnv returns a proposer, which writes history to erigon if
// withHistory is set, a validator, and the context to mint block 1 in. The
// poison protocol runs ahead of the account protocol, so it sees every
// action before the transfer is handled.
func newPoisonMintEnv(t *testing.T, poison *poisonProtocol, withHistory bool, parent context.Context) (Factory, Factory, context.Context) {
	require := require.New(t)
	if poison.bad == nil {
		poison.bad = map[string]bool{_poisonSender.String(): true}
	}
	poison.scratch = identityset.Address(30)
	registry := protocol.NewRegistry()
	noDeposit := func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
		return nil, nil
	}
	require.NoError(poison.Register(registry))
	require.NoError(account.NewProtocol(noDeposit).Register(registry))
	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	for bad := range poison.bad {
		cfg.Genesis.InitBalanceMap[bad] = "100000000"
	}
	cfg.Genesis.InitBalanceMap[_honestSender.String()] = "100000000"
	newFactory := func(withHistory bool) Factory {
		cfg := cfg
		if withHistory {
			cfg.Chain.HistoryIndexPath = t.TempDir()
		}
		f, err := NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
		require.NoError(err)
		startCtx := protocol.WithBlockchainCtx(
			protocol.WithBlockCtx(
				genesis.WithGenesisContext(context.Background(), cfg.Genesis),
				protocol.BlockCtx{},
			),
			protocol.BlockchainCtx{ChainID: 1},
		)
		require.NoError(f.Start(startCtx))
		t.Cleanup(func() {
			// a draft that failed part way keeps its erigon store open, and
			// stopping the factory would then wait on it forever
			if t.Failed() {
				return
			}
			require.NoError(f.Stop(startCtx))
		})
		return f
	}

	ctx := protocol.WithBlockCtx(parent,
		protocol.BlockCtx{
			BlockHeight:    uint64(1),
			BlockTimeStamp: time.Unix(1700000000, 0),
			Producer:       identityset.Address(27),
			GasLimit:       testutil.TestGasLimit * 100000,
		})
	ctx = protocol.WithBlockchainCtx(
		genesis.WithGenesisContext(ctx, cfg.Genesis),
		protocol.BlockchainCtx{ChainID: 1},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	return newFactory(withHistory), newFactory(false), ctx
}

// newPoisonActPool returns a pool holding one action from the poison sender
// and a higher-priced one from an honest sender, which therefore runs first
// so the draft already holds it when the poison action fails. The pool
// expects the poison sender to be evicted evictions times.
func newPoisonActPool(t *testing.T, evictions int) (*mock_actpool.MockActPool, *action.SealedEnvelope) {
	return newPoisonActPoolWithPrices(t, evictions, 0, 1)
}

func newPoisonActPoolWithPrices(t *testing.T, evictions int, poisonPrice, okPrice int64) (*mock_actpool.MockActPool, *action.SealedEnvelope) {
	poison, ok := makeTransferFrom(t, 28, 1, poisonPrice), makeTransferFrom(t, 26, 1, okPrice)
	ap := mock_actpool.NewMockActPool(gomock.NewController(t))
	ap.EXPECT().BundlePool().Return(nil).Times(1)
	ap.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
		_poisonSender.String(): {poison},
		_honestSender.String(): {ok},
	}).Times(1)
	ap.EXPECT().DeleteAction(gomock.Any()).Do(func(addr address.Address) {
		require.Equal(t, _poisonSender.String(), addr.String())
	}).Times(evictions)
	return ap, ok
}

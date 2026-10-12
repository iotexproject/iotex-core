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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

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

// failingReceiptProtocol fails HandleReceipt for actions from one sender,
// after writing to state, as a post-action handler with a bad input would.
type failingReceiptProtocol struct {
	bad     address.Address
	scratch address.Address
}

func (p *failingReceiptProtocol) Handle(context.Context, action.Envelope, protocol.StateManager) (*action.Receipt, error) {
	return nil, nil
}

func (p *failingReceiptProtocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, uint64, error) {
	return nil, 0, nil
}

func (p *failingReceiptProtocol) Register(r *protocol.Registry) error {
	return r.Register(p.Name(), p)
}

func (p *failingReceiptProtocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(p.Name(), p)
}

func (p *failingReceiptProtocol) Name() string { return "failingReceipt" }

func (p *failingReceiptProtocol) HandleReceipt(ctx context.Context, _ action.Envelope, sm protocol.StateManager, _ *action.Receipt) error {
	if protocol.MustGetActionCtx(ctx).Caller.String() != p.bad.String() {
		return nil
	}
	acct, err := accountutil.LoadOrCreateAccount(sm, p.scratch)
	if err != nil {
		return err
	}
	if err := acct.AddBalance(big.NewInt(1000)); err != nil {
		return err
	}
	if err := accountutil.StoreAccount(sm, p.scratch, acct); err != nil {
		return err
	}
	return errors.New("injected receipt handling failure")
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

// TestWorkingSet_Mint_DropsActionFailingReceiptHandling verifies that an
// action whose receipt handling fails is left out of the block, with its
// state changes reverted and its sender evicted, while the rest of the draft
// is kept and validates on another node. It runs with a proposer that also
// writes history to erigon, where a revert after the tx was finalized panics.
func TestWorkingSet_Mint_DropsActionFailingReceiptHandling(t *testing.T) {
	t.Run("statedb", func(t *testing.T) {
		testMintDropsActionFailingReceiptHandling(t, false)
	})
	t.Run("erigon", func(t *testing.T) {
		testMintDropsActionFailingReceiptHandling(t, true)
	})
}

func testMintDropsActionFailingReceiptHandling(t *testing.T, withHistory bool) {
	require := require.New(t)
	bad, honest := identityset.Address(28), identityset.Address(26)
	registry := protocol.NewRegistry()
	noDeposit := func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
		return nil, nil
	}
	require.NoError(account.NewProtocol(noDeposit).Register(registry))
	require.NoError((&failingReceiptProtocol{bad: bad, scratch: identityset.Address(30)}).Register(registry))
	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	cfg.Genesis.InitBalanceMap[bad.String()] = "100000000"
	cfg.Genesis.InitBalanceMap[honest.String()] = "100000000"
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
	proposer, validator := newFactory(withHistory), newFactory(false)

	// the honest action is priced higher so it runs first and the draft
	// already holds it when the poison action fails
	poison, ok := makeTransferFrom(t, 28, 1, 0), makeTransferFrom(t, 26, 1, 1)
	ctrl := gomock.NewController(t)
	ap := mock_actpool.NewMockActPool(ctrl)
	ap.EXPECT().BundlePool().Return(nil).Times(1)
	ap.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
		bad.String():    {poison},
		honest.String(): {ok},
	}).Times(1)
	ap.EXPECT().DeleteAction(gomock.Any()).Do(func(addr address.Address) {
		require.Equal(bad.String(), addr.String())
	}).Times(1)

	ctx := protocol.WithBlockCtx(context.Background(),
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

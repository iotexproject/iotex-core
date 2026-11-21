// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// tipProtocol pays a fixed priority fee on every transfer, fails receipt
// handling for transfers to poison, and creates one system action that
// records the accumulated tips it is run with.
type tipProtocol struct {
	tip    *big.Int
	poison string
	seen   []*big.Int
}

func (p *tipProtocol) Name() string { return "tips" }

func (p *tipProtocol) Handle(ctx context.Context, elp action.Envelope, _ protocol.StateManager) (*action.Receipt, error) {
	if _, ok := elp.Action().(*action.GrantReward); !ok {
		return nil, nil
	}
	tips := protocol.MustGetBlockCtx(ctx).AccumulatedTips
	p.seen = append(p.seen, new(big.Int).Set(&tips))
	return &action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success)}, nil
}

func (p *tipProtocol) HandleReceipt(_ context.Context, elp action.Envelope, _ protocol.StateManager, receipt *action.Receipt) error {
	tsf, ok := elp.Action().(*action.Transfer)
	if !ok {
		return nil
	}
	if tsf.Recipient() == p.poison {
		return errors.New("injected failure inside bundle")
	}
	receipt.AddTransactionLogs(&action.TransactionLog{
		Type:      iotextypes.TransactionLogType_PRIORITY_FEE,
		Sender:    identityset.Address(28).String(),
		Recipient: identityset.Address(27).String(),
		Amount:    new(big.Int).Set(p.tip),
	})
	return nil
}

func (p *tipProtocol) CreatePostSystemActions(ctx context.Context, _ protocol.StateReader) ([]action.Envelope, error) {
	height := protocol.MustGetBlockCtx(ctx).BlockHeight
	return []action.Envelope{(&action.EnvelopeBuilder{}).
		SetNonce(0).
		SetGasPrice(big.NewInt(0)).
		SetAction(action.NewGrantReward(action.BlockReward, height)).
		Build()}, nil
}

func (p *tipProtocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, uint64, error) {
	return nil, 0, protocol.ErrUnimplemented
}

func (p *tipProtocol) Register(r *protocol.Registry) error { return r.Register(p.Name(), p) }

func (p *tipProtocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(p.Name(), p)
}

// TestWorkingSet_Mint_BundleTips pins the tips the post-block system actions
// see after bundles: only those of bundles that made it into the block, each
// counted once.
func TestWorkingSet_Mint_BundleTips(t *testing.T) {
	poison, ok := identityset.Address(30).String(), identityset.Address(29).String()
	small := big.NewInt(7)
	// above 2^64, so the running total is a multi-word big.Int with spare
	// capacity that an aliased copy can write into
	large, _ := new(big.Int).SetString("10000000000000000021", 10)
	times := func(n int64, x *big.Int) *big.Int { return new(big.Int).Mul(big.NewInt(n), x) }
	for _, tc := range []struct {
		name    string
		tip     *big.Int
		bundles [][]string // recipients, one transfer each, nonces in order
		want    *big.Int
	}{
		{"dropped bundle", small, [][]string{{ok, poison}}, big.NewInt(0)},
		{"kept then dropped", small, [][]string{{ok}, {ok, poison}}, small},
		{"two bundles", small, [][]string{{ok}, {ok}}, times(2, small)},
		{"three large bundles", large, [][]string{{ok}, {ok}, {ok}}, times(3, large)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)
			tips := &tipProtocol{tip: tc.tip, poison: poison}
			registry := protocol.NewRegistry()
			require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
			require.NoError(tips.Register(registry))

			cfg := Config{
				Chain:   blockchain.DefaultConfig,
				Genesis: genesis.TestDefault(),
			}
			cfg.Genesis.VanuatuBlockHeight = 1 // dynamic fee tx, so tips are accumulated
			testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
			cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100000000000000000000"
			f, err := NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
			require.NoError(err)
			startCtx := protocol.WithBlockCtx(
				genesis.WithGenesisContext(context.Background(), cfg.Genesis),
				protocol.BlockCtx{},
			)
			require.NoError(f.Start(startCtx))
			defer func() { require.NoError(f.Stop(startCtx)) }()

			bp := actpool.NewBundlePool(cfg.Genesis)
			// the sender is a fresh genesis account, so under
			// UseZeroNonceForFreshAccount its first nonce is 0; starting at 1
			// leaves a gap that block validation rejects
			nonce := uint64(0)
			for i, recipients := range tc.bundles {
				bundle := action.NewBundle()
				for _, to := range recipients {
					require.NoError(bundle.Add(bundleTransfer(t, nonce, to)))
					nonce++
				}
				bundle.SetTargetBlockHeight(1)
				require.NoError(bp.AddBundle(context.Background(), identityset.Address(28), string(rune('a'+i)), bundle))
			}

			ctrl := gomock.NewController(t)
			ap := mock_actpool.NewMockActPool(ctrl)
			ap.EXPECT().BundlePool().Return(bp).Times(1)
			ap.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{}).Times(1)

			ctx := protocol.WithBlockCtx(context.Background(),
				protocol.BlockCtx{
					BlockHeight: uint64(1),
					Producer:    identityset.Address(27),
					GasLimit:    testutil.TestGasLimit * 100000,
					BaseFee:     big.NewInt(0),
				})
			ctx = protocol.WithBlockchainCtx(
				genesis.WithGenesisContext(ctx, cfg.Genesis),
				protocol.BlockchainCtx{ChainID: 1},
			)
			ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
			require.True(protocol.MustGetFeatureCtx(ctx).EnableDynamicFeeTx)

			_, err = f.Mint(ctx, ap, identityset.PrivateKey(27))
			require.NoError(err)
			require.Len(tips.seen, 1)
			require.Equal(tc.want.String(), tips.seen[0].String())
		})
	}
}

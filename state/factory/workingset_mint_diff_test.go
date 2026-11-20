// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
)

// The tests below mint a draft in which a failing action follows one that
// ran through, then have a validator execute the block from scratch: if
// reverting the failing action left anything behind, or took anything of
// the action before it with it, the two would disagree.

// signFrom signs the action setAction puts in the envelope.
func signFrom(t *testing.T, signer int, nonce uint64, price int64, gas uint64, setAction func(*action.EnvelopeBuilder) *action.EnvelopeBuilder) *action.SealedEnvelope {
	selp, err := action.Sign(setAction(&action.EnvelopeBuilder{}).
		SetGasLimit(gas).
		SetGasPrice(big.NewInt(price)).
		SetNonce(nonce).
		SetChainID(1).
		SetVersion(1).
		Build(), identityset.PrivateKey(signer))
	require.NoError(t, err)
	return selp
}

// newDiffActPool returns a pool serving pending once and expecting each of
// evicted to be evicted once.
func newDiffActPool(t *testing.T, pending map[string][]*action.SealedEnvelope, evicted ...address.Address) *mock_actpool.MockActPool {
	ap := mock_actpool.NewMockActPool(gomock.NewController(t))
	ap.EXPECT().BundlePool().Return(nil).Times(1)
	ap.EXPECT().PendingActionMap().Return(pending).Times(1)
	for _, addr := range evicted {
		ap.EXPECT().DeleteAction(gomock.Any()).Do(func(got address.Address) {
			require.Equal(t, addr.String(), got.String())
		}).Times(1)
	}
	return ap
}

func requireBlockActions(t *testing.T, blk *block.Block, want ...*action.SealedEnvelope) {
	require.Len(t, blk.Actions, len(want))
	for i := range want {
		wantHash, err := want[i].Hash()
		require.NoError(t, err)
		gotHash, err := blk.Actions[i].Hash()
		require.NoError(t, err)
		require.Equal(t, wantHash, gotHash)
	}
}

func withExecution(t *testing.T, env *poisonEnv) {
	noDeposit := func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
		return nil, nil
	}
	require.NoError(t, execution.NewProtocol(
		func(uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
		noDeposit,
		func(uint64) (time.Time, error) { return time.Time{}, nil },
		nil,
	).Register(env.registry))
}

// TestWorkingSet_Mint_DropsFailingExecution has a contract deployment fail
// after the EVM wrote its storage, right after another deployment that is
// kept.
func TestWorkingSet_Mint_DropsFailingExecution(t *testing.T) {
	// stores 0x2a at slot 0, then deploys a contract whose code is STOP
	initCode, err := hex.DecodeString("602a60005560016000f3")
	require.NoError(t, err)
	for _, withHistory := range []bool{false, true} {
		name := "statedb"
		if withHistory {
			name = "erigon"
		}
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			proposer, validator, ctx := newPoisonMintEnv(t, &poisonProtocol{stage: failReceipt}, withHistory, context.Background(), withExecution)
			deploy := func(b *action.EnvelopeBuilder) *action.EnvelopeBuilder {
				return b.SetAction(action.NewExecution(action.EmptyAddress, big.NewInt(0), initCode))
			}
			ok := signFrom(t, 26, 1, 2, 1000000, deploy)
			poison := signFrom(t, 28, 1, 1, 1000000, deploy)
			ap := newDiffActPool(t, map[string][]*action.SealedEnvelope{
				_honestSender.String(): {ok},
				_poisonSender.String(): {poison},
			}, _poisonSender)

			blk, err := proposer.Mint(ctx, ap, identityset.PrivateKey(27))
			require.NoError(err)
			requireBlockActions(t, blk, ok)
			require.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[0].Status)
			require.NoError(validator.Validate(ctx, blk))
			require.NoError(proposer.PutBlock(ctx, blk))
		})
	}
}

func withStaking(t *testing.T, env *poisonEnv) {
	noDeposit := func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
		return nil, nil
	}
	sp, err := staking.NewProtocol(
		staking.HelperCtx{
			DepositGas:    noDeposit,
			BlockInterval: func(uint64) time.Duration { return time.Second },
		},
		&staking.BuilderConfig{
			Staking:                       env.cfg.Genesis.Staking,
			PersistStakingPatchBlock:      ^uint64(0),
			SkipContractStakingViewHeight: ^uint64(0),
		},
		nil, nil, nil, nil,
	)
	require.NoError(t, err)
	require.NoError(t, sp.Register(env.registry))
	for _, i := range []int{26, 28, 29} {
		env.cfg.Genesis.InitBalanceMap[identityset.Address(i).String()] = unit.ConvertIotxToRau(10000000).String()
	}
	env.onDisk = true
}

// TestWorkingSet_Mint_DropsFailingStakingAction has a candidate registration
// fail after staking wrote the candidate, its bucket and its view, right
// after another registration that is kept. The next block registers the
// dropped candidate's name again, which only succeeds if the staking view
// of the proposer forgot the dropped candidate.
func TestWorkingSet_Mint_DropsFailingStakingAction(t *testing.T) {
	selfStake := unit.ConvertIotxToRau(1200000).String()
	register := func(t *testing.T, signer int, name string, price int64) *action.SealedEnvelope {
		addr := identityset.Address(signer).String()
		act, err := action.NewCandidateRegister(name, addr, addr, addr, selfStake, 0, false, nil)
		require.NoError(t, err)
		return signFrom(t, signer, 1, price, 100000, func(b *action.EnvelopeBuilder) *action.EnvelopeBuilder {
			return b.SetAction(act)
		})
	}
	for _, withHistory := range []bool{false, true} {
		name := "statedb"
		if withHistory {
			name = "erigon"
		}
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			proposer, validator, ctx := newPoisonMintEnv(t, &poisonProtocol{stage: failReceipt}, withHistory, context.Background(), withStaking)

			ok := register(t, 26, "honest", 2)
			poison := register(t, 28, "poison", 1)
			ap := newDiffActPool(t, map[string][]*action.SealedEnvelope{
				_honestSender.String(): {ok},
				_poisonSender.String(): {poison},
			}, _poisonSender)
			blk1, err := proposer.Mint(ctx, ap, identityset.PrivateKey(27))
			require.NoError(err)
			requireBlockActions(t, blk1, ok)
			require.Equal(uint64(iotextypes.ReceiptStatus_Success), blk1.Receipts[0].Status)
			require.NoError(validator.Validate(ctx, blk1))
			require.NoError(validator.PutBlock(ctx, blk1))
			require.NoError(proposer.PutBlock(ctx, blk1))

			// block 2 takes the name the dropped registration tried to claim
			again := register(t, 29, "poison", 1)
			ap = newDiffActPool(t, map[string][]*action.SealedEnvelope{
				identityset.Address(29).String(): {again},
			})
			blkCtx := protocol.MustGetBlockCtx(ctx)
			blkCtx.BlockHeight = 2
			blkCtx.BlockTimeStamp = blkCtx.BlockTimeStamp.Add(time.Second)
			ctx2 := protocol.WithBlockCtx(ctx, blkCtx)
			bcCtx := protocol.MustGetBlockchainCtx(ctx)
			bcCtx.Tip = protocol.TipInfo{Height: 1, Hash: blk1.HashBlock(), Timestamp: blk1.Timestamp()}
			ctx2 = protocol.WithFeatureCtx(protocol.WithBlockchainCtx(ctx2, bcCtx))
			blk2, err := proposer.Mint(ctx2, ap, identityset.PrivateKey(27))
			require.NoError(err)
			requireBlockActions(t, blk2, again)
			require.Equal(uint64(iotextypes.ReceiptStatus_Success), blk2.Receipts[0].Status)
			require.NoError(validator.Validate(ctx2, blk2))
			require.NoError(proposer.PutBlock(ctx2, blk2))
		})
	}
}

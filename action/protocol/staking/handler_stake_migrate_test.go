// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math"
	"math/big"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

func TestHandleStakeMigrate(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	g := genesis.TestDefault()
	p, err := NewProtocol(
		HelperCtx{getBlockInterval, depositGas},
		&BuilderConfig{
			Staking:                       g.Staking,
			PersistStakingPatchBlock:      math.MaxUint64,
			SkipContractStakingViewHeight: math.MaxUint64,
			Revise: ReviseConfig{
				VoteWeight: g.Staking.VoteWeightCalConsts,
			},
		},
		nil, nil, nil, nil)
	r.NoError(err)
	cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
	initCfg := func(cfg *genesis.Genesis) {
		cfg.PacificBlockHeight = 1
		cfg.AleutianBlockHeight = 1
		cfg.BeringBlockHeight = 1
		cfg.CookBlockHeight = 1
		cfg.DardanellesBlockHeight = 1
		cfg.DaytonaBlockHeight = 1
		cfg.EasterBlockHeight = 1
		cfg.FbkMigrationBlockHeight = 1
		cfg.FairbankBlockHeight = 1
		cfg.GreenlandBlockHeight = 1
		cfg.HawaiiBlockHeight = 1
		cfg.IcelandBlockHeight = 1
		cfg.JutlandBlockHeight = 1
		cfg.KamchatkaBlockHeight = 1
		cfg.LordHoweBlockHeight = 1
		cfg.MidwayBlockHeight = 1
		cfg.NewfoundlandBlockHeight = 1
		cfg.OkhotskBlockHeight = 1
		cfg.PalauBlockHeight = 1
		cfg.QuebecBlockHeight = 1
		cfg.RedseaBlockHeight = 1
		cfg.SumatraBlockHeight = 1
		cfg.TsunamiBlockHeight = 1
		cfg.UpernavikBlockHeight = 2 // enable CandidateIdentifiedByOwner feature
	}
	initCfg(&cfg)

	ctx := genesis.WithGenesisContext(context.Background(), cfg)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{}))
	view, err := p.Start(ctx, sm)
	r.NoError(err)
	r.NoError(sm.WriteView(p.Name(), view))
	blkGasLimit := uint64(5000000)
	gasPrice := big.NewInt(10)
	gasLimit := uint64(1000000)
	r.NoError(p.CreateGenesisStates(ctx, sm))
	r.NoError(p.PreCommit(ctx, sm))
	r.NoError(p.Commit(ctx, sm))
	runAction := func(ctx context.Context, p *Protocol, act *action.SealedEnvelope, sm protocol.StateManager) (*action.Receipt, error) {
		h, err := act.Hash()
		r.NoError(err)
		instriGas, err := act.IntrinsicGas()
		r.NoError(err)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       act.SenderAddress(),
			ActionHash:   h,
			GasPrice:     gasPrice,
			IntrinsicGas: instriGas,
			Nonce:        act.Nonce(),
		})
		r.NoError(p.Validate(ctx, act.Envelope, sm))
		return p.Handle(ctx, act.Envelope, sm)
	}
	runBlock := func(ctx context.Context, p *Protocol, sm protocol.StateManager, height uint64, t time.Time, acts ...*action.SealedEnvelope) ([]*action.Receipt, []error) {
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
			Tip: protocol.TipInfo{
				Height: height,
			},
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    height,
			BlockTimeStamp: t,
			GasLimit:       blkGasLimit,
		})
		ctx = protocol.WithFeatureCtx(ctx)
		r.NoError(p.CreatePreStates(ctx, sm))
		receipts := make([]*action.Receipt, 0)
		errs := make([]error, 0)
		for _, act := range acts {
			receipt, err := runAction(ctx, p, act, sm)
			receipts = append(receipts, receipt)
			errs = append(errs, err)
		}
		r.NoError(p.PreCommit(ctx, sm))
		r.NoError(p.Commit(ctx, sm))
		return receipts, errs
	}
	popNonce := func(n *uint64) uint64 {
		m := *n
		(*n)++
		return m
	}
	nonce := uint64(0)
	candOwnerID := 1
	balance, _ := big.NewInt(0).SetString("100000000000000000000000000", 10)
	registerAmount, _ := big.NewInt(0).SetString("1200000000000000000000000", 10)
	stakerID := 2
	stakerNonce := uint64(0)
	stakeAmount, _ := big.NewInt(0).SetString("1000000000000000000000", 10)
	stakeDurationDays := uint32(1)
	initAccountBalance(sm, identityset.Address(candOwnerID), balance)
	initAccountBalance(sm, identityset.Address(stakerID), balance)
	receipts, _ := runBlock(ctx, p, sm, 1, timeBlock,
		assertions.MustNoErrorV(action.SignedCandidateRegister(popNonce(&nonce), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(candOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID))),
		assertions.MustNoErrorV(action.SignedCreateStake(popNonce(&stakerNonce), "cand1", stakeAmount.String(), stakeDurationDays, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
	)
	r.Len(receipts, 2)
	r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
	r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[1].Status)
	excPrtl := execution.NewProtocol(
		func(u uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
		func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
			return nil, nil
		},
		func(uint64) (time.Time, error) { return time.Now(), nil },
		nil,
	)
	reg := protocol.NewRegistry()
	r.NoError(excPrtl.Register(reg))
	ctx = protocol.WithRegistry(ctx, reg)

	t.Run("non-owner is not permitted", func(t *testing.T) {
		receipts, _ := runBlock(ctx, p, sm, 2, timeBlock,
			assertions.MustNoErrorV(action.SignedMigrateStake(popNonce(&nonce), 1, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID))),
		)
		r.Len(receipts, 1)
		r.Equal(uint64(iotextypes.ReceiptStatus_ErrUnauthorizedOperator), receipts[0].Status)
	})
	t.Run("selfstaked bucket is not permitted", func(t *testing.T) {
		receipts, _ := runBlock(ctx, p, sm, 3, timeBlock,
			assertions.MustNoErrorV(action.SignedMigrateStake(popNonce(&nonce), 0, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID))),
		)
		r.Len(receipts, 1)
		r.Equal(uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), receipts[0].Status)
	})
	t.Run("endorse bucket is not permitted", func(t *testing.T) {
		receipts, _ := runBlock(ctx, p, sm, 3, timeBlock,
			assertions.MustNoErrorV(action.SignedCreateStake(popNonce(&stakerNonce), "cand1", registerAmount.String(), stakeDurationDays, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
			assertions.MustNoErrorV(action.SignedCandidateEndorsementLegacy(popNonce(&stakerNonce), 2, true, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
			assertions.MustNoErrorV(action.SignedMigrateStake(popNonce(&stakerNonce), 2, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
		)
		r.Len(receipts, 3)
		r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
		r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[1].Status)
		r.Equal(uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), receipts[2].Status)
	})
	t.Run("invalid bucket", func(t *testing.T) {
		receipts, _ := runBlock(ctx, p, sm, 4, timeBlock,
			assertions.MustNoErrorV(action.SignedMigrateStake(popNonce(&nonce), 100, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID))),
		)
		r.Len(receipts, 1)
		r.Equal(uint64(iotextypes.ReceiptStatus_ErrInvalidBucketIndex), receipts[0].Status)
	})
	t.Run("unstaked bucket is not permitted ", func(t *testing.T) {
		receipts, _ := runBlock(ctx, p, sm, 5, timeBlock,
			assertions.MustNoErrorV(action.SignedCreateStake(popNonce(&stakerNonce), "cand1", registerAmount.String(), stakeDurationDays, false, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
		)
		r.Len(receipts, 1)
		r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
		receipts, _ = runBlock(ctx, p, sm, 6, timeBlock.Add(time.Duration(stakeDurationDays*24+1)*time.Hour),
			assertions.MustNoErrorV(action.SignedReclaimStake(false, popNonce(&stakerNonce), 3, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
			assertions.MustNoErrorV(action.SignedMigrateStake(popNonce(&stakerNonce), 3, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
		)
		r.Len(receipts, 2)
		r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
		r.Equal(uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), receipts[1].Status)
	})
	t.Run("non-auto-staked is not permitted", func(t *testing.T) {
		receipts, _ = runBlock(ctx, p, sm, 7, timeBlock,
			assertions.MustNoErrorV(action.SignedCreateStake(popNonce(&stakerNonce), "cand1", registerAmount.String(), stakeDurationDays, false, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
			assertions.MustNoErrorV(action.SignedMigrateStake(popNonce(&stakerNonce), 4, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
		)
		r.Len(receipts, 2)
		r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
		r.Equal(uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), receipts[1].Status)
	})
	t.Run("failure from contract call", func(t *testing.T) {
		pa := NewPatches()
		defer pa.Reset()
		sm.EXPECT().Revert(gomock.Any()).Return(nil).Times(1)
		receipt := &action.Receipt{
			Status:      uint64(iotextypes.ReceiptStatus_Failure),
			GasConsumed: 1000000,
		}
		actLog := &action.Log{
			Address: address.ZeroAddress,
			Topics: action.Topics{
				hash.BytesToHash256([]byte("withdraw")),
			},
		}
		txLog := &action.TransactionLog{
			Type:      iotextypes.TransactionLogType_GAS_FEE,
			Sender:    "",
			Recipient: "",
			Amount:    new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasConsumed), gasPrice),
		}
		receipt.AddLogs(actLog)
		receipt.AddTransactionLogs(txLog)
		pa.ApplyMethodReturn(excPrtl, "Handle", receipt, nil)
		act := assertions.MustNoErrorV(action.SignedMigrateStake(popNonce(&stakerNonce), 1, gasLimit, gasPrice, identityset.PrivateKey(stakerID)))
		receipts, errs := runBlock(ctx, p, sm, 8, timeBlock, act)
		r.Len(receipts, 1)
		r.NoError(errs[0])
		h, err := act.Hash()
		r.NoError(err)
		expectReceipt := &action.Receipt{
			Status:          receipt.Status,
			ActionHash:      h,
			BlockHeight:     8,
			GasConsumed:     receipt.GasConsumed + action.MigrateStakeBaseIntrinsicGas,
			ContractAddress: address.StakingProtocolAddr,
			TxIndex:         uint32(0),
		}
		r.Equal(expectReceipt, receipts[0])
	})
	t.Run("error from contract call", func(t *testing.T) {
		pa := NewPatches()
		defer pa.Reset()
		pa.ApplyMethodFunc(excPrtl, "Handle", func(ctx context.Context, act action.Envelope, sm protocol.StateManager) (*action.Receipt, error) {
			return nil, errors.New("execution failed error")
		})
		sm.EXPECT().Revert(gomock.Any()).Return(nil).Times(1)
		receipts, errs := runBlock(ctx, p, sm, 9, timeBlock,
			assertions.MustNoErrorV(action.SignedCreateStake(popNonce(&stakerNonce), "cand1", stakeAmount.String(), stakeDurationDays, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
			assertions.MustNoErrorV(action.SignedMigrateStake(stakerNonce, 5, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
		)
		r.Len(receipts, 2)
		r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
		r.ErrorContains(errs[1], "execution failed error")
	})
	t.Run("success", func(t *testing.T) {
		pa := NewPatches()
		defer pa.Reset()
		sm.EXPECT().Revert(gomock.Any()).Return(nil).AnyTimes()
		bktIdx := uint64(6)
		receipts, errs := runBlock(ctx, p, sm, 10, timeBlock,
			assertions.MustNoErrorV(action.SignedCreateStake(popNonce(&stakerNonce), "cand1", stakeAmount.String(), stakeDurationDays, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
		)
		r.Len(receipts, 1)
		r.NoError(errs[0])
		r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
		csm, err := NewCandidateStateManagerWithContext(context.Background(), sm)
		r.NoError(err)
		preVotes := csm.GetByOwner(identityset.Address(candOwnerID)).Votes
		bkt, err := csm.NativeBucket(bktIdx)
		r.NoError(err)
		receipt := &action.Receipt{
			Status:      uint64(iotextypes.ReceiptStatus_Success),
			BlockHeight: 10,
			GasConsumed: 1000000,
		}
		contractAddress := address.ZeroAddress
		receipt.AddLogs(&action.Log{
			Address: contractAddress,
			Topics: action.Topics{
				hash.BytesToHash256([]byte("withdraw")),
			},
		})
		receipt.AddTransactionLogs(&action.TransactionLog{
			Type:   iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER,
			Amount: big.NewInt(100),
		})
		pa.ApplyMethodReturn(excPrtl, "Handle", receipt, nil)
		act := assertions.MustNoErrorV(action.SignedMigrateStake(popNonce(&stakerNonce), bktIdx, gasLimit, gasPrice, identityset.PrivateKey(stakerID)))
		receipts, _ = runBlock(ctx, p, sm, 11, timeBlock,
			act,
		)
		r.Len(receipts, 1)
		r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
		// gas = instrinsic  + contract call
		instriGas, _ := act.IntrinsicGas()
		r.Equal(instriGas+receipt.GasConsumed, receipts[0].GasConsumed)
		// withdraw log + stake log
		r.Len(receipts[0].Logs(), 2)
		r.Equal(&action.Log{
			Address: address.StakingProtocolAddr,
			Topics: action.Topics{
				hash.BytesToHash256([]byte(HandleWithdrawStake)),
				hash.BytesToHash256(byteutil.Uint64ToBytesBigEndian(bktIdx)),
				hash.BytesToHash256(identityset.Address(candOwnerID).Bytes()),
			},
			Data:        nil,
			BlockHeight: 11,
			ActionHash:  assertions.MustNoErrorV(act.Hash()),
		}, receipts[0].Logs()[0])
		r.Equal(receipt.Logs()[0], receipts[0].Logs()[1])
		r.Len(receipts[0].TransactionLogs(), 2)
		r.Equal(&action.TransactionLog{
			Type:      iotextypes.TransactionLogType_WITHDRAW_BUCKET,
			Amount:    stakeAmount,
			Sender:    address.StakingBucketPoolAddr,
			Recipient: identityset.Address(stakerID).String(),
		}, receipts[0].TransactionLogs()[0])
		r.Equal(receipt.TransactionLogs()[0], receipts[0].TransactionLogs()[1])
		// native bucket burned
		csm, err = NewCandidateStateManagerWithContext(context.Background(), sm)
		r.NoError(err)
		_, err = csm.NativeBucket(bktIdx)
		r.ErrorIs(err, state.ErrStateNotExist)
		// votes reduced for staking indexer not enabled
		cand := csm.GetByOwner(identityset.Address(candOwnerID))
		r.NotNil(cand)
		r.Equal(preVotes, cand.Votes.Add(cand.Votes, p.calculateVoteWeight(bkt, false)))
	})

}

// TestHandleStakeMigrateGasAccounting pins the two gas figures a stake
// migration produces: the gas limit handed to the inner contract call, and the
// total gas reported on the migration receipt.
//
// Before the fix height the inner call is budgeted with the whole declared gas
// limit of the migration and the migration's own intrinsic gas is reported on
// top of it, so the receipt can report more gas than the action ever declared.
// From the fix height on the intrinsic gas is taken out of the inner call's
// budget, so the reported total is bounded by the declared gas limit.
func TestHandleStakeMigrateGasAccounting(t *testing.T) {
	var (
		gasPrice   = big.NewInt(10)
		gasLimit   = uint64(1000000)
		migrateHgt = uint64(3)
	)
	for _, tt := range []struct {
		name string
		// height from which the corrected accounting takes effect
		gateHeight uint64
		// gas limit the inner contract call is expected to run under
		wantExecGasLimit uint64
		// gas the migration receipt is expected to report
		wantReceiptGas uint64
	}{
		{
			name:             "before fix height",
			gateHeight:       math.MaxUint64,
			wantExecGasLimit: gasLimit,
			wantReceiptGas:   gasLimit + action.MigrateStakeBaseIntrinsicGas,
		},
		{
			name:             "from fix height",
			gateHeight:       migrateHgt,
			wantExecGasLimit: gasLimit - action.MigrateStakeBaseIntrinsicGas,
			wantReceiptGas:   gasLimit,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctrl := gomock.NewController(t)
			sm := testdb.NewMockStateManager(ctrl)
			g := genesis.TestDefault()
			p, err := NewProtocol(
				HelperCtx{getBlockInterval, depositGas},
				&BuilderConfig{
					Staking:                       g.Staking,
					PersistStakingPatchBlock:      math.MaxUint64,
					SkipContractStakingViewHeight: math.MaxUint64,
					Revise: ReviseConfig{
						VoteWeight: g.Staking.VoteWeightCalConsts,
					},
				},
				nil, nil, nil, nil)
			r.NoError(err)
			cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
			cfg.PacificBlockHeight = 1
			cfg.AleutianBlockHeight = 1
			cfg.BeringBlockHeight = 1
			cfg.CookBlockHeight = 1
			cfg.DardanellesBlockHeight = 1
			cfg.DaytonaBlockHeight = 1
			cfg.EasterBlockHeight = 1
			cfg.FbkMigrationBlockHeight = 1
			cfg.FairbankBlockHeight = 1
			cfg.GreenlandBlockHeight = 1
			cfg.HawaiiBlockHeight = 1
			cfg.IcelandBlockHeight = 1
			cfg.JutlandBlockHeight = 1
			cfg.KamchatkaBlockHeight = 1
			cfg.LordHoweBlockHeight = 1
			cfg.MidwayBlockHeight = 1
			cfg.NewfoundlandBlockHeight = 1
			cfg.OkhotskBlockHeight = 1
			cfg.PalauBlockHeight = 1
			cfg.QuebecBlockHeight = 1
			cfg.RedseaBlockHeight = 1
			cfg.SumatraBlockHeight = 1
			cfg.TsunamiBlockHeight = 1
			cfg.UpernavikBlockHeight = 1
			// The corrections ride Zanzibar Gamma; a chain that has activated
			// none of the family carries the heights equal, so set all three
			// rather than leaving a partial-family genesis in a test.
			cfg.ZanzibarBlockHeight = tt.gateHeight
			cfg.ZanzibarBetaBlockHeight = tt.gateHeight
			cfg.ZanzibarGammaBlockHeight = tt.gateHeight

			ctx := genesis.WithGenesisContext(context.Background(), cfg)
			ctx = protocol.WithFeatureWithHeightCtx(ctx)
			ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{}))
			view, err := p.Start(ctx, sm)
			r.NoError(err)
			r.NoError(sm.WriteView(p.Name(), view))
			r.NoError(p.CreateGenesisStates(ctx, sm))
			r.NoError(p.PreCommit(ctx, sm))
			r.NoError(p.Commit(ctx, sm))

			excPrtl := execution.NewProtocol(
				func(u uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
				func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
					return nil, nil
				},
				func(uint64) (time.Time, error) { return time.Now(), nil },
				nil,
			)
			reg := protocol.NewRegistry()
			r.NoError(excPrtl.Register(reg))
			ctx = protocol.WithRegistry(ctx, reg)

			runBlock := func(height uint64, acts ...*action.SealedEnvelope) []*action.Receipt {
				blkCtx := protocol.WithBlockCtx(ctx, protocol.BlockCtx{
					BlockHeight:    height,
					BlockTimeStamp: timeBlock,
					GasLimit:       gasLimit,
				})
				blkCtx = protocol.WithBlockchainCtx(blkCtx, protocol.BlockchainCtx{
					Tip: protocol.TipInfo{Height: height},
				})
				blkCtx = protocol.WithFeatureCtx(blkCtx)
				r.NoError(p.CreatePreStates(blkCtx, sm))
				receipts := make([]*action.Receipt, 0, len(acts))
				for _, act := range acts {
					h, err := act.Hash()
					r.NoError(err)
					insGas, err := act.IntrinsicGas()
					r.NoError(err)
					actCtx := protocol.WithActionCtx(blkCtx, protocol.ActionCtx{
						Caller:       act.SenderAddress(),
						ActionHash:   h,
						GasPrice:     gasPrice,
						IntrinsicGas: insGas,
						Nonce:        act.Nonce(),
					})
					r.NoError(p.Validate(actCtx, act.Envelope, sm))
					receipt, err := p.Handle(actCtx, act.Envelope, sm)
					r.NoError(err)
					receipts = append(receipts, receipt)
				}
				r.NoError(p.PreCommit(blkCtx, sm))
				r.NoError(p.Commit(blkCtx, sm))
				return receipts
			}

			var (
				candOwnerID       = 1
				stakerID          = 2
				balance, _        = big.NewInt(0).SetString("100000000000000000000000000", 10)
				registerAmount, _ = big.NewInt(0).SetString("1200000000000000000000000", 10)
				stakeAmount, _    = big.NewInt(0).SetString("1000000000000000000000", 10)
			)
			r.NoError(initAccountBalance(sm, identityset.Address(candOwnerID), balance))
			r.NoError(initAccountBalance(sm, identityset.Address(stakerID), balance))
			receipts := runBlock(1,
				assertions.MustNoErrorV(action.SignedCandidateRegister(0, "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(candOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID))),
			)
			r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
			bktIdx := uint64(1)
			receipts = runBlock(2,
				assertions.MustNoErrorV(action.SignedCreateStake(0, "cand1", stakeAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID))),
			)
			r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)

			// the inner contract call burns every unit of gas it is given
			var execGasLimit uint64
			pa := NewPatches()
			defer pa.Reset()
			sm.EXPECT().Revert(gomock.Any()).Return(nil).AnyTimes()
			pa.ApplyMethodFunc(excPrtl, "Handle", func(_ context.Context, elp action.Envelope, _ protocol.StateManager) (*action.Receipt, error) {
				execGasLimit = elp.Gas()
				return &action.Receipt{
					Status:      uint64(iotextypes.ReceiptStatus_Success),
					BlockHeight: migrateHgt,
					GasConsumed: elp.Gas(),
				}, nil
			})
			act := assertions.MustNoErrorV(action.SignedMigrateStake(1, bktIdx, gasLimit, gasPrice, identityset.PrivateKey(stakerID)))
			receipts = runBlock(migrateHgt, act)
			r.Len(receipts, 1)
			r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
			r.Equal(tt.wantExecGasLimit, execGasLimit)
			r.Equal(tt.wantReceiptGas, receipts[0].GasConsumed)
			// the declared gas limit is the ceiling the block budget was
			// reserved against, so the receipt must never report beyond it
			if tt.gateHeight <= migrateHgt {
				r.LessOrEqual(receipts[0].GasConsumed, act.Gas())
			}
		})
	}
}

func initAccountBalance(sm protocol.StateManager, addr address.Address, initBalance *big.Int) error {
	acc, err := accountutil.LoadAccount(sm, addr)
	if err != nil {
		return err
	}
	acc.Balance = initBalance
	return accountutil.StoreAccount(sm, addr, acc)
}

// TestHandleStakeMigrateUnderfundedContractCall covers a migration whose gas
// limit clears its own intrinsic gas but leaves the staking contract call short
// of what that call needs just to start. The execution protocol reports those
// refusals as plain errors rather than failed receipts, and a plain error out
// of a handler abandons the block being built or validated. From the fix height
// the migration settles a failed receipt instead, so an underfunded action
// costs its sender a failed action and nothing more.
//
// There are two such refusals and they sit at different gas limits, which is
// why both are exercised. The contract call packs 68 bytes, so it needs 16,800
// intrinsic gas (10,000 base + 68x100) and, once Prague is active, an EIP-7623
// data floor of 27,000 (10,000 + 68x250) checked after it. Past the fix the
// call is handed the declared limit less the migration 10,000, so the two
// refusals answer for declared limits below 26,800 and below 37,000.
func TestHandleStakeMigrateUnderfundedContractCall(t *testing.T) {
	var (
		gasPrice   = big.NewInt(10)
		blockGas   = uint64(1000000)
		migrateHgt = uint64(3)
	)
	for _, tt := range []struct {
		name       string
		gateHeight uint64
		// declared gas limit of the migration
		actGasLimit uint64
		// whether Handle reports a plain error instead of a receipt
		wantErr    bool
		wantErrIs  error
		wantStatus uint64
	}{
		{
			name:        "intrinsic shortfall before fix height",
			gateHeight:  math.MaxUint64,
			actGasLimit: 12000,
			wantErr:     true,
			wantErrIs:   action.ErrInsufficientFunds,
		},
		{
			name:        "intrinsic shortfall from fix height",
			gateHeight:  migrateHgt,
			actGasLimit: 12000,
			wantStatus:  uint64(iotextypes.ReceiptStatus_ErrOutOfGas),
		},
		{
			// 20,000 clears the intrinsic cost and still falls under the data
			// floor, so this band costs a draft on the current code too -- the
			// fix does not open it, it only moves it
			name:        "data floor shortfall before fix height",
			gateHeight:  math.MaxUint64,
			actGasLimit: 20000,
			wantErr:     true,
			wantErrIs:   action.ErrFloorDataGas,
		},
		{
			// past the fix the call is handed 30,000 less the migration
			// 10,000, which lands in the same band one 10,000 higher
			name:        "data floor shortfall from fix height",
			gateHeight:  migrateHgt,
			actGasLimit: 30000,
			wantStatus:  uint64(iotextypes.ReceiptStatus_ErrOutOfGas),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctrl := gomock.NewController(t)
			sm := testdb.NewMockStateManager(ctrl)
			g := genesis.TestDefault()
			p, err := NewProtocol(
				HelperCtx{getBlockInterval, depositGas},
				&BuilderConfig{
					Staking:                       g.Staking,
					PersistStakingPatchBlock:      math.MaxUint64,
					SkipContractStakingViewHeight: math.MaxUint64,
					Revise: ReviseConfig{
						VoteWeight: g.Staking.VoteWeightCalConsts,
					},
				},
				nil, nil, nil, nil)
			r.NoError(err)
			cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
			cfg.PacificBlockHeight = 1
			cfg.AleutianBlockHeight = 1
			cfg.BeringBlockHeight = 1
			cfg.CookBlockHeight = 1
			cfg.DardanellesBlockHeight = 1
			cfg.DaytonaBlockHeight = 1
			cfg.EasterBlockHeight = 1
			cfg.FbkMigrationBlockHeight = 1
			cfg.FairbankBlockHeight = 1
			cfg.GreenlandBlockHeight = 1
			cfg.HawaiiBlockHeight = 1
			cfg.IcelandBlockHeight = 1
			cfg.JutlandBlockHeight = 1
			cfg.KamchatkaBlockHeight = 1
			cfg.LordHoweBlockHeight = 1
			cfg.MidwayBlockHeight = 1
			cfg.NewfoundlandBlockHeight = 1
			cfg.OkhotskBlockHeight = 1
			cfg.PalauBlockHeight = 1
			cfg.QuebecBlockHeight = 1
			cfg.RedseaBlockHeight = 1
			cfg.SumatraBlockHeight = 1
			cfg.TsunamiBlockHeight = 1
			cfg.UpernavikBlockHeight = 1
			// The corrections ride Zanzibar Gamma; a chain that has activated
			// none of the family carries the heights equal, so set all three
			// rather than leaving a partial-family genesis in a test.
			cfg.ZanzibarBlockHeight = tt.gateHeight
			cfg.ZanzibarBetaBlockHeight = tt.gateHeight
			cfg.ZanzibarGammaBlockHeight = tt.gateHeight

			ctx := genesis.WithGenesisContext(context.Background(), cfg)
			ctx = protocol.WithFeatureWithHeightCtx(ctx)
			ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{}))
			view, err := p.Start(ctx, sm)
			r.NoError(err)
			r.NoError(sm.WriteView(p.Name(), view))
			r.NoError(p.CreateGenesisStates(ctx, sm))
			r.NoError(p.PreCommit(ctx, sm))
			r.NoError(p.Commit(ctx, sm))

			excPrtl := execution.NewProtocol(
				func(u uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
				func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
					return nil, nil
				},
				func(uint64) (time.Time, error) { return time.Now(), nil },
				nil,
			)
			reg := protocol.NewRegistry()
			r.NoError(excPrtl.Register(reg))
			ctx = protocol.WithRegistry(ctx, reg)

			runBlock := func(height uint64, acts ...*action.SealedEnvelope) ([]*action.Receipt, error) {
				blkCtx := protocol.WithBlockCtx(ctx, protocol.BlockCtx{
					BlockHeight:    height,
					BlockTimeStamp: timeBlock,
					GasLimit:       blockGas,
				})
				blkCtx = protocol.WithBlockchainCtx(blkCtx, protocol.BlockchainCtx{
					Tip: protocol.TipInfo{Height: height},
				})
				blkCtx = protocol.WithFeatureCtx(blkCtx)
				r.NoError(p.CreatePreStates(blkCtx, sm))
				receipts := make([]*action.Receipt, 0, len(acts))
				for _, act := range acts {
					h, err := act.Hash()
					r.NoError(err)
					insGas, err := act.IntrinsicGas()
					r.NoError(err)
					actCtx := protocol.WithActionCtx(blkCtx, protocol.ActionCtx{
						Caller:       act.SenderAddress(),
						ActionHash:   h,
						GasPrice:     gasPrice,
						IntrinsicGas: insGas,
						Nonce:        act.Nonce(),
					})
					r.NoError(p.Validate(actCtx, act.Envelope, sm))
					receipt, err := p.Handle(actCtx, act.Envelope, sm)
					if err != nil {
						return nil, err
					}
					receipts = append(receipts, receipt)
				}
				r.NoError(p.PreCommit(blkCtx, sm))
				r.NoError(p.Commit(blkCtx, sm))
				return receipts, nil
			}

			var (
				candOwnerID       = 1
				stakerID          = 2
				balance, _        = big.NewInt(0).SetString("100000000000000000000000000", 10)
				registerAmount, _ = big.NewInt(0).SetString("1200000000000000000000000", 10)
				stakeAmount, _    = big.NewInt(0).SetString("1000000000000000000000", 10)
			)
			r.NoError(initAccountBalance(sm, identityset.Address(candOwnerID), balance))
			r.NoError(initAccountBalance(sm, identityset.Address(stakerID), balance))
			receipts, err := runBlock(1,
				assertions.MustNoErrorV(action.SignedCandidateRegister(0, "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(candOwnerID).String(), registerAmount.String(), 1, true, nil, blockGas, gasPrice, identityset.PrivateKey(candOwnerID))),
			)
			r.NoError(err)
			r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)
			bktIdx := uint64(1)
			receipts, err = runBlock(2,
				assertions.MustNoErrorV(action.SignedCreateStake(0, "cand1", stakeAmount.String(), 1, true, nil, blockGas, gasPrice, identityset.PrivateKey(stakerID))),
			)
			r.NoError(err)
			r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipts[0].Status)

			pa := NewPatches()
			defer pa.Reset()
			sm.EXPECT().Revert(gomock.Any()).Return(nil).AnyTimes()
			// mirrors the execution protocol, in the order it applies them: a
			// call whose gas limit is below its own intrinsic cost, or below
			// the EIP-7623 data floor checked after it, is reported as an
			// error rather than a receipt
			pa.ApplyMethodFunc(excPrtl, "Handle", func(_ context.Context, elp action.Envelope, _ protocol.StateManager) (*action.Receipt, error) {
				innerInsGas, err := elp.IntrinsicGas()
				if err != nil {
					return nil, err
				}
				if elp.Gas() < innerInsGas {
					return nil, errors.Wrap(action.ErrInsufficientFunds, "failed to execute contract")
				}
				exc, ok := elp.Action().(*action.Execution)
				r.True(ok, "the migration must hand the execution protocol an execution")
				floor, err := action.FloorDataGas(exc.Data())
				if err != nil {
					return nil, err
				}
				if elp.Gas() < floor {
					return nil, errors.Wrapf(action.ErrFloorDataGas, "have %d, want %d", elp.Gas(), floor)
				}
				return &action.Receipt{
					Status:      uint64(iotextypes.ReceiptStatus_Success),
					BlockHeight: migrateHgt,
					GasConsumed: elp.Gas(),
				}, nil
			})
			act := assertions.MustNoErrorV(action.SignedMigrateStake(1, bktIdx, tt.actGasLimit, gasPrice, identityset.PrivateKey(stakerID)))
			receipts, err = runBlock(migrateHgt, act)
			if tt.wantErr {
				r.Error(err)
				r.ErrorIs(err, tt.wantErrIs)
				return
			}
			r.NoError(err)
			r.Len(receipts, 1)
			r.Equal(tt.wantStatus, receipts[0].Status)
			// the failed migration still charges its sender, and never more
			// than the gas limit it declared
			r.LessOrEqual(receipts[0].GasConsumed, act.Gas())
		})
	}
}

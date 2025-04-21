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
	"github.com/golang/mock/gomock"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

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
			Staking:                  g.Staking,
			PersistStakingPatchBlock: math.MaxUint64,
			Revise: ReviseConfig{
				VoteWeight: g.Staking.VoteWeightCalConsts,
			},
		},
		nil, nil, nil)
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
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)
		preVotes := csm.GetByOwner(identityset.Address(candOwnerID)).Votes
		bkt, err := csm.getBucket(bktIdx)
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
		csm, err = NewCandidateStateManager(sm)
		r.NoError(err)
		_, err = csm.getBucket(bktIdx)
		r.ErrorIs(err, state.ErrStateNotExist)
		// votes reduced for staking indexer not enabled
		cand := csm.GetByOwner(identityset.Address(candOwnerID))
		r.NotNil(cand)
		r.Equal(preVotes, cand.Votes.Add(cand.Votes, p.calculateVoteWeight(bkt, false)))
	})

}

func initAccountBalance(sm protocol.StateManager, addr address.Address, initBalance *big.Int) error {
	acc, err := accountutil.LoadAccount(sm, addr)
	if err != nil {
		return err
	}
	acc.Balance = initBalance
	return accountutil.StoreAccount(sm, addr, acc)
}

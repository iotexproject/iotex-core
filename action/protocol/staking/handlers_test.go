// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

const (
	_reclaim = "This is to certify I am transferring the ownership of said bucket to said recipient on IoTeX blockchain"
)

var (
	testGasPrice = big.NewInt(unit.Qev)
	builder      = action.EnvelopeBuilder{}
)

// Delete should only be used by test
func (m *CandidateCenter) deleteForTestOnly(owner address.Address) {
	if owner == nil {
		return
	}

	if _, hit := m.base.getByOwner(owner.String()); hit {
		m.base.deleteByOwner(owner)
		if !m.change.containsOwner(owner) {
			m.size--
			return
		}
	}

	if cand := m.change.getByOwner(owner); cand != nil {
		if owner != nil {
			candidates := []*Candidate{}
			for _, c := range m.change.candidates {
				if c.Owner.String() != owner.String() {
					candidates = append(candidates, c)
				}
			}
			m.change.candidates = candidates
			delete(m.change.dirty, cand.GetIdentifier().String())
			return
		}
		m.size--
	}
}

func TestProtocol_HandleCreateStake(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	csr := newCandidateStateReader(sm)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	require.NoError(err)
	g := genesis.TestDefault()

	// create protocol
	p, err := NewProtocol(HelperCtx{
		DepositGas:    depositGas,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                  genesis.TestDefault().Staking,
		PersistStakingPatchBlock: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight: genesis.TestDefault().Staking.VoteWeightCalConsts,
		},
	}, nil, nil, nil)
	require.NoError(err)

	// set up candidate
	candidate := testCandidates[0].d.Clone()
	require.NoError(csm.putCandidate(candidate))
	candidateName := candidate.Name
	candidateAddr := candidate.Owner
	g.VanuatuBlockHeight = 1
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	cc, ok := v.(*ViewData)
	require.True(ok)
	require.NoError(sm.WriteView(_protocolID, cc))

	stakerAddr := identityset.Address(1)
	tests := []struct {
		// action fields
		initBalance int64
		candName    string
		amount      string
		duration    uint32
		autoStake   bool
		gasLimit    uint64
		nonce       uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		blkGasLimit  uint64
		// expected result
		err    error
		status iotextypes.ReceiptStatus
	}{
		{
			10,
			candidateName,
			"100000000000000000000",
			1,
			false,
			10000,
			1,
			1,
			time.Now(),
			10000,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		{
			100,
			"notExist",
			"100000000000000000000",
			1,
			false,
			10000,
			2,
			1,
			time.Now(),
			10000,
			action.ErrInvalidCanName,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		{
			101,
			candidateName,
			"10000000000000000000",
			1,
			false,
			10000,
			2,
			1,
			time.Now(),
			10000,
			action.ErrInvalidAmount,
			iotextypes.ReceiptStatus_Failure,
		},
		{
			101,
			candidateName,
			"100000000000000000000",
			1051,
			false,
			10000,
			2,
			1,
			time.Now(),
			10000,
			ErrDurationTooHigh,
			iotextypes.ReceiptStatus_Failure,
		},
		{
			101,
			candidateName,
			"100000000000000000000",
			1,
			false,
			10000,
			2,
			1,
			time.Now(),
			10000,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		require.NoError(setupAccount(sm, stakerAddr, test.initBalance))
		ctx := protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       stakerAddr,
			GasPrice:     testGasPrice,
			IntrinsicGas: test.gasLimit,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    test.blkHeight,
			BlockTimeStamp: test.blkTimestamp,
			GasLimit:       test.blkGasLimit,
		})
		ctx = protocol.WithFeatureCtx(protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: test.blkHeight - 1,
		}}))
		act, err := action.NewCreateStake(test.candName, test.amount, test.duration, test.autoStake, nil)
		require.NoError(err)
		elp := builder.SetNonce(test.nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		err = p.Validate(ctx, elp, sm)
		if test.err != nil {
			require.EqualError(test.err, errors.Cause(err).Error())
			continue
		}
		r, err := p.Handle(ctx, elp, sm)
		require.NoError(err)
		require.Equal(uint64(test.status), r.Status)

		if test.status == iotextypes.ReceiptStatus_Success {
			// check the special create bucket log
			tLogs := r.TransactionLogs()
			require.Equal(1, len(tLogs))
			cLog := tLogs[0]
			require.Equal(stakerAddr.String(), cLog.Sender)
			require.Equal(address.StakingBucketPoolAddr, cLog.Recipient)
			require.Equal(test.amount, cLog.Amount.String())

			// test bucket index and bucket
			bucketIndices, _, err := csr.candBucketIndices(candidateAddr)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.voterBucketIndices(stakerAddr)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := csr.getBucket(indices[0])
			require.NoError(err)
			require.Equal(candidateAddr, bucket.Candidate)
			require.Equal(stakerAddr, bucket.Owner)
			require.Equal(test.amount, bucket.StakedAmount.String())

			// test candidate
			candidate, _, err := csr.getCandidate(candidateAddr)
			require.NoError(err)
			require.LessOrEqual(test.amount, candidate.Votes.String())
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			candidate = csm.GetByOwner(candidateAddr)
			require.NotNil(candidate)
			require.LessOrEqual(test.amount, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, stakerAddr)
			require.NoError(err)
			actCost, err := elp.Cost()
			require.NoError(err)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), big.NewInt(0).Add(caller.Balance, actCost))
			require.Equal(test.nonce+1, caller.PendingNonce())
		}
	}
}

func TestProtocol_HandleCandidateRegister(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm, p, _, _ := initAll(t, ctrl)
	csr := newCandidateStateReader(sm)
	tests := []struct {
		initBalance     int64
		caller          address.Address
		nonce           uint64
		name            string
		operatorAddrStr string
		rewardAddrStr   string
		ownerAddrStr    string
		amountStr       string
		votesStr        string
		duration        uint32
		autoStake       bool
		payload         []byte
		gasLimit        uint64
		blkGasLimit     uint64
		gasPrice        *big.Int
		newProtocol     bool
		err             error
		status          iotextypes.ReceiptStatus
	}{
		// fetchCaller,ErrNotEnoughBalance
		{
			1200000,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"1200000000000000000000000",
			"",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// owner address is nil
		{
			1201000,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			"",
			"1200000000000000000000000",
			"1806204150552640363969204",
			uint32(10000),
			false,
			nil,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1),
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		// Upsert,check collision
		{
			1201000,
			identityset.Address(27),
			2,
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"1200000000000000000000000",
			"",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
		// invalid amount
		{
			1201000,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"120000000000000000000000",
			"1806204150552640363969204",
			uint32(10000),
			false,
			nil,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1),
			true,
			action.ErrInvalidAmount,
			iotextypes.ReceiptStatus_Failure,
		},
		// invalid candidate name
		{
			1201000,
			identityset.Address(27),
			1,
			"!invalid",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"1200000000000000000000000",
			"1806204150552640363969204",
			uint32(10000),
			false,
			nil,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1),
			true,
			action.ErrInvalidCanName,
			iotextypes.ReceiptStatus_Failure,
		},
		// success for the following test
		{
			1201000,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"1200000000000000000000000",
			"1806204150552640363969204",
			uint32(10000),
			false,
			nil,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1),
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		// act.OwnerAddress() is not nil,existing owner, but selfstake is not 0
		{
			1201000,
			identityset.Address(27),
			2,
			"test2",
			identityset.Address(10).String(),
			identityset.Address(10).String(),
			identityset.Address(30).String(), "1200000000000000000000000",
			"1200000000000000000000000",
			uint32(10000),
			false,
			[]byte("payload"),
			1000000,
			1000000,
			big.NewInt(1),
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateAlreadyExist,
		},
		// act.OwnerAddress() is not nil,existing candidate, collide with existing name,this case cannot happen,b/c if ownerExist,it will return ReceiptStatus_ErrCandidateAlreadyExist
		// act.OwnerAddress() is not nil,existing candidate, collide with existing operator,this case cannot happen,b/c if ownerExist,it will return ReceiptStatus_ErrCandidateAlreadyExist
		// act.OwnerAddress() is not nil,new candidate, collide with existing name
		{
			1201000,
			identityset.Address(27),
			3,
			"test",
			identityset.Address(10).String(),
			identityset.Address(10).String(),
			identityset.Address(29).String(),
			"1200000000000000000000000",
			"1200000000000000000000000",
			uint32(10000),
			false,
			[]byte("payload"),
			1000000,
			1000000,
			testGasPrice,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
		// act.OwnerAddress() is not nil,new candidate, collide with existing operator
		{
			1201000,
			identityset.Address(27),
			4,
			"test2",
			identityset.Address(28).String(),
			identityset.Address(10).String(),
			identityset.Address(29).String(),
			"1200000000000000000000000",
			"1200000000000000000000000",
			uint32(10000),
			false,
			[]byte("payload"),
			1000000,
			1000000,
			testGasPrice,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
		// act.OwnerAddress() is nil,existing owner, but selfstake is not 0
		{
			1201000,
			identityset.Address(30),
			1,
			"test2",
			identityset.Address(28).String(),
			identityset.Address(10).String(),
			"",
			"1200000000000000000000000",
			"1200000000000000000000000",
			uint32(10000),
			false,
			[]byte("payload"),
			1000000,
			1000000,
			testGasPrice,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateAlreadyExist,
		},
		// act.OwnerAddress() is nil,existing candidate, collide with existing name,this case cannot happen,b/c if ownerExist,it will return ReceiptStatus_ErrCandidateAlreadyExist
		// act.OwnerAddress() is nil,existing candidate, collide with existing operator,this case cannot happen,b/c if ownerExist,it will return ReceiptStatus_ErrCandidateAlreadyExist
		// act.OwnerAddress() is nil,new candidate, collide with existing name
		{
			1201000,
			identityset.Address(21),
			1,
			"test",
			identityset.Address(28).String(),
			identityset.Address(10).String(),
			"",
			"1200000000000000000000000",
			"1200000000000000000000000",
			uint32(10000),
			false,
			[]byte("payload"),
			1000000,
			1000000,
			testGasPrice,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
		// act.OwnerAddress() is nil,new candidate, collide with existing operator
		{
			1201000,
			identityset.Address(21),
			2,
			"test2",
			identityset.Address(28).String(),
			identityset.Address(10).String(),
			"",
			"1200000000000000000000000",
			"1200000000000000000000000",
			uint32(10000),
			false,
			[]byte("payload"),
			1000000,
			1000000,
			testGasPrice,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
		// register without self-stake
		{
			1201000,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"0",
			"0",
			uint32(10000),
			false,
			nil,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1),
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		if test.newProtocol {
			sm, p, _, _ = initAll(t, ctrl)
			csr = newCandidateStateReader(sm)
		}
		require.NoError(setupAccount(sm, test.caller, test.initBalance))
		act, err := action.NewCandidateRegister(test.name, test.operatorAddrStr, test.rewardAddrStr, test.ownerAddrStr, test.amountStr, test.duration, test.autoStake, test.payload)
		require.NoError(err)
		IntrinsicGas, _ := act.IntrinsicGas()
		elp := builder.SetNonce(test.nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(test.gasPrice).SetAction(act).Build()
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: IntrinsicGas,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       test.blkGasLimit,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{}})
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		require.Equal(test.err, errors.Cause(p.Validate(ctx, elp, sm)))
		if test.err != nil {
			continue
		}
		r, err := p.Handle(ctx, elp, sm)
		require.NoError(err)
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// check the special create bucket and candidate register log
			tLogs := r.TransactionLogs()
			if test.amountStr == "0" {
				require.Equal(1, len(tLogs))
				cLog := tLogs[0]
				require.Equal(test.caller.String(), cLog.Sender)
				require.Equal(address.RewardingPoolAddr, cLog.Recipient)
				require.Equal(p.config.RegistrationConsts.Fee.String(), cLog.Amount.String())
			} else {
				require.Equal(2, len(tLogs))
				cLog := tLogs[0]
				require.Equal(test.caller.String(), cLog.Sender)
				require.Equal(address.StakingBucketPoolAddr, cLog.Recipient)
				require.Equal(test.amountStr, cLog.Amount.String())

				cLog = tLogs[1]
				require.Equal(test.caller.String(), cLog.Sender)
				require.Equal(address.RewardingPoolAddr, cLog.Recipient)
				require.Equal(p.config.RegistrationConsts.Fee.String(), cLog.Amount.String())
			}

			// test candidate
			candidate, _, err := csr.getCandidate(act.OwnerAddress())
			if act.OwnerAddress() == nil {
				require.Nil(candidate)
				require.Equal(ErrNilParameters, errors.Cause(err))
				candidate, _, err = csr.getCandidate(test.caller)
				require.NoError(err)
				require.Equal(test.caller.String(), candidate.Owner.String())
			} else {
				require.NotNil(candidate)
				require.NoError(err)
				require.Equal(test.ownerAddrStr, candidate.Owner.String())
			}
			require.Equal(test.votesStr, candidate.Votes.String())
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			candidate = csm.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal(test.votesStr, candidate.Votes.String())
			require.Equal(test.name, candidate.Name)
			require.Equal(test.operatorAddrStr, candidate.Operator.String())
			require.Equal(test.rewardAddrStr, candidate.Reward.String())
			require.Equal(test.amountStr, candidate.SelfStake.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			actCost, err := elp.Cost()
			require.NoError(err)
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, p.config.RegistrationConsts.Fee))
			require.Equal(test.nonce+1, caller.PendingNonce())
		}
	}
}

func TestProtocol_HandleCandidateUpdate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	tests := []struct {
		initBalance     int64
		caller          address.Address
		nonce           uint64
		name            string
		operatorAddrStr string
		rewardAddrStr   string
		ownerAddrStr    string
		amountStr       string
		afterUpdate     string
		duration        uint32
		autoStake       bool
		payload         []byte
		gasLimit        uint64
		blkGasLimit     uint64
		gasPrice        *big.Int
		newProtocol     bool
		// candidate update
		updateName     string
		updateOperator string
		updateReward   string
		err            error
		status         iotextypes.ReceiptStatus
	}{
		// fetchCaller ErrNotEnoughBalance
		{
			1200101,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(27).String(),
			"1200000999999999989300000",
			"",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			"update",
			identityset.Address(31).String(),
			identityset.Address(32).String(),
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// only owner can update candidate
		{
			1000,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"1200000000000000000000000",
			"",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			"update",
			identityset.Address(31).String(),
			identityset.Address(32).String(),
			nil,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		// success,update name,operator,reward all empty
		{
			1201000,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			"",
			"1200000000000000000000000",
			"1806204150552640363969204",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			"",
			"",
			"",
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		// upsert,collision
		{
			1201000,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"",
			"1200000000000000000000000",
			"",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			"test2",
			"",
			"",
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
		// invalid candidate name
		{
			1201000,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(27).String(),
			identityset.Address(29).String(),
			identityset.Address(27).String(),
			"1200000000000000000000000",
			"1806204150552640363969204",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			"!invalidname",
			identityset.Address(31).String(),
			identityset.Address(32).String(),
			action.ErrInvalidCanName,
			iotextypes.ReceiptStatus_Failure,
		},
		// success,update name, operator and reward address
		{
			1201000,
			identityset.Address(27),
			1,
			"test",
			identityset.Address(27).String(),
			identityset.Address(29).String(),
			identityset.Address(27).String(),
			"1200000000000000000000000",
			"1806204150552640363969204",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			"update",
			identityset.Address(31).String(),
			identityset.Address(32).String(),
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		// upsert,collide with candidate name
		{
			1201000,
			identityset.Address(27),
			uint64(1),
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(27).String(),
			"1200000000000000000000000",
			"",
			uint32(10000),
			true,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			false,
			"test1",
			"",
			"",
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
		// upsert,collide with operator address
		{
			1201000,
			identityset.Address(27),
			uint64(1),
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(27).String(),
			"1200000000000000000000000",
			"",
			uint32(10000),
			true,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			false,
			"test",
			identityset.Address(7).String(),
			"",
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
	}

	for _, test := range tests {
		sm, p, _, _ := initAll(t, ctrl)
		csr := newCandidateStateReader(sm)
		require.NoError(setupAccount(sm, test.caller, test.initBalance))
		act, err := action.NewCandidateRegister(test.name, test.operatorAddrStr, test.rewardAddrStr, test.ownerAddrStr, test.amountStr, test.duration, test.autoStake, test.payload)
		require.NoError(err)
		intrinsic, _ := act.IntrinsicGas()
		elp := builder.SetNonce(test.nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(test.gasPrice).SetAction(act).Build()
		registerCost, err := elp.Cost()
		require.NoError(err)
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       test.blkGasLimit,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{}})
		ctx = genesis.WithGenesisContext(ctx, genesis.TestDefault())
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		_, err = p.Handle(ctx, elp, sm)
		require.NoError(err)

		cu, err := action.NewCandidateUpdate(test.updateName, test.updateOperator, test.updateReward)
		require.NoError(err)
		intrinsic, _ = cu.IntrinsicGas()
		elp = builder.SetNonce(test.nonce + 1).SetGasLimit(test.gasLimit).
			SetGasPrice(test.gasPrice).SetAction(cu).Build()
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce + 1,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       test.blkGasLimit,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{}})
		require.Equal(test.err, errors.Cause(p.Validate(ctx, elp, sm)))
		if test.err != nil {
			continue
		}
		r, err := p.Handle(ctx, elp, sm)
		require.NoError(err)
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test candidate
			candidate, _, err := csr.getCandidate(act.OwnerAddress())
			if act.OwnerAddress() == nil {
				require.Nil(candidate)
				require.Equal(ErrNilParameters, errors.Cause(err))
				candidate, _, err = csr.getCandidate(test.caller)
				require.NoError(err)
				require.Equal(test.caller.String(), candidate.Owner.String())
			} else {
				require.NotNil(candidate)
				require.NoError(err)
				require.Equal(test.ownerAddrStr, candidate.Owner.String())
			}
			require.Equal(test.afterUpdate, candidate.Votes.String())
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			candidate = csm.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal(test.afterUpdate, candidate.Votes.String())
			if test.updateName != "" {
				require.Equal(test.updateName, candidate.Name)
			}
			if test.updateOperator != "" {
				require.Equal(test.updateOperator, candidate.Operator.String())
			}
			if test.updateOperator != "" {
				require.Equal(test.updateReward, candidate.Reward.String())
			}
			require.Equal(test.afterUpdate, candidate.Votes.String())
			require.Equal(test.amountStr, candidate.SelfStake.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			cuCost, err := elp.Cost()
			require.NoError(err)
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, registerCost).Add(total, cuCost).Add(total, p.config.RegistrationConsts.Fee))
			require.Equal(test.nonce+2, caller.PendingNonce())
		}
	}
}

func TestProtocol_HandleUnstake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, p, candidate, candidate2 := initAll(t, ctrl)
	csr := newCandidateStateReader(sm)
	initCreateStake(t, sm, identityset.Address(2), 100, testGasPrice, 10000, 1, 1, time.Now(), 10000, p, candidate2, "100000000000000000000", false)

	callerAddr := identityset.Address(1)
	tests := []struct {
		// creat stake fields
		caller       address.Address
		amount       string
		autoStake    bool
		afterUnstake string
		initBalance  int64
		selfstaking  bool
		// action fields
		index uint64
		// block context
		blkTimestamp time.Time
		ctxTimestamp time.Time
		// clear flag for inMemCandidates
		clear bool
		// need new p
		newProtocol bool
		err         error
		// expected result
		status iotextypes.ReceiptStatus
	}{
		// fetchCaller ErrNotEnoughBalance
		{
			callerAddr,
			"100990000000000000000",
			false,
			"",
			101,
			false,
			0,
			time.Now(),
			time.Now(),
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// fetchBucket, bucket.Owner is not equal to actionCtx.Caller
		{
			identityset.Address(12),
			"100000000000000000000",
			false,
			"",
			101,
			false,
			0,
			time.Now(),
			time.Now(),
			false,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// fetchBucket,ReceiptStatus_ErrInvalidBucketType cannot happen,because allowSelfStaking is true
		// fetchBucket and updateBucket call getbucket, ReceiptStatus_ErrInvalidBucketIndex
		{
			identityset.Address(33),
			"100000000000000000000",
			false,
			"",
			101,
			false,
			1,
			time.Now(),
			time.Now(),
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketIndex,
		},
		// inMemCandidates.GetByOwner,ErrInvalidOwner
		{
			callerAddr,
			"100000000000000000000",
			false,
			"",
			101,
			false,
			0,
			time.Now(),
			time.Now(),
			true,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		// unstake before maturity
		{
			callerAddr,
			"100000000000000000000",
			false,
			"",
			101,
			false,
			0,
			time.Now(),
			time.Now(),
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrUnstakeBeforeMaturity,
		},
		// unstake with autoStake bucket
		{
			callerAddr,
			"100000000000000000000",
			true,
			"0",
			101,
			false,
			0,
			time.Now(),
			time.Now().Add(time.Duration(1) * 24 * time.Hour),
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
		},
		// Upsert error cannot happen,because collision cannot happen
		// success
		{
			callerAddr,
			"100000000000000000000",
			false,
			"0",
			101,
			false,
			0,
			time.Now(),
			time.Now().Add(time.Duration(1) * 24 * time.Hour),
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	var (
		ctx       context.Context
		gasLimit  uint64 = 10000
		nonce     uint64 = 1
		blkHeight uint64 = 1
	)

	for _, test := range tests {
		if test.newProtocol {
			sm, p, candidate, _ = initAll(t, ctrl)
		} else {
			candidate = candidate2
		}

		var createCost *big.Int
		ctx, createCost = initCreateStake(t, sm, test.caller, test.initBalance, testGasPrice, gasLimit, nonce, blkHeight, test.blkTimestamp, gasLimit, p, candidate, test.amount, test.autoStake)
		act := action.NewUnstake(test.index, nil)
		intrinsicGas, err := act.IntrinsicGas()
		elp := builder.SetNonce(nonce + 1).SetGasLimit(gasLimit).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		require.NoError(err)
		if test.blkTimestamp != test.ctxTimestamp {
			blkCtx := protocol.MustGetBlockCtx(ctx)
			blkCtx.BlockTimeStamp = test.ctxTimestamp
			ctx = protocol.WithBlockCtx(ctx, blkCtx)
		}
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsicGas,
			Nonce:        nonce + 1,
		})
		var r *action.Receipt
		if test.clear {
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			sc, ok := csm.(*candSM)
			require.True(ok)
			sc.candCenter.deleteForTestOnly(test.caller)
			require.False(csm.ContainsOwner(test.caller))
			r, err = p.handle(ctx, elp, csm)
			require.Equal(test.err, errors.Cause(err))
		} else {
			r, err = p.Handle(ctx, elp, sm)
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Success)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			csr = newCandidateStateReader(sm)
			bucketIndices, _, err := csr.candBucketIndices(candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.voterBucketIndices(candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := csr.getBucket(indices[0])
			require.NoError(err)
			require.Equal(candidate.Owner.String(), bucket.Candidate.String())
			require.Equal(test.caller.String(), bucket.Owner.String())
			require.Equal(test.amount, bucket.StakedAmount.String())

			// test candidate
			candidate, _, err = csr.getCandidate(candidate.Owner)
			require.NoError(err)
			require.Equal(test.afterUnstake, candidate.Votes.String())
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			candidate = csm.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal(test.afterUnstake, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			actCost, err := elp.Cost()
			require.NoError(err)
			require.Equal(nonce+2, caller.PendingNonce())
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, createCost))
		}
	}

	// verify bucket unstaked
	vb, err := csr.getBucket(0)
	require.NoError(err)
	require.True(vb.isUnstaked())

	unstakeAct := action.NewUnstake(0, nil)
	unstake := builder.SetNonce(nonce + 2).SetGasLimit(gasLimit).
		SetGasPrice(testGasPrice).SetAction(unstakeAct).Build()
	changeCandAct := action.NewChangeCandidate(candidate2.Name, 0, nil)
	changeCand := builder.SetNonce(nonce + 2).SetGasLimit(gasLimit).
		SetGasPrice(testGasPrice).SetAction(changeCandAct).Build()
	depositAct, err := action.NewDepositToStake(0, "10000", nil)
	deposit := builder.SetNonce(nonce + 2).SetGasLimit(gasLimit).
		SetGasPrice(testGasPrice).SetAction(depositAct).Build()
	require.NoError(err)
	restakeAct := action.NewRestake(0, 0, false, nil)
	restake := builder.SetNonce(nonce + 2).SetGasLimit(gasLimit).
		SetGasPrice(testGasPrice).SetAction(restakeAct).Build()

	unstakedBucketTests := []struct {
		act       action.Envelope
		greenland bool
		status    iotextypes.ReceiptStatus
	}{
		// unstake an already unstaked bucket again not allowed
		{unstake, true, iotextypes.ReceiptStatus_ErrInvalidBucketType},
		// change candidate for an unstaked bucket not allowed
		{changeCand, true, iotextypes.ReceiptStatus_ErrInvalidBucketType},
		// deposit to unstaked bucket not allowed
		{deposit, true, iotextypes.ReceiptStatus_ErrInvalidBucketType},
		// restake an unstaked bucket not allowed
		{restake, true, iotextypes.ReceiptStatus_ErrInvalidBucketType},
		// restake an unstaked bucket is allowed pre-Greenland
		{restake, false, iotextypes.ReceiptStatus_ErrNotEnoughBalance},
	}
	for i, v := range unstakedBucketTests {
		greenland := genesis.TestDefault()
		if v.greenland {
			blkCtx := protocol.MustGetBlockCtx(ctx)
			greenland.GreenlandBlockHeight = blkCtx.BlockHeight
		}
		actCtx := protocol.MustGetActionCtx(ctx)
		actCtx.Nonce = nonce + 2 + uint64(i)
		ctx = protocol.WithActionCtx(ctx, actCtx)
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: greenland.GreenlandBlockHeight - 1,
		}})
		ctx = genesis.WithGenesisContext(ctx, greenland)
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		_, err = p.Start(ctx, sm)
		require.NoError(err)
		r, err := p.Handle(ctx, v.act, sm)
		require.NoError(err)
		require.EqualValues(v.status, r.Status)

		if !v.greenland {
			// pre-Greenland allows restaking an unstaked bucket, and it is considered staked afterwards
			vb, err := csr.getBucket(0)
			require.NoError(err)
			require.True(vb.StakeStartTime.Unix() != 0)
			require.True(vb.UnstakeStartTime.Unix() != 0)
			require.False(vb.isUnstaked())
		}
	}
	t.Run("CleanSelfStake", func(t *testing.T) {
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		runtest := func(t *testing.T, height uint64, fn func(vb *VoteBucket, cand *Candidate, sm protocol.StateManager)) {
			ctrl := gomock.NewController(t)
			sm, p, vbs, cands := initTestState(t, ctrl, []*bucketConfig{
				{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, false, true, nil, 0},
			}, []*candidateConfig{
				{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
			})
			sk := identityset.PrivateKey(1)
			require.NoError(setupAccount(sm, sk.PublicKey().Address(), 100000000))
			// unstake
			unstake, err := action.SignedReclaimStake(false, 0, cands[0].SelfStakeBucketIdx, nil, gasLimit, testGasPrice, sk)
			require.NoError(err)
			ctx := genesis.WithGenesisContext(context.Background(), g)
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
				BlockHeight:    height,
				BlockTimeStamp: vbs[0].StakeStartTime.Add(vbs[0].StakedDuration),
			})
			ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
			ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
				Caller:       sk.PublicKey().Address(),
				GasPrice:     unstake.GasPrice(),
				IntrinsicGas: assertions.MustNoErrorV(unstake.IntrinsicGas()),
				Nonce:        unstake.Nonce(),
			})
			receipt, err := p.Handle(ctx, unstake.Envelope, sm)
			require.NoError(err)
			require.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
			fn(vbs[0], cands[0], sm)
		}
		t.Run("NotCleanAtUpernavik", func(t *testing.T) {
			runtest(t, g.UpernavikBlockHeight, func(vb *VoteBucket, cand *Candidate, sm protocol.StateManager) {
				csm, err := NewCandidateStateManager(sm)
				require.NoError(err)
				newCand := csm.GetByIdentifier(cand.GetIdentifier())
				selfStakeVotes := CalculateVoteWeight(p.config.VoteWeightCalConsts, vb, true)
				votes := CalculateVoteWeight(p.config.VoteWeightCalConsts, vb, false)
				require.Equal(new(big.Int).Sub(selfStakeVotes, votes).String(), newCand.Votes.String())
				require.Equal(cand.SelfStake, newCand.SelfStake)
				require.Equal(cand.SelfStakeBucketIdx, newCand.SelfStakeBucketIdx)
			})
		})
		t.Run("CleanAtVanuatu", func(t *testing.T) {
			runtest(t, g.VanuatuBlockHeight, func(vb *VoteBucket, cand *Candidate, sm protocol.StateManager) {
				csm, err := NewCandidateStateManager(sm)
				require.NoError(err)
				newCand := csm.GetByIdentifier(cand.GetIdentifier())
				require.Equal("0", newCand.Votes.String())
				require.Equal(big.NewInt(0), newCand.SelfStake)
				require.Equal(uint64(candidateNoSelfStakeBucketIndex), newCand.SelfStakeBucketIdx)
			})
		})
	})
}

func TestProtocol_HandleWithdrawStake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	tests := []struct {
		// create stake fields
		amount      string
		initBalance int64
		selfstaking bool
		// block context
		blkTimestamp time.Time
		ctxTimestamp time.Time
		// if unstake
		unstake bool
		// withdraw fields
		withdrawIndex uint64
		// expected result
		status iotextypes.ReceiptStatus
	}{
		// fetchCaller ErrNotEnoughBalance
		{
			"100990000000000000000",
			101,
			false,
			time.Now(),
			time.Now(),
			true,
			0,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// fetchBucket ReceiptStatus_ErrInvalidBucketIndex
		{
			"100000000000000000000",
			101,
			false,
			time.Now(),
			time.Now(),
			true,
			1,
			iotextypes.ReceiptStatus_ErrInvalidBucketIndex,
		},
		// check unstake time,ReceiptStatus_ErrWithdrawBeforeUnstake
		{
			"100000000000000000000",
			101,
			false,
			time.Now(),
			time.Now(),
			false,
			0,
			iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake,
		},
		// check ReceiptStatus_ErrWithdrawBeforeMaturity
		{
			"100000000000000000000",
			101,
			false,
			time.Now(),
			time.Now(),
			true,
			0,
			iotextypes.ReceiptStatus_ErrWithdrawBeforeMaturity,
		},
		// delxxx cannot happen,because unstake first called without error
		// ReceiptStatus_Success
		{
			"100000000000000000000",
			101,
			false,
			time.Now(),
			time.Now().Add(time.Hour * 500),
			true,
			0,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		sm, p, _, candidate := initAll(t, ctrl)
		csr := newCandidateStateReader(sm)
		caller := identityset.Address(2)
		require.NoError(setupAccount(sm, caller, test.initBalance))
		gasLimit := uint64(10000)
		ctx, createCost := initCreateStake(t, sm, candidate.Owner, test.initBalance, testGasPrice, gasLimit, 1, 1, test.blkTimestamp, gasLimit, p, candidate, test.amount, false)
		var actCost *big.Int
		if test.unstake {
			act := action.NewUnstake(0, nil)
			intrinsic, err := act.IntrinsicGas()
			require.NoError(err)
			elp := builder.SetNonce(1).SetGasLimit(gasLimit).
				SetGasPrice(testGasPrice).SetAction(act).Build()
			actCost, err = elp.Cost()
			require.NoError(err)
			ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
				Caller:       caller,
				GasPrice:     testGasPrice,
				IntrinsicGas: intrinsic,
				Nonce:        2,
			})
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
				BlockHeight:    1,
				BlockTimeStamp: time.Now().Add(time.Duration(1) * 24 * time.Hour),
				GasLimit:       1000000,
			})

			_, err = p.Handle(ctx, elp, sm)
			require.NoError(err)
			require.NoError(p.Commit(ctx, sm))
			require.Equal(0, sm.Snapshot())
		}

		withdraw := action.NewWithdrawStake(test.withdrawIndex, nil)
		elp := builder.SetNonce(1).SetGasLimit(gasLimit).SetGasPrice(testGasPrice).
			SetAction(withdraw).Build()
		actionCtx := protocol.MustGetActionCtx(ctx)
		blkCtx := protocol.MustGetBlockCtx(ctx)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       actionCtx.Caller,
			GasPrice:     actionCtx.GasPrice,
			IntrinsicGas: actionCtx.IntrinsicGas,
			Nonce:        actionCtx.Nonce + 1,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    blkCtx.BlockHeight,
			BlockTimeStamp: test.ctxTimestamp,
			GasLimit:       blkCtx.GasLimit,
		})
		r, err := p.Handle(ctx, elp, sm)
		require.NoError(err)
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.status == iotextypes.ReceiptStatus_Success {
			// check the special withdraw bucket log
			tLogs := r.TransactionLogs()
			require.Equal(1, len(tLogs))
			wLog := tLogs[0]
			require.Equal(address.StakingBucketPoolAddr, wLog.Sender)
			require.Equal(caller.String(), wLog.Recipient)
			require.Equal(test.amount, wLog.Amount.String())

			// test bucket index and bucket
			_, _, err := csr.candBucketIndices(candidate.Owner)
			require.Error(err)
			_, _, err = csr.voterBucketIndices(candidate.Owner)
			require.Error(err)

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, caller)
			require.NoError(err)
			withdrawCost, err := elp.Cost()
			require.NoError(err)
			require.Equal(uint64(4), caller.PendingNonce())
			total := big.NewInt(0)
			withdrawAmount, ok := new(big.Int).SetString(test.amount, 10)
			require.True(ok)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, withdrawCost).Add(total, createCost).Sub(total, withdrawAmount))
		}
	}
}

func TestProtocol_HandleChangeCandidate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	tests := []struct {
		// creat stake fields
		caller               address.Address
		amount               string
		afterChange          string
		afterChangeSelfStake string
		initBalance          int64
		selfstaking          bool
		// action fields
		index         uint64
		candidateName string
		gasLimit      uint64
		nonce         uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		blkGasLimit  uint64
		// clear flag for inMemCandidates
		clear bool
		// expected result
		err    error
		status iotextypes.ReceiptStatus
	}{
		// fetchCaller ReceiptStatus_ErrNotEnoughBalance
		{
			identityset.Address(1),
			"100990000000000000000",
			"0",
			"0",
			101,
			false,
			1,
			"test2",
			10000,
			1,
			1,
			time.Now(),
			10000,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// ReceiptStatus_ErrCandidateNotExist
		{
			identityset.Address(1),
			"100000000000000000000",
			"0",
			"0",
			101,
			false,
			1,
			"testname",
			10000,
			1,
			1,
			time.Now(),
			10000,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		// fetchBucket,ReceiptStatus_ErrInvalidBucketType
		{
			identityset.Address(1),
			"100000000000000000000",
			"0",
			"0",
			101,
			true,
			1,
			"test2",
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
		},
		// ErrInvalidOwner
		{
			identityset.Address(1),
			"100000000000000000000",
			"0",
			"0",
			101,
			false,
			1,
			"test2",
			10000,
			1,
			1,
			time.Now(),
			10000,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		// invalid candidate name
		{
			identityset.Address(2),
			"100000000000000000000",
			"200000000000000000000",
			"1200000000000000000000000",
			101,
			false,
			0,
			"~1",
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			action.ErrInvalidCanName,
			iotextypes.ReceiptStatus_Failure,
		},
		// invalid candidate name 2
		{
			identityset.Address(2),
			"100000000000000000000",
			"200000000000000000000",
			"1200000000000000000000000",
			101,
			false,
			0,
			"0123456789abc",
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			action.ErrInvalidCanName,
			iotextypes.ReceiptStatus_Failure,
		},
		// invalid candidate name 3
		{
			identityset.Address(2),
			"100000000000000000000",
			"200000000000000000000",
			"1200000000000000000000000",
			101,
			false,
			0,
			"",
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			action.ErrInvalidCanName,
			iotextypes.ReceiptStatus_Failure,
		},
		// Upsert error cannot happen,because CreateStake already check collision
		// change from 0 to test1
		{
			identityset.Address(2),
			"100000000000000000000",
			"200000000000000000000",
			"1200000000000000000000000",
			101,
			false,
			0,
			"test1",
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		// test change to same candidate (Hawaii fix)
		{
			identityset.Address(2),
			"100000000000000000000",
			"100000000000000000000",
			"1200000000000000000000000",
			101,
			false,
			0,
			"test2",
			10000,
			1,
			genesis.TestDefault().HawaiiBlockHeight,
			time.Now(),
			10000,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateAlreadyExist,
		},
	}

	for _, test := range tests {
		sm, p, candidate, candidate2 := initAll(t, ctrl)
		csr := newCandidateStateReader(sm)
		// candidate2 vote self,index 0
		ctx, _ := initCreateStake(t, sm, candidate2.Owner, 101, testGasPrice, 10000, 1, 1, time.Now(), 10000, p, candidate2, "100000000000000000000", false)
		// candidate vote self,index 1
		_, createCost := initCreateStake(t, sm, candidate.Owner, test.initBalance, testGasPrice, test.gasLimit, test.nonce, test.blkHeight, test.blkTimestamp, test.blkGasLimit, p, candidate, test.amount, false)

		act := action.NewChangeCandidate(test.candidateName, test.index, nil)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)
		elp := builder.SetNonce(test.nonce + 1).SetGasLimit(test.gasLimit).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce + 1,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    test.blkHeight,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: test.blkHeight - 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		var r *action.Receipt
		if test.clear {
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			sc, ok := csm.(*candSM)
			require.True(ok)
			cc := sc.candCenter.GetBySelfStakingIndex(test.index)
			sc.candCenter.deleteForTestOnly(cc.Owner)
			require.False(csm.ContainsOwner(cc.Owner))
			r, err = p.handle(ctx, elp, csm)
			require.Equal(test.err, errors.Cause(err))
		} else {
			require.Equal(test.err, errors.Cause(p.Validate(ctx, elp, sm)))
			if test.err != nil {
				continue
			}
			r, err = p.Handle(ctx, elp, sm)
			require.NoError(err)
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, _, err := csr.candBucketIndices(identityset.Address(1))
			require.NoError(err)
			require.Equal(2, len(*bucketIndices))
			bucketIndices, _, err = csr.voterBucketIndices(identityset.Address(1))
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := csr.getBucket(indices[0])
			require.NoError(err)
			require.Equal(identityset.Address(1).String(), bucket.Candidate.String())
			require.Equal(identityset.Address(1).String(), bucket.Owner.String())
			require.Equal(test.amount, bucket.StakedAmount.String())

			// test candidate
			candidate, _, err := csr.getCandidate(candidate.Owner)
			require.NotNil(candidate)
			require.NoError(err)
			require.Equal(test.afterChange, candidate.Votes.String())
			require.Equal(test.candidateName, candidate.Name)
			require.Equal(candidate.Operator.String(), candidate.Operator.String())
			require.Equal(candidate.Reward.String(), candidate.Reward.String())
			require.Equal(candidate.Owner.String(), candidate.Owner.String())
			require.Equal(test.afterChangeSelfStake, candidate.SelfStake.String())
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			candidate = csm.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal(test.afterChange, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			actCost, err := elp.Cost()
			require.NoError(err)
			require.Equal(test.nonce+2, caller.PendingNonce())
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, createCost))
		}
	}
}

func TestProtocol_HandleChangeCandidate_ClearPrevCandidateSelfStake(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	t.Run("clear if bucket is an expired endorse bucket", func(t *testing.T) {
		sm, p, buckets, _ := initTestStateWithHeight(t, ctrl,
			[]*bucketConfig{
				{identityset.Address(1), identityset.Address(5), "100000000000000000000", 1, true, true, nil, 1},
			},
			[]*candidateConfig{
				{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
				{identityset.Address(2), identityset.Address(12), identityset.Address(22), "test2"},
			}, 1)
		r.NoError(setupAccount(sm, identityset.Address(5), 10000))
		nonce := uint64(1)
		act := action.NewChangeCandidate("test2", buckets[0].Index, nil)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(5),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_Success, recipt.Status)
		// test previous candidate self stake
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)
		prevCand := csm.GetByOwner(identityset.Address(1))
		r.Equal("0", prevCand.SelfStake.String())
		r.EqualValues(uint64(candidateNoSelfStakeBucketIndex), prevCand.SelfStakeBucketIdx)
		r.Equal("0", prevCand.Votes.String())
	})
	t.Run("not clear if bucket is a vote bucket", func(t *testing.T) {
		sm, p, buckets, _ := initTestStateWithHeight(t, ctrl,
			[]*bucketConfig{
				{identityset.Address(1), identityset.Address(1), "120000000000000000000", 1, true, true, nil, 0},
				{identityset.Address(2), identityset.Address(2), "120000000000000000000", 1, true, true, nil, 0},
				{identityset.Address(1), identityset.Address(1), "100000000000000000000", 1, true, false, nil, 0},
			},
			[]*candidateConfig{
				{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
				{identityset.Address(2), identityset.Address(12), identityset.Address(22), "test2"},
			}, 1)
		r.NoError(setupAccount(sm, identityset.Address(1), 10000))
		nonce := uint64(1)
		act := action.NewChangeCandidate("test2", buckets[2].Index, nil)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(1),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_Success, recipt.Status)
		// test previous candidate self stake
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)
		prevCand := csm.GetByOwner(buckets[2].Candidate)
		r.Equal("120000000000000000000", prevCand.SelfStake.String())
		r.EqualValues(0, prevCand.SelfStakeBucketIdx)
		r.Equal("124562140820308711042", prevCand.Votes.String())
	})
	t.Run("not clear if bucket is a vote bucket", func(t *testing.T) {
		sm, p, buckets, _ := initTestStateWithHeight(t, ctrl,
			[]*bucketConfig{
				{identityset.Address(1), identityset.Address(1), "120000000000000000000", 1, true, true, nil, 0},
				{identityset.Address(2), identityset.Address(2), "120000000000000000000", 1, true, true, nil, 0},
				{identityset.Address(1), identityset.Address(1), "100000000000000000000", 1, true, false, nil, 0},
			},
			[]*candidateConfig{
				{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
				{identityset.Address(2), identityset.Address(12), identityset.Address(22), "test2"},
			}, 1)
		r.NoError(setupAccount(sm, identityset.Address(1), 10000))
		nonce := uint64(1)
		act := action.NewChangeCandidate("test2", buckets[2].Index, nil)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(1),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_Success, recipt.Status)
		// test previous candidate self stake
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)
		prevCand := csm.GetByOwner(buckets[2].Candidate)
		r.Equal("120000000000000000000", prevCand.SelfStake.String())
		r.EqualValues(0, prevCand.SelfStakeBucketIdx)
		r.Equal("124562140820308711042", prevCand.Votes.String())
	})
}

func TestProtocol_HandleTransferStake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	tests := []struct {
		// creat stake fields
		caller        address.Address
		amount        string
		afterTransfer uint64
		initBalance   int64
		// action fields
		index    uint64
		gasLimit uint64
		nonce    uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		blkGasLimit  uint64
		// NewTransferStake fields
		to            address.Address
		toInitBalance uint64
		init          bool
		// expected result
		err    error
		status iotextypes.ReceiptStatus
	}{
		// fetchCaller ReceiptStatus_ErrNotEnoughBalance
		{
			identityset.Address(2),
			"100990000000000000000",
			0,
			101,
			0,
			1000000000,
			2,
			1,
			time.Now(),
			10000,
			identityset.Address(1),
			1,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// fetchBucket,bucket.Owner not equal to actionCtx.Caller
		{
			identityset.Address(1),
			"100000000000000000000",
			0,
			1000,
			0,
			10000,
			1,
			1,
			time.Now(),
			10000,
			identityset.Address(2),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// fetchBucket,inMemCandidates.ContainsSelfStakingBucket is false
		{
			identityset.Address(1),
			"100000000000000000000",
			0,
			101,
			1,
			10000,
			1,
			1,
			time.Now(),
			10000,
			identityset.Address(2),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
		},
		{
			identityset.Address(2),
			"100000000000000000000",
			0,
			101,
			0,
			10000,
			2,
			1,
			time.Now(),
			10000,
			identityset.Address(1),
			1,
			false,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		// test transfer to same owner
		{
			identityset.Address(2),
			"100000000000000000000",
			0,
			101,
			0,
			10000,
			2,
			genesis.TestDefault().HawaiiBlockHeight,
			time.Now(),
			10000,
			identityset.Address(2),
			1,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
		},
	}

	for _, test := range tests {
		sm, p, candi, candidate2 := initAll(t, ctrl)
		csr := newCandidateStateReader(sm)
		nonce := uint64(1)
		if test.caller.String() == candidate2.Owner.String() {
			nonce++
		}
		ctx, createCost := initCreateStake(t, sm, candidate2.Owner, test.initBalance, testGasPrice, 10000, 1, 1, time.Now(), 10000, p, candidate2, test.amount, false)
		if test.init {
			initCreateStake(t, sm, candi.Owner, test.initBalance, testGasPrice, test.gasLimit, test.nonce, test.blkHeight, test.blkTimestamp, test.blkGasLimit, p, candi, test.amount, false)
			if test.caller.String() == candi.Owner.String() {
				nonce++
			}
		} else {
			require.NoError(setupAccount(sm, identityset.Address(1), 1))
		}

		act, err := action.NewTransferStake(test.to.String(), test.index, nil)
		require.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    test.blkHeight,
			BlockTimeStamp: time.Now(),
			GasLimit:       10000000,
		})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		r, err := p.Handle(ctx, elp, sm)
		require.Equal(test.err, errors.Cause(err))
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, _, err := csr.candBucketIndices(candidate2.GetIdentifier())
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.voterBucketIndices(test.to)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := csr.getBucket(indices[0])
			require.NoError(err)
			require.Equal(candidate2.GetIdentifier(), bucket.Candidate)
			require.Equal(test.to.String(), bucket.Owner.String())
			require.Equal(test.amount, bucket.StakedAmount.String())

			// test candidate
			candidate, _, err := csr.getCandidate(candi.Owner)
			require.NoError(err)
			require.Equal(test.afterTransfer, candidate.Votes.Uint64())
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			candidate = csm.GetByOwner(candi.Owner)
			require.NotNil(candidate)
			require.LessOrEqual(test.afterTransfer, candidate.Votes.Uint64())
			require.Equal(candi.Name, candidate.Name)
			require.Equal(candi.Operator, candidate.Operator)
			require.Equal(candi.Reward, candidate.Reward)
			require.Equal(candi.Owner, candidate.Owner)
			require.Equal(test.afterTransfer, candidate.Votes.Uint64())
			require.LessOrEqual(test.afterTransfer, candidate.SelfStake.Uint64())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			actCost, err := elp.Cost()
			require.NoError(err)
			require.Equal(test.nonce+1, caller.PendingNonce())
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, createCost))
		}
	}
}

func TestProtocol_HandleConsignmentTransfer(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	tests := []struct {
		bucketOwner string
		blkHeight   uint64
		to          address.Address
		// consignment fields
		nilPayload  bool
		consignType string
		reclaim     string
		wrongSig    bool
		sigIndex    uint64
		sigNonce    uint64
		status      iotextypes.ReceiptStatus
	}{
		// case I: p.hu.IsPreGreenland(blkCtx.BlockHeight)
		{
			identityset.PrivateKey(2).HexString(),
			1,
			identityset.Address(3),
			false,
			"Ethereum",
			_reclaim,
			false,
			0,
			1,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// case II: len(act.Payload()) == 0
		{
			identityset.PrivateKey(2).HexString(),
			5553821,
			identityset.Address(3),
			true,
			"Ethereum",
			_reclaim,
			false,
			0,
			1,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// case III: type is not Ethereum
		{
			identityset.PrivateKey(2).HexString(),
			5553821,
			identityset.Address(3),
			false,
			"xx",
			_reclaim,
			false,
			0,
			1,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// case IV: msg.Reclaim != _reclaim
		{
			identityset.PrivateKey(2).HexString(),
			5553821,
			identityset.Address(3),
			false,
			"Ethereum",
			"wrong reclaim",
			false,
			0,
			1,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// case V: RecoverPubkeyFromEccSig error
		{
			identityset.PrivateKey(2).HexString(),
			5553821,
			identityset.Address(3),
			false,
			"Ethereum",
			_reclaim,
			true,
			0,
			1,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// case VI: transferor is not bucket.Owner
		{
			identityset.PrivateKey(31).HexString(),
			5553821,
			identityset.Address(1),
			false,
			"Ethereum",
			_reclaim,
			false,
			0,
			1,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// case VII: transferee is not actCtx.Caller
		{
			identityset.PrivateKey(32).HexString(),
			5553821,
			identityset.Address(3),
			false,
			"Ethereum",
			_reclaim,
			false,
			0,
			1,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// case VIII: signed asset id is not equal to bucket.Index
		{
			identityset.PrivateKey(32).HexString(),
			5553821,
			identityset.Address(1),
			false,
			"Ethereum",
			_reclaim,
			false,
			1,
			1,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// case IX: transfereeNonce is not equal to actCtx.Nonce
		{
			identityset.PrivateKey(32).HexString(),
			5553821,
			identityset.Address(1),
			false,
			"Ethereum",
			_reclaim,
			false,
			0,
			2,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// case X: success
		{
			identityset.PrivateKey(32).HexString(),
			6544441,
			identityset.Address(1),
			false,
			"Ethereum",
			_reclaim,
			false,
			0,
			1,
			iotextypes.ReceiptStatus_Success,
		},
	}
	for _, test := range tests {
		sm, p, cand1, cand2 := initAll(t, ctrl)
		csr := newCandidateStateReader(sm)
		caller := identityset.Address(1)
		initBalance := int64(1000)
		require.NoError(setupAccount(sm, caller, initBalance))
		stakeAmount := "100000000000000000000"
		gasLimit := uint64(10000)
		ctx, _ := initCreateStake(t, sm, identityset.Address(32), initBalance, testGasPrice, gasLimit, 1, test.blkHeight, time.Now(), gasLimit, p, cand2, stakeAmount, false)
		initCreateStake(t, sm, identityset.Address(31), initBalance, testGasPrice, gasLimit, 1, test.blkHeight, time.Now(), gasLimit, p, cand1, stakeAmount, false)

		// transfer to test.to through consignment
		var consign []byte
		if !test.nilPayload {
			consign = newconsignment(require, test.sigIndex, test.sigNonce, test.bucketOwner, test.to.String(), test.consignType, test.reclaim, test.wrongSig)
		}

		act, err := action.NewTransferStake(caller.String(), 0, consign)
		require.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)
		elp := builder.SetNonce(1).SetGasLimit(gasLimit).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       caller,
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        1,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    test.blkHeight,
			BlockTimeStamp: time.Now(),
			GasLimit:       gasLimit,
		})
		r, err := p.Handle(ctx, elp, sm)
		require.NoError(err)
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, _, err := csr.candBucketIndices(cand2.GetIdentifier())
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.voterBucketIndices(test.to)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := csr.getBucket(indices[0])
			require.NoError(err)
			require.Equal(cand2.GetIdentifier(), bucket.Candidate)
			require.Equal(test.to.String(), bucket.Owner.String())
			require.Equal(stakeAmount, bucket.StakedAmount.String())

			// test candidate
			candidate, _, err := csr.getCandidate(cand1.GetIdentifier())
			require.NoError(err)
			require.LessOrEqual(uint64(0), candidate.Votes.Uint64())
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			candidate = csm.GetByOwner(cand1.GetIdentifier())
			require.NotNil(candidate)
			require.LessOrEqual(uint64(0), candidate.Votes.Uint64())
			require.Equal(cand1.Name, candidate.Name)
			require.Equal(cand1.Operator, candidate.Operator)
			require.Equal(cand1.Reward, candidate.Reward)
			require.Equal(cand1.GetIdentifier(), candidate.GetIdentifier())
			require.LessOrEqual(uint64(0), candidate.Votes.Uint64())
			require.LessOrEqual(uint64(0), candidate.SelfStake.Uint64())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, caller)
			require.NoError(err)
			actCost, err := elp.Cost()
			require.NoError(err)
			require.Equal(uint64(2), caller.PendingNonce())
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(initBalance), total.Add(total, caller.Balance).Add(total, actCost))
		}
	}
}

func TestProtocol_HandleRestake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	callerAddr := identityset.Address(2)

	tests := []struct {
		// creat stake fields
		caller       address.Address
		amount       string
		afterRestake string
		initBalance  int64
		selfstaking  bool
		// action fields
		index    uint64
		gasLimit uint64
		nonce    uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		blkGasLimit  uint64
		// restake fields
		duration  uint32
		autoStake bool
		// clear flag for inMemCandidates
		clear bool
		// need new p
		newAccount bool
		// expected result
		err    error
		status iotextypes.ReceiptStatus
	}{
		// fetchCaller ReceiptStatus_ErrNotEnoughBalance
		{
			callerAddr,
			"100990000000000000000",
			"0",
			101,
			false,
			0,
			10000,
			1,
			1,
			time.Now(),
			10000,
			1,
			true,
			false,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// fetchBucket, bucket.Owner is not equal to actionCtx.Caller
		{
			identityset.Address(12),
			"100000000000000000000",
			"0",
			101,
			false,
			0,
			10000,
			1,
			1,
			time.Now(),
			10000,
			1,
			true,
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// updateBucket getbucket ErrStateNotExist
		{
			identityset.Address(33),
			"100000000000000000000",
			"0",
			101,
			false,
			1,
			10000,
			1,
			1,
			time.Now(),
			10000,
			1,
			true,
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketIndex,
		},
		// for inMemCandidates.GetByOwner,ErrInvalidOwner
		{
			callerAddr,
			"100000000000000000000",
			"0",
			101,
			false,
			0,
			10000,
			1,
			1,
			time.Now(),
			10000,
			1,
			true,
			true,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		// autoStake = true, set up duration
		{
			callerAddr,
			"100000000000000000000",
			"103801784016923925869",
			101,
			false,
			0,
			10000,
			1,
			1,
			time.Now(),
			10000,
			0,
			true,
			false,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
		},
		// autoStake = false, set up duration
		{
			callerAddr,
			"100000000000000000000",
			"103801784016923925869",
			101,
			false,
			0,
			10000,
			1,
			1,
			time.Now(),
			10000,
			0,
			false,
			false,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrReduceDurationBeforeMaturity,
		},
		// candidate.SubVote,ErrInvalidAmount cannot happen,because previous vote with no error
		// ReceiptStatus_Success
		{
			callerAddr,
			"100000000000000000000",
			"103801784016923925869",
			101,
			false,
			0,
			10000,
			1,
			1,
			time.Now(),
			10000,
			1,
			true,
			false,
			false,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		sm, p, candidate, candidate2 := initAll(t, ctrl)
		csr := newCandidateStateReader(sm)
		ctx, createCost := initCreateStake(t, sm, candidate2.Owner, test.initBalance, testGasPrice, 10000, test.nonce, 1, time.Now(), 10000, p, candidate2, test.amount, test.autoStake)

		if test.newAccount {
			require.NoError(setupAccount(sm, test.caller, test.initBalance))
		} else {
			candidate = candidate2
		}
		nonce := test.nonce
		if test.caller.String() == candidate2.Owner.String() {
			nonce++
		}

		act := action.NewRestake(test.index, test.duration, test.autoStake, nil)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       10000000,
		})
		var r *action.Receipt
		if test.clear {
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			sc, ok := csm.(*candSM)
			require.True(ok)
			sc.candCenter.deleteForTestOnly(test.caller)
			require.False(csm.ContainsOwner(test.caller))
			ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
			r, err = p.handle(ctx, elp, csm)
			require.Equal(test.err, errors.Cause(err))
		} else {
			r, err = p.Handle(ctx, elp, sm)
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, _, err := csr.candBucketIndices(candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.voterBucketIndices(candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := csr.getBucket(indices[0])
			require.NoError(err)
			require.Equal(candidate.Owner.String(), bucket.Candidate.String())
			require.Equal(test.caller.String(), bucket.Owner.String())
			require.Equal(test.amount, bucket.StakedAmount.String())

			// test candidate
			candidate, _, err = csr.getCandidate(candidate.Owner)
			require.NoError(err)
			require.Equal(test.afterRestake, candidate.Votes.String())
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			candidate = csm.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal(test.afterRestake, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			actCost, err := elp.Cost()
			require.NoError(err)
			require.Equal(test.nonce+2, caller.PendingNonce())
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, createCost))
		}
	}
}

func TestProtocol_HandleDepositToStake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	tests := []struct {
		// creat stake fields
		caller       address.Address
		amount       string
		afterDeposit string
		initBalance  int64
		selfstaking  bool
		// action fields
		index    uint64
		gasLimit uint64
		nonce    uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		blkGasLimit  uint64
		autoStake    bool
		// clear flag for inMemCandidates
		clear bool
		// need new p
		newAccount bool
		// expected result
		err    error
		status iotextypes.ReceiptStatus
	}{
		// fetchCaller ErrNotEnoughBalance
		{
			identityset.Address(1),
			"100000000000000000000",
			"0",
			10,
			false,
			0,
			10000,
			2,
			1,
			time.Now(),
			10000,
			false,
			false,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// fetchBucket ReceiptStatus_ErrInvalidBucketIndex
		{
			identityset.Address(12),
			"100000000000000000000",
			"0",
			101,
			false,
			1,
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketIndex,
		},
		// fetchBucket ReceiptStatus_ErrInvalidBucketType
		{
			identityset.Address(33),
			"100000000000000000000",
			"0",
			101,
			false,
			0,
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			true,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
		},
		// for inMemCandidates.GetByOwner,ErrInvalidOwner
		{
			identityset.Address(1),
			"100000000000000000000",
			"0",
			201,
			false,
			0,
			10000,
			2,
			1,
			time.Now(),
			10000,
			true,
			true,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		// ReceiptStatus_Success
		{
			identityset.Address(1),
			"100000000000000000000",
			"207603568033847851737",
			201,
			false,
			0,
			10000,
			2,
			1,
			time.Now(),
			10000,
			true,
			false,
			false,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		sm, p, candidate, _ := initAll(t, ctrl)
		csr := newCandidateStateReader(sm)
		ctx, createCost := initCreateStake(t, sm, candidate.Owner, test.initBalance, testGasPrice, 10000, 1, 1, time.Now(), 10000, p, candidate, test.amount, test.autoStake)

		if test.newAccount {
			require.NoError(setupAccount(sm, test.caller, test.initBalance))
		}

		act, err := action.NewDepositToStake(test.index, test.amount, nil)
		require.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)
		elp := builder.SetNonce(test.nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       10000000,
		})
		var r *action.Receipt
		if test.clear {
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			sc, ok := csm.(*candSM)
			require.True(ok)
			sc.candCenter.deleteForTestOnly(test.caller)
			require.False(csm.ContainsOwner(test.caller))
			ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
			r, err = p.handle(ctx, elp, csm)
			require.Equal(test.err, errors.Cause(err))
		} else {
			r, err = p.Handle(ctx, elp, sm)
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// check the special deposit bucket log
			tLogs := r.TransactionLogs()
			require.Equal(1, len(tLogs))
			dLog := tLogs[0]
			require.Equal(test.caller.String(), dLog.Sender)
			require.Equal(address.StakingBucketPoolAddr, dLog.Recipient)
			require.Equal(test.amount, dLog.Amount.String())

			// test bucket index and bucket
			bucketIndices, _, err := csr.candBucketIndices(candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.voterBucketIndices(candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices

			bucket, err := csr.getBucket(indices[0])
			require.NoError(err)
			require.Equal(candidate.Owner.String(), bucket.Candidate.String())
			require.Equal(test.caller.String(), bucket.Owner.String())
			amount, _ := new(big.Int).SetString(test.amount, 10)
			require.Zero(new(big.Int).Mul(amount, big.NewInt(2)).Cmp(bucket.StakedAmount))

			// test candidate
			candidate, _, err = csr.getCandidate(candidate.Owner)
			require.NoError(err)
			require.Equal(test.afterDeposit, candidate.Votes.String())
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			candidate = csm.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal(test.afterDeposit, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			actCost, err := elp.Cost()
			require.NoError(err)
			require.Equal(test.nonce+1, caller.PendingNonce())
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, createCost))
		}
	}
}

func TestProtocol_FetchBucketAndValidate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm, p, _, _ := initAll(t, ctrl)

	t.Run("bucket not exist", func(t *testing.T) {
		csm, err := NewCandidateStateManager(sm)
		require.NoError(err)
		patches := gomonkey.ApplyPrivateMethod(csm, "getBucket", func(index uint64) (*VoteBucket, error) {
			return nil, state.ErrStateNotExist
		})
		defer patches.Reset()
		_, err = p.fetchBucketAndValidate(protocol.FeatureCtx{}, csm, identityset.Address(1), 1, true, true)
		require.ErrorContains(err, "failed to fetch bucket")
	})
	t.Run("validate owner", func(t *testing.T) {
		csm, err := NewCandidateStateManager(sm)
		require.NoError(err)
		patches := gomonkey.ApplyPrivateMethod(csm, "getBucket", func(index uint64) (*VoteBucket, error) {
			return &VoteBucket{
				Owner: identityset.Address(1),
			}, nil
		})
		defer patches.Reset()
		_, err = p.fetchBucketAndValidate(protocol.FeatureCtx{}, csm, identityset.Address(2), 1, true, true)
		require.ErrorContains(err, "bucket owner does not match")
		_, err = p.fetchBucketAndValidate(protocol.FeatureCtx{}, csm, identityset.Address(1), 1, true, true)
		require.NoError(err)
	})
	t.Run("validate selfstake", func(t *testing.T) {
		csm, err := NewCandidateStateManager(sm)
		require.NoError(err)
		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyPrivateMethod(csm, "getBucket", func(index uint64) (*VoteBucket, error) {
			return &VoteBucket{
				Owner: identityset.Address(1),
			}, nil
		})
		isSelfStake := true
		isSelfStakeErr := error(nil)
		patches.ApplyFunc(isSelfStakeBucket, func(featureCtx protocol.FeatureCtx, csm CandidiateStateCommon, bucket *VoteBucket) (bool, error) {
			return isSelfStake, isSelfStakeErr
		})
		_, err = p.fetchBucketAndValidate(protocol.FeatureCtx{}, csm, identityset.Address(1), 1, false, false)
		require.ErrorContains(err, "self staking bucket cannot be processed")
		isSelfStake = false
		_, err = p.fetchBucketAndValidate(protocol.FeatureCtx{}, csm, identityset.Address(1), 1, false, false)
		require.NoError(err)
		isSelfStakeErr = errors.New("unknown error")
		_, err = p.fetchBucketAndValidate(protocol.FeatureCtx{}, csm, identityset.Address(1), 1, false, false)
		require.ErrorContains(err, "unknown error")
	})
	t.Run("validate owner and selfstake", func(t *testing.T) {
		csm, err := NewCandidateStateManager(sm)
		require.NoError(err)
		patches := gomonkey.NewPatches()
		patches.ApplyPrivateMethod(csm, "getBucket", func(index uint64) (*VoteBucket, error) {
			return &VoteBucket{
				Owner: identityset.Address(1),
			}, nil
		})
		patches.ApplyFunc(isSelfStakeBucket, func(featureCtx protocol.FeatureCtx, csm CandidiateStateCommon, bucket *VoteBucket) (bool, error) {
			return false, nil
		})
		defer patches.Reset()

		_, err = p.fetchBucketAndValidate(protocol.FeatureCtx{}, csm, identityset.Address(1), 1, true, false)
		require.NoError(err)
	})
}

func TestChangeCandidate(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	t.Run("Self-Staked as owned", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, true, true, nil, 0},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
		}
		sm, p, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		r.NoError(setupAccount(sm, identityset.Address(1), 10000))
		// csm, err := NewCandidateStateManager(sm)
		nonce := uint64(1)
		act := action.NewChangeCandidate("test1", buckets[0].Index, nil)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(1),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_ErrInvalidBucketType, recipt.Status)
	})
	t.Run("Self-Staked with endorsement", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, true, true, nil, 0},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
		}
		sm, p, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		r.NoError(setupAccount(sm, identityset.Address(1), 10000))
		esm := NewEndorsementStateManager(sm)
		err := esm.Put(buckets[0].Index, &Endorsement{ExpireHeight: endorsementNotExpireHeight})
		r.NoError(err)
		nonce := uint64(1)
		act := action.NewChangeCandidate("test1", buckets[0].Index, nil)
		r.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(1),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_ErrInvalidBucketType, recipt.Status)
	})
	t.Run("Endorsement is withdrawing", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, true, false, nil, 0},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
		}
		sm, p, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		r.NoError(setupAccount(sm, identityset.Address(1), 10000))
		esm := NewEndorsementStateManager(sm)
		err := esm.Put(buckets[0].Index, &Endorsement{ExpireHeight: 10})
		r.NoError(err)
		nonce := uint64(1)
		act := action.NewChangeCandidate("test1", buckets[0].Index, nil)
		r.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(1),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_ErrInvalidBucketType, recipt.Status)
	})
	t.Run("Endorsement is expired", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, true, false, nil, 1},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
		}
		sm, p, buckets, _ := initTestStateWithHeight(t, ctrl, bucketCfgs, candCfgs, 2)
		r.NoError(setupAccount(sm, identityset.Address(1), 10000))
		nonce := uint64(1)
		act := action.NewChangeCandidate("test1", buckets[0].Index, nil)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(1),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_Success, recipt.Status)
	})
}

func TestUnstake(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	t.Run("Self-Staked as owned", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, false, true, nil, 0},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
		}
		sm, p, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		r.NoError(setupAccount(sm, identityset.Address(1), 10000))
		nonce := uint64(1)
		act := action.NewUnstake(buckets[0].Index, nil)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(1),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_Success, recipt.Status)
	})
	t.Run("Self-Staked with endorsement", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, false, true, nil, 0},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
		}
		sm, p, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		r.NoError(setupAccount(sm, identityset.Address(1), 10000))
		esm := NewEndorsementStateManager(sm)
		err := esm.Put(buckets[0].Index, &Endorsement{ExpireHeight: endorsementNotExpireHeight})
		r.NoError(err)
		nonce := uint64(1)
		act := action.NewUnstake(buckets[0].Index, nil)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(1),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_ErrInvalidBucketType, recipt.Status)
	})
	t.Run("Endorsement is withdrawing", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, false, false, nil, 0},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
		}
		sm, p, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		r.NoError(setupAccount(sm, identityset.Address(1), 10000))
		esm := NewEndorsementStateManager(sm)
		err := esm.Put(buckets[0].Index, &Endorsement{ExpireHeight: 10})
		r.NoError(err)
		nonce := uint64(1)
		act := action.NewUnstake(buckets[0].Index, nil)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(1),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_ErrInvalidBucketType, recipt.Status)
	})
	t.Run("Endorsement is expired", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, false, false, nil, 1},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "test1"},
		}
		sm, p, buckets, _ := initTestStateWithHeight(t, ctrl, bucketCfgs, candCfgs, 1)
		r.NoError(setupAccount(sm, identityset.Address(1), 10000))
		nonce := uint64(1)
		act := action.NewUnstake(buckets[0].Index, nil)
		intrinsic, err := act.IntrinsicGas()
		r.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(10000).
			SetGasPrice(testGasPrice).SetAction(act).Build()
		ctx := context.Background()
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       identityset.Address(1),
			GasPrice:     testGasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    2,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
			Height: 1,
		}})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		recipt, err := p.Handle(ctx, elp, sm)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_Success, recipt.Status)
	})
}

func initCreateStake(t *testing.T, sm protocol.StateManager, callerAddr address.Address, initBalance int64, gasPrice *big.Int, gasLimit uint64, nonce uint64, blkHeight uint64, blkTimestamp time.Time, blkGasLimit uint64, p *Protocol, candidate *Candidate, amount string, autoStake bool) (context.Context, *big.Int) {
	require := require.New(t)
	require.NoError(setupAccount(sm, callerAddr, initBalance))
	a, err := action.NewCreateStake(candidate.Name, amount, 1, autoStake, nil)
	require.NoError(err)
	intrinsic, err := a.IntrinsicGas()
	require.NoError(err)
	elp := builder.SetNonce(nonce).SetGasLimit(gasLimit).
		SetGasPrice(gasPrice).SetAction(a).Build()
	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller:       callerAddr,
		GasPrice:     gasPrice,
		IntrinsicGas: intrinsic,
		Nonce:        nonce,
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    blkHeight,
		BlockTimeStamp: blkTimestamp,
		GasLimit:       blkGasLimit,
	})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{
		Height: blkHeight - 1,
	}})
	ctx = genesis.WithGenesisContext(ctx, genesis.TestDefault())
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	cc, ok := v.(*ViewData)
	require.True(ok)
	require.NoError(sm.WriteView(_protocolID, cc))
	_, err = p.Handle(ctx, elp, sm)
	require.NoError(err)
	cost, err := elp.Cost()
	require.NoError(err)
	return ctx, cost
}

func initAll(t *testing.T, ctrl *gomock.Controller) (protocol.StateManager, *Protocol, *Candidate, *Candidate) {
	require := require.New(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	require.NoError(err)
	g := genesis.TestDefault()

	// create protocol
	p, err := NewProtocol(HelperCtx{
		DepositGas:    depositGas,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                  g.Staking,
		PersistStakingPatchBlock: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight: g.Staking.VoteWeightCalConsts,
		},
	}, nil, nil, nil)
	require.NoError(err)

	// set up candidate
	candidate := testCandidates[0].d.Clone()
	candidate.Votes = big.NewInt(0)
	require.NoError(csm.putCandidate(candidate))
	candidate2 := testCandidates[1].d.Clone()
	candidate2.Votes = big.NewInt(0)
	require.NoError(csm.putCandidate(candidate2))
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	cc, ok := v.(*ViewData)
	require.True(ok)
	require.NoError(sm.WriteView(_protocolID, cc))
	return sm, p, candidate, candidate2
}

func setupAccount(sm protocol.StateManager, addr address.Address, balance int64) error {
	if balance < 0 {
		return errors.New("balance cannot be negative")
	}
	account, err := accountutil.LoadOrCreateAccount(sm, addr, state.LegacyNonceAccountTypeOption())
	if err != nil {
		return err
	}
	if err := account.SubBalance(account.Balance); err != nil {
		return err
	}
	if err := account.AddBalance(unit.ConvertIotxToRau(balance)); err != nil {
		return err
	}
	return accountutil.StoreAccount(sm, addr, account)
}

func depositGas(ctx context.Context, sm protocol.StateManager, gasFee *big.Int, opts ...protocol.DepositOption) ([]*action.TransactionLog, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	// Subtract balance from caller
	acc, err := accountutil.LoadAccount(sm, actionCtx.Caller)
	if err != nil {
		return nil, err
	}
	// TODO: replace with SubBalance, and then change `Balance` to a function
	acc.Balance.Sub(acc.Balance, gasFee)
	return nil, accountutil.StoreAccount(sm, actionCtx.Caller, acc)
}

func getBlockInterval(height uint64) time.Duration {
	return 5 * time.Second
}

func newconsignment(r *require.Assertions, bucketIdx, nonce uint64, senderPrivate, recipient, consignTpye, reclaim string, wrongSig bool) []byte {
	msg := action.ConsignMsgEther{
		BucketIdx: bucketIdx,
		Nonce:     nonce,
		Recipient: recipient,
		Reclaim:   reclaim,
	}
	b, err := json.Marshal(msg)
	r.NoError(err)
	h, err := action.MsgHash(consignTpye, b)
	if err != nil {
		h = []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	}
	sk, err := crypto.HexStringToPrivateKey(senderPrivate)
	r.NoError(err)
	sig, err := sk.Sign(h)
	r.NoError(err)
	c := &action.ConsignJSON{
		Type: consignTpye,
		Msg:  string(b),
		Sig:  hex.EncodeToString(sig),
	}
	if wrongSig {
		c.Sig = "123456"
	}
	b, err = json.Marshal(c)
	r.NoError(err)
	return b
}

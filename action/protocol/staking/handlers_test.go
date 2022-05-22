// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil/testdb"
)

const (
	_reclaim = "This is to certify I am transferring the ownership of said bucket to said recipient on IoTeX blockchain"
)

// Delete should only be used by test
func (m *CandidateCenter) deleteForTestOnly(owner address.Address) {
	if owner == nil {
		return
	}

	if _, hit := m.base.getByOwner(owner.String()); hit {
		m.base.delete(owner)
		if !m.change.containsOwner(owner) {
			m.size--
			return
		}
	}

	if m.change.containsOwner(owner) {
		m.change.delete(owner)
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
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	require.NoError(err)

	// create protocol
	p, err := NewProtocol(depositGas, genesis.Default.Staking, nil, genesis.Default.GreenlandBlockHeight)
	require.NoError(err)

	// set up candidate
	candidate := testCandidates[0].d.Clone()
	require.NoError(csm.putCandidate(candidate))
	candidateName := candidate.Name
	candidateAddr := candidate.Owner
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
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
		gasPrice    *big.Int
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			ErrInvalidCanName,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		{
			101,
			candidateName,
			"10000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			ErrInvalidAmount,
			iotextypes.ReceiptStatus_Failure,
		},
		{
			101,
			candidateName,
			"100000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
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
			GasPrice:     test.gasPrice,
			IntrinsicGas: test.gasLimit,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    test.blkHeight,
			BlockTimeStamp: test.blkTimestamp,
			GasLimit:       test.blkGasLimit,
		})
		act, err := action.NewCreateStake(test.nonce, test.candName, test.amount, test.duration, test.autoStake,
			nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		err = p.Validate(ctx, act, sm)
		if test.err != nil {
			require.EqualError(test.err, errors.Cause(err).Error())
			continue
		}
		ctx = protocol.WithFeatureCtx(ctx)
		r, err := p.Handle(ctx, act, sm)
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
			bucketIndices, _, err := csr.(*candSR).CandBucketIndices(candidateAddr)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.(*candSR).VoterBucketIndices(stakerAddr)
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
			csm, err := NewCandidateStateManager(sm, false)
			require.NoError(err)
			candidate = csm.GetByOwner(candidateAddr)
			require.NotNil(candidate)
			require.LessOrEqual(test.amount, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, stakerAddr)
			require.NoError(err)
			actCost, err := act.Cost()
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
			uint64(10),
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
			uint64(10),
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
			uint64(10),
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
			uint64(10),
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
			ErrInvalidAmount,
			iotextypes.ReceiptStatus_Failure,
		},
		// invalid candidate name
		{
			1201000,
			identityset.Address(27),
			uint64(10),
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
			ErrInvalidCanName,
			iotextypes.ReceiptStatus_Failure,
		},
		// success for the following test
		{
			1201000,
			identityset.Address(27),
			uint64(10),
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
			uint64(10),
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
		{1201000,
			identityset.Address(27),
			uint64(10),
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
			big.NewInt(unit.Qev),
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
		// act.OwnerAddress() is not nil,new candidate, collide with existing operator
		{1201000,
			identityset.Address(27),
			uint64(10),
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
			big.NewInt(unit.Qev),
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
		// act.OwnerAddress() is nil,existing owner, but selfstake is not 0
		{1201000,
			identityset.Address(30),
			uint64(10),
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
			big.NewInt(unit.Qev),
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateAlreadyExist,
		},
		// act.OwnerAddress() is nil,existing candidate, collide with existing name,this case cannot happen,b/c if ownerExist,it will return ReceiptStatus_ErrCandidateAlreadyExist
		// act.OwnerAddress() is nil,existing candidate, collide with existing operator,this case cannot happen,b/c if ownerExist,it will return ReceiptStatus_ErrCandidateAlreadyExist
		// act.OwnerAddress() is nil,new candidate, collide with existing name
		{1201000,
			identityset.Address(21),
			uint64(10),
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
			big.NewInt(unit.Qev),
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
		// act.OwnerAddress() is nil,new candidate, collide with existing operator
		{1201000,
			identityset.Address(21),
			uint64(10),
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
			big.NewInt(unit.Qev),
			false,
			nil,
			iotextypes.ReceiptStatus_ErrCandidateConflict,
		},
	}

	for _, test := range tests {
		if test.newProtocol {
			sm, p, _, _ = initAll(t, ctrl)
			csr = newCandidateStateReader(sm)
		}
		require.NoError(setupAccount(sm, test.caller, test.initBalance))
		act, err := action.NewCandidateRegister(test.nonce, test.name, test.operatorAddrStr, test.rewardAddrStr, test.ownerAddrStr, test.amountStr, test.duration, test.autoStake, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		IntrinsicGas, _ := act.IntrinsicGas()
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
		ctx = genesis.WithGenesisContext(ctx, genesis.Default)
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		require.Equal(test.err, errors.Cause(p.Validate(ctx, act, sm)))
		if test.err != nil {
			continue
		}
		r, err := p.Handle(ctx, act, sm)
		require.NoError(err)
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// check the special create bucket and candidate register log
			tLogs := r.TransactionLogs()
			require.Equal(2, len(tLogs))
			cLog := tLogs[0]
			require.Equal(test.caller.String(), cLog.Sender)
			require.Equal(address.StakingBucketPoolAddr, cLog.Recipient)
			require.Equal(test.amountStr, cLog.Amount.String())

			cLog = tLogs[1]
			require.Equal(test.caller.String(), cLog.Sender)
			require.Equal(address.RewardingPoolAddr, cLog.Recipient)
			require.Equal(p.config.RegistrationConsts.Fee.String(), cLog.Amount.String())

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
			csm, err := NewCandidateStateManager(sm, false)
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
			actCost, err := act.Cost()
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
			uint64(10),
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
			uint64(10),
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
			uint64(10),
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
			uint64(10),
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
			uint64(10),
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
			ErrInvalidCanName,
			iotextypes.ReceiptStatus_Failure,
		},
		// success,update name, operator and reward address
		{
			1201000,
			identityset.Address(27),
			uint64(10),
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
		act, err := action.NewCandidateRegister(test.nonce, test.name, test.operatorAddrStr, test.rewardAddrStr, test.ownerAddrStr, test.amountStr, test.duration, test.autoStake, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		intrinsic, _ := act.IntrinsicGas()
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
		ctx = genesis.WithGenesisContext(ctx, genesis.Default)
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		_, err = p.Handle(ctx, act, sm)
		require.NoError(err)

		cu, err := action.NewCandidateUpdate(test.nonce, test.updateName, test.updateOperator, test.updateReward, test.gasLimit, test.gasPrice)
		require.NoError(err)
		intrinsic, _ = cu.IntrinsicGas()
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
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
		require.Equal(test.err, errors.Cause(p.Validate(ctx, cu, sm)))
		if test.err != nil {
			continue
		}
		r, err := p.Handle(ctx, cu, sm)
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
			csm, err := NewCandidateStateManager(sm, false)
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
			actCost, err := act.Cost()
			require.NoError(err)
			cuCost, err := cu.Cost()
			require.NoError(err)
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, cuCost).Add(total, p.config.RegistrationConsts.Fee))
			require.Equal(test.nonce+1, caller.PendingNonce())
		}
	}
}

func TestProtocol_HandleUnstake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, p, candidate, candidate2 := initAll(t, ctrl)
	csr := newCandidateStateReader(sm)
	initCreateStake(t, sm, identityset.Address(2), 100, big.NewInt(unit.Qev), 10000, 1, 1, time.Now(), 10000, p, candidate2, "100000000000000000000", false)

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
		gasPrice         = big.NewInt(unit.Qev)
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
		ctx, createCost = initCreateStake(t, sm, test.caller, test.initBalance, big.NewInt(unit.Qev), gasLimit, nonce, blkHeight, test.blkTimestamp, gasLimit, p, candidate, test.amount, test.autoStake)
		act, err := action.NewUnstake(nonce, test.index,
			nil, gasLimit, gasPrice)
		require.NoError(err)
		if test.blkTimestamp != test.ctxTimestamp {
			blkCtx := protocol.MustGetBlockCtx(ctx)
			blkCtx.BlockTimeStamp = test.ctxTimestamp
			ctx = protocol.WithBlockCtx(ctx, blkCtx)
		}
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		var r *action.Receipt
		if test.clear {
			csm, err := NewCandidateStateManager(sm, false)
			require.NoError(err)
			sc, ok := csm.(*candSM)
			require.True(ok)
			sc.candCenter.deleteForTestOnly(test.caller)
			require.False(csm.ContainsOwner(test.caller))
			r, err = p.handle(ctx, act, csm)
			require.Equal(test.err, errors.Cause(err))
		} else {
			r, err = p.Handle(ctx, act, sm)
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
			bucketIndices, _, err := csr.(*candSR).CandBucketIndices(candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.(*candSR).VoterBucketIndices(candidate.Owner)
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
			csm, err := NewCandidateStateManager(sm, false)
			require.NoError(err)
			candidate = csm.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal(test.afterUnstake, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			actCost, err := act.Cost()
			require.NoError(err)
			require.Equal(nonce+1, caller.PendingNonce())
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, createCost))
		}
	}

	// verify bucket unstaked
	vb, err := csr.getBucket(0)
	require.NoError(err)
	require.True(vb.isUnstaked())

	unstake, err := action.NewUnstake(nonce+1, 0, nil, gasLimit, gasPrice)
	require.NoError(err)
	changeCand, err := action.NewChangeCandidate(nonce+1, candidate2.Name, 0, nil, gasLimit, gasPrice)
	require.NoError(err)
	deposit, err := action.NewDepositToStake(nonce+1, 0, "10000", nil, gasLimit, gasPrice)
	require.NoError(err)
	restake, err := action.NewRestake(nonce+1, 0, 0, false, nil, gasLimit, gasPrice)
	require.NoError(err)

	unstakedBucketTests := []struct {
		act       action.Action
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

	for _, v := range unstakedBucketTests {
		greenland := genesis.Default
		if v.greenland {
			blkCtx := protocol.MustGetBlockCtx(ctx)
			greenland.GreenlandBlockHeight = blkCtx.BlockHeight
		}
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
		gasPrice := big.NewInt(unit.Qev)
		gasLimit := uint64(10000)
		ctx, createCost := initCreateStake(t, sm, candidate.Owner, test.initBalance, big.NewInt(unit.Qev), gasLimit, 1, 1, test.blkTimestamp, gasLimit, p, candidate, test.amount, false)
		var actCost *big.Int
		if test.unstake {
			act, err := action.NewUnstake(1, 0, nil, gasLimit, big.NewInt(unit.Qev))
			require.NoError(err)
			intrinsic, err := act.IntrinsicGas()
			require.NoError(err)
			actCost, err = act.Cost()
			require.NoError(err)
			ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
				Caller:       caller,
				GasPrice:     gasPrice,
				IntrinsicGas: intrinsic,
				Nonce:        2,
			})
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
				BlockHeight:    1,
				BlockTimeStamp: time.Now().Add(time.Duration(1) * 24 * time.Hour),
				GasLimit:       1000000,
			})

			_, err = p.Handle(ctx, act, sm)
			require.NoError(err)
			require.NoError(p.Commit(ctx, sm))
			require.Equal(0, sm.Snapshot())
		}

		withdraw, err := action.NewWithdrawStake(1, test.withdrawIndex,
			nil, gasLimit, gasPrice)
		require.NoError(err)
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
		r, err := p.Handle(ctx, withdraw, sm)
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
			_, _, err := csr.(*candSR).CandBucketIndices(candidate.Owner)
			require.Error(err)
			_, _, err = csr.(*candSR).VoterBucketIndices(candidate.Owner)
			require.Error(err)

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, caller)
			require.NoError(err)
			withdrawCost, err := withdraw.Cost()
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
		gasPrice      *big.Int
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			ErrInvalidCanName,
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
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			ErrInvalidCanName,
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
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			ErrInvalidCanName,
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
			10000,
			1,
			genesis.Default.HawaiiBlockHeight,
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
		ctx, _ := initCreateStake(t, sm, candidate2.Owner, 101, big.NewInt(unit.Qev), 10000, 1, 1, time.Now(), 10000, p, candidate2, "100000000000000000000", false)
		// candidate vote self,index 1
		_, createCost := initCreateStake(t, sm, candidate.Owner, test.initBalance, big.NewInt(unit.Qev), test.gasLimit, test.nonce, test.blkHeight, test.blkTimestamp, test.blkGasLimit, p, candidate, test.amount, false)

		act, err := action.NewChangeCandidate(test.nonce, test.candidateName, test.index, nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    test.blkHeight,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		var r *action.Receipt
		if test.clear {
			csm, err := NewCandidateStateManager(sm, false)
			require.NoError(err)
			sc, ok := csm.(*candSM)
			require.True(ok)
			cc := sc.candCenter.GetBySelfStakingIndex(test.index)
			sc.candCenter.deleteForTestOnly(cc.Owner)
			require.False(csm.ContainsOwner(cc.Owner))
			r, err = p.handle(ctx, act, csm)
			require.Equal(test.err, errors.Cause(err))
		} else {
			require.Equal(test.err, errors.Cause(p.Validate(ctx, act, sm)))
			if test.err != nil {
				continue
			}
			r, err = p.Handle(ctx, act, sm)
			require.NoError(err)
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, _, err := csr.(*candSR).CandBucketIndices(identityset.Address(1))
			require.NoError(err)
			require.Equal(2, len(*bucketIndices))
			bucketIndices, _, err = csr.(*candSR).VoterBucketIndices(identityset.Address(1))
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
			csm, err := NewCandidateStateManager(sm, false)
			require.NoError(err)
			candidate = csm.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal(test.afterChange, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			actCost, err := act.Cost()
			require.NoError(err)
			require.Equal(test.nonce+1, caller.PendingNonce())
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, createCost))
		}
	}
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
		gasPrice *big.Int
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
			big.NewInt(unit.Qev),
			1000000000,
			1,
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
			10000,
			1,
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
			big.NewInt(unit.Qev),
			10000,
			1,
			genesis.Default.HawaiiBlockHeight,
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
		ctx, createCost := initCreateStake(t, sm, candidate2.Owner, test.initBalance, big.NewInt(unit.Qev), 10000, 1, 1, time.Now(), 10000, p, candidate2, test.amount, false)
		if test.init {
			initCreateStake(t, sm, candi.Owner, test.initBalance, test.gasPrice, test.gasLimit, test.nonce, test.blkHeight, test.blkTimestamp, test.blkGasLimit, p, candi, test.amount, false)
		} else {
			require.NoError(setupAccount(sm, identityset.Address(1), 1))
		}

		act, err := action.NewTransferStake(test.nonce, test.to.String(), test.index, nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)

		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    test.blkHeight,
			BlockTimeStamp: time.Now(),
			GasLimit:       10000000,
		})
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		r, err := p.Handle(ctx, act, sm)
		require.Equal(test.err, errors.Cause(err))
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, _, err := csr.(*candSR).CandBucketIndices(candidate2.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.(*candSR).VoterBucketIndices(test.to)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := csr.getBucket(indices[0])
			require.NoError(err)
			require.Equal(candidate2.Owner, bucket.Candidate)
			require.Equal(test.to.String(), bucket.Owner.String())
			require.Equal(test.amount, bucket.StakedAmount.String())

			// test candidate
			candidate, _, err := csr.getCandidate(candi.Owner)
			require.NoError(err)
			require.Equal(test.afterTransfer, candidate.Votes.Uint64())
			csm, err := NewCandidateStateManager(sm, false)
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
			actCost, err := act.Cost()
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
		gasPrice := big.NewInt(unit.Qev)
		gasLimit := uint64(10000)
		ctx, _ := initCreateStake(t, sm, identityset.Address(32), initBalance, gasPrice, gasLimit, 1, test.blkHeight, time.Now(), gasLimit, p, cand2, stakeAmount, false)
		initCreateStake(t, sm, identityset.Address(31), initBalance, gasPrice, gasLimit, 1, test.blkHeight, time.Now(), gasLimit, p, cand1, stakeAmount, false)

		// transfer to test.to through consignment
		var consign []byte
		if !test.nilPayload {
			consign = newconsignment(require, int(test.sigIndex), int(test.sigNonce), test.bucketOwner, test.to.String(), test.consignType, test.reclaim, test.wrongSig)
		}

		act, err := action.NewTransferStake(1, caller.String(), 0, consign, gasLimit, gasPrice)
		require.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)

		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       caller,
			GasPrice:     gasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        1,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    test.blkHeight,
			BlockTimeStamp: time.Now(),
			GasLimit:       gasLimit,
		})
		r, err := p.Handle(ctx, act, sm)
		require.NoError(err)
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, _, err := csr.(*candSR).CandBucketIndices(cand2.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.(*candSR).VoterBucketIndices(test.to)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := csr.getBucket(indices[0])
			require.NoError(err)
			require.Equal(cand2.Owner, bucket.Candidate)
			require.Equal(test.to.String(), bucket.Owner.String())
			require.Equal(stakeAmount, bucket.StakedAmount.String())

			// test candidate
			candidate, _, err := csr.getCandidate(cand1.Owner)
			require.NoError(err)
			require.LessOrEqual(uint64(0), candidate.Votes.Uint64())
			csm, err := NewCandidateStateManager(sm, false)
			require.NoError(err)
			candidate = csm.GetByOwner(cand1.Owner)
			require.NotNil(candidate)
			require.LessOrEqual(uint64(0), candidate.Votes.Uint64())
			require.Equal(cand1.Name, candidate.Name)
			require.Equal(cand1.Operator, candidate.Operator)
			require.Equal(cand1.Reward, candidate.Reward)
			require.Equal(cand1.Owner, candidate.Owner)
			require.LessOrEqual(uint64(0), candidate.Votes.Uint64())
			require.LessOrEqual(uint64(0), candidate.SelfStake.Uint64())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, caller)
			require.NoError(err)
			actCost, err := act.Cost()
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
		gasPrice *big.Int
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
		ctx, createCost := initCreateStake(t, sm, candidate2.Owner, test.initBalance, big.NewInt(unit.Qev), 10000, 1, 1, time.Now(), 10000, p, candidate2, test.amount, test.autoStake)

		if test.newAccount {
			require.NoError(setupAccount(sm, test.caller, test.initBalance))
		} else {
			candidate = candidate2
		}

		act, err := action.NewRestake(test.nonce, test.index, test.duration, test.autoStake, nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
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
			csm, err := NewCandidateStateManager(sm, false)
			require.NoError(err)
			sc, ok := csm.(*candSM)
			require.True(ok)
			sc.candCenter.deleteForTestOnly(test.caller)
			require.False(csm.ContainsOwner(test.caller))
			ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
			r, err = p.handle(ctx, act, csm)
			require.Equal(test.err, errors.Cause(err))
		} else {
			r, err = p.Handle(ctx, act, sm)
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}

		if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, _, err := csr.(*candSR).CandBucketIndices(candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.(*candSR).VoterBucketIndices(candidate.Owner)
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
			csm, err := NewCandidateStateManager(sm, false)
			require.NoError(err)
			candidate = csm.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal(test.afterRestake, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			actCost, err := act.Cost()
			require.NoError(err)
			require.Equal(test.nonce+1, caller.PendingNonce())
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
		gasPrice *big.Int
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
			big.NewInt(unit.Qev),
			10000,
			1,
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
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
			big.NewInt(unit.Qev),
			10000,
			1,
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
			big.NewInt(unit.Qev),
			10000,
			1,
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
		ctx, createCost := initCreateStake(t, sm, candidate.Owner, test.initBalance, big.NewInt(unit.Qev), 10000, 1, 1, time.Now(), 10000, p, candidate, test.amount, test.autoStake)

		if test.newAccount {
			require.NoError(setupAccount(sm, test.caller, test.initBalance))
		}

		act, err := action.NewDepositToStake(test.nonce, test.index, test.amount, nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
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
			csm, err := NewCandidateStateManager(sm, false)
			require.NoError(err)
			sc, ok := csm.(*candSM)
			require.True(ok)
			sc.candCenter.deleteForTestOnly(test.caller)
			require.False(csm.ContainsOwner(test.caller))
			ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
			r, err = p.handle(ctx, act, csm)
			require.Equal(test.err, errors.Cause(err))
		} else {
			r, err = p.Handle(ctx, act, sm)
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
			bucketIndices, _, err := csr.(*candSR).CandBucketIndices(candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, _, err = csr.(*candSR).VoterBucketIndices(candidate.Owner)
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
			csm, err := NewCandidateStateManager(sm, false)
			require.NoError(err)
			candidate = csm.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal(test.afterDeposit, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, test.caller)
			require.NoError(err)
			actCost, err := act.Cost()
			require.NoError(err)
			require.Equal(test.nonce+1, caller.PendingNonce())
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, createCost))
		}
	}
}

func initCreateStake(t *testing.T, sm protocol.StateManager, callerAddr address.Address, initBalance int64, gasPrice *big.Int, gasLimit uint64, nonce uint64, blkHeight uint64, blkTimestamp time.Time, blkGasLimit uint64, p *Protocol, candidate *Candidate, amount string, autoStake bool) (context.Context, *big.Int) {
	require := require.New(t)
	require.NoError(setupAccount(sm, callerAddr, initBalance))
	a, err := action.NewCreateStake(nonce, candidate.Name, amount, 1, autoStake,
		nil, gasLimit, gasPrice)
	require.NoError(err)
	intrinsic, err := a.IntrinsicGas()
	require.NoError(err)
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
	ctx = genesis.WithGenesisContext(ctx, genesis.Default)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	cc, ok := v.(*ViewData)
	require.True(ok)
	require.NoError(sm.WriteView(_protocolID, cc))
	_, err = p.Handle(ctx, a, sm)
	require.NoError(err)
	cost, err := a.Cost()
	require.NoError(err)
	return ctx, cost
}

func initAll(t *testing.T, ctrl *gomock.Controller) (protocol.StateManager, *Protocol, *Candidate, *Candidate) {
	require := require.New(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	require.NoError(err)

	// create protocol
	p, err := NewProtocol(depositGas, genesis.Default.Staking, nil, genesis.Default.GreenlandBlockHeight)
	require.NoError(err)

	// set up candidate
	candidate := testCandidates[0].d.Clone()
	candidate.Votes = big.NewInt(0)
	require.NoError(csm.putCandidate(candidate))
	candidate2 := testCandidates[1].d.Clone()
	candidate2.Votes = big.NewInt(0)
	require.NoError(csm.putCandidate(candidate2))
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
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
	account, err := accountutil.LoadOrCreateAccount(sm, addr)
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

func depositGas(ctx context.Context, sm protocol.StateManager, gasFee *big.Int) (*action.TransactionLog, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	// Subtract balance from caller
	acc, err := accountutil.LoadAccount(sm, actionCtx.Caller)
	if err != nil {
		return nil, err
	}
	// TODO: replace with SubBalance, and then change `Balance` to a function
	acc.Balance = big.NewInt(0).Sub(acc.Balance, gasFee)
	return nil, accountutil.StoreAccount(sm, actionCtx.Caller, acc)
}

func newconsignment(r *require.Assertions, bucketIdx, nonce int, senderPrivate, recipient, consignTpye, reclaim string, wrongSig bool) []byte {
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

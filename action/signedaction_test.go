// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

var (
	// addr1   = identityset.Address(27).String()
	_priKey1 = identityset.PrivateKey(27)
	_addr2   = identityset.Address(28).String()
	// Create two candidates
	_cand1PriKey  = identityset.PrivateKey(11)
	_cand1Addr    = identityset.Address(12).String()
	_cand2PriKey  = identityset.PrivateKey(13)
	_cand2Addr    = identityset.Address(14).String()
	_selfStake, _ = new(big.Int).SetString("1200000000000000000000000", 10)
)

var (
	_gasPrice = big.NewInt(10)
	_gasLimit = uint64(1000000)
)

const (
	_candidate1Name = "candidate1"
	_candidate2Name = "candidate2"
)

func TestSignedTransfer(t *testing.T) {
	require := require.New(t)
	selp, err := SignedTransfer(_addr2, _priKey1, uint64(1), big.NewInt(2), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)

	tsf := selp.Action().(*Transfer)
	require.Equal(_addr2, tsf.Recipient())
	require.Equal(uint64(1), tsf.Nonce())
	require.Equal(big.NewInt(2), tsf.Amount())
	require.Equal([]byte{}, tsf.Payload())
	require.Equal(uint64(100000), tsf.GasLimit())
	require.Equal(big.NewInt(10), tsf.GasPrice())
	require.NotNil(selp.Signature())
}

func TestSignedExecution(t *testing.T) {
	require := require.New(t)
	selp, err := SignedExecution(EmptyAddress, _priKey1, uint64(1), big.NewInt(0), uint64(100000), big.NewInt(10), []byte{})
	require.NoError(err)

	exec := selp.Action().(*Execution)
	require.Equal(EmptyAddress, exec.Contract())
	require.Equal(uint64(1), exec.Nonce())
	require.Equal(big.NewInt(0), exec.Amount())
	require.Equal(uint64(100000), exec.GasLimit())
	require.Equal(big.NewInt(10), exec.GasPrice())
	require.Equal([]byte{}, exec.Data())
	require.NotNil(selp.Signature())
}

func TestSignedCandidateRegister(t *testing.T) {
	require := require.New(t)
	selp, err := SignedCandidateRegister(1, _candidate1Name, _cand1Addr, _cand1Addr, _cand1Addr, big.NewInt(10).String(), 91, true, []byte{}, _gasLimit, _gasPrice, _cand1PriKey)
	require.NoError(err)

	cand := selp.Action().(*CandidateRegister)
	require.Equal(uint64(1), cand.Nonce())
	require.Equal(_gasLimit, cand.GasLimit())
	require.Equal(_gasPrice, cand.GasPrice())
	require.Equal(_candidate1Name, cand.name)
	require.Equal(identityset.Address(12), cand.operatorAddress)
	require.Equal(identityset.Address(12), cand.rewardAddress)
	require.Equal(big.NewInt(10), cand.Amount())
	require.Equal(uint32(91), cand.duration)
	require.Equal(true, cand.autoStake)
	require.Equal([]byte{}, cand.payload)
	require.NotNil(selp.Signature())
}

func TestSignedCandidateUpdate(t *testing.T) {
	require := require.New(t)
	selp, err := SignedCandidateUpdate(1, _candidate1Name, _cand1Addr, _cand1Addr, _gasLimit, _gasPrice, _cand1PriKey)
	require.NoError(err)

	canu := selp.Action().(*CandidateUpdate)
	require.Equal(uint64(1), canu.Nonce())
	require.Equal(_gasLimit, canu.GasLimit())
	require.Equal(_gasPrice, canu.GasPrice())
	require.NotNil(selp.Signature())
}

func TestSignedCreateStake(t *testing.T) {
	require := require.New(t)
	selp, err := SignedCreateStake(1, _candidate1Name, big.NewInt(10).String(), 91, true, []byte{}, _gasLimit, _gasPrice, _cand1PriKey)
	require.NoError(err)

	exec := selp.Action().(*CreateStake)
	require.Equal(_candidate1Name, exec.candName)
	require.Equal(uint64(1), exec.Nonce())
	require.Equal(big.NewInt(10), exec.Amount())
	require.Equal(_gasLimit, exec.GasLimit())
	require.Equal(_gasPrice, exec.GasPrice())
	require.Equal([]byte{}, exec.payload)
	require.Equal(true, exec.autoStake)
	require.NotNil(selp.Signature())
}

func TestNewUnstakeSignedReclaimStake(t *testing.T) {
	require := require.New(t)
	selp, err := SignedReclaimStake(false, 1, 2, []byte{}, _gasLimit, _gasPrice, _priKey1)
	require.NoError(err)

	exec := selp.Action().(*Unstake)
	require.Equal(uint64(1), exec.Nonce())
	require.Equal(uint64(2), exec.bucketIndex)
	require.Equal(_gasLimit, exec.GasLimit())
	require.Equal(_gasPrice, exec.GasPrice())
	require.Equal([]byte{}, exec.payload)
	require.NotNil(selp.Signature())
}

func TestNewWithdrawStakeSignedReclaimStake(t *testing.T) {
	require := require.New(t)
	selp, err := SignedReclaimStake(true, 1, 2, []byte{}, _gasLimit, _gasPrice, _priKey1)
	require.NoError(err)

	exec := selp.Action().(*WithdrawStake)
	require.Equal(uint64(1), exec.Nonce())
	require.Equal(uint64(2), exec.bucketIndex)
	require.Equal(_gasLimit, exec.GasLimit())
	require.Equal(_gasPrice, exec.GasPrice())
	require.Equal([]byte{}, exec.payload)
	require.NotNil(selp.Signature())
}

func TestSignedChangeCandidate(t *testing.T) {
	require := require.New(t)
	selp, err := SignedChangeCandidate(1, _candidate1Name, 2, []byte{}, _gasLimit, _gasPrice, _priKey1)
	require.NoError(err)

	exec := selp.Action().(*ChangeCandidate)
	require.Equal(_candidate1Name, exec.candidateName)
	require.Equal(uint64(1), exec.Nonce())
	require.Equal(uint64(2), exec.bucketIndex)
	require.Equal(_gasLimit, exec.GasLimit())
	require.Equal(_gasPrice, exec.GasPrice())
	require.Equal([]byte{}, exec.payload)
	require.NotNil(selp.Signature())
}

func TestSignedTransferStake(t *testing.T) {
	require := require.New(t)
	selp, err := SignedTransferStake(1, _cand1Addr, 2, []byte{}, _gasLimit, _gasPrice, _priKey1)
	require.NoError(err)

	exec := selp.Action().(*TransferStake)
	require.Equal(identityset.Address(12), exec.voterAddress)
	require.Equal(uint64(1), exec.Nonce())
	require.Equal(uint64(2), exec.bucketIndex)
	require.Equal(_gasLimit, exec.GasLimit())
	require.Equal(_gasPrice, exec.GasPrice())
	require.Equal([]byte{}, exec.payload)
	require.NotNil(selp.Signature())
}

func TestSignedDepositToStake(t *testing.T) {
	require := require.New(t)
	selp, err := SignedDepositToStake(1, 2, big.NewInt(10).String(), []byte{}, _gasLimit, _gasPrice, _priKey1)
	require.NoError(err)

	exec := selp.Action().(*DepositToStake)
	require.Equal(uint64(2), exec.bucketIndex)
	require.Equal(uint64(1), exec.Nonce())
	require.Equal(big.NewInt(10), exec.Amount())
	require.Equal(_gasLimit, exec.GasLimit())
	require.Equal(_gasPrice, exec.GasPrice())
	require.Equal([]byte{}, exec.payload)
	require.NotNil(selp.Signature())
}

func TestSignedRestake(t *testing.T) {
	require := require.New(t)
	selp, err := SignedRestake(1, 2, 91, true, []byte{}, _gasLimit, _gasPrice, _priKey1)
	require.NoError(err)

	exec := selp.Action().(*Restake)
	require.Equal(uint64(1), exec.Nonce())
	require.Equal(uint32(91), exec.duration)
	require.Equal(true, exec.autoStake)
	require.Equal(uint64(2), exec.bucketIndex)
	require.Equal(_gasLimit, exec.GasLimit())
	require.Equal(_gasPrice, exec.GasPrice())
	require.Equal([]byte{}, exec.payload)
	require.NotNil(selp.Signature())
}

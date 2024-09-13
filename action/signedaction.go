// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"

	"github.com/pkg/errors"
)

// vars
var (
	ValidSig, _ = hex.DecodeString("15e73ad521ec9e06600c59e49b127c9dee114ad64fb2fcbe5e0d9f4c8d2b766e73d708cca1dc050dd27b20f2ee607f30428bf035f45d4da8ec2fb04a90c2c30901")
)

type SignedActionOption func(*EnvelopeBuilder)

func WithChainID(chainID uint32) SignedActionOption {
	return func(b *EnvelopeBuilder) {
		b.SetChainID(chainID)
	}
}

// SignedTransfer return a signed transfer
func SignedTransfer(recipientAddr string, senderPriKey crypto.PrivateKey, nonce uint64, amount *big.Int, payload []byte, gasLimit uint64, gasPrice *big.Int, options ...SignedActionOption) (*SealedEnvelope, error) {
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(NewTransfer(amount, recipientAddr, payload))
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, senderPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign transfer %v", elp)
	}
	return selp, nil
}

// SignedExecution return a signed execution
func SignedExecution(contractAddr string, executorPriKey crypto.PrivateKey, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, options ...SignedActionOption) (*SealedEnvelope, error) {
	execution := NewExecution(contractAddr, amount, data)
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(execution)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, executorPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign execution %v", elp)
	}
	return selp, nil
}

// SignedCandidateRegister returns a signed candidate register
func SignedCandidateRegister(
	nonce uint64,
	name, operatorAddrStr, rewardAddrStr, ownerAddrStr, amountStr string,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	registererPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	cr, err := NewCandidateRegister(name, operatorAddrStr, rewardAddrStr, ownerAddrStr, amountStr,
		duration, autoStake, payload)
	if err != nil {
		return nil, err
	}
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cr)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, registererPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign candidate register %v", elp)
	}
	return selp, nil
}

// SignedCandidateUpdate returns a signed candidate update
func SignedCandidateUpdate(
	nonce uint64,
	name, operatorAddrStr, rewardAddrStr string,
	gasLimit uint64,
	gasPrice *big.Int,
	registererPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	cu, err := NewCandidateUpdate(name, operatorAddrStr, rewardAddrStr)
	if err != nil {
		return nil, err
	}
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cu)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, registererPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign candidate update %v", elp)
	}
	return selp, nil
}

// SignedCandidateActivate returns a signed candidate selfstake
func SignedCandidateActivate(
	nonce uint64,
	bucketIndex uint64,
	gasLimit uint64,
	gasPrice *big.Int,
	registererPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	cu := NewCandidateActivate(bucketIndex)
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cu)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, registererPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign candidate selfstake %v", elp)
	}
	return selp, nil
}

// SignedCandidateEndorsementLegacy returns a signed candidate endorsement
func SignedCandidateEndorsementLegacy(
	nonce uint64,
	bucketIndex uint64,
	endorse bool,
	gasLimit uint64,
	gasPrice *big.Int,
	registererPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	cu := NewCandidateEndorsementLegacy(bucketIndex, endorse)
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cu)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, registererPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign candidate endorsement %v", elp)
	}
	return selp, nil
}

// SignedCandidateEndorsement returns a signed candidate endorsement
func SignedCandidateEndorsement(
	nonce uint64,
	bucketIndex uint64,
	op CandidateEndorsementOp,
	gasLimit uint64,
	gasPrice *big.Int,
	registererPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	cu, err := NewCandidateEndorsement(bucketIndex, op)
	if err != nil {
		return nil, err
	}
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cu)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, registererPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign candidate endorsement %v", elp)
	}
	return selp, nil
}

// SignedCreateStake returns a signed create stake
func SignedCreateStake(nonce uint64,
	candidateName, amount string,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	stakerPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	cs, err := NewCreateStake(candidateName, amount, duration, autoStake, payload)
	if err != nil {
		return nil, err
	}
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cs)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, stakerPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign create stake %v", elp)
	}
	return selp, nil
}

// SignedReclaimStake returns a signed unstake or withdraw stake
func SignedReclaimStake(
	withdraw bool,
	nonce uint64,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	reclaimerPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit)
	for _, opt := range options {
		opt(bd)
	}
	var elp Envelope
	// unstake
	if !withdraw {
		us := NewUnstake(bucketIndex, payload)
		elp = bd.SetAction(us).Build()
	} else {
		w := NewWithdrawStake(bucketIndex, payload)
		elp = bd.SetAction(w).Build()
	}
	selp, err := Sign(elp, reclaimerPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign reclaim stake %v", elp)
	}
	return selp, nil
}

// SignedChangeCandidate returns a signed change candidate
func SignedChangeCandidate(
	nonce uint64,
	candName string,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	stakerPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	cc := NewChangeCandidate(candName, bucketIndex, payload)
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cc)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, stakerPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign change candidate %v", elp)
	}
	return selp, nil
}

// SignedTransferStake returns a signed transfer stake
func SignedTransferStake(
	nonce uint64,
	voterAddress string,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	stakerPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	ts, err := NewTransferStake(voterAddress, bucketIndex, payload)
	if err != nil {
		return nil, err
	}
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(ts)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, stakerPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign transfer stake %v", elp)
	}
	return selp, nil
}

// SignedDepositToStake returns a signed deposit to stake
func SignedDepositToStake(
	nonce uint64,
	index uint64,
	amount string,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	depositorPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	ds, err := NewDepositToStake(index, amount, payload)
	if err != nil {
		return nil, err
	}
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(ds)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, depositorPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign deposit to stake %v", elp)
	}
	return selp, nil
}

// SignedRestake returns a signed restake
func SignedRestake(
	nonce uint64,
	index uint64,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	restakerPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	rs := NewRestake(index, duration, autoStake, payload)
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(rs)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, restakerPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign restake %v", elp)
	}
	return selp, nil
}

// SignedCandidateTransferOwnership returns a signed candidate transfer ownership
func SignedCandidateTransferOwnership(
	nonce uint64,
	ownerAddrStr string,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	senderPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	cto, err := NewCandidateTransferOwnership(ownerAddrStr, payload)
	if err != nil {
		return nil, err
	}
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cto)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, senderPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign candidate transfer ownership %v", elp)
	}
	return selp, nil
}

func SignedMigrateStake(
	nonce uint64,
	bucketIndex uint64,
	gasLimit uint64,
	gasPrice *big.Int,
	senderPriKey crypto.PrivateKey,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	cto := NewMigrateStake(bucketIndex)
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cto)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, senderPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign candidate transfer ownership %v", elp)
	}
	return selp, nil
}

func SignedClaimRewardLegacy(
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
	senderPriKey crypto.PrivateKey,
	amount *big.Int,
	payload []byte,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	return SignedClaimReward(nonce, gasLimit, gasPrice, senderPriKey, amount, payload, nil, options...)
}

func SignedClaimReward(
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
	senderPriKey crypto.PrivateKey,
	amount *big.Int,
	payload []byte,
	address address.Address,
	options ...SignedActionOption,
) (*SealedEnvelope, error) {
	act := NewClaimFromRewardingFund(amount, address, payload)
	bd := &EnvelopeBuilder{}
	bd = bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(act)
	for _, opt := range options {
		opt(bd)
	}
	elp := bd.Build()
	selp, err := Sign(elp, senderPriKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign candidate transfer ownership %v", elp)
	}
	return selp, nil
}

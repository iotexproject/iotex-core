// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"encoding/hex"
	"math/big"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
)

// vars
var (
	ValidSig, _ = hex.DecodeString("15e73ad521ec9e06600c59e49b127c9dee114ad64fb2fcbe5e0d9f4c8d2b766e73d708cca1dc050dd27b20f2ee607f30428bf035f45d4da8ec2fb04a90c2c30901")
)

// SignedTransfer return a signed transfer
func SignedTransfer(recipientAddr string, senderPriKey crypto.PrivateKey, nonce uint64, amount *big.Int, payload []byte, gasLimit uint64, gasPrice *big.Int) (action.SealedEnvelope, error) {
	transfer, err := action.NewTransfer(nonce, amount, recipientAddr, payload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(transfer).Build()
	selp, err := action.Sign(elp, senderPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign transfer %v", elp)
	}
	return selp, nil
}

// SignedExecution return a signed execution
func SignedExecution(contractAddr string, executorPriKey crypto.PrivateKey, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (action.SealedEnvelope, error) {
	execution, err := action.NewExecution(contractAddr, nonce, amount, gasLimit, gasPrice, data)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(execution).Build()
	selp, err := action.Sign(elp, executorPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign execution %v", elp)
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
) (action.SealedEnvelope, error) {
	cr, err := action.NewCandidateRegister(nonce, name, operatorAddrStr, rewardAddrStr, ownerAddrStr, amountStr,
		duration, autoStake, payload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cr).Build()
	selp, err := action.Sign(elp, registererPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign candidate register %v", elp)
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
) (action.SealedEnvelope, error) {
	cu, err := action.NewCandidateUpdate(nonce, name, operatorAddrStr, rewardAddrStr, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cu).Build()
	selp, err := action.Sign(elp, registererPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign candidate update %v", elp)
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
) (action.SealedEnvelope, error) {
	cs, err := action.NewCreateStake(nonce, candidateName, amount, duration, autoStake,
		payload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cs).Build()
	selp, err := action.Sign(elp, stakerPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign create stake %v", elp)
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
) (action.SealedEnvelope, error) {
	bd := &action.EnvelopeBuilder{}
	eb := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit)
	var elp action.Envelope
	// unstake
	if !withdraw {
		us, err := action.NewUnstake(nonce, bucketIndex, payload, gasLimit, gasPrice)
		if err != nil {
			return action.SealedEnvelope{}, err
		}
		elp = eb.SetAction(us).Build()
	} else {
		w, err := action.NewWithdrawStake(nonce, bucketIndex, payload, gasLimit, gasPrice)
		if err != nil {
			return action.SealedEnvelope{}, err
		}
		elp = eb.SetAction(w).Build()
	}
	selp, err := action.Sign(elp, reclaimerPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign reclaim stake %v", elp)
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
) (action.SealedEnvelope, error) {
	cc, err := action.NewChangeCandidate(nonce, candName, bucketIndex, payload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cc).Build()
	selp, err := action.Sign(elp, stakerPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign change candidate %v", elp)
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
) (action.SealedEnvelope, error) {
	ts, err := action.NewTransferStake(nonce, voterAddress, bucketIndex, payload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(ts).Build()
	selp, err := action.Sign(elp, stakerPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign transfer stake %v", elp)
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
) (action.SealedEnvelope, error) {
	ds, err := action.NewDepositToStake(nonce, index, amount, payload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(ds).Build()
	selp, err := action.Sign(elp, depositorPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign deposit to stake %v", elp)
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
) (action.SealedEnvelope, error) {
	rs, err := action.NewRestake(nonce, index, duration, autoStake, payload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(rs).Build()
	selp, err := action.Sign(elp, restakerPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign restake %v", elp)
	}
	return selp, nil
}

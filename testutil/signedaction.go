// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"math/big"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
)

// SignedTransfer return a signed transfer
func SignedTransfer(recipientAddr string, senderPriKey crypto.PrivateKey, nonce uint64, amount *big.Int, payload []byte, gasLimit uint64, gasPrice *big.Int) (action.SealedEnvelope, error) {
	transfer, err := action.NewTransfer(amount, recipientAddr, payload)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp, err := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(transfer).
		Build()
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	selp, err := action.Sign(elp, senderPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign transfer %v", elp)
	}
	return selp, nil
}

// SignedExecution return a signed execution
func SignedExecution(contractAddr string, executorPriKey crypto.PrivateKey, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (action.SealedEnvelope, error) {
	execution, err := action.NewExecution(contractAddr, amount, data)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp, err := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(execution).
		Build()
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrap(err, "failed to sign execution")
	}
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
	cr, err := action.NewCandidateRegister(name, operatorAddrStr, rewardAddrStr, ownerAddrStr, amountStr,
		duration, autoStake, payload)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp, err := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cr).Build()
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrap(err, "failed to sign candidate register")
	}
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
	cu, err := action.NewCandidateUpdate(name, operatorAddrStr, rewardAddrStr)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp, err := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cu).
		Build()
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrap(err, "failed to sign candidate update")
	}
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
	cs, err := action.NewCreateStake(candidateName, amount, duration, autoStake, payload)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp, err := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cs).Build()
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrap(err, "failed to sign create stake")
	}
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
	var err error
	// unstake
	if !withdraw {
		us, err := action.NewUnstake(bucketIndex, payload)
		if err != nil {
			return action.SealedEnvelope{}, err
		}
		elp, err = eb.SetAction(us).Build()
	} else {
		w, err := action.NewWithdrawStake(bucketIndex, payload)
		if err != nil {
			return action.SealedEnvelope{}, err
		}
		elp, err = eb.SetAction(w).Build()
	}
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrap(err, "failed to sign reclaim stake")
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
	cc, err := action.NewChangeCandidate(candName, bucketIndex, payload)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp, err := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(cc).
		Build()
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrap(err, "failed to sign change candidate")
	}
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
	ts, err := action.NewTransferStake(voterAddress, bucketIndex, payload)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp, err := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(ts).
		Build()
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrap(err, "failed to sign transfer stake")
	}
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
	ds, err := action.NewDepositToStake(index, amount, payload)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp, err := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(ds).
		Build()
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrap(err, "failed to sign deposit to stake")
	}

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
	rs, err := action.NewRestake(index, duration, autoStake, payload)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp, err := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(rs).
		Build()
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrap(err, "failed to sign restake")
	}
	selp, err := action.Sign(elp, restakerPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign restake %v", elp)
	}
	return selp, nil
}

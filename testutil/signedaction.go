// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
)

// SignedTransfer return a signed transfer
func SignedTransfer(sender *iotxaddress.Address, recipient *iotxaddress.Address, nonce uint64, amount *big.Int, payload []byte, gasLimit uint64, gasPrice *big.Int) (action.SealedEnvelope, error) {
	transfer, err := action.NewTransfer(nonce, amount, sender.RawAddress, recipient.RawAddress, payload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetDestinationAddress(recipient.RawAddress).
		SetGasLimit(gasLimit).
		SetAction(transfer).Build()
	selp, err := action.Sign(elp, sender.RawAddress, sender.PrivateKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign transfer %v", elp)
	}
	return selp, nil
}

// SignedVote return a signed vote
func SignedVote(voter *iotxaddress.Address, votee *iotxaddress.Address, nonce uint64, gasLimit uint64, gasPrice *big.Int) (action.SealedEnvelope, error) {
	vote, err := action.NewVote(nonce, voter.RawAddress, votee.RawAddress, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetDestinationAddress(votee.RawAddress).
		SetGasLimit(gasLimit).
		SetAction(vote).Build()
	selp, err := action.Sign(elp, voter.RawAddress, voter.PrivateKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign vote %v", elp)
	}
	return selp, nil
}

// SignedExecution return a signed execution
func SignedExecution(executor *iotxaddress.Address, contractAddr string, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (action.SealedEnvelope, error) {
	execution, err := action.NewExecution(executor.RawAddress, contractAddr, nonce, amount, gasLimit, gasPrice, data)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetDestinationAddress(contractAddr).
		SetGasLimit(gasLimit).
		SetAction(execution).Build()
	selp, err := action.Sign(elp, executor.RawAddress, executor.PrivateKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign execution %v", elp)
	}
	return selp, nil
}

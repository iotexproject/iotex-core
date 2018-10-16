// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"math/big"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
)

// SignedTransfer return a signed transfer
func SignedTransfer(sender *iotxaddress.Address, recipient *iotxaddress.Address, nonce uint64, amount *big.Int, payload []byte, gasLimit uint64, gasPrice *big.Int) (*action.Transfer, error) {
	transfer, err := action.NewTransfer(nonce, amount, sender.RawAddress, recipient.RawAddress, payload, gasLimit, gasPrice)
	if err != nil {
		return nil, err
	}
	if err := action.Sign(transfer, sender.PrivateKey); err != nil {
		return nil, err
	}
	return transfer, nil
}

// SignedVote return a signed vote
func SignedVote(voter *iotxaddress.Address, votee *iotxaddress.Address, nonce uint64, gasLimit uint64, gasPrice *big.Int) (*action.Vote, error) {
	vote, err := action.NewVote(nonce, voter.RawAddress, votee.RawAddress, gasLimit, gasPrice)
	if err != nil {
		return nil, err
	}
	if err := action.Sign(vote, voter.PrivateKey); err != nil {
		return nil, err
	}
	return vote, nil
}

// SignedExecution return a signed execution
func SignedExecution(executor *iotxaddress.Address, contractAddr string, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (*action.Execution, error) {
	execution, err := action.NewExecution(executor.RawAddress, contractAddr, nonce, amount, gasLimit, gasPrice, data)
	if err != nil {
		return nil, err
	}
	if err := action.Sign(execution, executor.PrivateKey); err != nil {
		return nil, err
	}
	return execution, nil
}

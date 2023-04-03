// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import "github.com/iotexproject/iotex-address/address"

// ProposerRegister is the action to register an execution node to produce block proposal
type ProposerRegister struct {
	AbstractAction

	operatorAddress address.Address
	rewardAddress   address.Address
	ownerAddress    address.Address
	duration        uint32
	autoStake       bool
	payload         []byte
}

func (pr *ProposerRegister) OwnerAddress() address.Address { return pr.ownerAddress }

func (pr *ProposerRegister) OperatorAddress() address.Address { return pr.operatorAddress }

func (pr *ProposerRegister) Duration() uint32 { return pr.duration }

func (pr *ProposerRegister) AutoStake() bool { return pr.autoStake }

func (pr *ProposerRegister) RewardAddress() address.Address { return pr.rewardAddress }

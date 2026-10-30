// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const SetVoterRewardOptInBaseIntrinsicGas = uint64(10000)

var (
	setVoterRewardOptInMethod abi.Method
	_voterRewardOptInSetEvent abi.Event
	_                         EthCompatibleAction = (*SetVoterRewardOptIn)(nil)
)

// SetVoterRewardOptIn enables protocol-native voter reward distribution for a candidate.
type SetVoterRewardOptIn struct {
	stake_common
}

func init() {
	var ok bool
	setVoterRewardOptInMethod, ok = NativeStakingContractABI().Methods["setVoterRewardOptIn"]
	if !ok {
		panic("fail to load the setVoterRewardOptIn method")
	}
	_voterRewardOptInSetEvent, ok = NativeStakingContractABI().Events["VoterRewardOptInSet"]
	if !ok {
		panic("fail to load the VoterRewardOptInSet event")
	}
}

func VoterRewardOptInSetEvent(candidateIdentifier []byte) Topics {
	topics := make(Topics, 2)
	topics[0] = hash.Hash256(_voterRewardOptInSetEvent.ID)
	topics[1] = hash.BytesToHash256(candidateIdentifier)
	return topics
}

func NewSetVoterRewardOptIn() *SetVoterRewardOptIn { return &SetVoterRewardOptIn{} }

func (s *SetVoterRewardOptIn) IntrinsicGas() (uint64, error) {
	return SetVoterRewardOptInBaseIntrinsicGas, nil
}

func (*SetVoterRewardOptIn) SanityCheck() error { return nil }

func (s *SetVoterRewardOptIn) FillAction(act *iotextypes.ActionCore) {
	act.Action = &iotextypes.ActionCore_SetVoterRewardOptIn{SetVoterRewardOptIn: s.Proto()}
}

func (s *SetVoterRewardOptIn) Proto() *iotextypes.SetVoterRewardOptIn {
	return &iotextypes.SetVoterRewardOptIn{}
}

func (s *SetVoterRewardOptIn) LoadProto(pbAct *iotextypes.SetVoterRewardOptIn) error {
	if pbAct == nil {
		return ErrNilProto
	}
	return nil
}

func (s *SetVoterRewardOptIn) EthData() ([]byte, error) {
	return append([]byte(nil), setVoterRewardOptInMethod.ID...), nil
}

func NewSetVoterRewardOptInFromABIBinary(data []byte) (*SetVoterRewardOptIn, error) {
	if len(data) != 4 || !bytes.Equal(setVoterRewardOptInMethod.ID, data) {
		return nil, errDecodeFailure
	}
	return NewSetVoterRewardOptIn(), nil
}

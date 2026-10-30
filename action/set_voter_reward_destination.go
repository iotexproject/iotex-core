// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

const (
	SetVoterRewardDestinationBaseIntrinsicGas = uint64(10000)
	_setVoterRewardDestinationABI             = `[
		{
			"inputs":[{"internalType":"address","name":"recipient","type":"address"}],
			"name":"setVoterRewardDestination",
			"outputs":[],
			"stateMutability":"nonpayable",
			"type":"function"
		},
		{
			"anonymous":false,
			"inputs":[
				{"indexed":true,"internalType":"address","name":"voter","type":"address"},
				{"indexed":false,"internalType":"address","name":"oldRecipient","type":"address"},
				{"indexed":false,"internalType":"address","name":"newRecipient","type":"address"}
			],
			"name":"VoterRewardDestinationSet",
			"type":"event"
		}
	]`
)

var (
	setVoterRewardDestinationMethod abi.Method
	voterRewardDestinationSetEvent  abi.Event
	_                               EthCompatibleAction = (*SetVoterRewardDestination)(nil)
)

// SetVoterRewardDestination changes the account used for direct IIP-59 voter payouts.
// An empty or zero recipient clears the override.
type SetVoterRewardDestination struct {
	reward_common
	recipient []byte
}

func init() {
	contractABI, err := abi.JSON(strings.NewReader(_setVoterRewardDestinationABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	setVoterRewardDestinationMethod, ok = contractABI.Methods["setVoterRewardDestination"]
	if !ok {
		panic("fail to load the setVoterRewardDestination method")
	}
	voterRewardDestinationSetEvent, ok = contractABI.Events["VoterRewardDestinationSet"]
	if !ok {
		panic("fail to load the VoterRewardDestinationSet event")
	}
}

// PackVoterRewardDestinationSetEvent encodes the EVM-compatible configuration event.
func PackVoterRewardDestinationSetEvent(
	voter address.Address,
	oldRecipient address.Address,
	newRecipient address.Address,
) (Topics, []byte, error) {
	if voter == nil || oldRecipient == nil || newRecipient == nil {
		return nil, nil, errors.New("nil voter reward destination event address")
	}
	data, err := voterRewardDestinationSetEvent.Inputs.NonIndexed().Pack(
		common.BytesToAddress(oldRecipient.Bytes()),
		common.BytesToAddress(newRecipient.Bytes()),
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to pack VoterRewardDestinationSet event")
	}
	topics := make(Topics, 2)
	topics[0] = hash.Hash256(voterRewardDestinationSetEvent.ID)
	topics[1] = hash.BytesToHash256(voter.Bytes())
	return topics, data, nil
}

func NewSetVoterRewardDestination(recipient []byte) *SetVoterRewardDestination {
	return &SetVoterRewardDestination{recipient: append([]byte(nil), recipient...)}
}

func (s *SetVoterRewardDestination) Recipient() []byte {
	return append([]byte(nil), s.recipient...)
}

func (s *SetVoterRewardDestination) IntrinsicGas() (uint64, error) {
	return SetVoterRewardDestinationBaseIntrinsicGas, nil
}

func (s *SetVoterRewardDestination) SanityCheck() error {
	if len(s.recipient) == 0 {
		return nil
	}
	if len(s.recipient) != common.AddressLength {
		return errors.Wrapf(ErrAddress, "voter reward destination must be %d bytes", common.AddressLength)
	}
	return nil
}

func (s *SetVoterRewardDestination) FillAction(act *iotextypes.ActionCore) {
	act.Action = &iotextypes.ActionCore_SetVoterRewardDestination{SetVoterRewardDestination: s.Proto()}
}

func (s *SetVoterRewardDestination) Proto() *iotextypes.SetVoterRewardDestination {
	return &iotextypes.SetVoterRewardDestination{Recipient: s.Recipient()}
}

func (s *SetVoterRewardDestination) LoadProto(pbAct *iotextypes.SetVoterRewardDestination) error {
	if pbAct == nil {
		return ErrNilProto
	}
	s.recipient = append([]byte(nil), pbAct.GetRecipient()...)
	return nil
}

func (s *SetVoterRewardDestination) EthData() ([]byte, error) {
	if err := s.SanityCheck(); err != nil {
		return nil, err
	}
	recipient := common.Address{}
	if len(s.recipient) > 0 {
		recipient = common.BytesToAddress(s.recipient)
	}
	data, err := setVoterRewardDestinationMethod.Inputs.Pack(recipient)
	if err != nil {
		return nil, err
	}
	return append(setVoterRewardDestinationMethod.ID, data...), nil
}

func NewSetVoterRewardDestinationFromABIBinary(data []byte) (*SetVoterRewardDestination, error) {
	if len(data) <= 4 || !bytes.Equal(setVoterRewardDestinationMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	params := map[string]any{}
	if err := setVoterRewardDestinationMethod.Inputs.UnpackIntoMap(params, data[4:]); err != nil {
		return nil, err
	}
	recipient, ok := params["recipient"].(common.Address)
	if !ok {
		return nil, errDecodeFailure
	}
	if recipient == (common.Address{}) {
		return NewSetVoterRewardDestination(nil), nil
	}
	return NewSetVoterRewardDestination(recipient.Bytes()), nil
}

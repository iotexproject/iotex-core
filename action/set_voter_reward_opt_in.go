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
	"github.com/pkg/errors"
)

const (
	// SetVoterRewardOptInBaseIntrinsicGas is the base intrinsic gas for SetVoterRewardOptIn.
	SetVoterRewardOptInBaseIntrinsicGas = uint64(10000)
)

var (
	setVoterRewardOptInMethod abi.Method
	_voterRewardOptInSetEvent abi.Event
	_                         EthCompatibleAction = (*SetVoterRewardOptIn)(nil)
)

// SetVoterRewardOptIn toggles IIP-59 §2.2 opt-in flag on the target
// candidate. The delegate owner sends this to enable (or disable)
// on-chain voter reward auto-split for their delegate. The flag is
// consumed by rewarding one epoch later, when it's frozen into the
// poll snapshot at PutPollResult.
type SetVoterRewardOptIn struct {
	stake_common
	candidateIdentifier []byte
	optIn               bool
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

// PackVoterRewardOptInSetEvent packs the VoterRewardOptInSet event for the
// staking handler's receipt log emission.
func PackVoterRewardOptInSetEvent(candidateIdentifier []byte, optIn bool) (Topics, []byte, error) {
	data, err := _voterRewardOptInSetEvent.Inputs.NonIndexed().Pack(optIn)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to pack VoterRewardOptInSet event")
	}
	topics := make(Topics, 2)
	topics[0] = hash.Hash256(_voterRewardOptInSetEvent.ID)
	topics[1] = hash.BytesToHash256(candidateIdentifier)
	return topics, data, nil
}

// NewSetVoterRewardOptIn returns a SetVoterRewardOptIn action.
func NewSetVoterRewardOptIn(candidateIdentifier []byte, optIn bool) *SetVoterRewardOptIn {
	return &SetVoterRewardOptIn{
		candidateIdentifier: candidateIdentifier,
		optIn:               optIn,
	}
}

// CandidateIdentifier returns the identifier of the target candidate.
func (s *SetVoterRewardOptIn) CandidateIdentifier() []byte {
	return append([]byte(nil), s.candidateIdentifier...)
}

// OptIn returns the target opt-in flag value.
func (s *SetVoterRewardOptIn) OptIn() bool { return s.optIn }

// IntrinsicGas returns the intrinsic gas of a SetVoterRewardOptIn.
func (s *SetVoterRewardOptIn) IntrinsicGas() (uint64, error) {
	return SetVoterRewardOptInBaseIntrinsicGas, nil
}

// SanityCheck validates the action envelope.
func (s *SetVoterRewardOptIn) SanityCheck() error {
	if len(s.candidateIdentifier) == 0 {
		return ErrAddress
	}
	return nil
}

// FillAction embeds this action into an ActionCore envelope.
func (s *SetVoterRewardOptIn) FillAction(act *iotextypes.ActionCore) {
	act.Action = &iotextypes.ActionCore_SetVoterRewardOptIn{SetVoterRewardOptIn: s.Proto()}
}

// Proto converts SetVoterRewardOptIn to protobuf.
func (s *SetVoterRewardOptIn) Proto() *iotextypes.SetVoterRewardOptIn {
	return &iotextypes.SetVoterRewardOptIn{
		CandidateIdentifier: append([]byte(nil), s.candidateIdentifier...),
		OptIn:               s.optIn,
	}
}

// LoadProto converts a protobuf into SetVoterRewardOptIn.
func (s *SetVoterRewardOptIn) LoadProto(pbAct *iotextypes.SetVoterRewardOptIn) error {
	if pbAct == nil {
		return ErrNilProto
	}
	s.candidateIdentifier = append([]byte(nil), pbAct.GetCandidateIdentifier()...)
	s.optIn = pbAct.GetOptIn()
	return nil
}

// EthData returns the ABI-encoded data for converting to eth tx.
func (s *SetVoterRewardOptIn) EthData() ([]byte, error) {
	data, err := setVoterRewardOptInMethod.Inputs.Pack(s.candidateIdentifier, s.optIn)
	if err != nil {
		return nil, err
	}
	return append(setVoterRewardOptInMethod.ID, data...), nil
}

// NewSetVoterRewardOptInFromABIBinary parses the smart contract input and creates an action.
func NewSetVoterRewardOptInFromABIBinary(data []byte) (*SetVoterRewardOptIn, error) {
	var (
		paramsMap = map[string]any{}
		s         SetVoterRewardOptIn
	)
	if len(data) <= 4 || !bytes.Equal(setVoterRewardOptInMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := setVoterRewardOptInMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	id, ok := paramsMap["candidateIdentifier"].([]byte)
	if !ok {
		return nil, errDecodeFailure
	}
	optIn, ok := paramsMap["optIn"].(bool)
	if !ok {
		return nil, errDecodeFailure
	}
	s.candidateIdentifier = id
	s.optIn = optIn
	return &s, nil
}

// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	// CandidateUpdateBaseIntrinsicGas represents the base intrinsic gas for CandidateUpdate
	CandidateUpdateBaseIntrinsicGas = uint64(10000)
)

var (
	// _candidateUpdateMethod is the interface of the abi encoding of stake action
	_candidateUpdateMethod        abi.Method
	_candidateUpdateWithBLSMethod abi.Method
	_candidateUpdateWithBLSEvent  abi.Event
	_                             EthCompatibleAction = (*CandidateUpdate)(nil)
)

// CandidateUpdate is the action to update a candidate
type CandidateUpdate struct {
	stake_common
	name            string
	operatorAddress address.Address
	rewardAddress   address.Address
	pubKey          []byte
}

// CandidateUpdateOption defines the method to customize CandidateUpdate
type CandidateUpdateOption func(*CandidateUpdate) error

// WithCandidateUpdatePubKey sets the BLS public key for CandidateUpdate
func WithCandidateUpdatePubKey(pubKey []byte) CandidateUpdateOption {
	return func(cu *CandidateUpdate) error {
		_, err := crypto.BLS12381PublicKeyFromBytes(pubKey)
		if err != nil {
			return errors.Wrap(err, "failed to parse BLS public key")
		}
		cu.pubKey = make([]byte, len(pubKey))
		copy(cu.pubKey, pubKey)
		return nil
	}
}

func init() {
	var ok bool
	_candidateUpdateMethod, ok = NativeStakingContractABI().Methods["candidateUpdate"]
	if !ok {
		panic("fail to load the method")
	}
	_candidateUpdateWithBLSMethod, ok = NativeStakingContractABI().Methods["candidateUpdateWithBLS"]
	if !ok {
		panic("fail to load the method")
	}
	_candidateUpdateWithBLSEvent, ok = NativeStakingContractABI().Events["CandidateUpdated"]
	if !ok {
		panic("fail to load the event")
	}
}

// NewCandidateUpdate creates a CandidateUpdate instance
func NewCandidateUpdate(name, operatorAddrStr, rewardAddrStr string, opts ...CandidateUpdateOption) (*CandidateUpdate, error) {
	cu := &CandidateUpdate{
		name: name,
	}

	var err error
	if len(operatorAddrStr) > 0 {
		cu.operatorAddress, err = address.FromString(operatorAddrStr)
		if err != nil {
			return nil, err
		}
	}

	if len(rewardAddrStr) > 0 {
		cu.rewardAddress, err = address.FromString(rewardAddrStr)
		if err != nil {
			return nil, err
		}
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(cu); err != nil {
			return nil, errors.Wrap(err, "failed to apply option")
		}
	}
	return cu, nil
}

// Name returns candidate name to update
func (cu *CandidateUpdate) Name() string { return cu.name }

// OperatorAddress returns candidate operatorAddress to update
func (cu *CandidateUpdate) OperatorAddress() address.Address { return cu.operatorAddress }

// RewardAddress returns candidate rewardAddress to update
func (cu *CandidateUpdate) RewardAddress() address.Address { return cu.rewardAddress }

// PubKey returns candidate public key to update
func (cu *CandidateUpdate) PubKey() []byte {
	return cu.pubKey
}

// WithBLS returns true if the candidate update action is with BLS public key
func (cu *CandidateUpdate) WithBLS() bool {
	return len(cu.pubKey) > 0
}

// Serialize returns a raw byte stream of the CandidateUpdate struct
func (cu *CandidateUpdate) Serialize() []byte {
	return byteutil.Must(proto.Marshal(cu.Proto()))
}

func (act *CandidateUpdate) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_CandidateUpdate{CandidateUpdate: act.Proto()}
}

// Proto converts to protobuf CandidateUpdate Action
func (cu *CandidateUpdate) Proto() *iotextypes.CandidateBasicInfo {
	act := &iotextypes.CandidateBasicInfo{
		Name: cu.name,
	}

	if cu.operatorAddress != nil {
		act.OperatorAddress = cu.operatorAddress.String()
	}

	if cu.rewardAddress != nil {
		act.RewardAddress = cu.rewardAddress.String()
	}

	if len(cu.pubKey) > 0 {
		act.PubKey = make([]byte, len(cu.pubKey))
		copy(act.PubKey, cu.pubKey)
	}
	return act
}

// LoadProto converts a protobuf's Action to CandidateUpdate
func (cu *CandidateUpdate) LoadProto(pbAct *iotextypes.CandidateBasicInfo) error {
	if pbAct == nil {
		return ErrNilProto
	}

	cu.name = pbAct.GetName()

	if len(pbAct.GetOperatorAddress()) > 0 {
		operatorAddr, err := address.FromString(pbAct.GetOperatorAddress())
		if err != nil {
			return err
		}
		cu.operatorAddress = operatorAddr
	}

	if len(pbAct.GetRewardAddress()) > 0 {
		rewardAddr, err := address.FromString(pbAct.GetRewardAddress())
		if err != nil {
			return err
		}
		cu.rewardAddress = rewardAddr
	}
	if len(pbAct.GetPubKey()) > 0 {
		cu.pubKey = make([]byte, len(pbAct.GetPubKey()))
		copy(cu.pubKey, pbAct.GetPubKey())
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a CandidateUpdate
func (cu *CandidateUpdate) IntrinsicGas() (uint64, error) {
	return CandidateUpdateBaseIntrinsicGas, nil
}

// SanityCheck validates the variables in the action
func (cu *CandidateUpdate) SanityCheck() error {
	if !IsValidCandidateName(cu.Name()) {
		return ErrInvalidCanName
	}
	return nil
}

// EthData returns the ABI-encoded data for converting to eth tx
func (cu *CandidateUpdate) EthData() ([]byte, error) {
	if cu.operatorAddress == nil {
		return nil, ErrAddress
	}
	if cu.rewardAddress == nil {
		return nil, ErrAddress
	}
	switch {
	case cu.WithBLS():
		data, err := _candidateUpdateWithBLSMethod.Inputs.Pack(cu.name,
			common.BytesToAddress(cu.operatorAddress.Bytes()),
			common.BytesToAddress(cu.rewardAddress.Bytes()), cu.pubKey)
		if err != nil {
			return nil, err
		}
		return append(_candidateUpdateWithBLSMethod.ID, data...), nil
	default:
		data, err := _candidateUpdateMethod.Inputs.Pack(cu.name,
			common.BytesToAddress(cu.operatorAddress.Bytes()),
			common.BytesToAddress(cu.rewardAddress.Bytes()))
		if err != nil {
			return nil, err
		}
		return append(_candidateUpdateMethod.ID, data...), nil
	}
}

// NewCandidateUpdateFromABIBinary decodes data into CandidateUpdate action
func NewCandidateUpdateFromABIBinary(data []byte) (*CandidateUpdate, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		err       error
		cu        CandidateUpdate
		method    *abi.Method
		withBLS   bool
	)
	// sanity check
	if len(data) <= 4 {
		return nil, errDecodeFailure
	}
	switch {
	case bytes.Equal(_candidateUpdateMethod.ID, data[:4]):
		method = &_candidateUpdateMethod
	case bytes.Equal(_candidateUpdateWithBLSMethod.ID, data[:4]):
		method = &_candidateUpdateWithBLSMethod
		withBLS = true
	default:
		return nil, errors.Wrapf(errDecodeFailure, "unknown method prefix %x", data[:4])
	}
	if err := method.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if cu.name, ok = paramsMap["name"].(string); !ok {
		return nil, errDecodeFailure
	}
	if cu.operatorAddress, err = ethAddrToNativeAddr(paramsMap["operatorAddress"]); err != nil {
		return nil, err
	}
	if cu.rewardAddress, err = ethAddrToNativeAddr(paramsMap["rewardAddress"]); err != nil {
		return nil, err
	}
	if withBLS {
		if cu.pubKey, ok = paramsMap["pubKey"].([]byte); !ok {
			return nil, errors.Wrapf(errDecodeFailure, "pubKey is not []byte: %v", paramsMap["pubKey"])
		}
		if len(cu.pubKey) == 0 {
			return nil, errors.Wrapf(errDecodeFailure, "empty BLS public key")
		}
		_, err := crypto.BLS12381PublicKeyFromBytes(cu.pubKey)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse BLS public key")
		}
	}
	return &cu, nil
}

func PackCandidateUpdatedEvent(
	candidate,
	operatorAddress,
	ownerAddress address.Address,
	name string,
	rewardAddress address.Address,
	blsPublicKey []byte,
) (Topics, []byte, error) {
	data, err := _candidateUpdateWithBLSEvent.Inputs.NonIndexed().Pack(
		rewardAddress.Bytes(),
		name,
		common.BytesToAddress(operatorAddress.Bytes()),
		blsPublicKey,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to pack CandidateUpdateWithBLS event data")
	}
	topics := make(Topics, 3)
	topics[0] = hash.Hash256(_candidateUpdateWithBLSEvent.ID)
	topics[1] = hash.Hash256(candidate.Bytes())
	topics[2] = hash.Hash256(ownerAddress.Bytes())
	return topics, data, nil
}

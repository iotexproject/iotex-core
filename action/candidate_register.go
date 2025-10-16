// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	// CandidateRegisterPayloadGas represents the CandidateRegister payload gas per uint
	CandidateRegisterPayloadGas = uint64(100)
	// CandidateRegisterBaseIntrinsicGas represents the base intrinsic gas for CandidateRegister
	CandidateRegisterBaseIntrinsicGas = uint64(10000)
)

var (
	// _candidateRegisterInterface is the interface of the abi encoding of stake action
	_candidateRegisterMethod        abi.Method
	_candidateRegisterWithBLSMethod abi.Method
	_candidateRegisteredEvent       abi.Event
	_stakedEvent                    abi.Event
	_candidateActivatedEvent        abi.Event

	// ErrInvalidAmount represents that amount is 0 or negative
	ErrInvalidAmount = errors.New("invalid amount")

	//ErrInvalidCanName represents that candidate name is invalid
	ErrInvalidCanName = errors.New("invalid candidate name")

	// ErrInvalidOwner represents that owner address is invalid
	ErrInvalidOwner = errors.New("invalid owner address")

	// ErrInvalidBLSPubKey represents that BLS public key is invalid
	ErrInvalidBLSPubKey = errors.New("invalid BLS public key")

	_ EthCompatibleAction = (*CandidateRegister)(nil)
	_ amountForCost       = (*CandidateRegister)(nil)
)

// CandidateRegister is the action to register a candidate
type CandidateRegister struct {
	stake_common
	name            string
	operatorAddress address.Address
	rewardAddress   address.Address
	ownerAddress    address.Address
	amount          *big.Int
	duration        uint32
	autoStake       bool
	payload         []byte
	pubKey          []byte // BLS public key
}

func init() {
	var ok bool
	_candidateRegisterMethod, ok = NativeStakingContractABI().Methods["candidateRegister"]
	if !ok {
		panic("fail to load the method")
	}
	_candidateRegisterWithBLSMethod, ok = NativeStakingContractABI().Methods["candidateRegisterWithBLS"]
	if !ok {
		panic("fail to load the method")
	}
	_candidateRegisteredEvent, ok = NativeStakingContractABI().Events["CandidateRegistered"]
	if !ok {
		panic("fail to load the event")
	}
	_stakedEvent, ok = NativeStakingContractABI().Events["Staked"]
	if !ok {
		panic("fail to load the event")
	}
	_candidateActivatedEvent, ok = NativeStakingContractABI().Events["CandidateActivated"]
	if !ok {
		panic("fail to load the event")
	}
}

// NewCandidateRegister creates a CandidateRegister instance
func NewCandidateRegister(
	name, operatorAddrStr, rewardAddrStr, ownerAddrStr, amountStr string,
	duration uint32,
	autoStake bool,
	payload []byte,
) (*CandidateRegister, error) {
	operatorAddr, err := address.FromString(operatorAddrStr)
	if err != nil {
		return nil, err
	}

	rewardAddress, err := address.FromString(rewardAddrStr)
	if err != nil {
		return nil, err
	}

	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		return nil, errors.Wrapf(ErrInvalidAmount, "amount %s", amount)
	}

	cr := &CandidateRegister{
		name:            name,
		operatorAddress: operatorAddr,
		rewardAddress:   rewardAddress,
		amount:          amount,
		duration:        duration,
		autoStake:       autoStake,
		payload:         payload,
	}

	if len(ownerAddrStr) > 0 {
		ownerAddress, err := address.FromString(ownerAddrStr)
		if err != nil {
			return nil, err
		}
		cr.ownerAddress = ownerAddress
	}
	return cr, nil
}

// NewCandidateRegisterWithBLS creates a CandidateRegister instance with BLS public key
func NewCandidateRegisterWithBLS(
	name, operatorAddrStr, rewardAddrStr, ownerAddrStr, amountStr string,
	duration uint32,
	autoStake bool,
	payload []byte,
	pubKey []byte,
) (*CandidateRegister, error) {
	cr, err := NewCandidateRegister(name, operatorAddrStr, rewardAddrStr, ownerAddrStr, amountStr, duration, autoStake, payload)
	if err != nil {
		return nil, err
	}
	_, err = crypto.BLS12381PublicKeyFromBytes(pubKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse BLS public key")
	}
	cr.pubKey = make([]byte, len(pubKey))
	copy(cr.pubKey, pubKey)
	return cr, nil
}

// LegacyAmount returns the legacy amount
func (cr *CandidateRegister) LegacyAmount() *big.Int {
	return cr.amount
}

// Amount returns the amount
func (cr *CandidateRegister) Amount() *big.Int {
	if cr.WithBLS() {
		return cr.value
	}
	return cr.amount
}

// Payload returns the payload bytes
func (cr *CandidateRegister) Payload() []byte { return cr.payload }

// Duration returns the self-stake duration
func (cr *CandidateRegister) Duration() uint32 { return cr.duration }

// AutoStake returns the if staking is auth stake
func (cr *CandidateRegister) AutoStake() bool { return cr.autoStake }

// Name returns candidate name to register
func (cr *CandidateRegister) Name() string { return cr.name }

// OperatorAddress returns candidate operatorAddress to register
func (cr *CandidateRegister) OperatorAddress() address.Address { return cr.operatorAddress }

// RewardAddress returns candidate rewardAddress to register
func (cr *CandidateRegister) RewardAddress() address.Address { return cr.rewardAddress }

// OwnerAddress returns candidate ownerAddress to register
func (cr *CandidateRegister) OwnerAddress() address.Address { return cr.ownerAddress }

// WithBLS returns true if the candidate register action is with BLS public key
func (cr *CandidateRegister) WithBLS() bool {
	return len(cr.pubKey) > 0
}

// PubKey returns the BLS public key if the candidate register action is with BLS public key
func (cr *CandidateRegister) PubKey() []byte {
	return cr.pubKey
}

// Serialize returns a raw byte stream of the CandidateRegister struct
func (cr *CandidateRegister) Serialize() []byte {
	return byteutil.Must(proto.Marshal(cr.Proto()))
}

func (act *CandidateRegister) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_CandidateRegister{CandidateRegister: act.Proto()}
}

// Proto converts to protobuf CandidateRegister Action
func (cr *CandidateRegister) Proto() *iotextypes.CandidateRegister {
	act := iotextypes.CandidateRegister{
		Candidate: &iotextypes.CandidateBasicInfo{
			Name:            cr.name,
			OperatorAddress: cr.operatorAddress.String(),
			RewardAddress:   cr.rewardAddress.String(),
		},
		StakedDuration: cr.duration,
		AutoStake:      cr.autoStake,
	}

	if cr.ownerAddress != nil {
		act.OwnerAddress = cr.ownerAddress.String()
	}

	if len(cr.payload) > 0 {
		act.Payload = make([]byte, len(cr.payload))
		copy(act.Payload, cr.payload)
	}

	switch {
	case cr.WithBLS():
		act.Candidate.BlsPubKey = make([]byte, len(cr.pubKey))
		copy(act.Candidate.BlsPubKey, cr.pubKey)
		if cr.value != nil {
			act.StakedAmount = cr.value.String()
		}
	default:
		if cr.amount != nil {
			act.StakedAmount = cr.amount.String()
		}
	}

	return &act
}

// LoadProto converts a protobuf's Action to CandidateRegister
func (cr *CandidateRegister) LoadProto(pbAct *iotextypes.CandidateRegister) error {
	if pbAct == nil {
		return ErrNilProto
	}

	cInfo := pbAct.GetCandidate()
	cr.name = cInfo.GetName()

	operatorAddr, err := address.FromString(cInfo.GetOperatorAddress())
	if err != nil {
		return err
	}
	rewardAddr, err := address.FromString(cInfo.GetRewardAddress())
	if err != nil {
		return err
	}

	cr.operatorAddress = operatorAddr
	cr.rewardAddress = rewardAddr
	cr.duration = pbAct.GetStakedDuration()
	cr.autoStake = pbAct.GetAutoStake()

	withBLS := len(pbAct.Candidate.GetBlsPubKey()) > 0
	if withBLS {
		cr.pubKey = make([]byte, len(pbAct.Candidate.GetBlsPubKey()))
		copy(cr.pubKey, pbAct.Candidate.GetBlsPubKey())
	}
	if len(pbAct.GetStakedAmount()) > 0 {
		amount, ok := new(big.Int).SetString(pbAct.GetStakedAmount(), 10)
		if !ok {
			return errors.Errorf("invalid amount %s", pbAct.GetStakedAmount())
		}
		if withBLS {
			cr.value = amount
		} else {
			cr.amount = amount
		}
	}

	cr.payload = nil
	if len(pbAct.GetPayload()) > 0 {
		cr.payload = make([]byte, len(pbAct.GetPayload()))
		copy(cr.payload, pbAct.GetPayload())
	}

	if len(pbAct.GetOwnerAddress()) > 0 {
		ownerAddr, err := address.FromString(pbAct.GetOwnerAddress())
		if err != nil {
			return err
		}
		cr.ownerAddress = ownerAddr
	}

	return nil
}

// IntrinsicGas returns the intrinsic gas of a CandidateRegister
func (cr *CandidateRegister) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(cr.Payload()))
	return CalculateIntrinsicGas(CandidateRegisterBaseIntrinsicGas, CandidateRegisterPayloadGas, payloadSize)
}

// SanityCheck validates the variables in the action
func (cr *CandidateRegister) SanityCheck() error {
	if cr.Amount().Sign() < 0 {
		return errors.Wrap(ErrInvalidAmount, "negative value")
	}
	if !IsValidCandidateName(cr.Name()) {
		return ErrInvalidCanName
	}
	return nil
}

// EthData returns the ABI-encoded data for converting to eth tx
func (cr *CandidateRegister) EthData() ([]byte, error) {
	if cr.operatorAddress == nil {
		return nil, ErrAddress
	}
	if cr.rewardAddress == nil {
		return nil, ErrAddress
	}
	if cr.ownerAddress == nil {
		return nil, ErrAddress
	}
	switch {
	case cr.WithBLS():
		data, err := _candidateRegisterWithBLSMethod.Inputs.Pack(
			cr.name,
			common.BytesToAddress(cr.operatorAddress.Bytes()),
			common.BytesToAddress(cr.rewardAddress.Bytes()),
			common.BytesToAddress(cr.ownerAddress.Bytes()),
			cr.amount,
			cr.duration,
			cr.autoStake,
			cr.pubKey,
			cr.payload)
		if err != nil {
			return nil, err
		}
		return append(_candidateRegisterMethod.ID, data...), nil
	default:
		data, err := _candidateRegisterMethod.Inputs.Pack(
			cr.name,
			common.BytesToAddress(cr.operatorAddress.Bytes()),
			common.BytesToAddress(cr.rewardAddress.Bytes()),
			common.BytesToAddress(cr.ownerAddress.Bytes()),
			cr.amount,
			cr.duration,
			cr.autoStake,
			cr.payload)
		if err != nil {
			return nil, err
		}
		return append(_candidateRegisterMethod.ID, data...), nil
	}
}

// PackCandidateRegisteredEvent packs the CandidateRegisterWithBLS event
func PackCandidateRegisteredEvent(
	candidate,
	operatorAddress,
	ownerAddress address.Address,
	name string,
	rewardAddress address.Address,
	blsPublicKey []byte,
) (Topics, []byte, error) {
	data, err := _candidateRegisteredEvent.Inputs.NonIndexed().Pack(
		operatorAddress.Bytes(),
		name,
		rewardAddress.Bytes(),
		blsPublicKey,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to pack CandidateRegisterWithBLS event")
	}
	topics := make(Topics, 3)
	topics[0] = hash.Hash256(_candidateRegisteredEvent.ID)
	topics[1] = hash.Hash256(candidate.Bytes())
	topics[2] = hash.Hash256(ownerAddress.Bytes())
	return topics, data, nil
}

func PackStakedEvent(
	voter,
	candidate address.Address,
	bucketIndex uint64,
	amount *big.Int,
	duration uint32,
	autoStake bool) (Topics, []byte, error) {
	data, err := _stakedEvent.Inputs.NonIndexed().Pack(
		bucketIndex,
		amount,
		duration,
		autoStake,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to pack Staked event")
	}
	topics := make(Topics, 3)
	topics[0] = hash.Hash256(_stakedEvent.ID)
	topics[1] = hash.Hash256(voter.Bytes())
	topics[2] = hash.Hash256(candidate.Bytes())
	return topics, data, nil
}

func PackCandidateActivatedEvent(
	candidate address.Address, bucketIndex uint64,
) (Topics, []byte, error) {
	data, err := _candidateActivatedEvent.Inputs.NonIndexed().Pack(
		bucketIndex,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to pack CandidateActivated event")
	}
	topics := make(Topics, 2)
	topics[0] = hash.Hash256(_candidateActivatedEvent.ID)
	topics[1] = hash.Hash256(candidate.Bytes())
	return topics, data, nil
}

// NewCandidateRegisterFromABIBinary decodes data into CandidateRegister action
func NewCandidateRegisterFromABIBinary(data []byte, value *big.Int) (*CandidateRegister, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		err       error
		cr        CandidateRegister
		method    abi.Method
		withBLS   bool
	)
	// sanity check
	if len(data) <= 4 {
		return nil, errDecodeFailure
	}
	switch {
	case bytes.Equal(_candidateRegisterMethod.ID, data[:4]):
		method = _candidateRegisterMethod
	case bytes.Equal(_candidateRegisterWithBLSMethod.ID, data[:4]):
		method = _candidateRegisterWithBLSMethod
		withBLS = true
	default:
		return nil, errDecodeFailure
	}
	// common fields parsing
	if err := method.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if cr.name, ok = paramsMap["name"].(string); !ok {
		return nil, errDecodeFailure
	}
	if cr.operatorAddress, err = ethAddrToNativeAddr(paramsMap["operatorAddress"]); err != nil {
		return nil, err
	}
	if cr.rewardAddress, err = ethAddrToNativeAddr(paramsMap["rewardAddress"]); err != nil {
		return nil, err
	}
	if cr.ownerAddress, err = ethAddrToNativeAddr(paramsMap["ownerAddress"]); err != nil {
		return nil, err
	}
	if cr.duration, ok = paramsMap["duration"].(uint32); !ok {
		return nil, errDecodeFailure
	}
	if cr.autoStake, ok = paramsMap["autoStake"].(bool); !ok {
		return nil, errDecodeFailure
	}
	if cr.payload, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	// specific fields parsing for methods
	if withBLS {
		if value != nil {
			cr.value.Set(value)
		}
		if cr.pubKey, ok = paramsMap["pubKey"].([]byte); !ok {
			return nil, errors.Wrapf(errDecodeFailure, "invalid pubKey %+v", paramsMap["pubKey"])
		}
		if len(cr.pubKey) == 0 {
			return nil, errors.Wrap(errDecodeFailure, "pubKey is empty")
		}
		_, err := crypto.BLS12381PublicKeyFromBytes(cr.pubKey)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse BLS public key")
		}
	} else {
		if cr.amount, ok = paramsMap["amount"].(*big.Int); !ok {
			return nil, errDecodeFailure
		}
	}
	return &cr, nil
}

func ethAddrToNativeAddr(in interface{}) (address.Address, error) {
	ethAddr, ok := in.(common.Address)
	if !ok {
		return nil, errDecodeFailure
	}
	return address.FromBytes(ethAddr.Bytes())
}

// IsValidCandidateName check if a candidate name string is valid.
func IsValidCandidateName(s string) bool {
	if len(s) == 0 || len(s) > 12 {
		return false
	}
	for _, c := range s {
		if !(('a' <= c && c <= 'z') || ('0' <= c && c <= '9')) {
			return false
		}
	}
	return true
}

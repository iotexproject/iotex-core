package action

import (
	"bytes"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	// CandidateTransferOwnershipPayloadGas represents the CandidateTransferOwnership payload gas per uint
	CandidateTransferOwnershipPayloadGas = uint64(100)
	// CandidateTransferOwnershipBaseIntrinsicGas represents the base intrinsic gas for CandidateTransferOwnership
	CandidateTransferOwnershipBaseIntrinsicGas = uint64(10000)

	_candidateTransferOwnershipInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "newOwner",
					"type": "address"
				},
				{
					"internalType": "uint8[]",
					"name": "payload",
					"type": "uint8[]"
				}
			],
			"name": "candidateTransferOwnership",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	// _candidateTransferOwnershipMethod is the interface of the abi encoding of candidate transfer ownership action
	_candidateTransferOwnershipMethod abi.Method
	_                                 EthCompatibleAction = (*CandidateTransferOwnership)(nil)
)

func init() {
	candidateTransferOwnershipInterface, err := abi.JSON(strings.NewReader(_candidateTransferOwnershipInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_candidateTransferOwnershipMethod, ok = candidateTransferOwnershipInterface.Methods["candidateTransferOwnership"]
	if !ok {
		panic("fail to load the method")
	}
}

// CandidateTransferOwnership is the action to transfer ownership of a candidate
type CandidateTransferOwnership struct {
	stake_common
	newOwner address.Address
	payload  []byte
}

// NewCandidateTransferOwnership returns a CandidateTransferOwnership action
func NewCandidateTransferOwnership(newOwnerStr string, payload []byte) (*CandidateTransferOwnership, error) {
	newOwner, err := address.FromString(newOwnerStr)
	if err != nil {
		return nil, err
	}
	return &CandidateTransferOwnership{
		newOwner: newOwner,
		payload:  payload,
	}, nil
}

// NewCandidateTransferOwnershipFromABIBinary decode data to CandidateTransferOwnership
func NewCandidateTransferOwnershipFromABIBinary(data []byte) (*CandidateTransferOwnership, error) {
	var (
		paramsMap = map[string]any{}
		ok        bool
		err       error
		cr        CandidateTransferOwnership
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_candidateTransferOwnershipMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err = _candidateTransferOwnershipMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if cr.newOwner, err = ethAddrToNativeAddr(paramsMap["newOwner"]); err != nil {
		return nil, err
	}
	if cr.payload, ok = paramsMap["payload"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &cr, nil
}

// Payload returns the payload bytes
func (act *CandidateTransferOwnership) Payload() []byte { return act.payload }

// NewOwner returns the new owner address
func (act *CandidateTransferOwnership) NewOwner() address.Address { return act.newOwner }

// IntrinsicGas returns the intrinsic gas of a CandidateTransferOwnership
func (act *CandidateTransferOwnership) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(act.Payload()))
	return CalculateIntrinsicGas(CandidateTransferOwnershipBaseIntrinsicGas, CandidateTransferOwnershipPayloadGas, payloadSize)
}

// Serialize returns a raw byte stream of the CandidateTransferOwnership struct
func (act *CandidateTransferOwnership) Serialize() []byte {
	return byteutil.Must(proto.Marshal(act.Proto()))
}

func (act *CandidateTransferOwnership) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_CandidateTransferOwnership{CandidateTransferOwnership: act.Proto()}
}

// Proto converts to protobuf CandidateTransferOwnership Action
func (act *CandidateTransferOwnership) Proto() *iotextypes.CandidateTransferOwnership {
	ac := iotextypes.CandidateTransferOwnership{
		NewOwnerAddress: act.newOwner.String(),
	}

	if len(act.payload) > 0 {
		ac.Payload = make([]byte, len(act.payload))
		copy(ac.Payload, act.payload)
	}
	return &ac
}

// LoadProto loads the CandidateTransferOwnership Action from protobuf
func (act *CandidateTransferOwnership) LoadProto(pbAct *iotextypes.CandidateTransferOwnership) error {
	if pbAct == nil {
		return ErrNilProto
	}
	newOwner, err := address.FromString(pbAct.GetNewOwnerAddress())
	if err != nil {
		return err
	}
	act.newOwner = newOwner
	act.payload = nil
	if payload := pbAct.GetPayload(); len(payload) > 0 {
		act.payload = make([]byte, len(payload))
		copy(act.payload, payload)
	}
	return nil
}

// EthData returns the ABI-encoded data for converting to eth tx
func (act *CandidateTransferOwnership) EthData() ([]byte, error) {
	if act.newOwner == nil {
		return nil, ErrAddress
	}
	data, err := _candidateTransferOwnershipMethod.Inputs.Pack(common.BytesToAddress(act.newOwner.Bytes()), act.payload)
	if err != nil {
		return nil, err
	}
	return append(_candidateTransferOwnershipMethod.ID, data...), nil
}

// SanityCheck validates the variables in the action
func (act *CandidateTransferOwnership) SanityCheck() error {
	return nil
}

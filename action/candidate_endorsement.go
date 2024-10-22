package action

import (
	"bytes"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

const (
	// CandidateEndorsementBaseIntrinsicGas represents the base intrinsic gas for CandidateEndorsement
	CandidateEndorsementBaseIntrinsicGas = uint64(10000)

	candidateEndorsementInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				},
				{
					"internalType": "bool",
					"name": "endorse",
					"type": "bool"
				}
			],
			"name": "candidateEndorsement",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				}
			],
			"name": "endorseCandidate",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				}
			],
			"name": "intentToRevokeEndorsement",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				}
			],
			"name": "revokeEndorsement",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

// CandidateEndorsementOp defines the operation of CandidateEndorsement
const (
	// CandidateEndorsementOpLegacy is the operation to endorse or unendorse a candidate in legacy version
	CandidateEndorsementOpLegacy CandidateEndorsementOp = iota
	// CandidateEndorsementOpEndorse is the operation to endorse a candidate
	CandidateEndorsementOpEndorse
	// CandidateEndorsementOpIntentToRevoke is the operation to intent to revoke an endorsement
	CandidateEndorsementOpIntentToRevoke
	// CandidateEndorsementOpRevoke is the operation to revoke an endorsement
	CandidateEndorsementOpRevoke
)

var (
	candidateEndorsementLegacyMethod         abi.Method
	candidateEndorsementEndorseMethod        abi.Method
	caniddateEndorsementIntentToRevokeMethod abi.Method
	candidateEndorsementRevokeMethod         abi.Method
	_                                        EthCompatibleAction = (*CandidateEndorsement)(nil)
)

type (
	// CandidateEndorsement is the action to endorse or unendorse a candidate
	CandidateEndorsement struct {
		stake_common
		// bucketIndex is the bucket index want to be endorsed or unendorsed
		bucketIndex uint64
		// endorse is true if the action is to endorse a candidate, false if unendorse
		endorse bool
		// op is the operation of the endorsement
		op CandidateEndorsementOp
	}

	// CandidateEndorsementOp defines the operation of CandidateEndorsement
	CandidateEndorsementOp uint8
)

func init() {
	candidateEndorsementInterface, err := abi.JSON(strings.NewReader(candidateEndorsementInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	candidateEndorsementLegacyMethod, ok = candidateEndorsementInterface.Methods["candidateEndorsement"]
	if !ok {
		panic("fail to load the candidateEndorsement method")
	}
	candidateEndorsementEndorseMethod, ok = candidateEndorsementInterface.Methods["endorseCandidate"]
	if !ok {
		panic("fail to load the endorseCandidate method")
	}
	caniddateEndorsementIntentToRevokeMethod, ok = candidateEndorsementInterface.Methods["intentToRevokeEndorsement"]
	if !ok {
		panic("fail to load the intentToRevokeEndorsement method")
	}
	candidateEndorsementRevokeMethod, ok = candidateEndorsementInterface.Methods["revokeEndorsement"]
	if !ok {
		panic("fail to load the revokeEndorsement method")
	}
}

// NewCandidateEndorsementLegacy returns a CandidateEndorsement action
func NewCandidateEndorsementLegacy(bucketIndex uint64, endorse bool) *CandidateEndorsement {
	return &CandidateEndorsement{
		bucketIndex: bucketIndex,
		endorse:     endorse,
	}
}

// NewCandidateEndorsement returns a CandidateEndorsement action
func NewCandidateEndorsement(bucketIndex uint64, op CandidateEndorsementOp) (*CandidateEndorsement, error) {
	if op == CandidateEndorsementOpLegacy {
		return nil, errors.New("invalid operation")
	}
	return &CandidateEndorsement{
		bucketIndex: bucketIndex,
		op:          op,
	}, nil
}

// BucketIndex returns the bucket index of the action
func (act *CandidateEndorsement) BucketIndex() uint64 {
	return act.bucketIndex
}

// IntrinsicGas returns the intrinsic gas of a CandidateEndorsement
func (act *CandidateEndorsement) IntrinsicGas() (uint64, error) {
	return CandidateEndorsementBaseIntrinsicGas, nil
}

func (act *CandidateEndorsement) SanityCheck() error {
	return nil
}

// IsLegacy returns true if the action is in legacy version
func (act *CandidateEndorsement) IsLegacy() bool {
	return act.op == CandidateEndorsementOpLegacy
}

// Op returns the operation of the endorsement
func (act *CandidateEndorsement) Op() CandidateEndorsementOp {
	if act.op == CandidateEndorsementOpLegacy {
		if act.endorse {
			return CandidateEndorsementOpEndorse
		}
		return CandidateEndorsementOpIntentToRevoke
	}
	return act.op
}

func (act *CandidateEndorsement) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_CandidateEndorsement{CandidateEndorsement: act.Proto()}
}

// Proto converts CandidateEndorsement to protobuf's Action
func (act *CandidateEndorsement) Proto() *iotextypes.CandidateEndorsement {
	return &iotextypes.CandidateEndorsement{
		BucketIndex: act.bucketIndex,
		Endorse:     act.endorse,
		Op:          uint32(act.op),
	}
}

// LoadProto converts a protobuf's Action to CandidateEndorsement
func (act *CandidateEndorsement) LoadProto(pbAct *iotextypes.CandidateEndorsement) error {
	if pbAct == nil {
		return ErrNilProto
	}
	act.bucketIndex = pbAct.GetBucketIndex()
	act.op = CandidateEndorsementOp(pbAct.GetOp())
	act.endorse = pbAct.GetEndorse()
	return nil
}

// EthData returns the ABI-encoded data for converting to eth tx
func (act *CandidateEndorsement) EthData() ([]byte, error) {
	var method abi.Method
	switch act.op {
	case CandidateEndorsementOpLegacy:
		data, err := candidateEndorsementLegacyMethod.Inputs.Pack(act.bucketIndex, act.endorse)
		if err != nil {
			return nil, err
		}
		return append(candidateEndorsementLegacyMethod.ID, data...), nil
	case CandidateEndorsementOpEndorse:
		method = candidateEndorsementEndorseMethod
	case CandidateEndorsementOpIntentToRevoke:
		method = caniddateEndorsementIntentToRevokeMethod
	case CandidateEndorsementOpRevoke:
		method = candidateEndorsementRevokeMethod
	default:
		return nil, errors.New("invalid operation")
	}
	data, err := method.Inputs.Pack(act.bucketIndex)
	if err != nil {
		return nil, err
	}
	return append(method.ID, data...), nil
}

// NewCandidateEndorsementFromABIBinary parses the smart contract input and creates an action
func NewCandidateEndorsementFromABIBinary(data []byte) (*CandidateEndorsement, error) {
	if len(data) <= 4 {
		return nil, errDecodeFailure
	}
	var (
		paramsMap = map[string]any{}
		cr        CandidateEndorsement
		method    abi.Method
	)
	switch {
	case bytes.Equal(candidateEndorsementLegacyMethod.ID, data[:4]):
		method = candidateEndorsementLegacyMethod
		cr.op = CandidateEndorsementOpLegacy
	case bytes.Equal(candidateEndorsementEndorseMethod.ID, data[:4]):
		method = candidateEndorsementEndorseMethod
		cr.op = CandidateEndorsementOpEndorse
	case bytes.Equal(caniddateEndorsementIntentToRevokeMethod.ID, data[:4]):
		method = caniddateEndorsementIntentToRevokeMethod
		cr.op = CandidateEndorsementOpIntentToRevoke
	case bytes.Equal(candidateEndorsementRevokeMethod.ID, data[:4]):
		method = candidateEndorsementRevokeMethod
		cr.op = CandidateEndorsementOpRevoke
	default:
		return nil, errDecodeFailure
	}
	if err := method.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	bucketID, ok := paramsMap["bucketIndex"].(uint64)
	if !ok {
		return nil, errDecodeFailure
	}
	cr.bucketIndex = bucketID
	if cr.op == CandidateEndorsementOpLegacy {
		endorse, ok := paramsMap["endorse"].(bool)
		if !ok {
			return nil, errDecodeFailure
		}
		cr.endorse = endorse
	}
	return &cr, nil
}

package action

import (
	"bytes"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

const (
	// CandidateDeactivateBaseIntrinsicGas represents the base intrinsic gas for CandidateActivate
	CandidateDeactivateBaseIntrinsicGas = uint64(10000)

	// CandidateDeactivateOpRequest is an operation to request deactivation
	CandidateDeactivateOpRequest = iota
	// CandidateDeactivateOpCancel is an operation to cancel deactivation
	CandidateDeactivateOpCancel
	// CandidateDeactivateOpConfirm is an operation to confirm deactivation
	CandidateDeactivateOpConfirm
)

var (
	requestCandidateDeactivationMethod abi.Method
	cancelCandidateDeactivationMethod  abi.Method
	confirmCandidateDeactivationMethod abi.Method
	_                                  EthCompatibleAction = (*CandidateDeactivate)(nil)
)

type (
	// CandidateDeactivateOp is operation type of candidate deactivation
	CandidateDeactivateOp uint8
	// CandidateDeactivate is the action to deactivate a candidate
	CandidateDeactivate struct {
		stake_common
		op CandidateDeactivateOp
	}
)

func init() {
	var ok bool
	methods := NativeStakingContractABI().Methods
	requestCandidateDeactivationMethod, ok = methods["requestCandidateDeactivation"]
	if !ok {
		panic("fail to load the requestCandidateDeactivation method")
	}
	cancelCandidateDeactivationMethod, ok = methods["cancelCandidateDeactivation"]
	if !ok {
		panic("fail to load the cancelCandidateDeactivation method")
	}
}

// NewCandidateDeactivate returns a CandidateDeactivate action
func NewCandidateDeactivate() *CandidateDeactivate {
	return &CandidateDeactivate{}
}

// IntrinsicGas returns the intrinsic gas of a CandidateDeactivate
func (cd *CandidateDeactivate) IntrinsicGas() (uint64, error) {
	return CandidateActivateBaseIntrinsicGas, nil
}

func (cd *CandidateDeactivate) SanityCheck() error {
	return nil
}

func (cd *CandidateDeactivate) FillAction(act *iotextypes.ActionCore) {
	act.Action = &iotextypes.ActionCore_CandidateDeactivate{CandidateDeactivate: cd.Proto()}
}

func (cd *CandidateDeactivate) Op() CandidateDeactivateOp {
	return cd.op
}

// Proto converts CandidateDeactivate to protobuf's Action
func (cd *CandidateDeactivate) Proto() *iotextypes.CandidateDeactivate {
	return &iotextypes.CandidateDeactivate{
		Op: uint32(cd.op),
	}
}

// LoadProto converts a protobuf's Action to CandidateDeactivate
func (cd *CandidateDeactivate) LoadProto(pbAct *iotextypes.CandidateDeactivate) error {
	if pbAct == nil {
		return ErrNilProto
	}
	cd.op = CandidateDeactivateOp(pbAct.GetOp())
	return nil
}

// EthData returns the ABI-encoded data for converting to eth tx
func (cd *CandidateDeactivate) EthData() ([]byte, error) {
	var method abi.Method
	switch cd.op {
	case CandidateDeactivateOpCancel:
		method = requestCandidateDeactivationMethod
	case CandidateDeactivateOpRequest:
		method = cancelCandidateDeactivationMethod
	case CandidateDeactivateOpConfirm:
		method = confirmCandidateDeactivationMethod
	default:
		return nil, errors.New("invalid operation")
	}
	data, err := method.Inputs.Pack()
	if err != nil {
		return nil, err
	}
	return append(method.ID, data...), nil
}

// NewCandidateDeactivateFromABIBinary parses the smart contract input and creates an action
func NewCandidateDeactivateFromABIBinary(data []byte) (*CandidateDeactivate, error) {
	var cd CandidateDeactivate
	// sanity check
	switch {
	case len(data) <= 4:
		return nil, errDecodeFailure
	case bytes.Equal(requestCandidateDeactivationMethod.ID, data[:4]):
		cd.op = CandidateDeactivateOpRequest
	case bytes.Equal(cancelCandidateDeactivationMethod.ID, data[:4]):
		cd.op = CandidateDeactivateOpCancel
	case bytes.Equal(confirmCandidateDeactivationMethod.ID, data[:4]):
		cd.op = CandidateDeactivateOpConfirm
	default:
		return nil, errDecodeFailure
	}

	return &cd, nil
}

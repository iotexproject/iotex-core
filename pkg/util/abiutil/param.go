package abiutil

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
)

type (
	// EventParam is a struct to hold smart contract event parameters, which can easily convert a param to go type
	EventParam struct {
		params      []any
		nameToIndex map[string]int
	}
)

var (
	// ErrInvlidEventParam is an error for invalid event param
	ErrInvlidEventParam = errors.New("invalid event param")
)

// EventField is a helper function to get a field from event param
func EventField[T any](e EventParam, name string) (T, error) {
	id, ok := e.nameToIndex[name]
	if !ok {
		var zeroValue T
		return zeroValue, errors.Wrapf(ErrInvlidEventParam, "field %s not found", name)
	}
	return EventFieldByID[T](e, id)
}

// EventFieldByID is a helper function to get a field from event param
func EventFieldByID[T any](e EventParam, id int) (T, error) {
	field, ok := e.fieldByID(id).(T)
	if !ok {
		return field, errors.Wrapf(ErrInvlidEventParam, "field %d got %#v, expect %T", id, e.fieldByID(id), field)
	}
	return field, nil
}

func (e EventParam) field(name string) any {
	return e.params[e.nameToIndex[name]]
}

func (e EventParam) fieldByID(id int) any {
	return e.params[id]
}

func (e EventParam) String() string {
	return fmt.Sprintf("%+v", e.params)
}

// FieldUint256 is a helper function to get a uint256 field from event param
func (e EventParam) FieldUint256(name string) (*big.Int, error) {
	return EventField[*big.Int](e, name)
}

// FieldByIDUint256 is a helper function to get a uint256 field from event param
func (e EventParam) FieldByIDUint256(id int) (*big.Int, error) {
	return EventFieldByID[*big.Int](e, id)
}

// FieldBytes12 is a helper function to get a bytes12 field from event param
func (e EventParam) FieldBytes12(name string) (string, error) {
	id, ok := e.nameToIndex[name]
	if !ok {
		return "", errors.Wrapf(ErrInvlidEventParam, "field %s not found", name)
	}
	return e.FieldByIDBytes12(id)
}

// FieldByIDBytes12 is a helper function to get a bytes12 field from event param
func (e EventParam) FieldByIDBytes12(id int) (string, error) {
	data, err := EventFieldByID[[12]byte](e, id)
	if err != nil {
		return "", err
	}
	// remove trailing zeros
	tail := len(data) - 1
	for ; tail >= 0 && data[tail] == 0; tail-- {
	}
	return string(data[:tail+1]), nil
}

// FieldUint256Slice is a helper function to get a uint256 slice field from event param
func (e EventParam) FieldUint256Slice(name string) ([]*big.Int, error) {
	return EventField[[]*big.Int](e, name)
}

// FieldByIDUint256Slice is a helper function to get a uint256 slice field from event param
func (e EventParam) FieldByIDUint256Slice(id int) ([]*big.Int, error) {
	return EventFieldByID[[]*big.Int](e, id)
}

// FieldAddress is a helper function to get an address field from event param
func (e EventParam) FieldAddress(name string) (address.Address, error) {
	commAddr, err := EventField[common.Address](e, name)
	if err != nil {
		return nil, err
	}
	return address.FromBytes(commAddr.Bytes())
}

// FieldByIDAddress is a helper function to get an address field from event param
func (e EventParam) FieldByIDAddress(id int) (address.Address, error) {
	commAddr, err := EventFieldByID[common.Address](e, id)
	if err != nil {
		return nil, err
	}
	return address.FromBytes(commAddr.Bytes())
}

// UnpackEventParam is a helper function to unpack event parameters
func UnpackEventParam(abiEvent *abi.Event, log *action.Log) (*EventParam, error) {
	// unpack non-indexed fields
	params := make(map[string]any)
	if len(log.Data) > 0 {
		if err := abiEvent.Inputs.UnpackIntoMap(params, log.Data); err != nil {
			return nil, errors.Wrap(err, "unpack event data failed")
		}
	}
	// unpack indexed fields
	args := make(abi.Arguments, 0)
	for _, arg := range abiEvent.Inputs {
		if arg.Indexed {
			args = append(args, arg)
		}
	}
	topics := make([]common.Hash, 0)
	for i, topic := range log.Topics {
		if i > 0 {
			topics = append(topics, common.Hash(topic))
		}
	}
	err := abi.ParseTopicsIntoMap(params, args, topics)
	if err != nil {
		return nil, errors.Wrap(err, "unpack event indexed fields failed")
	}
	// create event param
	event := &EventParam{
		params:      make([]any, 0, len(abiEvent.Inputs)),
		nameToIndex: make(map[string]int),
	}
	for i, arg := range abiEvent.Inputs {
		event.params = append(event.params, params[arg.Name])
		event.nameToIndex[arg.Name] = i
	}
	return event, nil
}

package abiutil

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
)

type (
	// EventParam is a struct to hold smart contract event parameters, which can easily convert a param to go type
	EventParam map[string]any
)

var (
	// ErrInvlidEventParam is an error for invalid event param
	ErrInvlidEventParam = errors.New("invalid event param")
)

// EventField is a helper function to get a field from event param
func EventField[T any](e EventParam, name string) (T, error) {
	field, ok := e[name].(T)
	if !ok {
		return field, errors.Wrapf(ErrInvlidEventParam, "field %s got %#v, expect %T", name, e[name], field)
	}
	return field, nil
}

// FieldUint256 is a helper function to get a uint256 field from event param
func (e EventParam) FieldUint256(name string) (*big.Int, error) {
	return EventField[*big.Int](e, name)
}

// FieldBytes12 is a helper function to get a bytes12 field from event param
func (e EventParam) FieldBytes12(name string) (string, error) {
	data, err := EventField[[12]byte](e, name)
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

// FieldAddress is a helper function to get an address field from event param
func (e EventParam) FieldAddress(name string) (address.Address, error) {
	commAddr, err := EventField[common.Address](e, name)
	if err != nil {
		return nil, err
	}
	return address.FromBytes(commAddr.Bytes())
}

// IndexedFieldAddress is a helper function to get an indexed address field from event param
func (e EventParam) IndexedFieldAddress(name string) (address.Address, error) {
	return e.FieldAddress(name)
}

// IndexedFieldUint256 is a helper function to get an indexed uint256 field from event param
func (e EventParam) IndexedFieldUint256(name string) (*big.Int, error) {
	return EventField[*big.Int](e, name)
}

// UnpackEventParam is a helper function to unpack event parameters
func UnpackEventParam(abiEvent *abi.Event, log *action.Log) (EventParam, error) {
	event := make(EventParam)
	// unpack non-indexed fields
	if len(log.Data) > 0 {
		if err := abiEvent.Inputs.UnpackIntoMap(event, log.Data); err != nil {
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
	err := abi.ParseTopicsIntoMap(event, args, topics)
	if err != nil {
		return nil, errors.Wrap(err, "unpack event indexed fields failed")
	}
	return event, nil
}

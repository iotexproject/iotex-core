// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
)

type (
	// eventParam is a struct to hold smart contract event parameters, which can easily convert a param to go type
	eventParam map[string]any
)

var (
	errInvlidEventParam = errors.New("invalid event param")
)

func eventField[T any](e eventParam, name string) (T, error) {
	field, ok := e[name].(T)
	if !ok {
		return field, errors.Wrapf(errInvlidEventParam, "field %s got %#v, expect %T", name, e[name], field)
	}
	return field, nil
}

func (e eventParam) FieldUint256(name string) (*big.Int, error) {
	return eventField[*big.Int](e, name)
}

func (e eventParam) FieldBytes12(name string) (string, error) {
	data, err := eventField[[12]byte](e, name)
	if err != nil {
		return "", err
	}
	// remove trailing zeros
	tail := len(data) - 1
	for ; tail >= 0 && data[tail] == 0; tail-- {
	}
	return string(data[:tail+1]), nil
}

func (e eventParam) FieldUint256Slice(name string) ([]*big.Int, error) {
	return eventField[[]*big.Int](e, name)
}

func (e eventParam) FieldAddress(name string) (address.Address, error) {
	commAddr, err := eventField[common.Address](e, name)
	if err != nil {
		return nil, err
	}
	return address.FromBytes(commAddr.Bytes())
}

func (e eventParam) IndexedFieldAddress(name string) (address.Address, error) {
	return e.FieldAddress(name)
}

func (e eventParam) IndexedFieldUint256(name string) (*big.Int, error) {
	return eventField[*big.Int](e, name)
}

func unpackEventParam(abiEvent *abi.Event, log *action.Log) (eventParam, error) {
	event := make(eventParam)
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

func sortByIds(ids []uint64, bts []*BucketType, bis []*bucketInfo) ([]uint64, []*BucketType, []*bucketInfo) {
	if len(ids) == 0 {
		return ids, bts, bis
	}
	if len(bts) != len(ids) || len(bis) != len(ids) {
		panic("length of ids, bts and bis should be the same")
	}
	sorted := make([]int, len(ids))
	for i := range sorted {
		sorted[i] = i
	}
	sort.Slice(sorted, func(i, j int) bool { return ids[sorted[i]] < ids[sorted[j]] })
	sortedIds := make([]uint64, len(ids))
	sortedBts := make([]*BucketType, len(bts))
	sortedBis := make([]*bucketInfo, len(bis))
	for i, idx := range sorted {
		sortedIds[i] = ids[idx]
		sortedBts[i] = bts[idx]
		sortedBis[i] = bis[idx]
	}
	return sortedIds, sortedBts, sortedBis
}

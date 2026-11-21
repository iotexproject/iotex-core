// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package freezelog

import (
	"encoding/binary"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
)

var (
	// ErrNilAddress is returned when a required address argument is nil.
	ErrNilAddress = errors.New("freezelog: nil address")

	parseOnce sync.Once
	parsedABI abi.ABI
	parseErr  error
)

func loadABI() (abi.ABI, error) {
	parseOnce.Do(func() {
		parsedABI, parseErr = abi.JSON(strings.NewReader(ABIJSON))
	})
	return parsedABI, parseErr
}

// EventArgs are the fields of one DelegateRewardFrozen log. Field order matches
// the Solidity signature so a struct literal reads left-to-right the same way as
// the event declaration.
type EventArgs struct {
	Era                  uint64          // indexed → Topics[1]
	Delegate             address.Address // indexed → Topics[2]
	FreezeHeight         uint64
	BlockCommissionBps   uint64
	EpochCommissionBps   uint64
	CommissionConfigured bool
	TotalWeight          *big.Int
	SelfStakeBucketIdx   uint64
}

// Pack encodes args as an EVM-shaped receipt log, so off-chain consumers can
// decode it with stock ethers.js / web3.py against ABIJSON.
//
// Topics layout:
//
//	[0] keccak256(EventSignature)
//	[1] uint64 era, left-padded to 32 bytes
//	[2] address delegate, left-padded to 32 bytes
//
// Data layout: ABI-standard tuple of the remaining inputs in declaration order.
//
// CommissionConfigured is the field to read before trusting the basis points. A
// delegate that published no portions is frozen at 10000/10000, which is
// indistinguishable by value from one that deliberately takes everything.
func Pack(args EventArgs) (action.Topics, []byte, error) {
	if args.Delegate == nil {
		return nil, nil, errors.Wrap(ErrNilAddress, "delegate")
	}
	weight := args.TotalWeight
	if weight == nil {
		weight = new(big.Int)
	}
	parsed, err := loadABI()
	if err != nil {
		return nil, nil, errors.Wrap(err, "freezelog: parse ABI")
	}
	ev, ok := parsed.Events[EventName]
	if !ok {
		return nil, nil, errors.Errorf("freezelog: event %q not found in parsed ABI", EventName)
	}
	data, err := ev.Inputs.NonIndexed().Pack(
		args.FreezeHeight,
		args.BlockCommissionBps,
		args.EpochCommissionBps,
		args.CommissionConfigured,
		weight,
		args.SelfStakeBucketIdx,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "freezelog: pack DelegateRewardFrozen data")
	}
	topics := make(action.Topics, 3)
	topics[0] = hash.Hash256(ev.ID)
	topics[1] = encodeUint64Topic(args.Era)
	topics[2] = hash.BytesToHash256(args.Delegate.Bytes())
	return topics, data, nil
}

// encodeUint64Topic left-pads a uint64 into a 32-byte topic word, matching how
// the EVM encodes an indexed integer.
func encodeUint64Topic(v uint64) hash.Hash256 {
	var out hash.Hash256
	binary.BigEndian.PutUint64(out[24:], v)
	return out
}

// EthAddress is a helper for consumers building topic filters.
func EthAddress(a address.Address) common.Address {
	return common.BytesToAddress(a.Bytes())
}

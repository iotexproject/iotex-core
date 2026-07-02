// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

const (
	// SetCommissionRateBaseIntrinsicGas is the base intrinsic gas cost for a
	// SetCommissionRate action. The action does a single owner lookup and a
	// candidate state write, so we charge the same as the other small
	// staking actions (CandidateActivate, CandidateDeactivate).
	SetCommissionRateBaseIntrinsicGas = uint64(10000)

	// MaxCommissionRate is the upper bound on the commission rate in basis
	// points (10000 = 100%). 0 means legacy behavior (no auto-distribution),
	// and any rate strictly between 0 and MaxCommissionRate splits the
	// epoch reward between the delegate and its voters.
	MaxCommissionRate = uint64(10000)
)

var (
	_setCommissionRateMethod abi.Method
	_commissionRateSetEvent  abi.Event

	// ErrInvalidCommissionRate is returned when the requested commission
	// rate is above MaxCommissionRate.
	ErrInvalidCommissionRate = errors.New("invalid commission rate")

	_ EthCompatibleAction = (*SetCommissionRate)(nil)
)

// SetCommissionRate is IIP-59's action to set a delegate's voter reward
// commission rate (basis points, 0-10000). Only the candidate owner can
// submit it; the new rate is staged on the staking candidate record and
// promoted into the per-epoch poll snapshot at the next PutPollResult,
// which gives voters ~1.5 epochs of warning before the new rate takes
// effect at distribution time.
type SetCommissionRate struct {
	stake_common
	rate uint64
}

func init() {
	var ok bool
	abi := NativeStakingContractABI()
	_setCommissionRateMethod, ok = abi.Methods["setCommissionRate"]
	if !ok {
		panic("fail to load the setCommissionRate method")
	}
	_commissionRateSetEvent, ok = abi.Events["CommissionRateSet"]
	if !ok {
		panic("fail to load the CommissionRateSet event")
	}
}

// NewSetCommissionRate creates a SetCommissionRate action with the given
// rate (basis points). The caller is responsible for ensuring rate is
// within range; the action's SanityCheck rejects out-of-range values
// during validation.
func NewSetCommissionRate(rate uint64) *SetCommissionRate {
	return &SetCommissionRate{rate: rate}
}

// Rate returns the requested commission rate in basis points.
func (s *SetCommissionRate) Rate() uint64 { return s.rate }

// IntrinsicGas returns the intrinsic gas cost of the action.
func (s *SetCommissionRate) IntrinsicGas() (uint64, error) {
	return SetCommissionRateBaseIntrinsicGas, nil
}

// SanityCheck validates that rate is within [0, MaxCommissionRate].
func (s *SetCommissionRate) SanityCheck() error {
	if s.rate > MaxCommissionRate {
		return errors.Wrapf(ErrInvalidCommissionRate, "rate %d exceeds max %d", s.rate, MaxCommissionRate)
	}
	return nil
}

// FillAction installs the action into an ActionCore oneof.
func (s *SetCommissionRate) FillAction(act *iotextypes.ActionCore) {
	act.Action = &iotextypes.ActionCore_SetCommissionRate{SetCommissionRate: s.Proto()}
}

// Proto converts SetCommissionRate to its protobuf representation.
func (s *SetCommissionRate) Proto() *iotextypes.SetCommissionRate {
	return &iotextypes.SetCommissionRate{
		Rate: s.rate,
	}
}

// LoadProto restores a SetCommissionRate from its protobuf form.
func (s *SetCommissionRate) LoadProto(pbAct *iotextypes.SetCommissionRate) error {
	if pbAct == nil {
		return ErrNilProto
	}
	s.rate = pbAct.GetRate()
	return nil
}

// EthData returns the ABI-encoded data for converting to an eth tx —
// matches setCommissionRate(uint64) in native_staking_contract_interface.sol.
func (s *SetCommissionRate) EthData() ([]byte, error) {
	data, err := _setCommissionRateMethod.Inputs.Pack(s.rate)
	if err != nil {
		return nil, err
	}
	return append(_setCommissionRateMethod.ID, data...), nil
}

// NewSetCommissionRateFromABIBinary parses ETH-compatible call data and
// reconstructs the SetCommissionRate action.
func NewSetCommissionRateFromABIBinary(data []byte) (*SetCommissionRate, error) {
	var (
		paramsMap = map[string]any{}
		s         SetCommissionRate
	)
	if len(data) <= 4 || !bytes.Equal(_setCommissionRateMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _setCommissionRateMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	rate, ok := paramsMap["rate"].(uint64)
	if !ok {
		return nil, errDecodeFailure
	}
	s.rate = rate
	return &s, nil
}

// PackCommissionRateSetEvent packs the CommissionRateSet event topics +
// data — emitted by the handler at receipt time so indexers can subscribe
// to commission-rate changes without re-reading staking state.
func PackCommissionRateSetEvent(candidate address.Address, newRate uint64) (Topics, []byte, error) {
	data, err := _commissionRateSetEvent.Inputs.NonIndexed().Pack(newRate)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to pack CommissionRateSet event")
	}
	topics := make(Topics, 2)
	topics[0] = hash.Hash256(_commissionRateSetEvent.ID)
	topics[1] = hash.BytesToHash256(candidate.Bytes())
	return topics, data, nil
}

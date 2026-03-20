// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	// SetCommissionRateBaseIntrinsicGas represents the base intrinsic gas for SetCommissionRate
	SetCommissionRateBaseIntrinsicGas = uint64(10000)
	// MaxCommissionRate is the maximum commission rate in basis points (100%)
	MaxCommissionRate = uint64(10000)
	// CommissionRateCooldownEpochs is the minimum number of epochs between commission rate changes
	// ~7 days at 1-hour epochs
	CommissionRateCooldownEpochs = uint64(168)
)

// SetCommissionRate is the action to set a delegate's voter reward commission rate (IIP-59)
type SetCommissionRate struct {
	stake_common
	rate uint64 // basis points, 0-10000
}

// NewSetCommissionRate creates a SetCommissionRate instance
func NewSetCommissionRate(rate uint64) (*SetCommissionRate, error) {
	if rate > MaxCommissionRate {
		return nil, errors.Errorf("commission rate %d exceeds max %d bps", rate, MaxCommissionRate)
	}
	return &SetCommissionRate{rate: rate}, nil
}

// Rate returns the commission rate in basis points
func (s *SetCommissionRate) Rate() uint64 { return s.rate }

// IntrinsicGas returns the intrinsic gas of a SetCommissionRate
func (s *SetCommissionRate) IntrinsicGas() (uint64, error) {
	return SetCommissionRateBaseIntrinsicGas, nil
}

// SanityCheck validates the SetCommissionRate action
func (s *SetCommissionRate) SanityCheck() error {
	if s.rate > MaxCommissionRate {
		return errors.Errorf("commission rate %d exceeds max %d bps", s.rate, MaxCommissionRate)
	}
	return nil
}

// Serialize returns a raw byte stream of the SetCommissionRate
func (s *SetCommissionRate) Serialize() []byte {
	return byteutil.Uint64ToBytesBigEndian(s.rate)
}

// TODO(IIP-59): The following methods require iotex-proto to add SetCommissionRate message
// as field 54 in the ActionCore oneof. Once that's done:
//   - Add FillAction() that sets core.Action = &iotextypes.ActionCore_SetCommissionRate{...}
//   - Add Proto() / LoadProto() for serialization
//   - Add deserialization case in envelope.go loadProtoActionPayload()
//   - Add EthABI encoding for MetaMask/Hardhat compatibility
//
// Until then, the action is handled internally by the staking protocol's handler
// dispatch but cannot be submitted via external RPC.

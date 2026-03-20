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
)

// SetCommissionRate is the action to set a delegate's voter reward commission rate (IIP-59)
type SetCommissionRate struct {
	stake_common
	rate uint64 // basis points, 0-10000
}

// NewSetCommissionRate creates a SetCommissionRate instance
func NewSetCommissionRate(rate uint64) (*SetCommissionRate, error) {
	if rate > 10000 {
		return nil, errors.New("commission rate exceeds 10000 bps")
	}
	return &SetCommissionRate{rate: rate}, nil
}

// Rate returns the commission rate in basis points
func (s *SetCommissionRate) Rate() uint64 { return s.rate }

// IntrinsicGas returns the intrinsic gas of a SetCommissionRate
func (s *SetCommissionRate) IntrinsicGas() (uint64, error) {
	return SetCommissionRateBaseIntrinsicGas, nil
}

// Serialize returns a raw byte stream of the SetCommissionRate
func (s *SetCommissionRate) Serialize() []byte {
	return byteutil.Uint64ToBytesBigEndian(s.rate)
}

// SanityCheck validates the SetCommissionRate action
func (s *SetCommissionRate) SanityCheck() error {
	if s.rate > 10000 {
		return errors.New("commission rate exceeds 10000 bps")
	}
	return nil
}

// LoadFromBytes loads SetCommissionRate from a byte stream
func (s *SetCommissionRate) LoadFromBytes(data []byte) error {
	if len(data) < 8 {
		return errors.New("invalid data length for SetCommissionRate")
	}
	s.rate = byteutil.BytesToUint64BigEndian(data[:8])
	if s.rate > 10000 {
		return errors.New("commission rate exceeds 10000 bps")
	}
	return nil
}

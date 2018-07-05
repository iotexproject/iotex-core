// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"encoding/hex"

	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
)

// ConstructAddress constructs an iotex address
func ConstructAddress(pubkey, prikey string) *iotxaddress.Address {
	pubk, err := hex.DecodeString(pubkey)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to construct the address")
	}
	prik, err := hex.DecodeString(prikey)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to construct the address")
	}
	addr, err := iotxaddress.GetAddress(pubk, iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to construct the address")
	}
	addr.PrivateKey = prik
	return addr
}

// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	// IotxDecimalNum defines the number of decimal digits for IoTeX
	IotxDecimalNum = 18
	// GasPriceDecimalNum defines the number of decimal digits for gas price
	GasPriceDecimalNum = 12
)

// ConnectToEndpoint starts a new connection
func ConnectToEndpoint() (*grpc.ClientConn, error) {
	endpoint := config.Get("endpoint")
	if endpoint == config.ErrEmptyEndpoint {
		log.L().Error(config.ErrEmptyEndpoint)
		return nil, errors.New("use \"ioctl config set endpoint\" to config endpoint first")
	}
	return grpc.Dial(endpoint, grpc.WithInsecure())
}

// StringToRau convert different unit string into Rau big int
func StringToRau(amount string, numDecimals int) (*big.Int, error) {
	amountStrings := strings.Split(amount, ".")
	if len(amountStrings) != 1 {
		if len(amountStrings) > 2 || len(amountStrings[1]) > numDecimals {
			return nil, fmt.Errorf("failed to convert string into big int")
		}
		amountStrings[0] += amountStrings[1]
		numDecimals -= len(amountStrings[1])
	}
	if len(amountStrings[0]) == 0 {
		return nil, fmt.Errorf("failed to convert string into big int")
	}
	for i := 0; i < numDecimals; i++ {
		amountStrings[0] += "0"
	}
	amountRau, ok := big.NewInt(0).SetString(amountStrings[0], 10)
	if !ok {
		return nil, fmt.Errorf("failed to convert string into big int")
	}
	if amountRau.Sign() < 0 {
		return nil, fmt.Errorf("invalid number that is minus")
	}
	return amountRau, nil
}

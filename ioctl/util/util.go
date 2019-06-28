// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"crypto/tls"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/validator"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
)

const (
	// IotxDecimalNum defines the number of decimal digits for IoTeX
	IotxDecimalNum = 18
	// GasPriceDecimalNum defines the number of decimal digits for gas price
	GasPriceDecimalNum = 12
)

// ConnectToEndpoint starts a new connection
func ConnectToEndpoint(secure bool) (*grpc.ClientConn, error) {
	endpoint := config.ReadConfig.Endpoint
	if endpoint == "" {
		return nil, fmt.Errorf(`use "ioctl config set endpoint" to config endpoint first`)
	}
	if !secure {
		return grpc.Dial(endpoint, grpc.WithInsecure())
	}
	return grpc.Dial(endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
}

// StringToRau converts different unit string into Rau big int
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
	zeroString := strings.Repeat("0", numDecimals)
	amountStrings[0] += zeroString
	amountRau, ok := big.NewInt(0).SetString(amountStrings[0], 10)
	if !ok {
		return nil, fmt.Errorf("failed to convert string into big int")
	}
	if amountRau.Sign() < 0 {
		return nil, fmt.Errorf("invalid number that is minus")
	}
	return amountRau, nil
}

// RauToString converts Rau big int into Iotx string
func RauToString(amount *big.Int, numDecimals int) string {
	var targetUnit int64
	switch numDecimals {
	case 18:
		targetUnit = unit.Iotx
	case 12:
		targetUnit = unit.Qev
	default:
		targetUnit = unit.Rau
	}
	amountInt, amountDec := big.NewInt(0), big.NewInt(0)
	amountInt.DivMod(amount, big.NewInt(targetUnit), amountDec)
	if amountDec.Sign() != 0 {
		decString := strings.TrimRight(amountDec.String(), "0")
		zeroString := strings.Repeat("0", numDecimals-len(amountDec.String()))
		decString = zeroString + decString
		return amountInt.String() + "." + decString
	}
	return amountInt.String()
}

// IoAddrToEvmAddr converts IoTeX address into evm address
func IoAddrToEvmAddr(ioAddr string) (common.Address, error) {
	if err := validator.ValidateAddress(ioAddr); err != nil {
		return common.Address{}, err
	}
	address, err := address.FromString(ioAddr)
	if err != nil {
		return common.Address{}, err
	}
	return common.BytesToAddress(address.Bytes()), nil
}

// StringToIOTX converts Rau string to Iotx string
func StringToIOTX(amount string) (iotx string, err error) {
	amountInt, err := StringToRau(amount, 0)
	if err != nil {
		return "", err
	}
	iotx = RauToString(amountInt, 18)
	return
}

// ReadSecretFromStdin used to safely get password input
func ReadSecretFromStdin() (string, error) {
	signalListener := make(chan os.Signal, 1)
	signal.Notify(signalListener, os.Interrupt)
	routineTerminate := make(chan struct{})
	sta, err := terminal.GetState(1)
	if err != nil {
		return "", err
	}
	go func() {
		for {
			select {
			case <-signalListener:
				err = terminal.Restore(1, sta)
				if err != nil {
					log.L().Error("failed restore terminal", zap.Error(err))
					return
				}
				os.Exit(130)
			case <-routineTerminate:
				return
			default:
			}
		}
	}()
	bytePass, err := terminal.ReadPassword(int(syscall.Stdin))
	close(routineTerminate)
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	return string(bytePass), nil
}

// GetAddress get address from address or alias
func GetAddress(args []string) (addr string, err error) {
	addr, err = config.GetAddressOrAlias(args)
	if err != nil {
		return
	}
	addr, err = Address(addr)
	return
}

// Address returns the address corresponding to alias. if 'in' is an IoTeX address, returns 'in'
func Address(in string) (string, error) {
	if len(in) >= validator.IoAddrLen {
		if err := validator.ValidateAddress(in); err != nil {
			return "", err
		}
		return in, nil
	}
	addr, ok := config.ReadConfig.Aliases[in]
	if ok {
		return addr, nil
	}
	return "", fmt.Errorf("cannot find address from " + in)
}

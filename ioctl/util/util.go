// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"google.golang.org/grpc/metadata"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	// IotxDecimalNum defines the number of decimal digits for IoTeX
	IotxDecimalNum = 18
	// GasPriceDecimalNum defines the number of decimal digits for gas price
	GasPriceDecimalNum = 12
)

// ExecuteCmd executes cmd with args, and return system output, e.g., help info, and error
func ExecuteCmd(cmd *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	cmd.SetOutput(buf)
	cmd.SetArgs(args)
	_, err := cmd.ExecuteC()

	return buf.String(), err
}

// ConnectToEndpoint starts a new connection
func ConnectToEndpoint(secure bool) (*grpc.ClientConn, error) {
	endpoint := config.ReadConfig.Endpoint
	if endpoint == "" {
		return nil, output.NewError(output.ConfigError, `use "ioctl config set endpoint" to config endpoint first`, nil)
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
			return nil, output.NewError(output.ConvertError, "failed to convert string into big int", nil)
		}
		amountStrings[0] += amountStrings[1]
		numDecimals -= len(amountStrings[1])
	}
	if len(amountStrings[0]) == 0 {
		return nil, output.NewError(output.ConvertError, "failed to convert string into big int", nil)
	}
	zeroString := strings.Repeat("0", numDecimals)
	amountStrings[0] += zeroString
	amountRau, ok := big.NewInt(0).SetString(amountStrings[0], 10)
	if !ok {
		return nil, output.NewError(output.ConvertError, "failed to convert string into big int", nil)
	}
	if amountRau.Sign() < 0 {
		return nil, output.NewError(output.ConvertError, "invalid number that is minus", nil)
	}
	return amountRau, nil
}

// RauToString converts Rau big int into Iotx string
func RauToString(amount *big.Int, numDecimals int) string {
	if numDecimals == 0 {
		return amount.String()
	}
	targetUnit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(numDecimals)), nil)
	amountInt, amountDec := big.NewInt(0), big.NewInt(0)
	amountInt.DivMod(amount, targetUnit, amountDec)
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
		return common.Address{}, output.NewError(output.ValidationError, "", err)
	}
	address, err := address.FromString(ioAddr)
	if err != nil {
		return common.Address{}, output.NewError(output.ConvertError, "", err)
	}
	return common.BytesToAddress(address.Bytes()), nil
}

// StringToIOTX converts Rau string to Iotx string
func StringToIOTX(amount string) (string, error) {
	amountInt, err := StringToRau(amount, 0)
	if err != nil {
		return "", output.NewError(output.ConvertError, "", err)
	}
	return RauToString(amountInt, IotxDecimalNum), nil
}

// ReadSecretFromStdin used to safely get password input
func ReadSecretFromStdin() (string, error) {
	signalListener := make(chan os.Signal, 1)
	signal.Notify(signalListener, os.Interrupt)
	routineTerminate := make(chan struct{})
	sta, err := terminal.GetState(int(syscall.Stdin))
	if err != nil {
		return "", output.NewError(output.RuntimeError, "", err)
	}
	go func() {
		for {
			select {
			case <-signalListener:
				err = terminal.Restore(int(syscall.Stdin), sta)
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
		return "", output.NewError(output.RuntimeError, "failed to read password", nil)
	}
	return string(bytePass), nil
}

// GetAddress get address from address or alias or context
func GetAddress(in string) (string, error) {
	addr, err := config.GetAddressOrAlias(in)
	if err != nil {
		return "", output.NewError(output.AddressError, "", err)
	}
	return Address(addr)
}

// Address returns the address corresponding to alias. if 'in' is an IoTeX address, returns 'in'
func Address(in string) (string, error) {
	if len(in) >= validator.IoAddrLen {
		if err := validator.ValidateAddress(in); err != nil {
			return "", output.NewError(output.ValidationError, "", err)
		}
		return in, nil
	}
	addr, ok := config.ReadConfig.Aliases[in]
	if ok {
		return addr, nil
	}
	return "", output.NewError(output.ConfigError, "cannot find address from "+in, nil)
}

// JwtAuth used for ioctl set auth and send for every grpc request
func JwtAuth() (jwt metadata.MD, err error) {
	jwtFile := os.Getenv("HOME") + "/.config/ioctl/default/auth.jwt"
	jwtString, err := ioutil.ReadFile(jwtFile)
	if err != nil {
		return nil, err
	}
	return metadata.Pairs("authorization", "bearer "+string(jwtString)), nil
}

// CheckArgs used for check ioctl cmd arg(s)'s num
func CheckArgs(validNum ...int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		for _, n := range validNum {
			if len(args) == n {
				return nil
			}
		}
		nums := strings.Replace(strings.Trim(fmt.Sprint(validNum), "[]"), " ", " or ", -1)
		return fmt.Errorf("accepts "+nums+" arg(s), received %d", len(args))
	}
}

// TrimHexPrefix removes 0x prefix from a string if it has
func TrimHexPrefix(s string) string {
	return strings.TrimPrefix(s, "0x")
}

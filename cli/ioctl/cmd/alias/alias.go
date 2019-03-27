// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/validator"
)

// Errors
var (
	ErrNoAliasFound = errors.New("no alias is found")
)

// AliasCmd represents the alias command
var AliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage aliases of IoTeX addresses",
	Args:  cobra.MaximumNArgs(3),
}

func init() {
	AliasCmd.AddCommand(aliasSetCmd)
	AliasCmd.AddCommand(aliasListCmd)
	AliasCmd.AddCommand(aliasRemoveCmd)
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
	return "", fmt.Errorf("cannot find account from #%s", in)
}

// Alias returns the alias corresponding to address
func Alias(address string) (string, error) {
	if err := validator.ValidateAddress(address); err != nil {
		return "", err
	}
	for alias, addr := range config.ReadConfig.Aliases {
		if addr == address {
			return alias, nil
		}
	}
	return "", ErrNoAliasFound
}

// GetAliasMap gets the map from address to alias
func GetAliasMap() map[string]string {
	aliases := make(map[string]string)
	for alias, addr := range config.ReadConfig.Aliases {
		aliases[addr] = alias
	}
	return aliases
}

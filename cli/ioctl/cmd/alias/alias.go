// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/cli/ioctl/validator"
)

// Errors
var (
	ErrNoAliasFound = errors.New("no alias is found")
)

// Flags
var (
	format      string
	forceImport bool
)

type alias struct {
	Name    string `json:"name" yaml:"name"`
	Address string `json:"address" yaml:"address"`
}

type aliases struct {
	Aliases []alias `json:"aliases" yaml:"aliases"`
}

// AliasCmd represents the alias command
var AliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage aliases of IoTeX addresses",
}

func init() {
	AliasCmd.AddCommand(aliasSetCmd)
	AliasCmd.AddCommand(aliasListCmd)
	AliasCmd.AddCommand(aliasRemoveCmd)
	AliasCmd.AddCommand(aliasImportCmd)
	AliasCmd.AddCommand(aliasExportCmd)
}

// IOAddress returns the address in iotex address format
func IOAddress(in string) (address.Address, error) {
	addr, err := util.Address(in)
	if err != nil {
		return nil, err
	}
	return address.FromString(addr)
}

// EtherAddress returns the address in ether format
func EtherAddress(in string) (common.Address, error) {
	addr, err := util.Address(in)
	if err != nil {
		return common.Address{}, err
	}
	return util.IoAddrToEvmAddr(addr)
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

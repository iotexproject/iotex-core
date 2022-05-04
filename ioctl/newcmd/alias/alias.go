// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
)

// Multi-language support
var (
	_aliasCmdShorts = map[config.Language]string{
		config.English: "Manage aliases of IoTeX addresses",
		config.Chinese: "管理IoTeX的地址别名",
	}
	_aliasCmdUses = map[config.Language]string{
		config.English: "alias",
		config.Chinese: "alias",
	}
)

// Errors
var (
	ErrNoAliasFound = errors.New("no alias is found")
)

// Flags
var (
	_format      string
	_forceImport bool
)

//type alias struct {
//	Name    string `json:"name" yaml:"name"`
//	Address string `json:"address" yaml:"address"`
//}
//
//type aliases struct {
//	Aliases []alias `json:"aliases" yaml:"aliases"`
//}

// NewAliasCmd represents the alias command
func NewAliasCmd(client ioctl.Client) *cobra.Command {
	aliasShorts, _ := client.SelectTranslation(_aliasCmdShorts)
	aliasUses, _ := client.SelectTranslation(_aliasCmdUses)

	ec := &cobra.Command{
		Use:   aliasUses,
		Short: aliasShorts,
	}

	ec.AddCommand(NewAliasImportCmd(client))

	return ec
}

// IOAddress returns the address in IoTeX address format
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
	return addrutil.IoAddrToEvmAddr(addr)
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

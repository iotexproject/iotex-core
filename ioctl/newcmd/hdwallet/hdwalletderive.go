// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"fmt"

	ecrypt "github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/go-pkgs/crypto"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_hdwalletDeriveCmdUses = map[config.Language]string{
		config.English: "derive id1/id2/id3",
		config.Chinese: "derive id1/id2/id3",
	}
	_hdwalletDeriveCmdShorts = map[config.Language]string{
		config.English: "derive key from HDWallet",
		config.Chinese: "查询HDWallet钱包的派生key地址",
	}
)

// NewHdwalletDeriveCmd represents the hdwallet derive command
func NewHdwalletDeriveCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_hdwalletDeriveCmdUses)
	short, _ := client.SelectTranslation(_hdwalletDeriveCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			signer := "hdw::" + args[0]
			account, change, index, err := util.ParseHdwPath(signer)
			if err != nil {
				return errors.New("invalid hdwallet key format")
			}

			cmd.Println("Enter password:")
			password, err := client.ReadSecret()
			if err != nil {
				return errors.New("failed to get password")
			}

			addr, _, err := DeriveKey(client, account, change, index, password)
			if err != nil {
				return err
			}
			cmd.Println(fmt.Sprintf("address: %s\n", addr))
			return nil
		},
	}
	return cmd
}

// DeriveKey derives the key according to path
func DeriveKey(client ioctl.Client, account, change, index uint32, password string) (string, crypto.PrivateKey, error) {
	mnemonic, err := client.HdwalletMnemonic(password)
	if err != nil {
		return "", nil, err
	}
	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		return "", nil, err
	}

	derivationPath := fmt.Sprintf("%s/%d'/%d/%d", DefaultRootDerivationPath, account, change, index)
	path := hdwallet.MustParseDerivationPath(derivationPath)
	walletAccount, err := wallet.Derive(path, false)
	if err != nil {
		return "", nil, errors.New("failed to get account by derive path")
	}

	private, err := wallet.PrivateKey(walletAccount)
	if err != nil {
		return "", nil, errors.New("failed to get private key")
	}
	prvKey, err := crypto.BytesToPrivateKey(ecrypt.FromECDSA(private))
	if err != nil {
		return "", nil, errors.New("failed to Bytes private key")
	}

	addr := prvKey.PublicKey().Address()
	if addr == nil {
		return "", nil, errors.New("failed to convert public key into address")
	}
	return addr.String(), prvKey, nil
}

// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"bytes"
	"fmt"
	"os"

	ecrypt "github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/go-pkgs/crypto"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
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

			cmd.Println("Enter password\n")
			password, err := client.ReadSecret()
			if err != nil {
				return errors.New("failed to get password")
			}

			addr, _, err := DeriveKey(account, change, index, password)
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
func DeriveKey(account, change, index uint32, password string) (string, crypto.PrivateKey, error) {
	// derive key as "m/44'/304'/account'/change/index"
	hdWalletConfigFile := config.ReadConfig.Wallet + "/hdwallet"
	if !fileutil.FileExists(hdWalletConfigFile) {
		return "", nil, errors.New("run 'ioctl hdwallet create' to create your HDWallet first")
	}

	enctxt, err := os.ReadFile(hdWalletConfigFile)
	if err != nil {
		return "", nil, errors.New("failed to read config")
	}

	enckey := util.HashSHA256([]byte(password))
	dectxt, err := util.Decrypt(enctxt, enckey)
	if err != nil {
		return "", nil, errors.New("failed to decrypt")
	}

	dectxtLen := len(dectxt)
	if dectxtLen <= 32 {
		return "", nil, errors.New("incorrect data")
	}

	mnemonic, hash := dectxt[:dectxtLen-32], dectxt[dectxtLen-32:]
	if !bytes.Equal(hash, util.HashSHA256(mnemonic)) {
		return "", nil, errors.New("password error")
	}

	wallet, err := hdwallet.NewFromMnemonic(string(mnemonic))
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

// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"bytes"
	"fmt"
	"io/ioutil"

	ecrypt "github.com/ethereum/go-ethereum/crypto"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/spf13/cobra"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

// Multi-language support
var (
	hdwalletDeriveCmdShorts = map[config.Language]string{
		config.English: "derive key from HDWallet",
		config.Chinese: "查询HDWallet钱包的派生key地址",
	}
	hdwalletDeriveCmdUses = map[config.Language]string{
		config.English: "derive id1/id2/id3",
		config.Chinese: "derive id1/id2/id3",
	}
)

// hdwalletDeriveCmd represents the hdwallet derive command
var hdwalletDeriveCmd = &cobra.Command{
	Use:   config.TranslateInLang(hdwalletDeriveCmdUses, config.UILanguage),
	Short: config.TranslateInLang(hdwalletDeriveCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := hdwalletDerive(args[0])
		return output.PrintError(err)
	},
}

func hdwalletDerive(path string) error {
	signer := "hdw::" + path
	account, change, index, err := util.ParseHdwPath(signer)
	if err != nil {
		return output.NewError(output.InputError, "invalid hdwallet key format", err)
	}

	output.PrintQuery("Enter password\n")
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}

	addr, _, err := DeriveKey(account, change, index, password)
	if err != nil {
		return err
	}
	output.PrintResult(fmt.Sprintf("address: %s\n", addr))
	return nil
}

// DeriveKey derives the key according to path
func DeriveKey(account, change, index uint32, password string) (string, crypto.PrivateKey, error) {
	// derive key as "m/44'/304'/account'/change/index"
	hdWalletConfigFile := config.ReadConfig.Wallet + "/hdwallet"
	if !fileutil.FileExists(hdWalletConfigFile) {
		return "", nil, output.NewError(output.InputError, "Run 'ioctl hdwallet create' to create your HDWallet first.", nil)
	}

	enctxt, err := ioutil.ReadFile(hdWalletConfigFile)
	if err != nil {
		return "", nil, output.NewError(output.InputError, "failed to read config", err)
	}

	enckey := util.HashSHA256([]byte(password))
	dectxt, err := util.Decrypt(enctxt, enckey)
	if err != nil {
		return "", nil, output.NewError(output.InputError, "failed to decrypt", err)
	}

	dectxtLen := len(dectxt)
	if dectxtLen <= 32 {
		return "", nil, output.NewError(output.ValidationError, "incorrect data", nil)
	}

	mnemonic, hash := dectxt[:dectxtLen-32], dectxt[dectxtLen-32:]
	if !bytes.Equal(hash, util.HashSHA256(mnemonic)) {
		return "", nil, output.NewError(output.ValidationError, "password error", nil)
	}

	wallet, err := hdwallet.NewFromMnemonic(string(mnemonic))
	if err != nil {
		return "", nil, err
	}

	derivationPath := fmt.Sprintf("%s/%d'/%d/%d", DefaultRootDerivationPath, account, change, index)
	path := hdwallet.MustParseDerivationPath(derivationPath)
	walletAccount, err := wallet.Derive(path, false)
	if err != nil {
		return "", nil, output.NewError(output.InputError, "failed to get account by derive path", err)
	}

	private, err := wallet.PrivateKey(walletAccount)
	if err != nil {
		return "", nil, output.NewError(output.InputError, "failed to get private key", err)
	}
	prvKey, err := crypto.BytesToPrivateKey(ecrypt.FromECDSA(private))
	if err != nil {
		return "", nil, output.NewError(output.InputError, "failed to Bytes private key", err)
	}

	addr := prvKey.PublicKey().Address()
	if addr == nil {
		return "", nil, output.NewError(output.ConvertError, "failed to convert public key into address", nil)
	}
	return addr.String(), prvKey, nil
}

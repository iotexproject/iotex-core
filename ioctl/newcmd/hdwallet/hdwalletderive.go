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
	"github.com/iotexproject/go-pkgs/crypto"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

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

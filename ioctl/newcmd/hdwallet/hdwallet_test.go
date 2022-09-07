// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"encoding/hex"
	"fmt"
	"testing"

	ecrypt "github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/stretchr/testify/require"
	"github.com/tyler-smith/go-bip39"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewHdwalletCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("hdwallet usage", config.English).Times(6)
	cmd := NewHdwalletCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NoError(err)
	require.Contains(result, "Available Commands")
}

func Test_Hdwallet(t *testing.T) {
	require := require.New(t)

	// simulating 'hdwallet create' here
	password := "123"

	entropy, _ := bip39.NewEntropy(128)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	enctxt := append([]byte(mnemonic), util.HashSHA256([]byte(mnemonic))...)
	enckey := util.HashSHA256([]byte(password))

	out, err := util.Encrypt(enctxt, enckey)
	require.NoError(err)

	// simulating 'hdwallet use' here
	enckey = util.HashSHA256([]byte(password))

	dectxt, err := util.Decrypt(out, enckey)
	require.NoError(err)

	dectxtLen := len(dectxt)
	require.True(dectxtLen > 32)

	mnemonic1, hash := dectxt[:dectxtLen-32], dectxt[dectxtLen-32:]

	require.Equal(hash, util.HashSHA256(mnemonic1))

	wallet, err := hdwallet.NewFromMnemonic(string(mnemonic))
	require.NoError(err)

	derivationPath := fmt.Sprintf("%s/0'/%d/%d", DefaultRootDerivationPath, 1, 2)

	path := hdwallet.MustParseDerivationPath(derivationPath)
	account, err := wallet.Derive(path, false)
	require.NoError(err)

	private1, err := wallet.PrivateKey(account)
	require.NoError(err)

	// simulating 'hdwallet import' here
	enctxt = append([]byte(mnemonic), util.HashSHA256([]byte(mnemonic))...)
	enckey = util.HashSHA256([]byte(password))

	_, err = util.Encrypt(enctxt, enckey)
	require.NoError(err)

	// compare import and create
	wallet, err = hdwallet.NewFromMnemonic(string(mnemonic))
	require.NoError(err)

	derivationPath = fmt.Sprintf("%s/0'/%d/%d", DefaultRootDerivationPath, 1, 2)

	path = hdwallet.MustParseDerivationPath(derivationPath)
	account, err = wallet.Derive(path, false)
	require.NoError(err)

	private2, err := wallet.PrivateKey(account)
	require.NoError(err)
	require.Equal(private1, private2)

	//test DeriveKey
	account1 := 0
	change := 1
	index := 2

	derivationPath = fmt.Sprintf("%s/%d'/%d/%d", DefaultRootDerivationPath, account1, change, index)
	path = hdwallet.MustParseDerivationPath(derivationPath)
	account, err = wallet.Derive(path, false)
	require.NoError(err)

	private3, err := wallet.PrivateKey(account)
	require.NoError(err)

	require.Equal(private2, private3)

	account1 = 123
	derivationPath = fmt.Sprintf("%s/%d'/%d/%d", DefaultRootDerivationPath, account1, change, index)
	path = hdwallet.MustParseDerivationPath(derivationPath)
	account, err = wallet.Derive(path, false)
	require.NoError(err)

	private4, err := wallet.PrivateKey(account)
	require.NoError(err)
	require.NotEqual(private2, private4)
}

func TestEncryptDecryptWithMnemonic(t *testing.T) {
	require := require.New(t)

	mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
	password := "123"

	enckey := util.HashSHA256([]byte(password))

	storedTxt, err := hex.DecodeString("4da6571c2897e88568cbcce59dcf9574355d718da25f2ea6f2e6847b9254fc18eacd852d282f1be7b51024fb05e70ee41bc08c0a8a07c549b0c29f185de0fb35462f98b429ebc4b79bfbcab41b795fe1e59262ae0695a4107dcc57a8bad24eb2323b2c976a9fb3dafc8788a9fccbf5b8c36e3388e458")
	require.NoError(err)

	dectxt, err := util.Decrypt(storedTxt, enckey)
	require.NoError(err)

	dectxtLen := len(dectxt)

	mnemonic1, hash := dectxt[:dectxtLen-32], dectxt[dectxtLen-32:]

	require.Equal(mnemonic, string(mnemonic1))
	require.Equal(util.HashSHA256(mnemonic1), hash)
}

func TestFixedMnemonicAndDerivationPath(t *testing.T) {
	require := require.New(t)

	mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	require.NoError(err)

	derivationPath := "m/44'/304'/0'/1/3"

	path := hdwallet.MustParseDerivationPath(derivationPath)
	account, err := wallet.Derive(path, false)
	require.NoError(err)

	private, err := wallet.PrivateKey(account)
	require.NoError(err)
	prvKey, err := crypto.BytesToPrivateKey(ecrypt.FromECDSA(private))
	require.NoError(err)
	addr := prvKey.PublicKey().Address()
	require.NotNil(addr)

	require.Equal(addr.String(), "io13hwqt04le40puf73aa9w9zm9fq04qqn7qcjc6z")

}

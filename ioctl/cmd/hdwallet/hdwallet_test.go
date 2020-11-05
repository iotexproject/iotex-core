// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/util"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/stretchr/testify/require"
	"github.com/tyler-smith/go-bip39"
)

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
	if dectxtLen <= 32 {
		err = fmt.Errorf("incorrect data")
	}
	require.NoError(err)

	mnemonic1, hash := dectxt[:dectxtLen-32], dectxt[dectxtLen-32:]

	if !bytes.Equal(hash, util.HashSHA256(mnemonic1)) {
		err = fmt.Errorf("password error")
	}
	require.NoError(err)

	wallet, err := hdwallet.NewFromMnemonic(string(mnemonic))
	require.NoError(err)

	derivationPath := fmt.Sprintf("%s/%d/%d", DefaultRootDerivationPath[:len(DefaultRootDerivationPath)-2], 1, 2)

	path := hdwallet.MustParseDerivationPath(derivationPath)
	account, err := wallet.Derive(path, false)
	require.NoError(err)

	private, err := wallet.PrivateKey(account)
	require.NoError(err)
	addr1, err := address.FromBytes(hashECDSAPublicKey(&private.PublicKey))
	require.NoError(err)

	// simulating 'hdwallet import' here
	enctxt = append([]byte(mnemonic), util.HashSHA256([]byte(mnemonic))...)
	enckey = util.HashSHA256([]byte(password))

	out, err = util.Encrypt(enctxt, enckey)
	require.NoError(err)

	// compare import and create
	wallet, err = hdwallet.NewFromMnemonic(string(mnemonic))
	require.NoError(err)

	derivationPath = fmt.Sprintf("%s/%d/%d", DefaultRootDerivationPath[:len(DefaultRootDerivationPath)-2], 1, 2)

	path = hdwallet.MustParseDerivationPath(derivationPath)
	account, err = wallet.Derive(path, false)
	require.NoError(err)

	private, err = wallet.PrivateKey(account)
	require.NoError(err)
	addr2, err := address.FromBytes(hashECDSAPublicKey(&private.PublicKey))
	require.NoError(err)

	require.Equal(addr1, addr2)

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

	mnemonic1 := dectxt[:dectxtLen-32]

	require.Equal(mnemonic, string(mnemonic1))

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
	addr, err := address.FromBytes(hashECDSAPublicKey(&private.PublicKey))
	require.NoError(err)

	require.Equal(addr.String(), "io13hwqt04le40puf73aa9w9zm9fq04qqn7qcjc6z")

}

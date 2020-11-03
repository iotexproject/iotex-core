// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"bytes"
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

	password := "123"

	entropy, _ := bip39.NewEntropy(128)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	seed, _ := hdwallet.NewSeedFromMnemonic(mnemonic)

	out, err := util.Encrypt(seed, []byte(password))
	require.NoError(err)

	content := append(out, util.HashSHA256(seed)...)

	cl := len(content)
	enc, hash := content[:cl-32], content[cl-32:]
	seed, err = util.Decrypt(enc, []byte(password))
	require.NoError(err)

	err = nil
	if !bytes.Equal(hash, util.HashSHA256(seed)) {
		err = fmt.Errorf("password error")
	}
	require.NoError(err)

	wallet, err := hdwallet.NewFromSeed(seed)
	require.NoError(err)

	derivationPath := fmt.Sprintf("%s/%d/%d", DefaultRootDerivationPath[:len(DefaultRootDerivationPath)-2], 1, 2)

	path := hdwallet.MustParseDerivationPath(derivationPath)
	account, err := wallet.Derive(path, false)
	require.NoError(err)

	private, err := wallet.PrivateKey(account)
	require.NoError(err)
	addr1, err := address.FromBytes(hashECDSAPublicKey(&private.PublicKey))
	require.NoError(err)

	seed, _ = hdwallet.NewSeedFromMnemonic(mnemonic)

	out, err = util.Encrypt(seed, []byte(password))
	require.NoError(err)

	out = append(out, util.HashSHA256(seed)...)

	wallet, err = hdwallet.NewFromSeed(seed)
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

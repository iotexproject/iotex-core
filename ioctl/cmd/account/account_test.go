// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"crypto/ecdsa"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

const (
	testPath = "kstest"
)

func TestAccount(t *testing.T) {
	require := require.New(t)

	require.NoError(testInit())

	ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
		keystore.StandardScryptN, keystore.StandardScryptP)
	require.NotNil(ks)

	// create accounts
	nonce := strconv.FormatInt(rand.Int63(), 10)
	passwd := "3dj,<>@@SF{}rj0ZF#" + nonce

	account, err := ks.NewAccount(passwd)
	require.NoError(err)
	addr, err := address.FromBytes(account.Address.Bytes())
	require.NoError(err)

	account2, err := crypto.GenerateKeySm2()
	require.NoError(err)
	require.NotNil(account2)
	addr2, err := address.FromBytes(account2.PublicKey().Hash())
	require.NoError(err)
	filePath := filepath.Join(config.ReadConfig.Wallet, "sm2sk-"+addr2.String()+".pem")
	require.NoError(crypto.WritePrivateKeyToPem(filePath, account2.(*crypto.P256sm2PrvKey), passwd))

	// test keystore conversion and signing
	prvKey, err := LocalAccountToPrivateKey(addr.String(), passwd)
	require.NoError(err)
	msg := hash.Hash256b([]byte(nonce))
	sig, err := prvKey.Sign(msg[:])
	require.NoError(err)
	require.True(prvKey.PublicKey().Verify(msg[:], sig))

	prvKey2, err := LocalAccountToPrivateKey(addr2.String(), passwd)
	require.NoError(err)
	msg2 := hash.Hash256b([]byte(nonce))
	sig2, err := prvKey2.Sign(msg2[:])
	require.NoError(err)
	require.True(prvKey2.PublicKey().Verify(msg2[:], sig2))

	// test import existing key
	sk, err := crypto.GenerateKey()
	require.NoError(err)
	p256k1, ok := sk.EcdsaPrivateKey().(*ecdsa.PrivateKey)
	require.Equal(true, ok)
	account, err = ks.ImportECDSA(p256k1, passwd)
	require.NoError(err)
	require.Equal(sk.PublicKey().Hash(), account.Address.Bytes())
}

func testInit() error {
	testPathd, _ := ioutil.TempDir(os.TempDir(), testPath)
	config.ConfigDir = testPathd

	var err error
	config.DefaultConfigFile = config.ConfigDir + "/config.default"
	config.ReadConfig, err = config.LoadConfig()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	config.ReadConfig.Wallet = config.ConfigDir
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return err
	}
	return nil
}

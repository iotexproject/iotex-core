// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
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

	// create an account
	nonce := strconv.FormatInt(rand.Int63(), 10)
	passwd := "3dj,<>@@SF{}rj0ZF#" + nonce
	account, err := ks.NewAccount(passwd)
	require.NoError(err)
	addr, err := address.FromBytes(account.Address.Bytes())
	require.NoError(err)

	// test keystore conversion and signing
	prvkey, err := KsAccountToPrivateKey(addr.String(), passwd)
	require.NoError(err)
	msg := hash.Hash256b([]byte(nonce))
	sig, err := prvkey.Sign(msg[:])
	require.NoError(err)
	require.True(prvkey.PublicKey().Verify(msg[:], sig))
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

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
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

const (
	testPath = "ksTest"
)

func TestAccount(t *testing.T) {
	r := require.New(t)

	testWallet, err := ioutil.TempDir(os.TempDir(), testPath)
	r.NoError(err)
	config.ReadConfig.Wallet = testWallet

	ks := keystore.NewKeyStore(config.ReadConfig.Wallet, keystore.StandardScryptN, keystore.StandardScryptP)
	r.NotNil(ks)

	// create accounts
	nonce := strconv.FormatInt(rand.Int63(), 10)
	passwd := "3dj,<>@@SF{}rj0ZF#" + nonce

	account, err := ks.NewAccount(passwd)
	r.NoError(err)
	addr, err := address.FromBytes(account.Address.Bytes())
	r.NoError(err)
	r.True(IsSignerExist(addr.String()))

	account2, err := crypto.GenerateKeySm2()
	r.NoError(err)
	r.NotNil(account2)
	addr2, err := address.FromBytes(account2.PublicKey().Hash())
	r.NoError(err)
	r.False(IsSignerExist(addr2.String()))
	filePath := sm2KeyPath(addr2)
	addrString, err := storeKey(account2.HexString(), config.ReadConfig.Wallet, passwd)
	r.NoError(err)
	r.Equal(addr2.String(), addrString)
	r.True(IsSignerExist(addr2.String()))
	path, err := findSm2PemFile(addr2)
	r.NoError(err)
	r.Equal(filePath, path)

	accounts, err := listSm2Account()
	r.NoError(err)
	r.Equal(1, len(accounts))
	r.Equal(addr2.String(), accounts[0])

	// test keystore conversion and signing
	prvKey, err := LocalAccountToPrivateKey(addr.String(), passwd)
	r.NoError(err)
	msg := hash.Hash256b([]byte(nonce))
	sig, err := prvKey.Sign(msg[:])
	r.NoError(err)
	r.True(prvKey.PublicKey().Verify(msg[:], sig))

	prvKey2, err := LocalAccountToPrivateKey(addr2.String(), passwd)
	r.NoError(err)
	msg2 := hash.Hash256b([]byte(nonce))
	sig2, err := prvKey2.Sign(msg2[:])
	r.NoError(err)
	r.True(prvKey2.PublicKey().Verify(msg2[:], sig2))

	// test import existing key
	sk, err := crypto.GenerateKey()
	r.NoError(err)
	p256k1, ok := sk.EcdsaPrivateKey().(*ecdsa.PrivateKey)
	r.Equal(true, ok)
	account, err = ks.ImportECDSA(p256k1, passwd)
	r.NoError(err)
	r.Equal(sk.PublicKey().Hash(), account.Address.Bytes())
}

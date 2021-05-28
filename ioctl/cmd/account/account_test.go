// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"crypto/ecdsa"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	testPath = "ksTest"
)

func setupKeyStore(path string) (*keystore.KeyStore, string) {
	testWallet := filepath.Join(os.TempDir(), path)
	config.ReadConfig.Wallet = testWallet
	ks := keystore.NewKeyStore(config.ReadConfig.Wallet, keystore.StandardScryptN, keystore.StandardScryptP)
	return ks, testWallet
}

func TestAccount(t *testing.T) {
	r := require.New(t)

	ks, testWallet := setupKeyStore(testPath)
	defer testutil.CleanupPath(t, testWallet)
	r.NotNil(ks)

	// create accounts
	nonce := strconv.FormatInt(rand.Int63(), 10)
	passwd := "3dj,<>@@SF{}rj0ZF#" + nonce

	account, err := ks.NewAccount(passwd)
	r.NoError(err)
	addr, err := address.FromBytes(account.Address.Bytes())
	r.NoError(err)
	r.True(IsSignerExist(addr.String()))

	t.Log(CryptoSm2)
	CryptoSm2 = true
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
	CryptoSm2 = false
	prvKey, err := LocalAccountToPrivateKey(addr.String(), passwd)
	r.NoError(err)
	msg := hash.Hash256b([]byte(nonce))
	sig, err := prvKey.Sign(msg[:])
	r.NoError(err)
	r.True(prvKey.PublicKey().Verify(msg[:], sig))

	CryptoSm2 = true
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

func TestAccountDelete(t *testing.T) {
	r := require.New(t)

	ks, testWallet := setupKeyStore(testPath)
	r.NotNil(ks)
	input := readInputFromStdin
	readInputFromStdin = func() string {
		return "yes"
	}
	defer func() {
		testutil.CleanupPath(t, testWallet)
		readInputFromStdin = input
	}()
	CryptoSm2 = false
	// create accounts
	nonce := strconv.FormatInt(rand.Int63(), 10)
	passwd := "3dj,<>@@SF{}rj0ZF#" + nonce

	account, err := ks.NewAccount(passwd)
	r.NoError(err)
	addr, err := address.FromBytes(account.Address.Bytes())
	r.NoError(err)
	r.True(IsSignerExist(addr.String()))

	privateKey, err := LocalAccountToPrivateKey(addr.String(), passwd)
	r.NoError(err)

	_, err = storeKey(privateKey.HexString(), config.ReadConfig.Wallet, passwd)
	r.Error(err, "check to store a key that existed.")

	err = accountDelete(addr.String())
	r.NoError(err, "check delete works fine")

	err = accountDelete(addr.String())
	r.Error(err, "check double delete")

	err = accountDelete("NotExistedAccount")
	r.Error(err)

	filepath := getCryptoFile(account.Address, CryptoSm2)
	_, err = os.Stat(filepath)
	r.Equal(true, os.IsNotExist(err), "Check the crypto file has been deleted")

	_, ok := config.ReadConfig.Aliases[addr.String()]
	r.Equal(ok, false, "alias does not remove yet.")

	_, err = storeKey(privateKey.HexString(), config.ReadConfig.Wallet, passwd)
	r.NoError(err, "account does not remove yet.")

	////////////////////test sm2
	CryptoSm2 = true
	sm2PrivateKey, err := crypto.GenerateKeySm2()
	r.NoError(err)
	r.NotNil(sm2PrivateKey)
	sm2Addr, err := address.FromBytes(sm2PrivateKey.PublicKey().Hash())
	r.False(IsSignerExist(sm2Addr.String()))

	_, err = storeKey(sm2PrivateKey.HexString(), config.ReadConfig.Wallet, passwd)
	r.NoError(err, "storing a sm2 private key")

	err = accountDelete(sm2Addr.String())
	r.NoError(err, "delete the sm2 key")

	err = accountDelete(sm2Addr.String())
	r.Error(err, "check double delete")

	sm2CryptoPath := sm2KeyPath(sm2Addr)
	_, err = os.Stat(sm2CryptoPath)
	r.Equal(true, os.IsNotExist(err), "Check the crypto file has been deleted")

	_, ok = config.ReadConfig.Aliases[sm2PrivateKey.HexString()]
	r.Equal(ok, false, "alias does not remove yet.")

	_, err = storeKey(sm2PrivateKey.HexString(), config.ReadConfig.Wallet, passwd)
	r.NoError(err, "account does not remove yet.")
}

// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"crypto/ecdsa"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	testPath = "ksTest"
)

func TestNewAccountCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.ReadConfig).AnyTimes()
	cmd := NewAccountCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.NoError(t, err)
}

func TestSign(t *testing.T) {
	request := require.New(t)
	testWallet := filepath.Join(os.TempDir(), testPath)
	request.NoError(os.MkdirAll(testWallet, os.ModePerm))
	defer testutil.CleanupPath(t, testWallet)
	config.ReadConfig.Wallet = testWallet

	ks := keystore.NewKeyStore(config.ReadConfig.Wallet, keystore.StandardScryptN, keystore.StandardScryptP)
	request.NotNil(ks)

	// create accounts
	nonce := strconv.FormatInt(rand.Int63(), 10)
	passwd := "3dj,<>@@SF{}rj0ZF#" + nonce

	account, err := ks.NewAccount(passwd)
	request.NoError(err)
	addr, err := address.FromBytes(account.Address.Bytes())
	request.NoError(err)
	request.True(IsSignerExist(addr.String()))

	result, err := Sign(addr.String(), passwd, "abcd")
	request.NoError(err)
	request.NotEmpty(result)

	result, err = Sign(addr.String(), passwd, "0xe3a1")
	request.NoError(err)
	request.NotEmpty(result)

	result, err = Sign(addr.String(), passwd, "abc")
	request.Error(err)
	request.Contains(err.Error(), "odd length hex string")
	request.Equal("", result)

	result, err = Sign("hdw::aaaa", passwd, "0xe3a1")
	request.Error(err)
	request.Contains(err.Error(), "invalid HDWallet key format")
	request.Equal("", result)

	result, err = Sign(addr.String(), "123456", "abcd")
	request.Error(err)
	request.Contains(err.Error(), "could not decrypt key with given password")
	request.Equal("", result)

	result, err = Sign("bace9b2435db45b119e1570b4ea9c57993b2311e0c408d743d87cd22838ae892", "123456", "test")
	request.Error(err)
	request.Contains(err.Error(), "invalid address")
	request.Equal("", result)

	prvKey, err := PrivateKeyFromSigner(addr.String(), passwd)
	request.NoError(err)
	request.NotNil(prvKey)

	prvKey, err = PrivateKeyFromSigner(addr.String(), "123456")
	request.Error(err)
	request.Contains(err.Error(), "could not decrypt key with given password")
	request.Nil(prvKey)
}

func TestAccount(t *testing.T) {
	request := require.New(t)
	testWallet := filepath.Join(os.TempDir(), testPath)
	request.NoError(os.MkdirAll(testWallet, os.ModePerm))
	defer testutil.CleanupPath(t, testWallet)
	config.ReadConfig.Wallet = testWallet

	ks := keystore.NewKeyStore(config.ReadConfig.Wallet, keystore.StandardScryptN, keystore.StandardScryptP)
	request.NotNil(ks)

	// create accounts
	nonce := strconv.FormatInt(rand.Int63(), 10)
	passwd := "3dj,<>@@SF{}rj0ZF#" + nonce

	account, err := ks.NewAccount(passwd)
	request.NoError(err)
	addr, err := address.FromBytes(account.Address.Bytes())
	request.NoError(err)
	request.True(IsSignerExist(addr.String()))

	CryptoSm2 = true
	account2, err := crypto.GenerateKeySm2()
	request.NoError(err)
	request.NotNil(account2)
	addr2 := account2.PublicKey().Address()
	request.NotNil(addr2)
	request.False(IsSignerExist(addr2.String()))
	_, err = keyStoreAccountToPrivateKey(addr2.String(), passwd)
	request.Contains(err.Error(), "does not match all local keys")
	filePath := sm2KeyPath(addr2)
	addrString, err := storeKey(account2.HexString(), config.ReadConfig.Wallet, passwd)
	request.NoError(err)
	request.Equal(addr2.String(), addrString)
	request.True(IsSignerExist(addr2.String()))

	// test findSm2PemFile
	path, err := findSm2PemFile(addr2)
	request.NoError(err)
	request.Equal(filePath, path)

	// test listSm2Account
	accounts, err := listSm2Account()
	request.NoError(err)
	request.Equal(1, len(accounts))
	request.Equal(addr2.String(), accounts[0])

	// test keystore conversion and signing
	CryptoSm2 = false
	prvKey, err := keyStoreAccountToPrivateKey(addr.String(), passwd)
	request.NoError(err)
	msg := hash.Hash256b([]byte(nonce))
	sig, err := prvKey.Sign(msg[:])
	request.NoError(err)
	request.True(prvKey.PublicKey().Verify(msg[:], sig))

	CryptoSm2 = true
	prvKey2, err := keyStoreAccountToPrivateKey(addr2.String(), passwd)
	request.NoError(err)
	msg2 := hash.Hash256b([]byte(nonce))
	sig2, err := prvKey2.Sign(msg2[:])
	request.NoError(err)
	request.True(prvKey2.PublicKey().Verify(msg2[:], sig2))

	// test import existing key
	sk, err := crypto.GenerateKey()
	request.NoError(err)
	p256k1, ok := sk.EcdsaPrivateKey().(*ecdsa.PrivateKey)
	request.Equal(true, ok)
	account, err = ks.ImportECDSA(p256k1, passwd)
	request.NoError(err)
	request.Equal(sk.PublicKey().Hash(), account.Address.Bytes())
}

func TestGetAccountMeta(t *testing.T) {
	request := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().Config().Return(config.ReadConfig).AnyTimes()

	apiServiceClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil)

	accAddr := identityset.Address(28).String()
	accountResponse := &iotexapi.GetAccountResponse{AccountMeta: &iotextypes.AccountMeta{
		Address:      accAddr,
		Nonce:        1,
		PendingNonce: 2,
	}}
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResponse, nil)
	result, err := GetAccountMeta(accAddr, client)
	request.NoError(err)
	request.Equal(accountResponse.AccountMeta, result)

	expectedErr := output.NewError(output.NetworkError, "failed to dial grpc connection", nil)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(nil, expectedErr)
	result, err = GetAccountMeta(accAddr, client)
	request.Error(err)
	request.Equal(expectedErr, err)
	request.Nil(result)

	expectedErr = output.NewError(output.NetworkError, "failed to invoke GetAccount api", nil)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil)
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, expectedErr)
	result, err = GetAccountMeta(accAddr, client)
	request.Error(err)
	request.Contains(err.Error(), expectedErr.Error())
	request.Nil(result)
}

func TestAccountError(t *testing.T) {
	request := require.New(t)
	testFilePath := filepath.Join(os.TempDir(), testPath)
	defer testutil.CleanupPath(t, testFilePath)
	alias := "aaa"
	passwordOfKeyStore := "123456"
	keyStorePath := testFilePath
	walletDir := config.ReadConfig.Wallet

	result, err := newAccountByKeyStore(alias, passwordOfKeyStore, keyStorePath, walletDir)
	request.Error(err)
	request.Contains(err.Error(), fmt.Sprintf("keystore file \"%s\" read error", keyStorePath))
	request.Equal("", result)

	asswordOfPem := "abc1234"
	pemFilePath := testFilePath
	result, err = newAccountByPem(alias, asswordOfPem, pemFilePath, walletDir)
	request.Error(err)
	request.Contains(err.Error(), "failed to read private key from pem file")
	request.Equal("", result)

	addr2, err := address.FromString("io1aqazxjx4d6useyhdsq02ah5effg6293wumtldh")
	request.NoError(err)
	path, err := findSm2PemFile(addr2)
	request.Error(err)
	request.Contains(err.Error(), "crypto file not found")
	request.Equal("", path)

	config.ReadConfig.Wallet = ""
	accounts, err := listSm2Account()
	request.Error(err)
	request.Contains(err.Error(), "failed to read files in wallet")
	request.Equal(0, len(accounts))
}

func TestStoreKey(t *testing.T) {
	request := require.New(t)
	testWallet := filepath.Join(os.TempDir(), testPath)
	request.NoError(os.MkdirAll(testWallet, os.ModePerm))
	defer testutil.CleanupPath(t, testWallet)
	config.ReadConfig.Wallet = testWallet

	ks := keystore.NewKeyStore(config.ReadConfig.Wallet, keystore.StandardScryptN, keystore.StandardScryptP)
	request.NotNil(ks)

	// create accounts
	nonce := strconv.FormatInt(rand.Int63(), 10)
	passwd := "3dj,<>@@SF{}rj0ZF#" + nonce

	// test CryptoSm2 is false
	CryptoSm2 = false
	account, err := ks.NewAccount(passwd)
	request.NoError(err)
	addr, err := address.FromBytes(account.Address.Bytes())
	request.NoError(err)
	request.True(IsSignerExist(addr.String()))

	addrString, err := storeKey(account.Address.String(), config.ReadConfig.Wallet, passwd)
	request.Error(err)
	request.Contains(err.Error(), "failed to generate private key from hex string")
	request.Equal("", addrString)

	prvKey, err := keyStoreAccountToPrivateKey(addr.String(), passwd)
	request.NoError(err)
	addrString, err = storeKey(prvKey.HexString(), config.ReadConfig.Wallet, passwd)
	request.Error(err)
	request.Contains(err.Error(), "failed to import private key into keystore")
	request.Equal("", addrString)

	prvKey, err = crypto.GenerateKey()
	request.NoError(err)
	addr = prvKey.PublicKey().Address()
	request.NotNil(addr)
	request.False(IsSignerExist(addr.String()))
	addrString, err = storeKey(prvKey.HexString(), config.ReadConfig.Wallet, passwd)
	request.NoError(err)
	request.Equal(addr.String(), addrString)
	request.True(IsSignerExist(addr.String()))

	// test CryptoSm2 is true
	CryptoSm2 = true
	priKey2, err := crypto.GenerateKeySm2()
	request.NoError(err)
	addr2 := priKey2.PublicKey().Address()
	request.NotNil(addr2)
	request.False(IsSignerExist(addr2.String()))

	pemFilePath := sm2KeyPath(addr2)
	request.NoError(crypto.WritePrivateKeyToPem(pemFilePath, priKey2.(*crypto.P256sm2PrvKey), passwd))
	request.True(IsSignerExist(addr2.String()))

	addrString2, err := storeKey(priKey2.HexString(), config.ReadConfig.Wallet, passwd)
	request.NoError(err)
	request.Equal(addr2.String(), addrString2)
}

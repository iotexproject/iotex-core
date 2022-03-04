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
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
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
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.Config{}).AnyTimes()
	cmd := NewAccountCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NotNil(result)
	require.NoError(err)
}

func TestSign(t *testing.T) {
	require := require.New(t)
	testWallet, ks, passwd, _, err := newTestAccount()
	require.NoError(err)
	defer testutil.CleanupPath(t, testWallet)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().NewKeyStore().Return(ks).Times(15)
	client.EXPECT().IsCryptoSm2().Return(false).Times(15)

	account, err := ks.NewAccount(passwd)
	require.NoError(err)
	addr, err := address.FromBytes(account.Address.Bytes())
	require.NoError(err)
	require.True(IsSignerExist(client, addr.String()))

	result, err := Sign(client, addr.String(), passwd, "abcd")
	require.NoError(err)
	require.NotEmpty(result)

	result, err = Sign(client, addr.String(), passwd, "0xe3a1")
	require.NoError(err)
	require.NotEmpty(result)

	// wrong message
	result, err = Sign(client, addr.String(), passwd, "abc")
	require.Error(err)
	require.Contains(err.Error(), "odd length hex string")
	require.Equal("", result)

	// invalid singer
	result, err = Sign(client, "hdw::aaaa", passwd, "0xe3a1")
	require.Error(err)
	require.Contains(err.Error(), "invalid HDWallet key format")
	require.Equal("", result)

	// wrong password
	result, err = Sign(client, addr.String(), "123456", "abcd")
	require.Error(err)
	require.Contains(err.Error(), "could not decrypt key with given password")
	require.Equal("", result)

	// invalid signer
	result, err = Sign(client, "bace9b2435db45b119e1570b4ea9c57993b2311e0c408d743d87cd22838ae892", "123456", "test")
	require.Error(err)
	require.Contains(err.Error(), "invalid address")
	require.Equal("", result)

	_, err = PrivateKeyFromSigner(client, addr.String(), passwd)
	require.NoError(err)

	// wrong password
	prvKey, err := PrivateKeyFromSigner(client, addr.String(), "123456")
	require.Error(err)
	require.Contains(err.Error(), "could not decrypt key with given password")
	require.Nil(prvKey)

	// empty password
	client.EXPECT().PrintInfo(gomock.Any())
	client.EXPECT().ReadSecret().Return(passwd, nil)
	_, err = PrivateKeyFromSigner(client, addr.String(), "")
	require.NoError(err)
}

func TestAccount(t *testing.T) {
	require := require.New(t)
	testWallet, ks, passwd, nonce, err := newTestAccount()
	defer testutil.CleanupPath(t, testWallet)
	require.NoError(err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)

	t.Run("CryptoSm2 is false", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(false).Times(2)
		client.EXPECT().NewKeyStore().Return(ks).Times(2)

		// test new account by ks
		account, err := ks.NewAccount(passwd)
		require.NoError(err)
		addr, err := address.FromBytes(account.Address.Bytes())
		require.NoError(err)
		require.True(IsSignerExist(client, addr.String()))

		// test keystore conversion and signing
		prvKey, err := keyStoreAccountToPrivateKey(client, addr.String(), passwd)
		require.NoError(err)
		msg := hash.Hash256b([]byte(nonce))
		sig, err := prvKey.Sign(msg[:])
		require.NoError(err)
		require.True(prvKey.PublicKey().Verify(msg[:], sig))

		// test import existing key
		sk, err := crypto.GenerateKey()
		require.NoError(err)
		p256k1, ok := sk.EcdsaPrivateKey().(*ecdsa.PrivateKey)
		require.Equal(true, ok)
		account, err = ks.ImportECDSA(p256k1, passwd)
		require.NoError(err)
		require.Equal(sk.PublicKey().Hash(), account.Address.Bytes())
	})

	t.Run("CryptoSm2 is true", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(true).Times(4)
		client.EXPECT().Config().Return(config.Config{Wallet: testWallet}).Times(8)

		// test store unexisted key
		account2, err := crypto.GenerateKeySm2()
		require.NoError(err)
		require.NotNil(account2)
		addr2 := account2.PublicKey().Address()
		require.NotNil(addr2)
		require.False(IsSignerExist(client, addr2.String()))
		_, err = keyStoreAccountToPrivateKey(client, addr2.String(), passwd)
		require.Contains(err.Error(), "does not match all local keys")
		filePath := sm2KeyPath(client, addr2)
		addrString, err := storeKey(client, account2.HexString(), passwd)
		require.NoError(err)
		require.Equal(addr2.String(), addrString)
		require.True(IsSignerExist(client, addr2.String()))

		// test findSm2PemFile
		path, err := findSm2PemFile(client, addr2)
		require.NoError(err)
		require.Equal(filePath, path)

		// test listSm2Account
		accounts, err := listSm2Account(client)
		require.NoError(err)
		require.Equal(1, len(accounts))
		require.Equal(addr2.String(), accounts[0])

		// test keyStoreAccountToPrivateKey
		prvKey2, err := keyStoreAccountToPrivateKey(client, addr2.String(), passwd)
		require.NoError(err)
		msg2 := hash.Hash256b([]byte(nonce))
		sig2, err := prvKey2.Sign(msg2[:])
		require.NoError(err)
		require.True(prvKey2.PublicKey().Verify(msg2[:], sig2))
	})
}

func TestMeta(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().Config().Return(config.Config{}).AnyTimes()

	apiServiceClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil)

	accAddr := identityset.Address(28).String()
	accountResponse := &iotexapi.GetAccountResponse{AccountMeta: &iotextypes.AccountMeta{
		Address:      accAddr,
		Nonce:        1,
		PendingNonce: 2,
	}}
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResponse, nil)
	result, err := Meta(client, accAddr)
	require.NoError(err)
	require.Equal(accountResponse.AccountMeta, result)

	expectedErr := errors.New("failed to dial grpc connection")
	client.EXPECT().APIServiceClient(gomock.Any()).Return(nil, expectedErr)
	result, err = Meta(client, accAddr)
	require.Error(err)
	require.Equal(expectedErr, err)
	require.Nil(result)

	expectedErr = errors.New("failed to invoke GetAccount api")
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil)
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, expectedErr)
	result, err = Meta(client, accAddr)
	require.Error(err)
	require.Contains(err.Error(), expectedErr.Error())
	require.Nil(result)
}

func TestAccountError(t *testing.T) {
	require := require.New(t)
	testFilePath := filepath.Join(os.TempDir(), testPath)
	defer testutil.CleanupPath(t, testFilePath)
	alias := "aaa"
	passwordOfKeyStore := "123456"
	keyStorePath := testFilePath

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	testWallet, _, _, _, _ := newTestAccount()
	defer testutil.CleanupPath(t, testWallet)

	client.EXPECT().DecryptPrivateKey(gomock.Any(), gomock.Any()).DoAndReturn(
		func(passwordOfKeyStore, keyStorePath string) (*ecdsa.PrivateKey, error) {
			_, err := os.ReadFile(keyStorePath)
			require.Error(err)
			return nil, fmt.Errorf("keystore file \"%s\" read error", keyStorePath)
		})
	result, err := newAccountByKeyStore(client, alias, passwordOfKeyStore, keyStorePath)
	require.Error(err)
	require.Contains(err.Error(), fmt.Sprintf("keystore file \"%s\" read error", keyStorePath))
	require.Equal("", result)

	asswordOfPem := "abc1234"
	pemFilePath := testFilePath
	result, err = newAccountByPem(client, alias, asswordOfPem, pemFilePath)
	require.Error(err)
	require.Contains(err.Error(), "failed to read private key from pem file")
	require.Equal("", result)

	addr2, err := address.FromString("io1aqazxjx4d6useyhdsq02ah5effg6293wumtldh")
	require.NoError(err)
	client.EXPECT().Config().Return(config.Config{Wallet: testWallet}).Times(1)
	path, err := findSm2PemFile(client, addr2)
	require.Error(err)
	require.Contains(err.Error(), "crypto file not found")
	require.Equal("", path)

	client.EXPECT().Config().Return(config.Config{Wallet: ""}).Times(1)
	accounts, err := listSm2Account(client)
	require.Error(err)
	require.Contains(err.Error(), "failed to read files in wallet")
	require.Equal(0, len(accounts))
}

func TestStoreKey(t *testing.T) {
	require := require.New(t)
	testWallet, ks, passwd, _, err := newTestAccount()
	require.NoError(err)
	defer testutil.CleanupPath(t, testWallet)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)

	t.Run("CryptoSm2 is false", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(false).Times(4)
		client.EXPECT().NewKeyStore().Return(ks).Times(6)

		account, err := ks.NewAccount(passwd)
		require.NoError(err)
		addr, err := address.FromBytes(account.Address.Bytes())
		require.NoError(err)
		require.True(IsSignerExist(client, addr.String()))

		// invalid private key
		addrString, err := storeKey(client, account.Address.String(), passwd)
		require.Error(err)
		require.Contains(err.Error(), "failed to generate private key from hex string")
		require.Equal("", addrString)

		// valid private key
		prvKey, err := keyStoreAccountToPrivateKey(client, addr.String(), passwd)
		require.NoError(err)
		// import the existed account addr
		addrString, err = storeKey(client, prvKey.HexString(), passwd)
		require.Error(err)
		require.Contains(err.Error(), "failed to import private key into keystore")
		require.Equal("", addrString)

		// import the unexisted account addr
		prvKey, err = crypto.GenerateKey()
		require.NoError(err)
		addr = prvKey.PublicKey().Address()
		require.NotNil(addr)
		require.False(IsSignerExist(client, addr.String()))
		addrString, err = storeKey(client, prvKey.HexString(), passwd)
		require.NoError(err)
		require.Equal(addr.String(), addrString)
		require.True(IsSignerExist(client, addr.String()))
	})

	t.Run("CryptoSm2 is true", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(true).Times(2)
		client.EXPECT().Config().Return(config.Config{Wallet: testWallet}).Times(4)

		priKey2, err := crypto.GenerateKeySm2()
		require.NoError(err)
		addr2 := priKey2.PublicKey().Address()
		require.NotNil(addr2)
		require.False(IsSignerExist(client, addr2.String()))

		pemFilePath := sm2KeyPath(client, addr2)
		require.NoError(crypto.WritePrivateKeyToPem(pemFilePath, priKey2.(*crypto.P256sm2PrvKey), passwd))
		require.True(IsSignerExist(client, addr2.String()))

		addrString2, err := storeKey(client, priKey2.HexString(), passwd)
		require.NoError(err)
		require.Equal(addr2.String(), addrString2)
	})
}

func TestNewAccount(t *testing.T) {
	require := require.New(t)
	testWallet, ks, passwd, _, err := newTestAccount()
	require.NoError(err)
	defer testutil.CleanupPath(t, testWallet)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().PrintInfo(gomock.Any()).Times(2)
	client.EXPECT().ReadSecret().Return(passwd, nil).Times(2)
	client.EXPECT().NewKeyStore().Return(ks)

	_, err = newAccount(client, "alias1234")
	require.NoError(err)
}

func TestNewAccountSm2(t *testing.T) {
	require := require.New(t)
	testWallet, _, passwd, _, err := newTestAccount()
	require.NoError(err)
	defer testutil.CleanupPath(t, testWallet)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().PrintInfo(gomock.Any()).Times(2)
	client.EXPECT().ReadSecret().Return(passwd, nil).Times(2)
	client.EXPECT().Config().Return(config.Config{Wallet: testWallet}).Times(1)

	_, err = newAccountSm2(client, "alias1234")
	require.NoError(err)
}

func TestNewAccountByKey(t *testing.T) {
	require := require.New(t)
	testWallet, ks, passwd, _, err := newTestAccount()
	require.NoError(err)
	defer testutil.CleanupPath(t, testWallet)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().PrintInfo(gomock.Any()).Times(2)
	client.EXPECT().ReadSecret().Return(passwd, nil).Times(2)
	client.EXPECT().NewKeyStore().Return(ks)

	prvKey, err := crypto.GenerateKey()
	require.NoError(err)
	_, err = newAccountByKey(client, "alias1234", prvKey.HexString())
	require.NoError(err)
}

func newTestAccount() (string, *keystore.KeyStore, string, string, error) {
	testWallet := filepath.Join(os.TempDir(), testPath)
	if err := os.MkdirAll(testWallet, os.ModePerm); err != nil {
		return testWallet, nil, "", "", err
	}

	// create accounts
	ks := keystore.NewKeyStore(testWallet, keystore.StandardScryptN, keystore.StandardScryptP)
	nonce := strconv.FormatInt(rand.Int63(), 10)
	passwd := "3dj,<>@@SF{}rj0ZF#" + nonce
	return testWallet, ks, passwd, nonce, nil
}

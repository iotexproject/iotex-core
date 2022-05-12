// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"crypto/ecdsa"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestNewAccountImportCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("key", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.Config{}).AnyTimes()

	cmd := NewAccountImportCmd(client)
	_, err := util.ExecuteCmd(cmd, "key", "-h")
	require.NoError(err)
	_, err = util.ExecuteCmd(cmd, "hhcmd", "-h")
	require.Contains(err.Error(), "unknown command")
}

func TestNewAccountImportKeyCmd(t *testing.T) {
	require := require.New(t)
	testWallet, ks, _, _, err := newTestAccountWithKeyStore(veryLightScryptN, veryLightScryptP)
	require.NoError(err)
	defer testutil.CleanupPath(testWallet)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()

	prvKey, err := crypto.GenerateKey()
	require.NoError(err)

	client.EXPECT().ReadSecret().Return(prvKey.HexString(), nil).AnyTimes()
	client.EXPECT().NewKeyStore().Return(ks).Times(1)
	client.EXPECT().Config().Return(config.Config{Wallet: testWallet}).Times(1)
	client.EXPECT().SetAliasAndSave(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	cmd := NewAccountImportKeyCmd(client)
	result, err := util.ExecuteCmd(cmd, "hhalias")
	require.NoError(err)
	require.Contains(result, "hhalias: Enter your private key")
}

func TestNewAccountImportKeyStoreCmd(t *testing.T) {
	require := require.New(t)
	testWallet, ks, passwd, _, err := newTestAccountWithKeyStore(veryLightScryptN, veryLightScryptP)
	require.NoError(err)
	defer testutil.CleanupPath(testWallet)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
	client.EXPECT().ReadSecret().Return(passwd, nil).AnyTimes()
	client.EXPECT().NewKeyStore().Return(ks).Times(1)
	client.EXPECT().Config().Return(config.Config{Wallet: testWallet}).Times(1)
	client.EXPECT().SetAliasAndSave(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	client.EXPECT().DecryptPrivateKey(gomock.Any(), gomock.Any()).DoAndReturn(
		func(passwordOfKeyStore, keyStorePath string) (*ecdsa.PrivateKey, error) {
			sk, err := crypto.GenerateKey()
			require.NoError(err)
			p256k1, ok := sk.EcdsaPrivateKey().(*ecdsa.PrivateKey)
			require.True(ok)
			return p256k1, nil
		})

	cmd := NewAccountImportKeyStoreCmd(client)
	result, err := util.ExecuteCmd(cmd, "hhalias", testWallet)
	require.NoError(err)
	require.Contains(result, "hhalias: Enter your password of keystore")
}

func TestNewAccountImportPemCmd(t *testing.T) {
	require := require.New(t)
	testWallet, passwd, _, err := newTestAccount()
	require.NoError(err)
	defer testutil.CleanupPath(testWallet)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.Config{Wallet: testWallet}).Times(3)

	priKey2, err := crypto.GenerateKeySm2()
	require.NoError(err)
	addr2 := priKey2.PublicKey().Address()
	pemFilePath := sm2KeyPath(client, addr2)
	require.NoError(crypto.WritePrivateKeyToPem(pemFilePath, priKey2.(*crypto.P256sm2PrvKey), passwd))

	client.EXPECT().ReadSecret().Return(passwd, nil).AnyTimes()
	client.EXPECT().SetAliasAndSave(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	cmd := NewAccountImportPemCmd(client)
	result, err := util.ExecuteCmd(cmd, "hhalias", pemFilePath)
	require.NoError(err)
	require.Contains(result, "hhalias: Enter your password of pem file")
}

func TestValidateAlias(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().Config().Return(config.Config{
		Aliases: map[string]string{
			"aaa": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"bbb": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
		},
	}).Times(2)

	err := validateAlias(client, "ccc")
	require.NoError(err)

	err = validateAlias(client, "aaa")
	require.Contains(err.Error(), `alias "aaa" has already used for`)

	err = validateAlias(client, strings.Repeat("a", 50))
	require.Contains(err.Error(), "invalid long alias that is more than 40 characters")
}

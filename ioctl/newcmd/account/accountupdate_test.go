// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountUpdate_FindKeystore(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	testAccountFolder := t.TempDir()
	ks := keystore.NewKeyStore(testAccountFolder, veryLightScryptN, veryLightScryptP)
	client.EXPECT().NewKeyStore().Return(ks).AnyTimes()
	const pwd = "test"
	acc, err := ks.NewAccount(pwd)
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)
	client.EXPECT().IsCryptoSm2().Return(false).Times(3)

	t.Run("invalid_current_password", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).Times(1)
		client.EXPECT().ReadSecret().Return("1234", nil).Times(1)
		cmd := NewAccountUpdate(client)
		_, err := util.ExecuteCmd(cmd)
		require.Equal("error occurs when checking current password: could not decrypt key with given password", err.Error())
	})

	t.Run("new_password_not_match", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).Times(1)
		client.EXPECT().ReadSecret().Return(pwd, nil).Times(1)
		client.EXPECT().ReadSecret().Return("1234", nil).Times(1)
		client.EXPECT().ReadSecret().Return("12345", nil).Times(1)
		cmd := NewAccountUpdate(client)
		_, err := util.ExecuteCmd(cmd)
		require.Equal(ErrPasswdNotMatch, err)
	})

	t.Run("success", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).Times(1)
		client.EXPECT().ReadSecret().Return(pwd, nil).Times(1)
		client.EXPECT().ReadSecret().Return("1234", nil).Times(1)
		client.EXPECT().ReadSecret().Return("1234", nil).Times(1)
		cmd := NewAccountUpdate(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, fmt.Sprintf("Account #%s has been updated.", accAddr.String()))
	})
}

func TestNewAccountUpdate_FindPemFile(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	testAccountFolder := t.TempDir()
	ks := keystore.NewKeyStore(testAccountFolder, veryLightScryptN, veryLightScryptP)
	client.EXPECT().NewKeyStore().Return(ks).AnyTimes()
	const pwd = "test"
	acc, err := ks.NewAccount(pwd)
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)

	client.EXPECT().Config().Return(config.Config{Wallet: testAccountFolder}).Times(4)
	skPemPath := sm2KeyPath(client, accAddr)
	sk, err := crypto.GenerateKeySm2()
	require.NoError(err)
	k, ok := sk.EcdsaPrivateKey().(*crypto.P256sm2PrvKey)
	require.True(ok)
	require.NoError(crypto.WritePrivateKeyToPem(skPemPath, k, pwd))
	client.EXPECT().IsCryptoSm2().Return(true).Times(3)

	t.Run("invalid_current_password", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).Times(1)
		client.EXPECT().ReadSecret().Return("1234", nil).Times(1)
		cmd := NewAccountUpdate(client)
		_, err := util.ExecuteCmd(cmd)
		require.Equal("error occurs when checking current password: pkcs8: incorrect password", err.Error())
	})

	t.Run("new_password_not_match", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).Times(1)
		client.EXPECT().ReadSecret().Return(pwd, nil).Times(1)
		client.EXPECT().ReadSecret().Return("1234", nil).Times(1)
		client.EXPECT().ReadSecret().Return("12345", nil).Times(1)
		cmd := NewAccountUpdate(client)
		_, err := util.ExecuteCmd(cmd)
		require.Equal(ErrPasswdNotMatch, err)
	})

	t.Run("success", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).Times(1)
		client.EXPECT().ReadSecret().Return(pwd, nil).Times(1)
		client.EXPECT().ReadSecret().Return("1234", nil).Times(1)
		client.EXPECT().ReadSecret().Return("1234", nil).Times(1)
		cmd := NewAccountUpdate(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, fmt.Sprintf("Account #%s has been updated.", accAddr.String()))
	})
}

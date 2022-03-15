// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountExportPublic(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	testAccountFolder := filepath.Join(os.TempDir(), "testNewAccountExportPublic")
	require.NoError(os.MkdirAll(testAccountFolder, os.ModePerm))
	defer func() {
		require.NoError(os.RemoveAll(testAccountFolder))
	}()
	ks := keystore.NewKeyStore(testAccountFolder, keystore.StandardScryptN, keystore.StandardScryptP)
	client.EXPECT().NewKeyStore().Return(ks).AnyTimes()

	t.Run("true AliasIsHdwalletKey", func(t *testing.T) {
		client.EXPECT().ReadSecret().Return("", nil).Times(1)
		cmd := NewAccountExportPublic(client)
		_, err := util.ExecuteCmd(cmd, "hdw::1234")
		require.Contains(err.Error(), "failed to get private key from keystore")
	})

	t.Run("false AliasIsHdwalletKey", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(false).Times(2)
		acc, err := ks.NewAccount("")
		require.NoError(err)
		accAddr, err := address.FromBytes(acc.Address.Bytes())
		require.NoError(err)
		client.EXPECT().ReadSecret().Return("", nil).Times(1)
		client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil).Times(2)
		cmd := NewAccountExportPublic(client)
		result, err := util.ExecuteCmd(cmd, "1234")
		require.NoError(err)
		require.Contains(result, accAddr.String())
	})

	t.Run("invalid address", func(t *testing.T) {
		client.EXPECT().Address(gomock.Any()).Return("", errors.New("mock error")).Times(1)
		cmd := NewAccountExportPublic(client)
		_, err := util.ExecuteCmd(cmd, "1234")
		require.Contains(err.Error(), "failed to get address")
	})
}

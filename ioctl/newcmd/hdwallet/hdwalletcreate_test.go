// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestNewHdwalletCreateCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	password := "123"

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(9)
	client.EXPECT().IsHdWalletConfigFileExist().Return(false).Times(3)

	t.Run("create hdwallet", func(t *testing.T) {
		client.EXPECT().ReadSecret().Return(password, nil)
		client.EXPECT().ReadSecret().Return(password, nil)
		client.EXPECT().WriteHdWalletConfigFile(gomock.Any(), gomock.Any()).Return(nil)

		cmd := NewHdwalletCreateCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, "It is used to recover your wallet in case you forgot the password. Write them down and store it in a safe place.")
	})

	t.Run("failed to get password", func(t *testing.T) {
		expectedErr := errors.New("failed to get password")
		client.EXPECT().ReadSecret().Return("", expectedErr)

		cmd := NewHdwalletCreateCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("password not match", func(t *testing.T) {
		expectedErr := errors.New("password doesn't match")
		client.EXPECT().ReadSecret().Return(password, nil)
		client.EXPECT().ReadSecret().Return("test", nil)

		cmd := NewHdwalletCreateCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Equal(err.Error(), expectedErr.Error())
	})
}

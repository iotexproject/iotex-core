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
	"github.com/stretchr/testify/require"
)

func TestNewHdwalletDeleteCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(4)

	t.Run("delete hdwallet", func(t *testing.T) {
		createClient := mock_ioctlclient.NewMockClient(ctrl)
		password := "123"

		createClient.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(9)
		createClient.EXPECT().IsHdWalletConfigFileExist().Return(false).Times(3)
		createClient.EXPECT().ReadSecret().Return(password, nil).Times(2)
		createClient.EXPECT().WriteHdWalletConfigFile(gomock.Any(), gomock.Any()).Return(nil)

		client.EXPECT().AskToConfirm(gomock.Any()).Return(true)
		client.EXPECT().RemoveHdWalletConfigFile().Return(nil)

		cmd := NewHdwalletCreateCmd(createClient)
		_, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		cmd = NewHdwalletDeleteCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Equal("", result)
	})

	t.Run("quit hdwallet delete command", func(t *testing.T) {
		client.EXPECT().AskToConfirm(gomock.Any()).Return(false)

		cmd := NewHdwalletDeleteCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Equal("quit\n", result)
	})
}

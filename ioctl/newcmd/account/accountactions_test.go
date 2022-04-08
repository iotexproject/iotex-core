// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/stretchr/testify/require"
)

func TestNewAccountAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString",
		config.English).AnyTimes()

	t.Run("failed to send request", func(t *testing.T) {
		accAddr := identityset.Address(28).String()
		client.EXPECT().Config().Return(config.Config{})
		client.EXPECT().Address(gomock.Any()).Return(accAddr, nil)
		cmd := NewAccountActionsCmd(client)
		_, err := util.ExecuteCmd(cmd, accAddr, "0")
		require.Error(err)
		require.Contains(err.Error(), "failed to send request")
	})

	client.EXPECT().Config().Return(config.Config{
		AnalyserEndpoint: "https://iotex-analyser-api-mainnet.chainanalytics.org",
	}).AnyTimes()

	t.Run("get account action", func(t *testing.T) {
		accAddr := identityset.Address(28).String()
		client.EXPECT().Address(gomock.Any()).Return(accAddr, nil)
		cmd := NewAccountActionsCmd(client)
		_, err := util.ExecuteCmd(cmd, accAddr, "0")
		require.NoError(err)
	})

	t.Run("empty offset", func(t *testing.T) {
		accAddr := identityset.Address(28).String()
		cmd := NewAccountActionsCmd(client)
		_, err := util.ExecuteCmd(cmd, accAddr, "")
		require.Error(err)
	})

	t.Run("empty address", func(t *testing.T) {
		client.EXPECT().Address(gomock.Any()).Return("", nil)
		cmd := NewAccountActionsCmd(client)
		_, err := util.ExecuteCmd(cmd, "", "0")
		require.Error(err)
	})
}

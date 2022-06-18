// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewNodeRewardCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(24)

	apiClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)

	var endpoint string
	var insecure bool

	client.EXPECT().APIServiceClient(ioctl.APIServiceConfig{
		Endpoint: endpoint,
		Insecure: insecure,
	}).Return(apiClient, nil).Times(7)

	t.Run("get node reward pool", func(t *testing.T) {
		t.Run("get available reward & total reward", func(t *testing.T) {
			apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
				ProtocolID: []byte("rewarding"),
				MethodName: []byte("AvailableBalance"),
			}).Return(&iotexapi.ReadStateResponse{
				Data: []byte("24361490367906930338205776")},
				nil)

			apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
				ProtocolID: []byte("rewarding"),
				MethodName: []byte("TotalBalance"),
			}).Return(&iotexapi.ReadStateResponse{
				Data: []byte("52331682309272536203174665")},
				nil)

			cmd := NewNodeRewardCmd(client)
			result, err := util.ExecuteCmd(cmd, "pool")
			require.NoError(err)
			require.Contains(result, "24361490.367906930338205776")
			require.Contains(result, "52331682.309272536203174665")
		})

		t.Run("failed to invoke AvailableBalance api", func(t *testing.T) {
			expectedErr := errors.New("failed to invoke ReadState api")

			apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
				ProtocolID: []byte("rewarding"),
				MethodName: []byte("AvailableBalance"),
			}).Return(nil, expectedErr)

			cmd := NewNodeRewardCmd(client)
			_, err := util.ExecuteCmd(cmd, "pool")
			require.Contains(err.Error(), expectedErr.Error())
		})

		t.Run("failed to convert string into big int", func(t *testing.T) {
			expectedErr := errors.New("failed to convert string into big int")

			apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
				ProtocolID: []byte("rewarding"),
				MethodName: []byte("AvailableBalance"),
			}).Return(&iotexapi.ReadStateResponse{
				Data: []byte("0x24361490367906930338205776")},
				nil)

			cmd := NewNodeRewardCmd(client)
			_, err := util.ExecuteCmd(cmd, "pool")
			require.Contains(err.Error(), expectedErr.Error())
		})

		t.Run("failed to invoke TotalBalance api", func(t *testing.T) {
			expectedErr := errors.New("failed to invoke ReadState api")

			apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
				ProtocolID: []byte("rewarding"),
				MethodName: []byte("AvailableBalance"),
			}).Return(&iotexapi.ReadStateResponse{
				Data: []byte("24361490367906930338205776")},
				nil)

			apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
				ProtocolID: []byte("rewarding"),
				MethodName: []byte("TotalBalance"),
			}).Return(nil, expectedErr)

			cmd := NewNodeRewardCmd(client)
			_, err := util.ExecuteCmd(cmd, "pool")
			require.Contains(err.Error(), expectedErr.Error())
		})
	})

	t.Run("get unclaimed node reward", func(t *testing.T) {
		t.Run("get balance by address", func(t *testing.T) {
			client.EXPECT().Address(gomock.Any()).Return("test_address", nil).Times(1)

			apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
				ProtocolID: []byte("rewarding"),
				MethodName: []byte("UnclaimedBalance"),
				Arguments:  [][]byte{[]byte("test_address")},
			}).Return(&iotexapi.ReadStateResponse{
				Data: []byte("0"),
			}, nil)

			cmd := NewNodeRewardCmd(client)
			result, err := util.ExecuteCmd(cmd, "unclaimed", "test")
			require.NoError(err)
			require.Contains(result, "test_address: 0 IOTX")
		})

		t.Run("failed to get address", func(t *testing.T) {
			expectedErr := errors.New("failed to get address")
			client.EXPECT().Address(gomock.Any()).Return("", expectedErr)

			cmd := NewNodeRewardCmd(client)
			_, err := util.ExecuteCmd(cmd, "unclaimed", "test")
			require.Contains(err.Error(), expectedErr.Error())
		})

		t.Run("failed to get version from server", func(t *testing.T) {
			expectedErr := errors.New("failed to get version from server")

			client.EXPECT().Address(gomock.Any()).Return("test_address", nil).Times(1)

			apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
				ProtocolID: []byte("rewarding"),
				MethodName: []byte("UnclaimedBalance"),
				Arguments:  [][]byte{[]byte("test_address")},
			}).Return(nil, expectedErr)

			cmd := NewNodeRewardCmd(client)
			_, err := util.ExecuteCmd(cmd, "unclaimed", "test")
			require.Contains(err.Error(), expectedErr.Error())
		})
	})

	t.Run("unknown command", func(t *testing.T) {
		expectedErr := errors.New("unknown command. \nRun 'ioctl node reward --help' for usage")

		cmd := NewNodeRewardCmd(client)
		_, err := util.ExecuteCmd(cmd, "")
		require.Contains(err.Error(), expectedErr.Error())
	})
}

// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package action

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

func TestNewActionSendRawCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(8)

	t.Run("action send raw", func(t *testing.T) {
		client.EXPECT().APIServiceClient().Return(apiServiceClient, nil)
		client.EXPECT().Config().Return(config.Config{
			Explorer: "iotexscan",
			Endpoint: "testnet1",
		}).Times(2)
		apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(&iotexapi.SendActionResponse{}, nil)
		actBytes := "0a12080118a08d0622023130280162040a023130124104dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b1a41328c6912fa0e36414c38089c03e2fa8c88bba82ccc4ce5fb8ac4ef9f529dfce249a5b2f93a45b818e7f468a742b4e87be3b8077f95d1b3c49e9165b971848ead01"
		cmd := NewActionSendRawCmd(client)
		_, err := util.ExecuteCmd(cmd, actBytes)
		require.NoError(err)
	})

	t.Run("failed to unmarshal data bytes", func(t *testing.T) {
		expectedErr := errors.New("failed to unmarshal data bytes")
		cmd := NewActionSendRawCmd(client)
		_, err := util.ExecuteCmd(cmd, "02e940dd0fd5b5df4cfb8d6bcd9c74ec433e9a5c21acb72cbcb5be9e711b678f")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to decode data", func(t *testing.T) {
		expectedErr := errors.New("failed to decode data")
		cmd := NewActionSendRawCmd(client)
		_, err := util.ExecuteCmd(cmd, "test")
		require.Contains(err.Error(), expectedErr.Error())
	})
}

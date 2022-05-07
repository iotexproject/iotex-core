// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package version

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestVersionCommand(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	cfg := config.Config{}
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	response := iotexapi.GetServerMetaResponse{
		ServerMeta: &iotextypes.ServerMeta{PackageVersion: "1.0"},
	}

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(12)
	endpoint := "111:222:333:444:5678"
	insecure := true
	callbackEndpoint := func(cb func(*string, string, string, string), usage string) {
		cb(&endpoint, "endpoint", endpoint, usage)
	}
	callbackInsecure := func(cb func(*bool, string, bool, string), usage string) {
		cb(&insecure, "insecure", !insecure, usage)
	}
	client.EXPECT().SetEndpointWithFlag(gomock.Any(), gomock.Any()).Do(callbackEndpoint).Times(3)
	client.EXPECT().SetInsecureWithFlag(gomock.Any(), gomock.Any()).Do(callbackInsecure).Times(3)
	client.EXPECT().Config().Return(cfg).Times(1)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(2)

	t.Run("get ioctl version", func(t *testing.T) {
		expectedValue := "packageVersion:\"1.0\""
		apiServiceClient.EXPECT().GetServerMeta(gomock.Any(), gomock.Any()).Return(&response, nil).Times(1)

		cmd := NewVersionCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, expectedValue)
	})

	t.Run("failed to get version from server", func(t *testing.T) {
		expectedErr := errors.New("failed to get version from server")
		apiServiceClient.EXPECT().GetServerMeta(gomock.Any(), gomock.Any()).Return(nil, expectedErr).Times(1)

		cmd := NewVersionCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Error(err)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("use \"ioctl config set endpoint\" to config endpoint first", func(t *testing.T) {
		expectedErr := errors.New("use \"ioctl config set endpoint\" to config endpoint first")
		client.EXPECT().APIServiceClient().Return(nil, expectedErr).Times(1)
		apiServiceClient.EXPECT().GetServerMeta(gomock.Any(), gomock.Any()).Return(&response, nil).Times(1)

		cmd := NewVersionCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Error(err)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

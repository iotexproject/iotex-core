// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package version

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestVersionCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(2)
	cfg := config.Config{}
	client.EXPECT().Config().Return(cfg).Times(2)
	apiClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	response := iotexapi.GetServerMetaResponse{
		ServerMeta: &iotextypes.ServerMeta{PackageVersion: "1.0"},
	}
	apiClient.EXPECT().GetServerMeta(gomock.Any(), gomock.Any()).Return(&response, nil).Times(1)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiClient, nil).Times(1)
	cmd := NewVersionCmd(client)
	err := util.ExecuteCmd(cmd)
	require.NoError(t, err)
}

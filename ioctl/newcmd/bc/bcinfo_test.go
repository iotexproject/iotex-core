// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

// test for bc info command
func TestNewBCInfoCmd(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", ioctl.English).Times(3)
	client.EXPECT().Config().Return(config.ReadConfig).Times(3)

	apiServiceClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(1)

	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)

	cmd := NewBCInfoCmd(client)
	_, err := util.ExecuteCmd(cmd)
	require.NoError(t, err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", ioctl.English).Times(3)
	client.EXPECT().Config().Return(config.ReadConfig).Times(2)

	expectedErr := errors.New("failed to dial grpc connection")
	client.EXPECT().APIServiceClient(gomock.Any()).Return(nil, expectedErr).Times(1)

	cmd = NewBCInfoCmd(client)
	_, err = util.ExecuteCmd(cmd)
	require.Error(t, err)
	require.Equal(t, expectedErr, err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", ioctl.English).Times(3)
	client.EXPECT().Config().Return(config.ReadConfig).Times(2)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(1)

	expectedErr = errors.New("failed to get chain meta")
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(nil, expectedErr).Times(1)

	cmd = NewBCInfoCmd(client)
	_, err = util.ExecuteCmd(cmd)
	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}

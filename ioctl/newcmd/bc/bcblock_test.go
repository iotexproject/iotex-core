// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

// test for bc info command
func TestNewBCBlockCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(5)
	cfg := config.Config{}
	client.EXPECT().Config().Return(cfg).Times(2)

	apiServiceClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(1)

	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)

	blockMeta := []*iotextypes.BlockMeta{
		{
			Hash:   "abcd",
			Height: 1,
		},
	}
	blockMetaResponse := &iotexapi.GetBlockMetasResponse{BlkMetas: blockMeta}
	apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(blockMetaResponse, nil).Times(1)

	cmd := NewBCBlockCmd(client)
	_, err := util.ExecuteCmd(cmd)
	require.NoError(t, err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(5)
	cfg = config.Config{}
	client.EXPECT().Config().Return(cfg).Times(2)

	apiServiceClient = mock_apiserviceclient.NewMockServiceClient(ctrl)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(1)

	chainMetaResponse = &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)

	expectedErr := output.ErrorMessage{
		Code: 3,
		Info: "failed to get block meta: failed to invoke GetBlockMetas api: ",
	}
	err = output.ErrorMessage{}
	apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(nil, err).Times(1)

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd)
	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}

// Copyright (c) 2022 IoTeX
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

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// test for bc info command
func TestNewBCBlockCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	cfg := config.Config{}
	apiServiceClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
	blockMeta := []*iotextypes.BlockMeta{
		{
			Hash:   "abcd",
			Height: 1,
		},
	}
	blockMetaResponse := &iotexapi.GetBlockMetasResponse{BlkMetas: blockMeta}

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(35)
	client.EXPECT().Config().Return(cfg).Times(18)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(3)
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)
	apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(blockMetaResponse, nil).Times(3)

	cmd := NewBCBlockCmd(client)
	_, err := util.ExecuteCmd(cmd)
	require.NoError(err)

	_, err = util.ExecuteCmd(cmd, "1")
	require.NoError(err)

	_, err = util.ExecuteCmd(cmd, "abcd")
	require.NoError(err)

	expectedError := errors.New("failed to dial grpc connection")
	client.EXPECT().APIServiceClient(gomock.Any()).Return(nil, expectedError).Times(1)

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd)
	require.Error(err)
	require.Equal(expectedError, err)
	client.EXPECT().Config().Return(cfg).Times(2)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(1)

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd, "0")
	require.Error(err)

	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(1)
	expectedError = errors.New("failed to get chain meta")
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(nil, expectedError).Times(1)

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd)
	require.Error(err)
	require.Equal(expectedError, err)

	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(1)

	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)
	apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(blockMetaResponse, nil).Times(1)
	expectedError = errors.New("failed to get raw block")
	apiServiceClient.EXPECT().GetRawBlocks(gomock.Any(), gomock.Any()).Return(nil, expectedError).Times(1)

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd, "--verbose")
	require.Error(err)

	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(1)

	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)
	apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(blockMetaResponse, nil).Times(1)
	blockInfo := []*iotexapi.BlockInfo{
		{
			Block: &iotextypes.Block{
				Body: &iotextypes.BlockBody{
					Actions: []*iotextypes.Action{
						&iotextypes.Action{
							Core: &iotextypes.ActionCore{
								Version:  1,
								Nonce:    2,
								GasLimit: 3,
								GasPrice: "4",
							},
						},
						&iotextypes.Action{
							Core: &iotextypes.ActionCore{
								Version:  5,
								Nonce:    6,
								GasLimit: 7,
								GasPrice: "8",
							},
						},
					},
				},
			},
		},
	}
	rawBlocksResponse := &iotexapi.GetRawBlocksResponse{Blocks: blockInfo}
	apiServiceClient.EXPECT().GetRawBlocks(gomock.Any(), gomock.Any()).Return(rawBlocksResponse, nil).Times(1)

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd, "--verbose")
	require.NoError(err)

	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(1)
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)

	err = output.ErrorMessage{}
	apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(nil, err).Times(1)

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd)
	require.Error(err)
}

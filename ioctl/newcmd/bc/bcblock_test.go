// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

// test for bc info command
func TestNewBCBlockCmd(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := mock_ioctlclient.NewMockClient(ctrl)
	cfg := config.Config{}
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
	blockMeta := []*iotextypes.BlockMeta{
		{
			Hash:   "abcd",
			Height: 1,
		},
	}
	blockMetaResponse := &iotexapi.GetBlockMetasResponse{BlkMetas: blockMeta}

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(5)
	client.EXPECT().Config().Return(cfg).Times(2)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(3)
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)
	apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(blockMetaResponse, nil).Times(3)

	cmd := NewBCBlockCmd(client)
	_, err := util.ExecuteCmd(cmd)
	require.NoError(t, err)

	_, err = util.ExecuteCmd(cmd, "1")
	require.NoError(t, err)

	_, err = util.ExecuteCmd(cmd, "abcd")
	require.NoError(t, err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(5)
	client.EXPECT().Config().Return(cfg).Times(2)

	expectedError := errors.New("failed to dial grpc connection")
	client.EXPECT().APIServiceClient().Return(nil, expectedError).Times(1)

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd)
	require.Error(t, err)
	require.Equal(t, expectedError, err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(5)
	client.EXPECT().Config().Return(cfg).Times(2)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(1)

	expectedErr := output.ErrorMessage{
		Code: 5,
		Info: "invalid height: invalid number that is not positive",
	}
	err = output.ErrorMessage{}

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd, "0")
	require.Error(t, err)
	require.Equal(t, expectedErr, err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(5)
	client.EXPECT().Config().Return(cfg).Times(2)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(1)
	expectedError = errors.New("failed to get chain meta")
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(nil, expectedError).Times(1)

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd)
	require.Error(t, err)
	require.Equal(t, expectedError, err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(5)
	client.EXPECT().Config().Return(cfg).Times(2)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(1)

	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)
	apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(blockMetaResponse, nil).Times(1)
	expectedError = errors.New("failed to get raw block")
	apiServiceClient.EXPECT().GetRawBlocks(gomock.Any(), gomock.Any()).Return(nil, expectedError).Times(1)
	expectedErr = output.ErrorMessage{
		Code: 3,
		Info: "failed to get actions info: failed to invoke GetRawBlocks api: failed to get raw block",
	}

	cmd = NewBCBlockCmd(client)
	_, err = util.ExecuteCmd(cmd, "--verbose")
	require.Error(t, err)
	require.Equal(t, expectedErr, err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(5)
	client.EXPECT().Config().Return(cfg).Times(2)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(1)

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
	require.NoError(t, err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(5)
	client.EXPECT().Config().Return(cfg).Times(2)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(1)
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)

	expectedErr = output.ErrorMessage{
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

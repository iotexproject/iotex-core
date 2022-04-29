// Copyright (c) 2022 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
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
	client.EXPECT().Config().Return(cfg).Times(20)

	t.Run("failed to dial grpc connection", func(t *testing.T) {
		expectedErr := errors.New("failed to dial grpc connection")
		client.EXPECT().APIServiceClient(gomock.Any()).Return(nil, expectedErr).Times(1)

		cmd := NewBCBlockCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Error(err)
		require.Equal(expectedErr, err)
	})

	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(16)

	t.Run("failed to get chain meta", func(t *testing.T) {
		expectedErr := errors.New("failed to get chain meta")
		apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(nil, expectedErr).Times(1)

		cmd := NewBCBlockCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Error(err)
		fmt.Println(err.Error())
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to invoke GetBlockMetas api", func(t *testing.T) {
		expectedErr := errors.New("failed to invoke GetBlockMetas api")
		apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)
		apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(nil, expectedErr).Times(1)

		cmd := NewBCBlockCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Error(err)
		require.Contains(err.Error(), expectedErr.Error())

	})

	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)
	apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(blockMetaResponse, nil).Times(3)

	t.Run("get blockchain block", func(t *testing.T) {
		expectedValue := "Blockchain Node: \n{\n  \"hash\": \"abcd\",\n  \"height\": 1\n}\nnull\n"

		cmd := NewBCBlockCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Equal(expectedValue, result)
	})

	t.Run("get block meta by height", func(t *testing.T) {
		expectedValue := "\"height\": 1"

		cmd := NewBCBlockCmd(client)
		result, err := util.ExecuteCmd(cmd, "1")
		require.NoError(err)
		require.Contains(result, expectedValue)
	})

	t.Run("get block meta by hash", func(t *testing.T) {
		expectedValue := "\"hash\": \"abcd\""

		cmd := NewBCBlockCmd(client)
		result, err := util.ExecuteCmd(cmd, "abcd")
		require.NoError(err)
		require.Contains(result, expectedValue)
	})

	t.Run("invalid height", func(t *testing.T) {
		expectedErr := errors.New("invalid height")

		cmd := NewBCBlockCmd(client)
		_, err := util.ExecuteCmd(cmd, "0")
		require.Error(err)
		require.Contains(err.Error(), expectedErr.Error())
	})

	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(2)
	apiServiceClient.EXPECT().GetBlockMetas(gomock.Any(), gomock.Any()).Return(blockMetaResponse, nil).Times(2)

	t.Run("failed to get actions info", func(t *testing.T) {
		expectedErr := errors.New("failed to get actions info")
		apiServiceClient.EXPECT().GetRawBlocks(gomock.Any(), gomock.Any()).Return(nil, expectedErr).Times(1)

		cmd := NewBCBlockCmd(client)
		_, err := util.ExecuteCmd(cmd, "--verbose")
		require.Error(err)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("verbose", func(t *testing.T) {
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
				Receipts: []*iotextypes.Receipt{
					{
						Status:          1,
						BlkHeight:       1,
						ActHash:         []byte("02ae2a956d21e8d481c3a69e146633470cf625ec"),
						GasConsumed:     1,
						ContractAddress: "test",
						Logs:            []*iotextypes.Log{},
					},
					{
						Status:          1,
						BlkHeight:       1,
						ActHash:         []byte("02ae2a956d21e8d481c3a69e146633470cf625ec"),
						GasConsumed:     1,
						ContractAddress: "test",
						Logs:            []*iotextypes.Log{},
					},
				},
			},
		}
		rawBlocksResponse := &iotexapi.GetRawBlocksResponse{Blocks: blockInfo}
		expectedValue1 := "\"version\": 1,\n    \"nonce\": 2,\n    \"gasLimit\": 3,\n    \"gasPrice\": \"4\""
		expectedValue2 := "\"version\": 5,\n    \"nonce\": 6,\n    \"gasLimit\": 7,\n    \"gasPrice\": \"8\""
		apiServiceClient.EXPECT().GetRawBlocks(gomock.Any(), gomock.Any()).Return(rawBlocksResponse, nil).Times(1)

		cmd := NewBCBlockCmd(client)
		result, err := util.ExecuteCmd(cmd, "--verbose")
		require.NoError(err)
		require.Contains(result, expectedValue1)
		require.Contains(result, expectedValue2)
	})
}

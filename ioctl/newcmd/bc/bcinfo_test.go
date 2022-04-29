// Copyright (c) 2022 IoTeX
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

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

// test for bc info command
func TestNewBCInfoCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(9)
	client.EXPECT().Config().Return(config.Config{}).Times(7)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(2)

	t.Run("get blockchain info", func(t *testing.T) {
		chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
		apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(1)

		cmd := NewBCInfoCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.NoError(err)
	})

	t.Run("failed to get chain meta", func(t *testing.T) {
		expectedErr := errors.New("failed to get chain meta")
		apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(nil, expectedErr).Times(1)

		cmd := NewBCInfoCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Error(err)
		require.Contains(err.Error(), "failed to get chain meta")
	})

	t.Run("failed to dial grpc connection", func(t *testing.T) {
		expectedErr := errors.New("failed to dial grpc connection")
		client.EXPECT().APIServiceClient(gomock.Any()).Return(nil, expectedErr).Times(1)

		cmd := NewBCInfoCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Error(err)
		require.Equal(expectedErr, err)
	})

}

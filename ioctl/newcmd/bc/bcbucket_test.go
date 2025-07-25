// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bc

import (
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestBCBucketCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(9)

	t.Run("get total blockchain bucket count", func(t *testing.T) {
		client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(2)
		apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.All()).Return(&iotexapi.ReadStateResponse{}, nil)

		cmd := NewBCBucketCmd(client)
		result, err := util.ExecuteCmd(cmd, "max")
		require.NoError(err)
		require.Equal("0\n", result)
	})

	t.Run("get active blockchain bucket count", func(t *testing.T) {
		client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).AnyTimes()
		apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.All()).Return(&iotexapi.ReadStateResponse{}, nil)

		cmd := NewBCBucketCmd(client)
		result, err := util.ExecuteCmd(cmd, "count")
		require.NoError(err)
		require.Equal("0\n", result)
	})

	t.Run("get default blockchain bucket count", func(t *testing.T) {
		cfg := config.Config{}
		vb := &iotextypes.VoteBucket{
			Index:            1,
			StakedAmount:     "10",
			UnstakeStartTime: timestamppb.New(testutil.TimestampNow()),
		}
		vblist, err := proto.Marshal(&iotextypes.VoteBucketList{
			Buckets: []*iotextypes.VoteBucket{vb},
		})
		require.NoError(err)
		client.EXPECT().Config().Return(cfg).AnyTimes()
		apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.All()).Return(&iotexapi.ReadStateResponse{
			Data: vblist,
		}, nil)

		cmd := NewBCBucketCmd(client)
		result, err := util.ExecuteCmd(cmd, "0")
		require.NoError(err)
		require.Contains(result, "index: 1")
	})
}

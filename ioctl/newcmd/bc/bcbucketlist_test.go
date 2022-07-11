// Copyright (c) 2022 IoTeX Foundation
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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestNewBCBucketListCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(21)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(2)
	client.EXPECT().Config().Return(config.Config{}).Times(2)

	t.Run("get bucket list by voter", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx", nil)
		vblist, err := proto.Marshal(&iotextypes.VoteBucketList{
			Buckets: []*iotextypes.VoteBucket{
				{
					Index:            1,
					StakedAmount:     "10",
					UnstakeStartTime: timestamppb.New(testutil.TimestampNow()),
				},
				{
					Index:            2,
					StakedAmount:     "20",
					UnstakeStartTime: timestamppb.New(testutil.TimestampNow()),
				},
			},
		})
		require.NoError(err)
		apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.All()).Return(&iotexapi.ReadStateResponse{
			Data: vblist,
		}, nil)

		cmd := NewBCBucketListCmd(client)
		result, err := util.ExecuteCmd(cmd, "voter", "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx")
		require.NoError(err)
		require.Contains(result,
			"index: 1",
			"stakedAmount: 0.00000000000000001 IOTX",
			"index: 2",
			"stakedAmount: 0.00000000000000002 IOTX")
	})

	t.Run("get bucket list by candidate", func(t *testing.T) {
		apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.All()).Return(&iotexapi.ReadStateResponse{}, nil)
		cmd := NewBCBucketListCmd(client)
		result, err := util.ExecuteCmd(cmd, "cand", "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx")
		require.NoError(err)
		require.Equal("Empty bucketlist with given address\n", result)
	})

	t.Run("invalid voter address", func(t *testing.T) {
		expectedErr := errors.New("cannot find address for alias test")
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("", expectedErr)
		cmd := NewBCBucketListCmd(client)
		_, err := util.ExecuteCmd(cmd, "voter", "test")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("unknown method", func(t *testing.T) {
		expectedErr := errors.New("unknown <method>")
		cmd := NewBCBucketListCmd(client)
		_, err := util.ExecuteCmd(cmd, "unknown", "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx")
		require.Equal(expectedErr.Error(), err.Error())
	})

	t.Run("invalid offset", func(t *testing.T) {
		expectedErr := errors.New("invalid offset")
		cmd := NewBCBucketListCmd(client)
		_, err := util.ExecuteCmd(cmd, "voter", "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx", "test")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("invalid limit", func(t *testing.T) {
		expectedErr := errors.New("invalid limit")
		cmd := NewBCBucketListCmd(client)
		_, err := util.ExecuteCmd(cmd, "voter", "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx", "0", "test")
		require.Contains(err.Error(), expectedErr.Error())
	})
}

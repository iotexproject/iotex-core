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

	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestGetBucketList(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	request := &iotexapi.ReadStakingDataRequest{}
	response := &iotexapi.ReadStateResponse{}
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(2)

	t.Run("get bucket list", func(t *testing.T) {
		expectedValue := &iotextypes.VoteBucketList{
			Buckets: []*iotextypes.VoteBucket{},
		}
		apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.Any()).Return(response, nil)

		result, err := GetBucketList(client, iotexapi.ReadStakingDataMethod_BUCKETS_BY_CANDIDATE, request)
		require.NoError(err)
		require.Contains(result.String(), expectedValue.String())
	})

	t.Run("failed to invoke ReadState api", func(t *testing.T) {
		expectedErr := errors.New("failed to invoke ReadState api")
		apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		_, err := GetBucketList(client, iotexapi.ReadStakingDataMethod_BUCKETS_BY_VOTER, request)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

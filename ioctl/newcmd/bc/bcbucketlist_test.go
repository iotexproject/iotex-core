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
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewBCBucketListCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(21)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(2)
	client.EXPECT().Config().Return(config.Config{}).Times(2)

	apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.All()).Return(&iotexapi.ReadStateResponse{}, nil).Times(2)

	t.Run("get bucket list by voter", func(t *testing.T) {
		cmd := NewBCBucketListCmd(client)
		result, err := util.ExecuteCmd(cmd, "voter", "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx")
		require.NoError(err)
		require.Equal("", result)
	})

	t.Run("get bucket list by candidate", func(t *testing.T) {
		cmd := NewBCBucketListCmd(client)
		result, err := util.ExecuteCmd(cmd, "cand", "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx")
		require.NoError(err)
		require.Equal("", result)
	})

	t.Run("invalid voter address", func(t *testing.T) {
		expectedErr := errors.New("cannot find address for alias test")

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

// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewDidGetURICmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	accAddr := identityset.Address(0).String()
	payload := "0000000000000000000000000000000000000000000000000000000000000020" +
		"0000000000000000000000000000000000000000000000000000000000000020" +
		"0000000000000000000000000000000000000000000000000000000000000001" +
		"0000000000000000000000000000000000000000000000000000000000000002"
	did := "did:io:0x11111111111111111"

	client.EXPECT().SelectTranslation(gomock.Any()).Return("did", config.English).Times(10)
	client.EXPECT().Address(gomock.Any()).Return(accAddr, nil).Times(4)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr, nil).Times(4)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(4)

	t.Run("get did uri", func(t *testing.T) {
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{
			Data: payload,
		}, nil)
		cmd := NewDidGetURICmd(client)
		result, err := util.ExecuteCmd(cmd, accAddr, "did:io:0x11111111111111111")
		require.NoError(err)
		require.Contains(result, "0000000000000000000000000000000000000000000000000000000000000001")
	})

	t.Run("failed to decode contract", func(t *testing.T) {
		expectedErr := errors.New("failed to decode contract")
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{
			Data: "test",
		}, nil)
		cmd := NewDidGetURICmd(client)
		_, err := util.ExecuteCmd(cmd, "test", did)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("DID does not exist", func(t *testing.T) {
		expectedErr := errors.New("DID does not exist")
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{
			Data: "0000000000000000000000000000000000000000000000000000000000000020",
		}, nil)
		cmd := NewDidGetURICmd(client)
		_, err := util.ExecuteCmd(cmd, accAddr, did)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to read contract", func(t *testing.T) {
		expectedErr := errors.New("failed to read contract")
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(nil, expectedErr)
		cmd := NewDidGetURICmd(client)
		_, err := util.ExecuteCmd(cmd, accAddr, did)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get contract address", func(t *testing.T) {
		expectedErr := errors.New("failed to get contract address")
		client.EXPECT().Address(gomock.Any()).Return("", expectedErr)
		cmd := NewDidGetURICmd(client)
		_, err := util.ExecuteCmd(cmd, "test", did)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

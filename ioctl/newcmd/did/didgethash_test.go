// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"encoding/hex"
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

func TestNewDidGetHashCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	accAddr := identityset.Address(0).String()
	did := "did:io:0x11111111111111111"

	client.EXPECT().SelectTranslation(gomock.Any()).Return("did", config.English).Times(12)
	client.EXPECT().Address(gomock.Any()).Return(accAddr, nil).Times(4)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr, nil).Times(4)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(4)

	t.Run("get did hash", func(t *testing.T) {
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{
			Data: hex.EncodeToString([]byte("60fe47b100000000000000000000000000000000000000000000000000000000")),
		}, nil)
		cmd := NewDidGetHash(client)
		_, err := util.ExecuteCmd(cmd, accAddr, did)
		require.NoError(err)
	})

	t.Run("failed to decode contract", func(t *testing.T) {
		expectedErr := errors.New("failed to decode contract")
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{
			Data: "test",
		}, nil)
		cmd := NewDidGetHash(client)
		_, err := util.ExecuteCmd(cmd, "test", did)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("DID does not exist", func(t *testing.T) {
		expectedErr := errors.New("DID does not exist")
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{
			Data: hex.EncodeToString([]byte("test")),
		}, nil)
		cmd := NewDidGetHash(client)
		_, err := util.ExecuteCmd(cmd, accAddr, did)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to read contract", func(t *testing.T) {
		expectedErr := errors.New("failed to read contract")
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(nil, expectedErr)
		cmd := NewDidGetHash(client)
		_, err := util.ExecuteCmd(cmd, accAddr, did)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("invalid contract address", func(t *testing.T) {
		expectedErr := errors.New("invalid contract address")
		client.EXPECT().Address(gomock.Any()).Return("test", nil)
		cmd := NewDidGetHash(client)
		_, err := util.ExecuteCmd(cmd, "test", did)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get contract address", func(t *testing.T) {
		expectedErr := errors.New("failed to get contract address")
		client.EXPECT().Address(gomock.Any()).Return("", expectedErr)
		cmd := NewDidGetHash(client)
		_, err := util.ExecuteCmd(cmd, "test", did)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

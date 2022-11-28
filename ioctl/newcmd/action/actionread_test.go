// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

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

func TestNewActionReadCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	addr := identityset.Address(0).String()

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(36)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(2)
	client.EXPECT().Address(gomock.Any()).Return(addr, nil).Times(3)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(addr, nil).Times(2)

	t.Run("read action", func(t *testing.T) {
		expectedVal := "36306665343762313030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030\n"
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{
			Data: hex.EncodeToString([]byte("60fe47b100000000000000000000000000000000000000000000000000000000")),
		}, nil)
		cmd := NewActionReadCmd(client)
		result, err := util.ExecuteCmd(cmd, addr, "--bytecode", "0x60fe47b100000000000000000000000000000000000000000000000000000000")
		require.NoError(err)
		require.Equal(expectedVal, result)
	})

	t.Run("failed to Read contract", func(t *testing.T) {
		expectedErr := errors.New("failed to Read contract")
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(nil, expectedErr)
		cmd := NewActionReadCmd(client)
		_, err := util.ExecuteCmd(cmd, addr, "--bytecode", "0x60fe47b100000000000000000000000000000000000000000000000000000000")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("invalid bytecode", func(t *testing.T) {
		expectedErr := errors.New("invalid bytecode")
		cmd := NewActionReadCmd(client)
		_, err := util.ExecuteCmd(cmd, addr, "--bytecode", "test")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get contract address", func(t *testing.T) {
		expectedErr := errors.New("failed to get contract address")
		client.EXPECT().Address(gomock.Any()).Return("", expectedErr)
		cmd := NewActionReadCmd(client)
		_, err := util.ExecuteCmd(cmd, "test")
		require.Contains(err.Error(), expectedErr.Error())
	})
}

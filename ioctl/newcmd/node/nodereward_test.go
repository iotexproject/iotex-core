// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewNodeRewardCmd(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	client.EXPECT().Address(gomock.Any()).Return("test_address", nil).Times(1)

	apiClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)

	apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("UnclaimedBalance"),
		Arguments:  [][]byte{[]byte("test_address")},
	}).Return(&iotexapi.ReadStateResponse{
		Data: []byte("0")},
		nil)

	apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("AvailableBalance"),
	}).Return(&iotexapi.ReadStateResponse{
		Data: []byte("24361490367906930338205776")},
		nil)

	apiClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("TotalBalance"),
	}).Return(&iotexapi.ReadStateResponse{
		Data: []byte("52331682309272536203174665")},
		nil)

	client.EXPECT().APIServiceClient().Return(apiClient, nil).AnyTimes()
	cmd := NewNodeRewardCmd(client)

	result, err := util.ExecuteCmd(cmd, "test")
	require.NotNil(t, result)
	require.NoError(t, err)

	result, err = util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.NoError(t, err)

}

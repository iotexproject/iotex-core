// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountNonce(t *testing.T) {
	accountNoneTests := []struct {
		// input
		inAddr string
		// output
		outNonce        int
		outPendingNonce int
	}{
		{
			inAddr:          "",
			outNonce:        0,
			outPendingNonce: 0,
		},
		{
			inAddr:          "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hc5r",
			outNonce:        0,
			outPendingNonce: 1,
		},
		{
			inAddr:          "io187evpmjdankjh0g5dfz83w2z3p23ljhn4s9jw7",
			outNonce:        2,
			outPendingNonce: 3,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()

	accAddr := identityset.Address(28).String()
	client.EXPECT().Config().Return(config.ReadConfig).AnyTimes()
	apiServiceClient := mock_apiserviceclient.NewMockServiceClient(ctrl)

	// success
	for i := 0; i < len(accountNoneTests); i++ {
		client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil)
		client.EXPECT().GetAddress(gomock.Any()).Return(accAddr, nil)
		accountResponse := &iotexapi.GetAccountResponse{AccountMeta: &iotextypes.AccountMeta{
			Address:      accAddr,
			Nonce:        uint64(accountNoneTests[i].outNonce),
			PendingNonce: uint64(accountNoneTests[i].outPendingNonce),
		}}
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResponse, nil)
		client.EXPECT().PrintInfo(gomock.Any()).Do(func(info string) {
			fmt.Println(info)
		})

		cmd := NewAccountNonce(client)
		err := util.ExecuteCmd(cmd, accountNoneTests[i].inAddr)
		require.NoError(t, err)
	}

	// fail to get account addr
	expectedErr := errors.New("failed to get address")
	client.EXPECT().GetAddress(gomock.Any()).Return("", expectedErr)
	cmd := NewAccountNonce(client)
	err := util.ExecuteCmd(cmd)
	require.Contains(t, err.Error(), expectedErr.Error())

	// fail to dial grpc
	expectedErr = errors.New("failed to dial grpc connection")
	client.EXPECT().GetAddress(gomock.Any()).Return(accAddr, nil)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(nil, expectedErr)
	cmd = NewAccountNonce(client)
	err = util.ExecuteCmd(cmd)
	require.Contains(t, err.Error(), expectedErr.Error())

	// fail to invoke grpc api
	expectedErr = errors.New("failed to invoke GetAccount api")
	client.EXPECT().GetAddress(gomock.Any()).Return(accAddr, nil)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil)
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, expectedErr)
	cmd = NewAccountNonce(client)
	err = util.ExecuteCmd(cmd)
	require.Contains(t, err.Error(), expectedErr.Error())
}

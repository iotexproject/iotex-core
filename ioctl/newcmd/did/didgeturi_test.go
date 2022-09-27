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
	payload := "60fe47b100000000000000000000000000000000000000000000000000000000"

	client.EXPECT().SelectTranslation(gomock.Any()).Return("did", config.English).Times(2)
	client.EXPECT().Address(gomock.Any()).Return(accAddr, nil).Times(1)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr, nil).Times(1)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(1)

	t.Run("get did hash", func(t *testing.T) {
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{
			Data: hex.EncodeToString([]byte(payload)),
		}, nil)
		cmd := NewDidGetURICmd(client)
		_, err := util.ExecuteCmd(cmd, accAddr, payload)
		require.NoError(err)
	})
}

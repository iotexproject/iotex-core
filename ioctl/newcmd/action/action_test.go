// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestSigner(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(3)
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(func(_ func(*string, string, string, string)) {})
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(func(_ func(*bool, string, bool, string)) {})

	t.Run("returns signer's address", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("test", nil).AnyTimes()

		cmd := NewActionCmd(client)
		registerSignerFlag(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--signer", "test")
		require.NoError(err)
		result, err := Signer(client, cmd)
		require.NoError(err)
		require.Equal(result, "test")
	})
}

func TestSendRaw(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	selp := &iotextypes.Action{}

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(12)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(7)

	for _, test := range []struct {
		endpoint string
		insecure bool
	}{
		{
			endpoint: "111:222:333:444:5678",
			insecure: false,
		},
		{
			endpoint: "",
			insecure: true,
		},
	} {
		callbackEndpoint := func(cb func(*string, string, string, string)) {
			cb(&test.endpoint, "endpoint", test.endpoint, "endpoint usage")
		}
		callbackInsecure := func(cb func(*bool, string, bool, string)) {
			cb(&test.insecure, "insecure", !test.insecure, "insecure usage")
		}
		client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(callbackEndpoint).Times(3)
		client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(callbackInsecure).Times(3)

		t.Run("sends raw action to blockchain", func(t *testing.T) {
			response := &iotexapi.SendActionResponse{}

			apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(response, nil).Times(3)

			cmd := NewActionCmd(client)
			_, err := util.ExecuteCmd(cmd)
			require.NoError(err)

			t.Run("endpoint iotexscan", func(t *testing.T) {
				client.EXPECT().Config().Return(config.Config{
					Explorer: "iotexscan",
					Endpoint: "testnet1",
				}).Times(2)

				err = SendRaw(client, cmd, selp)
				require.NoError(err)
			})

			t.Run("endpoint iotxplorer", func(t *testing.T) {
				client.EXPECT().Config().Return(config.Config{
					Explorer: "iotxplorer",
				}).Times(2)

				err := SendRaw(client, cmd, selp)
				require.NoError(err)
			})

			t.Run("endpoint default", func(t *testing.T) {
				client.EXPECT().Config().Return(config.Config{
					Explorer: "test",
				}).Times(2)

				err := SendRaw(client, cmd, selp)
				require.NoError(err)
			})
		})
	}

	t.Run("failed to invoke SendAction api", func(t *testing.T) {
		expectedErr := errors.New("failed to invoke SendAction api")

		apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		err = SendRaw(client, cmd, selp)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

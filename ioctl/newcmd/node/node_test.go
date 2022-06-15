package node

import (
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewNodeCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("node delegate", config.English).AnyTimes()

	testData := []struct {
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
	}
	for _, test := range testData {
		callbackEndpoint := func(cb func(*string, string, string, string)) {
			cb(&test.endpoint, "endpoint", test.endpoint, "endpoint usage")
		}
		callbackInsecure := func(cb func(*bool, string, bool, string)) {
			cb(&test.insecure, "insecure", !test.insecure, "insecure usage")
		}
		client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(callbackEndpoint)
		client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(callbackInsecure)

		cmd := NewNodeCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, "Available Commands")

		result, err = util.ExecuteCmd(cmd, "--endpoint", "0.0.0.0:1", "--insecure")
		require.NoError(err)
		require.Contains(result, "Available Commands")
		require.Equal("0.0.0.0:1", test.endpoint)
		require.True(test.insecure)
	}
}

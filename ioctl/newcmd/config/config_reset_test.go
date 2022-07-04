package config

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
)

func TestConfigReset(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().Config().Return(config.Config{})
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)

	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(1)

	cmd := NewConfigReset(client)
	result, err := util.ExecuteCmd(cmd, "reset")
	require.NoError(err)
	require.Contains(result, "successfully reset config")
	require.NoError(os.Remove("config.default"))
}

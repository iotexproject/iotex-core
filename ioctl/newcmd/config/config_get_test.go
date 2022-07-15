package config

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewConfigGetCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().Config().Return(config.Config{Endpoint: "test"}).AnyTimes()
	client.EXPECT().SelectTranslation(gomock.Any()).Return("config reset", config.English).AnyTimes()

	t.Run("get config value", func(t *testing.T) {
		client.EXPECT().ConfigFilePath().Return(fmt.Sprintf("%s/%s", t.TempDir(), "config.file"))
		cmd := NewConfigGetCmd(client)
		result, err := util.ExecuteCmd(cmd, "endpoint")
		require.NoError(err)
		require.Contains(result, "test")
	})

	t.Run("get unknown config value", func(t *testing.T) {
		cmd := NewConfigGetCmd(client)
		_, err := util.ExecuteCmd(cmd, "random-args")
		require.Contains(err.Error(), "invalid argument \"random-args\"")
	})

	t.Run("config value error", func(t *testing.T) {
		client.EXPECT().ConfigFilePath().Return(fmt.Sprintf("%s/%s", t.TempDir(), "config.file"))
		cmd := NewConfigGetCmd(client)
		_, err := util.ExecuteCmd(cmd, "defaultacc")
		require.Contains(err.Error(), "issue fetching config value defaultacc: default account not set")
	})
}

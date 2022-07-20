package config

import (
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfigSetCommand(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().Config().Return(config.Config{}).AnyTimes()
	client.EXPECT().SelectTranslation(gomock.Any()).Return("config reset", config.English).AnyTimes()

	t.Run("set config value", func(t *testing.T) {
		client.EXPECT().ConfigFilePath().Return(fmt.Sprintf("%s/%s", t.TempDir(), "config.file"))
		cmd := NewConfigSetCmd(client)
		result, err := util.ExecuteCmd(cmd, "nsv2height", "44")
		require.NoError(err)
		require.Contains(result, "Nsv2height is set to 44\n")
	})

	t.Run("config set error", func(t *testing.T) {
		client.EXPECT().ConfigFilePath().Return(fmt.Sprintf("%s/%s", t.TempDir(), "config.file"))
		cmd := NewConfigSetCmd(client)
		_, err := util.ExecuteCmd(cmd, "explorer", "invalid explorer")
		require.Contains(err.Error(), "problem setting config fields [explorer invalid explorer]")
	})

	t.Run("config set too many arguments", func(t *testing.T) {
		cmd := NewConfigSetCmd(client)
		_, err := util.ExecuteCmd(cmd, "explorer", "iotxplorer", "arg2", "arg3", "arg4")
		require.Contains(err.Error(), "accepts 2 arg(s), received 5")
	})

	t.Run("config file path error", func(t *testing.T) {
		client.EXPECT().ConfigFilePath().Return("\x00")
		// use invalid file name to force error
		cmd := NewConfigSetCmd(client)
		_, err := util.ExecuteCmd(cmd, "explorer", "iotxplorer")
		require.Contains(err.Error(), "problem setting config fields [explorer iotxplorer]: failed to write to config file")
	})
}

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

func TestConfigResetCommand(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().Config().Return(config.Config{}).Times(2)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("config reset", config.English).Times(2)

	t.Run("successful config reset", func(t *testing.T) {
		client.EXPECT().ConfigFilePath().Return(fmt.Sprintf("%s/%s", t.TempDir(), "config.file"))
		cmd := NewConfigReset(client)
		result, err := util.ExecuteCmd(cmd, "reset")
		require.NoError(err)
		require.Contains(result, "successfully reset config")
	})

	t.Run("config reset error", func(t *testing.T) {
		client.EXPECT().ConfigFilePath().Return("\x00")
		// use invalid file name to force error
		cmd := NewConfigReset(client)
		_, err := util.ExecuteCmd(cmd, "reset")
		require.Contains(err.Error(), "failed to reset config")
	})
}

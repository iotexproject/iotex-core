package config

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestConfigResetCommand(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().Config().Return(config.Config{}).Times(2)

	t.Run("successful config reset", func(t *testing.T) {
		cmd := NewConfigReset(client, _defaultConfigFileName)
		result, err := util.ExecuteCmd(cmd, "reset")
		require.NoError(err)
		require.Contains(result, "successfully reset config")

		defer testutil.CleanupPath("config.default")
	})

	t.Run("config reset error", func(t *testing.T) {
		// use invalid file name to force error
		cmd := NewConfigReset(client, "\x00")
		_, err := util.ExecuteCmd(cmd, "reset")
		require.Error(err)
	})
}

func TestConfigReset(t *testing.T) {
	require := require.New(t)

	info := newInfo(config.Config{
		Wallet:           "wallet",
		Endpoint:         "testEndpoint",
		SecureConnect:    false,
		DefaultAccount:   config.Context{AddressOrAlias: ""},
		Explorer:         "explorer",
		Language:         "RandomLanguage",
		AnalyserEndpoint: "testAnalyser",
	}, _defaultConfigFileName)

	require.NoError(info.reset())

	config.DefaultConfigFile = _defaultConfigFileName
	cfg, err := config.LoadConfig()
	require.NoError(err)

	// ensure config has been reset
	assert.Equal(t, ".", cfg.Wallet)
	assert.Equal(t, "", cfg.Endpoint)
	assert.Equal(t, true, cfg.SecureConnect)
	assert.Equal(t, "English", cfg.Language)
	assert.Equal(t, _defaultAnalyserEndpoint, cfg.AnalyserEndpoint)
	assert.Equal(t, "iotexscan", cfg.Explorer)
	assert.Equal(t, *new(config.Context), cfg.DefaultAccount)

	defer testutil.CleanupPath("config.default")
}

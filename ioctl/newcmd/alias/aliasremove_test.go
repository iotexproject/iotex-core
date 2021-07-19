package alias

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/stretchr/testify/require"
)

func TestNewAliasRemoveCmd(t *testing.T) {
	// mock a client
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("%s is removed", config.English).Times(6)

	// configuration files to temporary files
	defaultConfigFile := config.DefaultConfigFile
	config.DefaultConfigFile = filepath.Join(os.TempDir(), "config.default")
	defer func() {
		_ = os.Remove(config.DefaultConfigFile)
		config.DefaultConfigFile = defaultConfigFile
	}()

	cfg := config.Config{
		Aliases: map[string]string{
			"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"b": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"c": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
		},
	}
	client.EXPECT().Config().Return(cfg).AnyTimes()
	cmd := NewAliasRemove(client)
	res, err := util.ExecuteCmd(cmd, "a")
	require.NotNil(t, res)
	require.NoError(t, err)

	// read config file check aliases
	conf, err := config.LoadConfig()
	require.NoError(t, err)
	_, ok := conf.Aliases["a"]
	require.False(t, ok)
	_, ok = conf.Aliases["b"]
	require.True(t, ok)
}

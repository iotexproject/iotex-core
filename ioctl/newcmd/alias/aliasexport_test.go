// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAliasExport(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslation",
		config.English).Times(12)
	cfg := config.Config{
		Aliases: map[string]string{
			"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"b": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"c": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
			"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		},
	}
	client.EXPECT().AliasMap().Return(map[string]string{
		"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"b": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"c": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
		"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
	}).AnyTimes()
	client.EXPECT().Config().Return(cfg).AnyTimes()

	t.Run("invalid flag", func(t *testing.T) {
		cmd := NewAliasExport(client)
		_, err := util.ExecuteCmd(cmd, "-f", "")
		require.Error(err)
		require.Contains(err.Error(), "EXTRA string=")
	})

	t.Run("export alias with json format", func(t *testing.T) {
		cmd := NewAliasExport(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.NotNil(result)
		require.Contains(result, "alias")
	})

	t.Run("export alias with yaml format", func(t *testing.T) {
		cmd := NewAliasExport(client)
		result, err := util.ExecuteCmd(cmd, "-f", "yaml")
		require.NoError(err)
		require.NotNil(result)
		require.Contains(result, "alias")
	})
}

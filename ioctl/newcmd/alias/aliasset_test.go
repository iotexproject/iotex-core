// Copyright (c) 2022 IoTeX Foundation
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

func TestNewAliasSetCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	cfg := config.Config{
		Aliases: map[string]string{
			"a": "io19sdfxkwegeaenvxk2kjqf98al52gm56wa2eqks",
			"b": "io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng",
			"c": "io1tyc2yt68qx7hmxl2rhssm99jxnhrccz9sneh08",
		},
	}
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslation", config.English).Times(6)
	client.EXPECT().AliasMap().Return(cfg.Aliases).MaxTimes(2)
	client.EXPECT().Config().Return(cfg).AnyTimes()

	t.Run("set alias", func(t *testing.T) {
		client.EXPECT().SetAliasAndSave("d", "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx").Return(nil).Times(1)
		cmd := NewAliasSetCmd(client)
		result, err := util.ExecuteCmd(cmd, "d", "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx")
		require.NoError(err)
		require.Contains(result, "d has been set!")
	})

	t.Run("invalid alias", func(t *testing.T) {
		cmd := NewAliasSetCmd(client)
		_, err := util.ExecuteCmd(cmd, "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx", "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx")
		require.Error(err)
		require.Contains(err.Error(), "invalid alias")
	})

	t.Run("invalid address", func(t *testing.T) {
		cmd := NewAliasSetCmd(client)
		_, err := util.ExecuteCmd(cmd, "d", "d")
		require.Error(err)
		require.Contains(err.Error(), "invalid address")
	})
}

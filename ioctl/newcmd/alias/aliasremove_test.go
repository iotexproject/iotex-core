// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package alias

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/stretchr/testify/require"
)

func TestNewAliasRemoveCmd(t *testing.T) {
	require := require.New(t)
	// mock a client
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	cfg := config.Config{
		Aliases: map[string]string{
			"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx": "a",
			"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542se": "b",
			"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1": "c",
		},
	}
	client.EXPECT().AliasMap().Return(map[string]string{
		"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx": "a",
		"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542se": "b",
		"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1": "c",
	}).Times(2)
	client.EXPECT().Config().Return(cfg).Times(2)

	t.Run("remove alias", func(t *testing.T) {
		client.EXPECT().SelectTranslation(gomock.Any()).Return("%s is removed", config.English).Times(5)
		client.EXPECT().DeleteAlias("a").Return(nil)
		cmd := NewAliasRemove(client)
		result, err := util.ExecuteCmd(cmd, "a")
		require.NoError(err)
		require.Contains(result, "a is removed")
	})

	t.Run("invalid alias", func(t *testing.T) {
		client.EXPECT().SelectTranslation(gomock.Any()).Return("invalid alias %s", config.English).Times(5)
		cmd := NewAliasRemove(client)
		_, err := util.ExecuteCmd(cmd, "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542se")
		require.Error(err)
		require.Contains(err.Error(), "invalid alias io1uwnr55vqmhf3xeg5phgurlyl702af6eju542se")
	})
}

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

// test for alias list command
func TestNewAliasListCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(2)
	cfg := config.Config{
		Aliases: map[string]string{
			"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"b": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"c": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
		},
	}
	client.EXPECT().Config().Return(cfg).AnyTimes()

	t.Run("list aliases", func(t *testing.T) {
		cmd := NewAliasListCmd(client)
		res, err := util.ExecuteCmd(cmd)
		require.NotNil(t, res)
		require.NoError(t, err)
	})
}

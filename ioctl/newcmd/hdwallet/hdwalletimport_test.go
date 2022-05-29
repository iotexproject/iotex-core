// Copyright (c) 2022 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewNodeDelegateCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)

	mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
	password := "123"

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(2)
	client.EXPECT().ReadSecret().Return(mnemonic, nil)
	client.EXPECT().ReadSecret().Return(password, nil)
	client.EXPECT().ReadSecret().Return(password, nil)
	client.EXPECT().WriteConfig()

	t.Run("import hdwallet", func(t *testing.T) {
		cmd := NewHdwalletImportCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.NoError(err)
	})
}

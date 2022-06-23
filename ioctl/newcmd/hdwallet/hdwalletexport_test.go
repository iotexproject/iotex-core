// Copyright (c) 2022 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewHdwalletExportCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(4)
	mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
	password := "123"

	t.Run("export hdwallet", func(t *testing.T) {
		client.EXPECT().ReadSecret().Return(password, nil)
		client.EXPECT().HdwalletMnemonic(gomock.Any()).Return(mnemonic, nil)

		cmd := NewHdwalletExportCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, mnemonic)
	})

	t.Run("failed to export mnemonic", func(t *testing.T) {
		expectedErr := errors.New("failed to export mnemonic")
		client.EXPECT().ReadSecret().Return(password, nil)
		client.EXPECT().HdwalletMnemonic(gomock.Any()).Return("", expectedErr)

		cmd := NewHdwalletExportCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

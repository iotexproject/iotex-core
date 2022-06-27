// Copyright (c) 2022 IoTeX Foundation
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

func TestNewHdwalletDeriveCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(6)
	client.EXPECT().ReadSecret().Return("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", nil).Times(2)

	t.Run("get address from hdwallet", func(t *testing.T) {
		client.EXPECT().HdwalletMnemonic(gomock.Any()).Return(mnemonic, nil)

		cmd := NewHdwalletDeriveCmd(client)
		result, err := util.ExecuteCmd(cmd, "1/1/2")
		require.NoError(err)
		require.Contains(result, "address: io1pt2wml6v2tx683ctjn3k9mfgq97zx4c7rzwfuf")
	})

	t.Run("invalid hdwallet key format", func(t *testing.T) {
		expectedErr := errors.New("invalid hdwallet key format")

		cmd := NewHdwalletDeriveCmd(client)
		_, err := util.ExecuteCmd(cmd, "1")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to export mnemonic", func(t *testing.T) {
		expectedErr := errors.New("failed to export mnemonic")
		client.EXPECT().HdwalletMnemonic(gomock.Any()).Return("", expectedErr)

		cmd := NewHdwalletDeriveCmd(client)
		_, err := util.ExecuteCmd(cmd, "1/1/2")
		require.Contains(err.Error(), expectedErr.Error())
	})
}

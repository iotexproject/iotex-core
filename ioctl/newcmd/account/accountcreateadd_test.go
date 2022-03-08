// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

func TestNewAccountCreateAdd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()
	client.EXPECT().PrintInfo(gomock.Any()).Times(7)
	client.EXPECT().AliasMap().Return(map[string]string{
		"aaa": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"bbb": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
	}).Times(4)

	t.Run("CryptoSm2 is true", func(t *testing.T) {
		_, ks, pwd, _, err := newTestAccount()
		require.NoError(err)
		client.EXPECT().ReadSecret().Return(pwd, nil).Times(2)
		client.EXPECT().IsCryptoSm2().Return(true).Times(1)
		client.EXPECT().AskToConfirm(gomock.Any()).Return(true)
		client.EXPECT().Config().Return(config.Config{})
		client.EXPECT().NewKeyStore().Return(ks)
		cmd := NewAccountCreateAdd(client)
		_, err = util.ExecuteCmd(cmd, "aaa")
		require.NoError(err)
	})

	t.Run("CryptoSm2 is false", func(t *testing.T) {
		_, ks, pwd, _, err := newTestAccount()
		require.NoError(err)
		client.EXPECT().ReadSecret().Return(pwd, nil).Times(2)
		client.EXPECT().IsCryptoSm2().Return(false).Times(2)
		client.EXPECT().AskToConfirm(gomock.Any()).Return(true)
		client.EXPECT().Config().Return(config.Config{})
		client.EXPECT().NewKeyStore().Return(ks)
		cmd := NewAccountCreateAdd(client)
		_, err = util.ExecuteCmd(cmd, "aaa")
		require.NoError(err)
	})

	t.Run("failed to confirm", func(t *testing.T) {
		client.EXPECT().AskToConfirm(gomock.Any()).Return(false)
		cmd := NewAccountCreateAdd(client)
		_, err := util.ExecuteCmd(cmd, "aaa")
		require.NoError(err)
	})

	t.Run("invalid alias", func(t *testing.T) {
		expectedErr := errors.New("invalid long alias that is more than 40 characters")

		cmd := NewAccountCreateAdd(client)
		_, err := util.ExecuteCmd(cmd, strings.Repeat("a", 50))
		require.Contains(err.Error(), expectedErr.Error())
	})
}

// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package update

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewUpdateCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)

	expectedValue := "ioctl is up-to-date now."
	client.EXPECT().SelectTranslation(gomock.Any()).Return(expectedValue,
		config.English).Times(18)
	client.EXPECT().AskToConfirm(gomock.Any()).Return(true, nil).Times(2)
	client.EXPECT().Execute(gomock.Any()).Return(nil).Times(2)

	t.Run("update cli with stable", func(t *testing.T) {
		cmd := NewUpdateCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, expectedValue)
	})

	t.Run("update cli with unstable", func(t *testing.T) {
		cmd := NewUpdateCmd(client)
		result, err := util.ExecuteCmd(cmd, "-t", "unstable")
		require.NoError(err)
		require.Contains(result, expectedValue)
	})

	t.Run("failed to execute bash command", func(t *testing.T) {
		expectedError := errors.New("failed to execute bash command")
		client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationResult",
			config.English).Times(9)
		client.EXPECT().AskToConfirm(gomock.Any()).Return(true, nil).Times(1)
		client.EXPECT().Execute(gomock.Any()).Return(expectedError).Times(1)

		cmd := NewUpdateCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Equal("mockTranslationResult: "+expectedError.Error(), err.Error())
	})

	t.Run("invalid version type", func(t *testing.T) {
		expectedError := errors.New("invalid version-type flag:pre-release")
		client.EXPECT().SelectTranslation(gomock.Any()).Return("invalid version-type flag:%s",
			config.English).Times(9)
		client.EXPECT().Execute(gomock.Any()).Return(expectedError).AnyTimes()

		cmd := NewUpdateCmd(client)
		_, err := util.ExecuteCmd(cmd, "-t", "pre-release")
		require.Error(err)
		require.Equal(expectedError.Error(), err.Error())
	})
}

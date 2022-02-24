// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountVerify(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	testAccountFolder := filepath.Join(os.TempDir(), "testNewAccountSign")
	require.NoError(t, os.MkdirAll(testAccountFolder, os.ModePerm))
	defer func() {
		require.NoError(t, os.RemoveAll(testAccountFolder))
	}()

	t.Run("verify account successfully", func(t *testing.T) {
		RawPrivateKey := "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1"
		client.EXPECT().PrintInfo(gomock.Any()).Return().AnyTimes()

		cmd := NewAccountVerify(client)
		_, err := util.ExecuteCmd(cmd, RawPrivateKey)
		require.NoError(t, err)
	})

	t.Run("failed to generate private key from hex string", func(t *testing.T) {
		expectedErr := output.NewError(output.CryptoError, "failed to generate private key from hex string: invalid private key", nil)

		cmd := NewAccountVerify(client)
		_, err := util.ExecuteCmd(cmd, "1234")
		require.Error(t, err)
		require.Equal(t, expectedErr, err)
	})
}

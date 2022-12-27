// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountVerify(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	t.Run("verify account successfully", func(t *testing.T) {
		client.EXPECT().ReadSecret().Return("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", nil)

		cmd := NewAccountVerify(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, identityset.PrivateKey(27).PublicKey().Address().String())
		require.Contains(result, identityset.PrivateKey(27).PublicKey().HexString())
	})

	t.Run("failed to covert hex string to private key", func(t *testing.T) {
		client.EXPECT().ReadSecret().Return("1234", nil)
		expectedErr := errors.New("invalid private key")

		cmd := NewAccountVerify(client)
		_, err := util.ExecuteCmd(cmd)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

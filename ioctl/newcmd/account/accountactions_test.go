// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString",
		config.English).AnyTimes()
	accAddr := identityset.Address(28).String()

	t.Run("empty offset", func(t *testing.T) {
		cmd := NewAccountActions(client)
		_, err := util.ExecuteCmd(cmd, accAddr, "")
		require.Error(err)
		require.Contains(err.Error(), "failed to convert skip")
	})

	t.Run("failed to send request", func(t *testing.T) {
		client.EXPECT().QueryAnalyser(gomock.Any()).Return(nil, errors.New("failed to send request")).Times(1)
		client.EXPECT().Address(gomock.Any()).Return(accAddr, nil)
		cmd := NewAccountActions(client)
		_, err := util.ExecuteCmd(cmd, accAddr, "0")
		require.Error(err)
		require.Contains(err.Error(), "failed to send request")
	})

	client.EXPECT().QueryAnalyser(gomock.Any()).DoAndReturn(func(reqData interface{}) (*http.Response, error) {
		jsonData, err := json.Marshal(reqData)
		require.NoError(err)
		resp, err := http.Post("https://iotex-analyser-api-mainnet.chainanalytics.org/api.ActionsService.GetActionsByAddress", "application/json",
			bytes.NewBuffer(jsonData))
		require.NoError(err)
		return resp, nil
	}).Times(3)

	t.Run("get account action", func(t *testing.T) {
		client.EXPECT().Address(gomock.Any()).Return(accAddr, nil).Times(2)
		cmd := NewAccountActions(client)
		result, err := util.ExecuteCmd(cmd, accAddr, "0")
		require.NoError(err)
		require.Contains(result, "Total")
		result, err = util.ExecuteCmd(cmd, accAddr)
		require.NoError(err)
		require.Contains(result, "Total")
	})

	t.Run("empty address", func(t *testing.T) {
		client.EXPECT().Address(gomock.Any()).Return("", nil)
		cmd := NewAccountActions(client)
		_, err := util.ExecuteCmd(cmd, "", "0")
		require.Error(err)
		require.Contains(err.Error(), "failed to deserialize the response")
	})
}

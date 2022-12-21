// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package newcmd

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNew(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(func(_ func(*string, string, string, string)) {}).AnyTimes()
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(func(_ func(*bool, string, bool, string)) {}).AnyTimes()

	cmds := []*cobra.Command{
		NewIoctl(client),
		NewXctl(client),
	}
	for _, cmd := range cmds {
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, "Available Commands")
		require.Contains(result, "Flags")
	}
}

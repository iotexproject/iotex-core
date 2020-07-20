// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"testing"
	"time"
	"errors"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserver"
)

var (
	errorSend error = errors.New("send error")
)

func TestBlockListener(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	errChan := make(chan error, 10)

	server := mock_apiserver.NewMockStreamBlocksServer(ctrl)
	responder := NewBlockListener(server, errChan)

	receipts := []*action.Receipt{
		{
			BlockHeight: 1,
		},
		{
			BlockHeight: 2,
		},
	}
	builder := block.NewTestingBuilder()
	builder.SetHeight(1)
	builder.SetVersion(111)
	builder.SetTimeStamp(time.Now())
	builder.SetReceipts(receipts)
	testBlock, _ := builder.SignAndBuild(identityset.PrivateKey(0))

	server.EXPECT().Send(gomock.Any()).Return(nil).Times(1)
	err := responder.Respond(&testBlock)
	require.NoError(t, err)

	server.EXPECT().Send(gomock.Any()).Return(errorSend).Times(1)
	err = responder.Respond(&testBlock)
	require.Error(t, err)
	require.Equal(t, errorSend, err)

	responder.Exit()

	err = <-errChan
	require.Error(t, err)
	require.Equal(t, errorSend, err)

	err = <-errChan
	require.NoError(t, err)
}



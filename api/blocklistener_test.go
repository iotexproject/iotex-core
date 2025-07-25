// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_apiserver"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

var (
	errorSend error = errors.New("send error")
)

func TestBlockListener(t *testing.T) {
	ctrl := gomock.NewController(t)

	errChan := make(chan error, 10)

	server := mock_apiserver.NewMockStreamBlocksServer(ctrl)
	responder := NewGRPCBlockListener(
		func(in interface{}) (int, error) {
			return 0, server.Send(in.(*iotexapi.StreamBlocksResponse))
		},
		errChan)

	receipts := []*action.Receipt{
		{
			BlockHeight: 1,
		},
		{
			BlockHeight: 2,
		},
	}
	builder := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetTimeStamp(time.Now()).
		SetReceipts(receipts)
	testBlock, err := builder.SignAndBuild(identityset.PrivateKey(0))
	require.NoError(t, err)

	server.EXPECT().Send(gomock.Any()).Return(nil).Times(1)
	require.NoError(t, responder.Respond("", &testBlock))

	server.EXPECT().Send(gomock.Any()).Return(errorSend).Times(1)
	require.Equal(t, errorSend, responder.Respond("", &testBlock))

	responder.Exit()

	require.Equal(t, errorSend, <-errChan)
	require.NoError(t, <-errChan)
}

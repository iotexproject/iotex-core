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

var errorSend error = errors.New("send error")

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

	// The stream write happens on the subscription's own goroutine now, so
	// Respond returns as soon as the message is queued and a failed write is
	// reported on errChan rather than inline. Respond must never block: it runs
	// on the goroutine that fans blocks out from CommitBlock.
	sent := make(chan struct{})
	server.EXPECT().Send(gomock.Any()).DoAndReturn(func(*iotexapi.StreamBlocksResponse) error {
		close(sent)
		return nil
	}).Times(1)
	require.NoError(t, responder.Respond("", &testBlock))
	<-sent

	server.EXPECT().Send(gomock.Any()).Return(errorSend).Times(1)
	require.NoError(t, responder.Respond("", &testBlock))
	require.Equal(t, errorSend, <-errChan)

	// the subscription has already ended, so Exit is a no-op rather than a
	// second send onto a channel nobody is reading
	responder.Exit()
	select {
	case err := <-errChan:
		require.Failf(t, "unexpected second outcome", "%v", err)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWeb3BlockListener(t *testing.T) {
	require := require.New(t)

	var streamErr error
	handler := func(in interface{}) (int, error) {
		return 0, streamErr
	}

	responder := NewWeb3BlockListener(handler)

	builder := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetTimeStamp(time.Now())
	testBlock, err := builder.SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)

	t.Run("success send blockInfo", func(t *testing.T) {
		streamErr = nil
		err = responder.Respond("test", &testBlock)
		require.NoError(err)
	})

	t.Run("stream handle raise error", func(t *testing.T) {
		streamErr = errorSend
		err = responder.Respond("test", &testBlock)
		require.Equal(errorSend, err)
	})

	responder.Exit()
}

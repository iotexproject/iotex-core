// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiresponder"
)

// test for chainListener
func TestChainListener(t *testing.T) {
	ctrl := gomock.NewController(t)

	responder := mock_apiresponder.NewMockResponder(ctrl)
	listener := NewChainListener()

	err := listener.Start()
	require.NoError(t, err)

	err = listener.Stop()
	require.NoError(t, err)

	err = listener.AddResponder(responder)
	require.NoError(t, err)

	err = listener.AddResponder(responder)
	require.Error(t, err)
	require.Equal(t, errorResponderAdded, err)

	block := &block.Block{
		Header: block.Header{},
		Body:   block.Body{},
		Footer: block.Footer{},
	}
	responder.EXPECT().Respond(gomock.Any()).Return(nil).Times(1)
	err = listener.ReceiveBlock(block)
	require.NoError(t, err)

	expectedError := errors.New("Error when streaming the block")
	responder.EXPECT().Respond(gomock.Any()).Return(expectedError).Times(1)
	err = listener.ReceiveBlock(block)
	require.NoError(t, err)

	responder.EXPECT().Exit().Return().Times(1)
	err = listener.AddResponder(responder)
	require.NoError(t, err)
	err = listener.Stop()
	require.NoError(t, err)
}

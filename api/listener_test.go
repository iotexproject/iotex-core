// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiresponder"
)

// test for chainListener
func TestChainListener(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	responder := mock_apiresponder.NewMockResponder(ctrl)
	listener := NewChainListener(1)
	r.NoError(listener.Start())
	r.NoError(listener.Stop())

	r.NoError(listener.AddResponder(responder))
	r.Equal(errorResponderAdded, listener.AddResponder(responder))

	block := &block.Block{
		Header: block.Header{},
		Body:   block.Body{},
		Footer: block.Footer{},
	}
	responder.EXPECT().Respond(gomock.Any()).Return(nil).Times(1)
	r.NoError(listener.ReceiveBlock(block))

	responder.EXPECT().Respond(gomock.Any()).Return(errorKeyIsNotResponder).Times(1)
	r.NoError(listener.ReceiveBlock(block))

	responder.EXPECT().Exit().Return().Times(1)
	r.NoError(listener.AddResponder(responder))
	responder1 := mock_apiresponder.NewMockResponder(ctrl)
	r.Equal(errorCapacityReached, listener.AddResponder(responder1))
	r.NoError(listener.Stop())
}

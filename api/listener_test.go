// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/blockchain/block"
	mock_apitypes "github.com/iotexproject/iotex-core/test/mock/mock_apiresponder"
	"github.com/stretchr/testify/require"
)

// test for chainListener
func TestChainListener(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	responder := mock_apitypes.NewMockResponder(ctrl)
	listener := NewChainListener(1)
	r.NoError(listener.Start())
	r.NoError(listener.Stop())

	block := &block.Block{
		Header: block.Header{},
		Body:   block.Body{},
		Footer: block.Footer{},
	}
	t.Run("errorUnsupportedType", func(t *testing.T) {
		_, err := listener.AddResponder(responder)
		r.NoError(err)
		responder.EXPECT().Respond(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		r.NoError(listener.ReceiveBlock(block))
		responder.EXPECT().Respond(gomock.Any(), gomock.Any()).Return(errorUnsupportedType).Times(1)
		r.NoError(listener.ReceiveBlock(block))
	})

	t.Run("errorCapacityReached", func(t *testing.T) {
		responder.EXPECT().Exit().Return().AnyTimes()
		_, err := listener.AddResponder(responder)
		r.NoError(err)
		responder1 := mock_apitypes.NewMockResponder(ctrl)
		_, err = listener.AddResponder(responder1)
		r.Equal(errorCapacityReached, err)
		r.NoError(listener.Stop())
	})

	t.Run("removeResponder", func(t *testing.T) {
		id, err := listener.AddResponder(responder)
		r.NoError(err)
		responder.EXPECT().Respond(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		r.NoError(listener.ReceiveBlock(block))
		ret, err := listener.RemoveResponder(id)
		r.True(ret)
		r.NoError(err)
	})
}

func TestRandID(t *testing.T) {
	require := require.New(t)

	gen := newIDGenerator(_idSize)
	id := gen.newID()
	require.Equal(34, len(id))
}

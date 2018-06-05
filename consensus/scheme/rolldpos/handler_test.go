// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/consensus/fsm"
)

func TestInitProposeInjectError(t *testing.T) {
	t.Parallel()

	err := errors.New("error")
	cb := rollDPoSCB{
		propCb: func() (*blockchain.Block, error) {
			return nil, err
		},
	}
	h := initPropose{
		RollDPoS: &RollDPoS{
			rollDPoSCB: cb,
		},
	}
	h.roundCtx = &roundCtx{}

	evt := &fsm.Event{}
	h.Handle(evt)

	assert.Equal(t, evt.Err, err)
}

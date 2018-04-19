// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rdpos

import (
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInitProposeInjectError(t *testing.T) {
	t.Parallel()

	err := errors.New("error")
	h := initPropose{
		RDPoS: &RDPoS{
			propCb: func() (*blockchain.Block, error) {
				return nil, err
			},
		},
	}

	evt := &fsm.Event{}
	h.Handle(evt)

	assert.Equal(t, evt.Err, err)
}

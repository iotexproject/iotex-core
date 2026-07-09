// Copyright (c) 2024 IoTeX Foundation
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

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestServerV2ReceiveBlockAndCoreService(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	core := NewMockCoreService(ctrl)
	svr := &ServerV2{core: core}

	// CoreService returns the wrapped core.
	r.Equal(core, svr.CoreService())

	builder := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(1).
		SetTimeStamp(time.Now())
	blk, err := builder.SignAndBuild(identityset.PrivateKey(0))
	r.NoError(err)

	t.Run("receive block success delegates to core", func(t *testing.T) {
		core.EXPECT().ReceiveBlock(gomock.Any()).Return(nil).Times(1)
		r.NoError(svr.ReceiveBlock(&blk))
	})

	t.Run("receive block error is propagated", func(t *testing.T) {
		expectErr := errors.New("receive failed")
		core.EXPECT().ReceiveBlock(gomock.Any()).Return(expectErr).Times(1)
		r.Equal(expectErr, svr.ReceiveBlock(&blk))
	})
}

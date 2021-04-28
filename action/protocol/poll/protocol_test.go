// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
)

func TestNewProtocol(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	committee := mock_committee.NewMockCommittee(ctrl)
	cfg := config.Default
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.ScoreThreshold = "1200000"
	p, err := NewProtocol(
		cfg,
		nil,
		func(context.Context, string, []byte, bool) ([]byte, error) { return nil, nil },
		nil,
		nil,
		nil,
		committee,
		nil,
		func(uint64) (time.Time, error) { return time.Now(), nil },
		func(uint64, uint64) (map[string]uint64, error) {
			return nil, nil
		},
		func(context.Context, uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		},
	)
	require.NoError(err)
	require.NotNil(p)
}

func TestFindProtocol(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p, _, _, _, _ := initConstruct(ctrl)
	//if not registered
	re := protocol.NewRegistry()
	require.Nil(FindProtocol(re))

	//if registered
	require.NoError(p.Register(re))
	require.NotNil(FindProtocol(re))
}

func TestMustGetProtocol(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p, _, _, _, _ := initConstruct(ctrl)
	//if not registered
	re := protocol.NewRegistry()
	require.Panics(func() { MustGetProtocol(re) })

	//if registered
	require.NoError(p.Register(re))
	require.NotNil(FindProtocol(re))
}

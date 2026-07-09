// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockchain_test

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
)

// headerWithProducer builds a signed header whose producer is identityset key i.
func headerWithProducer(t *testing.T, i int) *block.Header {
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(1).
		SignAndBuild(identityset.PrivateKey(i))
	require.NoError(t, err)
	h := blk.Header
	return &h
}

func TestProductivity(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	bc := mock_blockchain.NewMockBlockchain(ctrl)

	producerA := identityset.Address(0).String()
	producerB := identityset.Address(1).String()
	hdrA := headerWithProducer(t, 0)
	hdrB := headerWithProducer(t, 1)

	// heights 1,2 produced by A; height 3 by B
	bc.EXPECT().BlockHeaderByHeight(uint64(1)).Return(hdrA, nil).Times(1)
	bc.EXPECT().BlockHeaderByHeight(uint64(2)).Return(hdrA, nil).Times(1)
	bc.EXPECT().BlockHeaderByHeight(uint64(3)).Return(hdrB, nil).Times(1)

	stats, err := blockchain.Productivity(bc, 1, 3)
	r.NoError(err)
	r.Equal(uint64(2), stats[producerA])
	r.Equal(uint64(1), stats[producerB])
}

func TestProductivityError(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	bc := mock_blockchain.NewMockBlockchain(ctrl)

	bc.EXPECT().BlockHeaderByHeight(uint64(1)).Return(nil, errors.New("missing header")).Times(1)
	_, err := blockchain.Productivity(bc, 1, 3)
	r.Error(err)
	r.Contains(err.Error(), "missing header")
}

func TestBlockchainAccessorsAndSubscriber(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	dao := mock_blockdao.NewMockBlockDAO(ctrl)

	cfg := blockchain.DefaultConfig
	cfg.ID = 42
	cfg.EVMNetworkID = 4689
	cfg.Address = "io1address"
	g := genesis.TestDefault()

	bc := blockchain.NewBlockchain(cfg, g, dao, nil)
	r.Equal(uint32(42), bc.ChainID())
	r.Equal(uint32(4689), bc.EvmNetworkID())
	r.Equal("io1address", bc.ChainAddress())
	gotGenesis := bc.Genesis()
	r.Equal(g.Hash(), gotGenesis.Hash())

	// a nil subscriber is rejected
	r.Error(bc.AddSubscriber(nil))
}

// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rdpos"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/txpool"
)

// Consensus is the interface for handling consensus view change.
type Consensus interface {
	Start() error
	Stop() error
	HandleViewChange(proto.Message, chan bool) error
	HandleBlockPropose(proto.Message, chan bool) error
}

type consensus struct {
	cfg    *config.Consensus
	scheme scheme.Scheme
}

// NewConsensus creates a consensus struct.
func NewConsensus(cfg *config.Config, bc blockchain.Blockchain, tp txpool.TxPool, bs blocksync.BlockSync, dlg delegate.Pool) Consensus {
	if bc == nil || bs == nil {
		glog.Error("Try to attach to chain or bs == nil")
		return nil
	}

	cs := &consensus{cfg: &cfg.Consensus}
	mintBlockCB := func() (*blockchain.Block, error) {
		blk, err := bc.MintNewBlock(tp.PickTxs(), cfg.Chain.MinerAddr, "")
		if err != nil {
			glog.Error("Failed to mint a block")
			return nil, err
		}
		glog.Infof("created a new block at height %v with %v txs", blk.Height(), len(blk.Tranxs))
		return blk, nil
	}

	tellBlockCB := func(msg proto.Message) error {
		return bs.P2P().Broadcast(msg)
	}

	commitBlockCB := func(blk *blockchain.Block) error {
		return bs.ProcessBlock(blk)
	}

	broadcastBlockCB := func(blk *blockchain.Block) error {
		if blkPb := blk.ConvertToBlockPb(); blkPb != nil {
			return bs.P2P().Broadcast(blkPb)
		}
		return nil
	}

	switch cfg.Consensus.Scheme {
	case "RDPOS":
		cs.scheme = rdpos.NewRDPoS(cfg.Consensus.RDPoS, mintBlockCB, tellBlockCB, commitBlockCB, broadcastBlockCB, bc, bs.P2P().Self(), dlg)
	case "NOOP":
		cs.scheme = scheme.NewNoop()
	case "STANDALONE":
		cs.scheme = scheme.NewStandalone(mintBlockCB, commitBlockCB, broadcastBlockCB, bc, cfg.Consensus.BlockCreationInterval)
	default:
		glog.Errorf("unexpected consensus scheme", cfg.Consensus.Scheme)
		return nil
	}

	return cs
}

func (c *consensus) Start() error {
	glog.Infof("Starting consensus scheme %v", c.cfg.Scheme)

	c.scheme.Start()
	return nil
}

func (c *consensus) Stop() error {
	glog.Infof("Stopping consensus scheme %v", c.cfg.Scheme)

	c.scheme.Stop()
	return nil
}

// HandleViewChange dispatches the call to different schemes
func (c *consensus) HandleViewChange(m proto.Message, done chan bool) error {
	return c.scheme.Handle(m)
}

// HandleBlockPropose handles a proposed block
func (c *consensus) HandleBlockPropose(m proto.Message, done chan bool) error {
	return nil
}

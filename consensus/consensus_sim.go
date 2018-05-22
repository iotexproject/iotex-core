// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"bytes"
	"encoding/gob"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	"encoding/hex"
	"github.com/iotexproject/iotex-core-internal/blockchain"
	"github.com/iotexproject/iotex-core-internal/blocksync"
	"github.com/iotexproject/iotex-core-internal/config"
	"github.com/iotexproject/iotex-core-internal/consensus/scheme"
	"github.com/iotexproject/iotex-core-internal/consensus/scheme/rdpos"
	"github.com/iotexproject/iotex-core-internal/delegate"
	pb "github.com/iotexproject/iotex-core-internal/simulator/proto/simulator"
	"github.com/iotexproject/iotex-core-internal/txpool"
)

// Consensus is the interface for handling consensus view change.
type ConsensusSim interface {
	Start() error
	Stop() error
	HandleViewChange(proto.Message, chan bool) error
	HandleBlockPropose(proto.Message, chan bool) error
}

// consensus struct with a stream parameter for writing to simulator stream
type consensus_sim struct {
	cfg    *config.Consensus
	scheme scheme.Scheme
	stream pb.Simulator_PingServer
}

// NewConsensus creates a consensus struct.
func NewConsensusSim(cfg *config.Config, bc blockchain.Blockchain, tp txpool.TxPool, bs blocksync.BlockSync, dlg delegate.Pool) ConsensusSim {
	if bc == nil || bs == nil {
		glog.Error("Try to attach to chain or bs == nil")
		return nil
	}

	cs := &consensus_sim{cfg: &cfg.Consensus}
	mintBlockCB := func() (*blockchain.Block, error) {
		blk, err := bc.MintNewBlock(tp.PickTxs(), &cfg.Chain.MinerAddr, "")
		if err != nil {
			glog.Error("Failed to mint a block")
			return nil, err
		}
		glog.Infof("created a new block at height %v with %v txs", blk.Height(), len(blk.Tranxs))
		return blk, nil
	}

	// broadcast a message across the P2P network
	tellBlockCB := func(msg proto.Message) error {
		s := serializeMsg(msg)
		cs.sendMessage(0, s)

		return nil
	}

	// commit a block to the blockchain
	commitBlockCB := func(blk *blockchain.Block) error {
		hash := [32]byte(blk.HashBlock())
		s := hex.EncodeToString(hash[:])
		cs.sendMessage(1, s)

		return nil
	}

	// broadcast a block across the P2P network
	broadcastBlockCB := func(blk *blockchain.Block) error {
		if blkPb := blk.ConvertToBlockPb(); blkPb != nil {
			s := serializeMsg(blkPb)
			cs.sendMessage(0, s)
		}
		return nil
	}

	cs.scheme = rdpos.NewRDPoS(cfg.Consensus.RDPoS, mintBlockCB, tellBlockCB, commitBlockCB, broadcastBlockCB, bc, bs.P2P().Self(), dlg)

	return cs
}

func (c *consensus_sim) sendMessage(messageType int, value string) {
	if err := c.stream.Send(&pb.Reply{MessageType: int32(messageType), Value: value}); err != nil {
		glog.Error("Message cannot be sent through stream")
	}
}

func (c *consensus_sim) SetStream(stream pb.Simulator_PingServer) {
	c.stream = stream
}

func (c *consensus_sim) Start() error {
	glog.Infof("Starting consensus scheme %v", c.cfg.Scheme)

	c.scheme.Start()
	return nil
}

func (c *consensus_sim) Stop() error {
	glog.Infof("Stopping consensus scheme %v", c.cfg.Scheme)

	c.scheme.Stop()
	return nil
}

// HandleViewChange dispatches the call to different schemes
func (c *consensus_sim) HandleViewChange(m proto.Message, done chan bool) error {
	return c.scheme.Handle(m)
}

// HandleBlockPropose handles a proposed block -- not used currently
func (c *consensus_sim) HandleBlockPropose(m proto.Message, done chan bool) error {
	return nil
}

func serializeMsg(m proto.Message) string {
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(m); err != nil {
		glog.Error("Message cannot be serialized")
	}
	return b.String()
}

func unserializeMsg(s string) proto.Message {
	var m proto.Message

	var b bytes.Buffer
	b.WriteString(s)

	d := gob.NewDecoder(&b)

	if err := d.Decode(&m); err != nil {
		glog.Error("Received message cannot be unserialized")
	}

	return m
}

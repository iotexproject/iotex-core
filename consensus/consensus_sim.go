// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"encoding/hex"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rdpos"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/proto"
	pb1 "github.com/iotexproject/iotex-core/proto"
	pb "github.com/iotexproject/iotex-core/simulator/proto/simulator"
	"github.com/iotexproject/iotex-core/txpool"
)

// ConsensusSim is the interface for handling consensus view change used in the simulator
type ConsensusSim interface {
	Start() error
	Stop() error
	HandleViewChange(proto.Message, chan bool) error
	HandleBlockPropose(proto.Message, chan bool) error
	SetStream(pb.Simulator_PingServer)
}

// consensus_sim struct with a stream parameter for writing to simulator stream
type consensusSim struct {
	cfg    *config.Consensus
	scheme scheme.Scheme
	stream pb.Simulator_PingServer
}

// NewConsensusSim creates a consensus_sim struct
func NewConsensusSim(cfg *config.Config, bc blockchain.Blockchain, tp txpool.TxPool, bs blocksync.BlockSync, dlg delegate.Pool) ConsensusSim {
	if bc == nil {
		glog.Error("Blockchain is nil")
		return nil
	}

	if bs == nil {
		glog.Error("Blocksync is nil")
		return nil
	}

	cs := &consensusSim{cfg: &cfg.Consensus}
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
		msgType, msgBody := SeparateMsg(msg)
		msgBodyS := hex.EncodeToString(msgBody)

		cs.sendMessage(0, msgType, msgBodyS)

		return nil
	}

	// commit a block to the blockchain
	commitBlockCB := func(blk *blockchain.Block) error {
		hash := [32]byte(blk.HashBlock())
		s := hex.EncodeToString(hash[:])
		cs.sendMessage(1, 0, s)

		return bc.AddBlockCommit(blk)
	}

	// broadcast a block across the P2P network
	broadcastBlockCB := func(blk *blockchain.Block) error {
		if blkPb := blk.ConvertToBlockPb(); blkPb != nil {
			msgType, msgBody := SeparateMsg(blkPb)
			msgBodyS := hex.EncodeToString(msgBody)

			cs.sendMessage(0, msgType, msgBodyS)
		}
		return nil
	}

	cs.scheme = rdpos.NewRDPoS(cfg.Consensus.RDPoS, mintBlockCB, tellBlockCB, commitBlockCB, broadcastBlockCB, bc, bs.P2P().Self(), dlg)

	return cs
}

func (c *consensusSim) sendMessage(messageType int, internalMsgType uint32, value string) {
	if err := c.stream.Send(&pb.Reply{MessageType: int32(messageType), InternalMsgType: internalMsgType, Value: value}); err != nil {
		glog.Error("Message cannot be sent through stream")
	}
}

func (c *consensusSim) SetStream(stream pb.Simulator_PingServer) {
	c.stream = stream
}

func (c *consensusSim) Start() error {
	glog.Infof("Starting consensus scheme %v", c.cfg.Scheme)

	c.scheme.Start()
	return nil
}

func (c *consensusSim) Stop() error {
	glog.Infof("Stopping consensus scheme %v", c.cfg.Scheme)

	c.scheme.Stop()
	return nil
}

// HandleViewChange dispatches the call to different schemes
func (c *consensusSim) HandleViewChange(m proto.Message, done chan bool) error {
	err := c.scheme.Handle(m)
	c.scheme.SetDoneStream(done)

	return err
}

// HandleBlockPropose handles a proposed block -- not used currently
func (c *consensusSim) HandleBlockPropose(m proto.Message, done chan bool) error {
	return nil
}

// SeparateMsg separates a proto.Message into its msgType and msgBody
func SeparateMsg(m proto.Message) (uint32, []byte) {
	msgType, err := iproto.GetTypeFromProtoMsg(m)

	if err != nil {
		glog.Error("Cannot retrieve message type from message")
	}

	msgBody, err := proto.Marshal(m)

	if err != nil {
		glog.Error("Cannot retrieve message body from message")
	}

	return msgType, msgBody
}

// CombineMsg combines a msgType and msgBody into a single proto.Message
func CombineMsg(msgType uint32, msgBody []byte) proto.Message {
	protoMsg, err := pb1.TypifyProtoMsg(msgType, msgBody)

	if err != nil {
		glog.Error("Could not combine msgType and msgBody into a proto.Message object")
	}

	return protoMsg
}

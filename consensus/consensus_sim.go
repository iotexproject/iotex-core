// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"encoding/hex"
	"fmt"

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

// Init is set to true if the simulator is in the process of initialization; the message type sent back during Init phase is different because proposals happen spontaneously without prompting
var Init bool

// ConsensusSim is the interface for handling consensus view change used in the simulator
type ConsensusSim interface {
	Start() error
	Stop() error
	HandleViewChange(proto.Message, chan bool) error
	HandleBlockPropose(proto.Message, chan bool) error
	SetStream(*pb.Simulator_PingServer)
	SetInitStream(*pb.Simulator_InitServer)
	SetDoneStream(chan bool)
	SetID(int)
}

// consensus_sim struct with a stream parameter for writing to simulator stream
type consensusSim struct {
	cfg        *config.Consensus
	scheme     scheme.Scheme
	stream     pb.Simulator_PingServer
	initStream pb.Simulator_InitServer
	ID         int
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
		fmt.Println("mintBlockCB called")

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
		fmt.Println("tellBlockCB called")

		fmt.Printf("id: %d, pointer: %p\n", cs.ID, cs)

		msgType, msgBody := SeparateMsg(msg)
		msgBodyS := hex.EncodeToString(msgBody)

		cs.sendMessage(0, msgType, msgBodyS)

		return nil
	}

	// commit a block to the blockchain
	commitBlockCB := func(blk *blockchain.Block) error {
		fmt.Println("commitBlockCB called")

		fmt.Printf("id: %d, pointer: %p\n", cs.ID, cs)

		hash := [32]byte(blk.HashBlock())
		s := hex.EncodeToString(hash[:])
		cs.sendMessage(1, 0, s)

		return bc.AddBlockCommit(blk)
	}

	// broadcast a block across the P2P network
	broadcastBlockCB := func(blk *blockchain.Block) error {
		fmt.Println("broadcastBlockCB called")
		fmt.Printf("id: %d, pointer: %p\n", cs.ID, cs)

		if blkPb := blk.ConvertToBlockPb(); blkPb != nil {
			msgType, msgBody := SeparateMsg(blkPb)
			msgBodyS := hex.EncodeToString(msgBody)

			cs.sendMessage(0, msgType, msgBodyS)
		}
		return nil
	}

	cs.scheme = rdpos.NewRDPoS(cfg.Consensus.RDPoS, mintBlockCB, tellBlockCB, commitBlockCB, broadcastBlockCB, bc, bs.P2P().Self(), dlg)

	fmt.Printf("cs pointer: %p\n", cs)
	return cs
}

func (c *consensusSim) SetID(ID int) {
	c.ID = ID
}

func (c *consensusSim) sendMessage(messageType int, internalMsgType uint32, value string) {
	if Init {
		fmt.Println("Sending init/proposal message")
		if c.initStream == nil {
			glog.Error("Init stream is nil")
		}

		if err := c.initStream.Send(&pb.Proposal{PlayerID: int32(c.ID), InternalMsgType: internalMsgType, Value: value}); err != nil {
			glog.Error("Message cannot be sent through stream")
			return
		}
	} else {
		fmt.Println("Sending view state change message")

		if c.stream == nil {
			fmt.Println(c.stream)
			fmt.Println(&c.stream)
			glog.Error("Stream is nil")
		}

		if err := c.stream.Send(&pb.Reply{MessageType: int32(messageType), InternalMsgType: internalMsgType, Value: value}); err != nil {
			glog.Error("Message cannot be sent through stream")
			return
		}
	}

	fmt.Println("Successfully sent message")
}

func (c *consensusSim) SetInitStream(stream *pb.Simulator_InitServer) {
	c.initStream = *stream
}
func (c *consensusSim) SetStream(stream *pb.Simulator_PingServer) {

	fmt.Println("Set stream")

	c.stream = *stream
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

// SetDoneStream takes in a boolean channel which will be filled when the consensus is done processing
func (c *consensusSim) SetDoneStream(done chan bool) {
	c.scheme.SetDoneStream(done)
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

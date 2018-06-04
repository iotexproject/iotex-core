// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"encoding/hex"
	"fmt"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/proto"
	pb1 "github.com/iotexproject/iotex-core/proto"
	pb "github.com/iotexproject/iotex-core/simulator/proto/simulator"
	"github.com/iotexproject/iotex-core/txpool"
)

// Sim is the interface for handling consensus view change used in the simulator
type Sim interface {
	Start() error
	Stop() error
	HandleViewChange(proto.Message, chan bool) error
	HandleBlockPropose(proto.Message, chan bool) error
	SetStream(*pb.Simulator_PingServer)
	SetDoneStream(chan bool)
	SendUnsent()
}

// consensus_sim struct with a stream parameter for writing to simulator stream
type sim struct {
	cfg    *config.Consensus
	scheme scheme.Scheme
	stream pb.Simulator_PingServer
	ID     int
	unsent []*pb.Reply
}

// NewSim creates a consensus_sim struct
func NewSim(cfg *config.Config, bc blockchain.Blockchain, tp txpool.TxPool, bs blocksync.BlockSync, dlg delegate.Pool) Sim {
	if bc == nil {
		logger.Error().Msg("Blockchain is nil")
		return nil
	}

	if bs == nil {
		logger.Error().Msg("Blocksync is nil")
		return nil
	}

	cs := &sim{cfg: &cfg.Consensus}

	mintBlockCB := func() (*blockchain.Block, error) {
		fmt.Println("mintBlockCB called")

		blk, err := bc.MintNewBlock(tp.PickTxs(), &cfg.Chain.MinerAddr, "")
		if err != nil {
			logger.Error().Msg("Failed to mint a block")
			return nil, err
		}
		logger.Info().
			Uint64("height", blk.Height()).
			Int("txs", len(blk.Tranxs)).
			Msg("created a new block")
		return blk, nil
	}

	// broadcast a message across the P2P network
	tellBlockCB := func(msg proto.Message) error {
		fmt.Println("tellBlockCB called")

		msgType, msgBody := SeparateMsg(msg)
		msgBodyS := hex.EncodeToString(msgBody)

		cs.sendMessage(0, msgType, msgBodyS)
		return nil
	}

	// commit a block to the blockchain
	commitBlockCB := func(blk *blockchain.Block) error {
		fmt.Println("commitBlockCB called")

		hash := [32]byte(blk.HashBlock())
		s := hex.EncodeToString(hash[:])
		cs.sendMessage(1, 0, s)
		return bc.AddBlockCommit(blk)
	}

	// broadcast a block across the P2P network
	broadcastBlockCB := func(blk *blockchain.Block) error {
		fmt.Println("broadcastBlockCB called")

		if blkPb := blk.ConvertToBlockPb(); blkPb != nil {
			msgType, msgBody := SeparateMsg(blkPb)
			msgBodyS := hex.EncodeToString(msgBody)

			cs.sendMessage(0, msgType, msgBodyS)
		}
		return nil
	}

	cs.scheme = rolldpos.NewRollDPoS(
		cfg.Consensus.RollDPoS,
		mintBlockCB,
		tellBlockCB,
		commitBlockCB,
		broadcastBlockCB,
		rolldpos.FixedProposer,
		rolldpos.GeneratePseudoDKG,
		bc,
		bs.P2P().Self(),
		dlg)
	cs.unsent = make([]*pb.Reply, 0)

	fmt.Printf("cs pointer: %p\n", cs)
	return cs
}

func (c *sim) SetID(ID int) {
	c.ID = ID
}

func (c *sim) sendMessage(messageType int, internalMsgType uint32, value string) {
	fmt.Println("Sending view state change message")
	if internalMsgType == 1999 {
		fmt.Println("what is going on")
	}

	msg := &pb.Reply{MessageType: int32(messageType), InternalMsgType: internalMsgType, Value: value}

	if c.stream == nil || c.stream.Send(msg) != nil {
		fmt.Println("Could not send message; stored in unsent message array")
		c.unsent = append(c.unsent, msg)
		return
	}

	fmt.Println("Successfully sent message")
}

func (c *sim) SetStream(stream *pb.Simulator_PingServer) {
	fmt.Println("Set stream")

	c.stream = *stream
}

func (c *sim) Start() error {
	logger.Info().
		Str("scheme", c.cfg.Scheme).
		Msg("Starting consensus scheme")

	c.scheme.Start()
	return nil
}

func (c *sim) Stop() error {
	logger.Info().
		Str("scheme", c.cfg.Scheme).
		Msg("Stopping consensus scheme")

	c.scheme.Stop()
	return nil
}

// HandleViewChange dispatches the call to different schemes
func (c *sim) HandleViewChange(m proto.Message, done chan bool) error {

	err := c.scheme.Handle(m)
	c.scheme.SetDoneStream(done)
	return err
}

// SendUnsent sends all the unsent messages that were not able to send previously
func (c *sim) SendUnsent() {
	for i := 0; i < len(c.unsent); i++ {
		fmt.Println("Sent previously unsent message")
		c.stream.Send(c.unsent[i])
	}
	c.unsent = make([]*pb.Reply, 0)
}

// SetDoneStream takes in a boolean channel which will be filled when the consensus is done processing
func (c *sim) SetDoneStream(done chan bool) {
	c.scheme.SetDoneStream(done)
}

// HandleBlockPropose handles a proposed block -- not used currently
func (c *sim) HandleBlockPropose(m proto.Message, done chan bool) error {
	return nil
}

// SeparateMsg separates a proto.Message into its msgType and msgBody
func SeparateMsg(m proto.Message) (uint32, []byte) {
	msgType, err := iproto.GetTypeFromProtoMsg(m)

	if err != nil {
		logger.Error().Msg("Cannot retrieve message type from message")
	}

	msgBody, err := proto.Marshal(m)

	if err != nil {
		logger.Error().Msg("Cannot retrieve message body from message")
	}
	return msgType, msgBody
}

// CombineMsg combines a msgType and msgBody into a single proto.Message
func CombineMsg(msgType uint32, msgBody []byte) proto.Message {
	protoMsg, err := pb1.TypifyProtoMsg(msgType, msgBody)

	if err != nil {
		logger.Error().Msg("Could not combine msgType and msgBody into a proto.Message object")
	}
	return protoMsg
}

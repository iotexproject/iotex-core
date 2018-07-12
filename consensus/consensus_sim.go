// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"context"
	"encoding/hex"
	"math/big"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/proto"
	pb "github.com/iotexproject/iotex-core/proto"
	pbsim "github.com/iotexproject/iotex-core/simulator/proto/simulator"
)

const (
	viewStateChangeMsg int32 = 0
	commitBlockMsg     int32 = 1
	proposeBlockMsg    int32 = 2
)

// Sim is the interface for handling IotxConsensus view change used in the simulator
type Sim interface {
	lifecycle.StartStopper

	HandleViewChange(proto.Message, chan bool) error
	HandleBlockPropose(proto.Message, chan bool) error
	SetStream(*pbsim.Simulator_PingServer)
	SetDoneStream(chan bool)
	SendUnsent()
}

// consensus_sim struct with a stream parameter for writing to simulator stream
type sim struct {
	cfg    *config.Consensus
	scheme scheme.Scheme
	stream pbsim.Simulator_PingServer
	unsent []*pbsim.Reply
}

// NewSim creates a consensus_sim struct
func NewSim(
	cfg *config.Config,
	bc blockchain.Blockchain,
	bs blocksync.BlockSync,
	dlg delegate.Pool,
) Sim {
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
		logger.Debug().Msg("mintBlockCB called")
		// TODO: get list of Transfer and Vote from actpool, instead of nil, nil below
		addr, err := cfg.ProducerAddr()
		if err != nil {
			return nil, err
		}
		blk, err := bc.MintNewBlock(nil, nil, addr, "")
		if err != nil {
			logger.Error().Msg("Failed to mint a block")
			return nil, err
		}
		logger.Info().
			Uint64("height", blk.Height()).
			Int("transfers", len(blk.Transfers)).
			Msg("created a new block")

		return blk, nil
	}

	// broadcast a message across the P2P network
	tellBlockCB := func(msg proto.Message) error {
		logger.Debug().Msg("tellBlockCB called")

		msgType, msgBody := SeparateMsg(msg)
		msgBodyS := hex.EncodeToString(msgBody)

		// check if message is a newly proposed block
		vc, ok := (msg).(*iproto.ViewChangeMsg)
		if ok && vc.Vctype == iproto.ViewChangeMsg_PROPOSE {
			// send msg + block hash for recording metrics on sim side
			cs.sendMessage(proposeBlockMsg, msgType, msgBodyS+"|"+hex.EncodeToString(vc.BlockHash))
		} else {
			cs.sendMessage(viewStateChangeMsg, msgType, msgBodyS)
		}

		return nil
	}

	// commit a block to the blockchain
	commitBlockCB := func(blk *blockchain.Block) error {
		logger.Debug().Msg("commitBlockCB called")

		hash := [32]byte(blk.HashBlock())
		s := hex.EncodeToString(hash[:])
		cs.sendMessage(commitBlockMsg, 0, s)
		return bc.CommitBlock(blk)
	}

	// broadcast a block across the P2P network
	broadcastBlockCB := func(blk *blockchain.Block) error {
		logger.Debug().Msg("broadcastBlockCB called")

		if blkPb := blk.ConvertToBlockPb(); blkPb != nil {
			msgType, msgBody := SeparateMsg(blkPb)
			msgBodyS := hex.EncodeToString(msgBody)

			cs.sendMessage(viewStateChangeMsg, msgType, msgBodyS)
		}
		return nil
	}

	addr, err := cfg.ProducerAddr()
	if err != nil {
		logger.Panic().Err(err).Msg("Fail to create new consensus")
	}

	cs.scheme = rolldpos.NewRollDPoS(
		cfg.Consensus.RollDPoS,
		mintBlockCB,
		tellBlockCB,
		commitBlockCB,
		broadcastBlockCB,
		chooseGetProposerCB(cfg.Consensus.RollDPoS.ProposerCB),
		chooseStartNextEpochCB(cfg.Consensus.RollDPoS.EpochCB),
		rolldpos.GeneratePseudoDKG,
		bc,
		addr.RawAddress,
		dlg,
	)
	cs.unsent = make([]*pbsim.Reply, 0)

	return cs
}

// NewSimByzantine creates a byzantine consensus_sim struct
func NewSimByzantine(
	cfg *config.Config,
	bc blockchain.Blockchain,
	bs blocksync.BlockSync,
	dlg delegate.Pool,
) Sim {
	if bc == nil {
		logger.Error().Msg("Blockchain is nil")
		return nil
	}

	if bs == nil {
		logger.Error().Msg("Blocksync is nil")
		return nil
	}

	cs := &sim{cfg: &cfg.Consensus}

	// modify mintBlockCB so that it returns a fraudulent block
	mintBlockCB := func() (*blockchain.Block, error) {
		logger.Debug().Msg("mintBlockCB called")

		// create sample transactions
		addr, err := cfg.ProducerAddr()
		if err != nil {
			return nil, err
		}
		tsf := []*action.Transfer{
			action.NewCoinBaseTransfer(big.NewInt(100), addr.RawAddress),
			action.NewCoinBaseTransfer(big.NewInt(200), addr.RawAddress),
			action.NewCoinBaseTransfer(big.NewInt(300), addr.RawAddress),
		}
		// TODO: create sample Transfer and Vote to replace nil, nil below
		blk, err := bc.MintNewBlock(tsf, nil, addr, "")
		if err != nil {
			logger.Error().Msg("Failed to mint a block")
			return nil, err
		}
		logger.Info().
			Uint64("height", blk.Height()).
			Int("transfers", len(blk.Transfers)).
			Msg("created a new block")

		return blk, nil
	}

	// broadcast a message across the P2P network
	tellBlockCB := func(msg proto.Message) error {
		logger.Debug().Msg("tellBlockCB called")

		msgType, msgBody := SeparateMsg(msg)
		msgBodyS := hex.EncodeToString(msgBody)

		// check if message is a newly proposed block
		vc, ok := (msg).(*iproto.ViewChangeMsg)
		if ok && vc.Vctype == iproto.ViewChangeMsg_PROPOSE {
			cs.sendMessage(proposeBlockMsg, msgType, msgBodyS+"|"+hex.EncodeToString(vc.BlockHash)) // send msg + block hash for recording metrics on sim side
		} else {
			cs.sendMessage(viewStateChangeMsg, msgType, msgBodyS)
		}

		return nil
	}

	// commit a block to the blockchain
	commitBlockCB := func(blk *blockchain.Block) error {
		logger.Debug().Msg("commitBlockCB called")

		hash := [32]byte(blk.HashBlock())
		s := hex.EncodeToString(hash[:])
		cs.sendMessage(commitBlockMsg, 0, s)
		return bc.CommitBlock(blk)
	}

	// broadcast a block across the P2P network
	broadcastBlockCB := func(blk *blockchain.Block) error {
		logger.Debug().Msg("broadcastBlockCB called")

		if blkPb := blk.ConvertToBlockPb(); blkPb != nil {
			msgType, msgBody := SeparateMsg(blkPb)
			msgBodyS := hex.EncodeToString(msgBody)

			cs.sendMessage(viewStateChangeMsg, msgType, msgBodyS)
		}
		return nil
	}

	addr, err := cfg.ProducerAddr()
	if err != nil {
		logger.Panic().Err(err).Msg("Fail to create new consensus")
	}

	cs.scheme = rolldpos.NewRollDPoS(
		cfg.Consensus.RollDPoS,
		mintBlockCB,
		tellBlockCB,
		commitBlockCB,
		broadcastBlockCB,
		rolldpos.FixedProposer,
		rolldpos.NeverStartNewEpoch,
		rolldpos.GeneratePseudoDKG,
		bc,
		addr.RawAddress,
		dlg,
	)
	cs.unsent = make([]*pbsim.Reply, 0)

	return cs
}

func (c *sim) sendMessage(messageType int32, internalMsgType uint32, value string) {
	logger.Debug().Msg("Sending view state change message")

	msg := &pbsim.Reply{MessageType: messageType, InternalMsgType: internalMsgType, Value: value}

	if c.stream == nil || c.stream.Send(msg) != nil {
		c.unsent = append(c.unsent, msg)
		return
	}

	logger.Debug().Msg("Successfully sent message")
}

func (c *sim) SetStream(stream *pbsim.Simulator_PingServer) {
	logger.Debug().Msg("Set stream")

	c.stream = *stream
}

func (c *sim) Start(ctx context.Context) error {
	logger.Info().
		Str("scheme", c.cfg.Scheme).
		Msg("Starting IotxConsensus scheme")

	c.scheme.Start(ctx)
	return nil
}

func (c *sim) Stop(ctx context.Context) error {
	logger.Info().
		Str("scheme", c.cfg.Scheme).
		Msg("Stopping IotxConsensus scheme")

	c.scheme.Stop(ctx)
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
		c.stream.Send(c.unsent[i])
	}
	c.unsent = make([]*pbsim.Reply, 0)
}

// SetDoneStream takes in a boolean channel which will be filled when the IotxConsensus is done processing
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
	protoMsg, err := pb.TypifyProtoMsg(msgType, msgBody)

	if err != nil {
		logger.Error().Msg("Could not combine msgType and msgBody into a proto.Message object")
	}
	return protoMsg
}

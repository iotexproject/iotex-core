// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sim

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	consensus "github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	pbsim "github.com/iotexproject/iotex-core/consensus/sim/proto"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	viewStateChangeMsg int32 = 0
	commitBlockMsg     int32 = 1
	proposeBlockMsg    int32 = 2
)

// Sim is the interface for handling IotxConsensus view change used in the simulator
type Sim interface {
	lifecycle.StartStopper

	HandleEndorse(*iproto.EndorsePb, chan bool) error
	HandleBlockPropose(*iproto.ProposePb, chan bool) error
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
	cfg config.Config,
	bc blockchain.Blockchain,
	ap actpool.ActPool,
	p2p network.Overlay,
) Sim {
	if bc == nil || ap == nil || p2p == nil {
		logger.Panic().Msg("Try to attach to nil blockchain, action pool or p2p interface")
	}

	cs := &sim{cfg: &cfg.Consensus}

	/*
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
	*/

	var err error
	cs.scheme, err = rolldpos.NewRollDPoSBuilder().
		SetAddr(consensus.GetAddr(cfg)).
		SetConfig(cfg.Consensus.RollDPoS).
		SetBlockchain(bc).
		SetActPool(ap).
		SetP2P(p2p).
		Build()
	if err != nil {
		logger.Panic().Err(err).Msg("error when constructing RollDPoS")
	}
	return cs
}

// NewSimByzantine creates a byzantine consensus_sim struct
func NewSimByzantine(
	cfg config.Config,
	bc blockchain.Blockchain,
	ap actpool.ActPool,
	p2p network.Overlay,
) Sim {
	if bc == nil || ap == nil || p2p == nil {
		logger.Panic().Msg("Try to attach to nil blockchain, action pool or p2p interface")
	}

	cs := &sim{cfg: &cfg.Consensus}

	/*
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
	*/

	var err error
	cs.scheme, err = rolldpos.NewRollDPoSBuilder().
		SetAddr(consensus.GetAddr(cfg)).
		SetConfig(cfg.Consensus.RollDPoS).
		SetBlockchain(bc).
		SetActPool(ap).
		SetP2P(p2p).
		Build()
	if err != nil {
		logger.Panic().Err(err).Msg("error when constructing RollDPoS")
	}
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

	err := c.scheme.Start(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to start scheme %s", c.cfg.Scheme)
	}
	return nil
}

func (c *sim) Stop(ctx context.Context) error {
	logger.Info().
		Str("scheme", c.cfg.Scheme).
		Msg("Stopping IotxConsensus scheme")

	err := c.scheme.Stop(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to stop scheme %s", c.cfg.Scheme)
	}
	return nil
}

// HandleBlockPropose handles incoming block propose
func (c *sim) HandleBlockPropose(m *iproto.ProposePb, done chan bool) error {
	if err := c.scheme.HandleBlockPropose(m); err != nil {
		return err
	}
	c.scheme.SetDoneStream(done)
	return nil
}

// HandleEndorse handles incoming endorse
func (c *sim) HandleEndorse(m *iproto.EndorsePb, done chan bool) error {
	if err := c.scheme.HandleEndorse(m); err != nil {
		return err
	}
	c.scheme.SetDoneStream(done)
	return nil
}

// SendUnsent sends all the unsent messages that were not able to send previously
func (c *sim) SendUnsent() {
	var errs []error
	for i := 0; i < len(c.unsent); i++ {
		err := c.stream.Send(c.unsent[i])
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		logger.Error().Errs("failed to send unsent messages", errs)
	}
	c.unsent = make([]*pbsim.Reply, 0)
}

// SetDoneStream takes in a boolean channel which will be filled when the IotxConsensus is done processing
func (c *sim) SetDoneStream(done chan bool) {
	c.scheme.SetDoneStream(done)
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
	protoMsg, err := iproto.TypifyProtoMsg(msgType, msgBody)

	if err != nil {
		logger.Error().Msg("Could not combine msgType and msgBody into a proto.Message object")
	}
	return protoMsg
}

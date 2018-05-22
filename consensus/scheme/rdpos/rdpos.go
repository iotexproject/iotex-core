// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rdpos

import (
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/routine"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/logger"
	pb "github.com/iotexproject/iotex-core/proto"
)

var (
	// ErrInvalidViewChangeMsg is the error that ViewChangeMsg is invalid
	ErrInvalidViewChangeMsg = errors.New("ViewChangeMsg is invalid")
)

// roundCtx keeps the context data for the current round and block.
type roundCtx struct {
	block     *blockchain.Block
	blockHash *common.Hash32B
	prevotes  map[net.Addr]*common.Hash32B
	votes     map[net.Addr]*common.Hash32B
}

// DNet is the delegate networks interface.
type DNet interface {
	Tell(node net.Addr, msg proto.Message) error
	Self() net.Addr
	Broadcast(msg proto.Message) error
}

// RDPoS is the RDPoS consensus scheme
type RDPoS struct {
	bc        blockchain.Blockchain
	propCb    scheme.CreateBlockCB
	voteCb    scheme.TellPeerCB
	consCb    scheme.ConsensusDoneCB
	pubCb     scheme.BroadcastCB
	fsm       fsm.Machine
	roundCtx  *roundCtx
	self      net.Addr
	proposer  bool // am i the proposer for this round or not
	delegates []net.Addr
	wg        sync.WaitGroup
	quit      chan struct{}
	eventChan chan *fsm.Event
	cfg       config.RDPoS
	pr        *routine.RecurringTask
	done      chan bool
}

// NewRDPoS creates a RDPoS struct
func NewRDPoS(cfg config.RDPoS, prop scheme.CreateBlockCB, vote scheme.TellPeerCB, cons scheme.ConsensusDoneCB,
	pub scheme.BroadcastCB, bc blockchain.Blockchain, myaddr net.Addr, dlg delegate.Pool) *RDPoS {
	delegates, err := dlg.AllDelegates()
	if err != nil {
		logger.Error().Err(err)
		syscall.Exit(syscall.SYS_EXIT)
	}
	sc := &RDPoS{
		propCb:    prop,
		voteCb:    vote,
		consCb:    cons,
		pubCb:     pub,
		bc:        bc,
		self:      myaddr,
		delegates: delegates,
		quit:      make(chan struct{}),
		eventChan: make(chan *fsm.Event, 100),
		cfg:       cfg,
	}
	sc.pr = NewProposerRotation(sc)
	sc.fsm = fsmCreate(sc)
	return sc
}

// Start initialize the RDPoS and start to consume requests from request channel.
func (n *RDPoS) Start() error {
	logger.Info().Msg("Starting RDPoS")
	n.wg.Add(1)
	go n.consume()
	if n.cfg.ProposerRotation.Enabled {
		n.pr.Start()
	}
	return nil
}

// Stop stops the RDPoS and stop consuming requests from request channel.
func (n *RDPoS) Stop() error {
	logger.Info().Msg("RDPoS is shutting down")
	close(n.quit)
	n.wg.Wait()
	return nil
}

// SetDoneStream sets a boolean channel which indicates to the simulator that the consensus is done
func (n *RDPoS) SetDoneStream(done chan bool) {
	n.done = done
}

// Handle handles incoming messages and publish to the channel.
func (n *RDPoS) Handle(m proto.Message) error {
	logger.Info().Msg("RDPoS scheme handles incoming requests")

	event, err := eventFromProto(m)
	if err != nil {
		return err
	}

	n.eventChan <- event
	return nil
}

func (n *RDPoS) consume() {
loop:
	for {
		select {
		case r := <-n.eventChan:
			err := n.fsm.HandleTransition(r)
			if err == nil {
				break
			}

			fErr := errors.Cause(err)
			switch fErr {
			case fsm.ErrStateHandlerNotMatched:
				if r.ExpireAt == nil {
					expireAt := time.Now().Add(n.cfg.UnmatchedEventTTL)
					r.ExpireAt = &expireAt
					n.eventChan <- r
				} else if time.Now().Before(*r.ExpireAt) {
					n.eventChan <- r
				}
			case fsm.ErrNoTransitionApplied:
			default:
				logger.Error().
					Str("RDPoS", n.self.String()).
					Err(err).
					Msg("Failed to fsm.HandleTransition")
			}
		case <-n.quit:
			break loop
		default:
			// if there are no events, try to write to done channel
			if n.done != nil {
				select {
				case n.done <- true: // try to write to done if possible
				}
			}
		}
	}

	n.wg.Done()
	logger.Info().Msg("consume done")
}

func (n *RDPoS) tellDelegates(msg *pb.ViewChangeMsg) {
	msg.SenderAddr = n.self.String()
	n.voteCb(msg)
}

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

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/common/routine"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/delegate"
	pb "github.com/iotexproject/iotex-core/proto"
)

var (
	// ErrInvalidViewChangeMsg is the error that ViewChangeMsg is invalid
	ErrInvalidViewChangeMsg = errors.New("ViewChangeMsg is invalid")
)

// roundCtx keeps the context data for the current round and block.
type roundCtx struct {
	block     *blockchain.Block
	blockHash *cp.Hash32B
	prevotes  map[net.Addr]*cp.Hash32B
	votes     map[net.Addr]*cp.Hash32B
}

// DNet is the delegate networks interface.
type DNet interface {
	Tell(node net.Addr, msg proto.Message) error
	Self() net.Addr
	Broadcast(msg proto.Message) error
}

// RDPoS is the RDPoS consensus scheme
type RDPoS struct {
	bc        blockchain.IBlockchain
	propCb    scheme.CreateBlockCB
	voteCb    scheme.TellPeerCB
	consCb    scheme.ConsensusDoneCB
	pubCb     scheme.BroadcastCB
	fsm       fsm.Machine
	roundCtx  *roundCtx
	self      net.Addr
	proposer  bool // am i the proposer for this round or not
	delegates []net.Addr
	majNum    int
	wg        sync.WaitGroup
	quit      chan struct{}
	eventChan chan *fsm.Event
	cfg       config.RDPoS
	pr        *routine.RecurringTask
}

// NewRDPoS creates a RDPoS struct
func NewRDPoS(cfg config.RDPoS, prop scheme.CreateBlockCB, vote scheme.TellPeerCB, cons scheme.ConsensusDoneCB, pub scheme.BroadcastCB, bc blockchain.IBlockchain, myaddr net.Addr, dlg delegate.Pool) *RDPoS {
	delegates, err := dlg.AllDelegates()
	if err != nil {
		glog.Error(err.Error())
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
		majNum:    len(delegates)*2/3 + 1,
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
	glog.Info("Starting RDPoS")
	n.wg.Add(1)
	go n.consume()
	if n.cfg.ProposerRotation.Enabled {
		n.pr.Start()
	}
	return nil
}

// Stop stops the RDPoS and stop consuming requests from request channel.
func (n *RDPoS) Stop() error {
	glog.Infof("RDPoS is shutting down")
	close(n.quit)
	n.wg.Wait()
	return nil
}

// Handle handles incoming messages and publish to the channel.
func (n *RDPoS) Handle(m proto.Message) error {
	glog.Info("RDPoS scheme handles incoming requests")

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
				glog.Errorf("%s failed to fsm.HandleTransition: %s", n.self, err)
			}
		case <-n.quit:
			break loop
		}
	}

	n.wg.Done()
	glog.Info("consume done")
}

func (n *RDPoS) tellDelegates(msg *pb.ViewChangeMsg) {
	msg.SenderAddr = n.self.String()
	n.voteCb(msg)
}

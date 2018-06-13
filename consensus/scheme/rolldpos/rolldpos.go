// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"net"
	"sync"
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
	"github.com/iotexproject/iotex-core/statefactory"
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
	isPr      bool
}

// epochCtx keeps the context data for the current epochStart
type epochCtx struct {
	// height means offset for current epochStart (i.e., the height of the first block generated in this epochStart)
	height uint64
	// numSubEpochs defines number of sub-epochs/rotations will happen in an epochStart
	numSubEpochs uint
	dkg          common.DKGHash
	delegates    []net.Addr
}

// DNet is the delegate networks interface.
type DNet interface {
	Tell(node net.Addr, msg proto.Message) error
	Self() net.Addr
	Broadcast(msg proto.Message) error
}

// rollDPoSCB contains all the callback functions used in RollDPoS
type rollDPoSCB struct {
	propCb       scheme.CreateBlockCB
	voteCb       scheme.TellPeerCB
	consCb       scheme.ConsensusDoneCB
	pubCb        scheme.BroadcastCB
	prCb         scheme.GetProposerCB
	dkgCb        scheme.GenerateDKGCB
	epochStartCb scheme.StartNextEpochCB
}

// RollDPoS is the RollDPoS consensus scheme
type RollDPoS struct {
	rollDPoSCB
	bc             blockchain.Blockchain
	fsm            *fsm.Machine
	epochCtx       *epochCtx
	roundCtx       *roundCtx
	self           net.Addr
	pool           delegate.Pool
	sf             statefactory.StateFactory
	wg             sync.WaitGroup
	quit           chan struct{}
	eventChan      chan *fsm.Event
	cfg            config.RollDPoS
	pr             *routine.RecurringTask
	prnd           *proposerRotation
	epochStartTask *routine.DelayTask
	done           chan bool
}

// NewRollDPoS creates a RollDPoS struct
func NewRollDPoS(
	cfg config.RollDPoS,
	prop scheme.CreateBlockCB,
	vote scheme.TellPeerCB,
	cons scheme.ConsensusDoneCB,
	pub scheme.BroadcastCB,
	pr scheme.GetProposerCB,
	epochStart scheme.StartNextEpochCB,
	dkg scheme.GenerateDKGCB,
	bc blockchain.Blockchain,
	myaddr net.Addr,
	dlg delegate.Pool,
	sf statefactory.StateFactory,
) *RollDPoS {
	cb := rollDPoSCB{
		propCb:       prop,
		voteCb:       vote,
		consCb:       cons,
		pubCb:        pub,
		prCb:         pr,
		dkgCb:        dkg,
		epochStartCb: epochStart,
	}
	sc := &RollDPoS{
		rollDPoSCB: cb,
		bc:         bc,
		self:       myaddr,
		pool:       dlg,
		sf:         sf,
		quit:       make(chan struct{}),
		eventChan:  make(chan *fsm.Event, cfg.EventChanSize),
		cfg:        cfg,
	}
	if cfg.ProposerInterval == 0 {
		sc.prnd = newProposerRotationNoDelay(sc)
	} else {
		sc.pr = newProposerRotation(sc)
	}
	sc.epochStartTask = routine.NewDelayTask(
		func() {
			sc.enqueueEvent(&fsm.Event{
				State: stateDKGGenerate,
			})
		},
		cfg.Delay,
	)
	sc.fsm = fsmCreate(sc)
	return sc
}

// Start initialize the RollDPoS and roundStart to consume requests from request channel.
func (n *RollDPoS) Start() error {
	logger.Info().Str("name", n.self.String()).Msg("Starting RollDPoS")

	n.wg.Add(1)
	go n.consume()
	if n.cfg.ProposerInterval > 0 {
		n.pr.Init()
		n.pr.Start()
	}
	n.epochStartTask.Init()
	n.epochStartTask.Start()
	return nil
}

// Stop stops the RollDPoS and stop consuming requests from request channel.
func (n *RollDPoS) Stop() error {
	logger.Info().Str("name", n.self.String()).Msg("RollDPoS is shutting down")
	close(n.quit)
	n.wg.Wait()
	return nil
}

// SetDoneStream sets a boolean channel which indicates to the simulator that the consensus is done
func (n *RollDPoS) SetDoneStream(done chan bool) {
	n.done = done
}

// Handle handles incoming messages and publish to the channel.
func (n *RollDPoS) Handle(m proto.Message) error {
	event, err := eventFromProto(m)
	if err != nil {
		return err
	}
	n.enqueueEvent(event)
	return nil
}

// EventChan returns the event chan
func (n *RollDPoS) EventChan() *chan *fsm.Event {
	return &n.eventChan
}

// FSM returns the FSM instance
func (n *RollDPoS) FSM() *fsm.Machine {
	return n.fsm
}

func (n *RollDPoS) enqueueEvent(e *fsm.Event) {
	logger.Debug().Msg("RollDPoS scheme handles incoming requests")
	if len(n.eventChan) == cap(n.eventChan) {
		logger.Warn().Msg("dispatcher event chan is full")
	}
	n.eventChan <- e
}

func (n *RollDPoS) consume() {
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
				// if fsm state has not changed since message was last seen, write to done channel
				if n.fsm.CurrentState() == r.SeenState && n.done != nil {
					select {
					case n.done <- true: // try to write to done if possible
					default:
					}
				}
				r.SeenState = n.fsm.CurrentState()

				if r.ExpireAt == nil {
					expireAt := time.Now().Add(n.cfg.UnmatchedEventTTL)
					r.ExpireAt = &expireAt
					n.enqueueEvent(r)
				} else if time.Now().Before(*r.ExpireAt) {
					n.enqueueEvent(r)
				}
			case fsm.ErrNoTransitionApplied:
			default:
				logger.Error().
					Str("RollDPoS", n.self.String()).
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
				default:
				}
			}
		}
	}

	n.wg.Done()
	logger.Info().Msg("consume done")
}

func (n *RollDPoS) tellDelegates(msg *pb.ViewChangeMsg) {
	msg.SenderAddr = n.self.String()
	n.voteCb(msg)
}

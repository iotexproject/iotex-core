package rolldpos

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/iotexproject/go-fsm"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/v2/consensus/scheme"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type (
	Round interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		Height() uint64
		RoundNum() uint32
		Handle(msg *iotextypes.ConsensusMessage)
		Result() chan int
		Calibrate(h uint64)
		State() fsm.State
	}

	ChainedRollDPoS struct {
		rounds  []Round
		quit    chan struct{}
		builder *Builder
		wg      sync.WaitGroup
		active  bool
		mutex   sync.RWMutex
	}
)

func NewChainedRollDPoS(builder *Builder) *ChainedRollDPoS {
	return &ChainedRollDPoS{
		rounds:  make([]Round, 0),
		quit:    make(chan struct{}),
		builder: builder,
		active:  true,
	}
}

func (cr *ChainedRollDPoS) Start(ctx context.Context) error {
	// create a new RollDPoS instance every 1s
	cr.wg.Add(1)
	go func() {
		defer cr.wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-cr.quit:
				return
			case <-ticker.C:
				func() {
					cr.mutex.Lock()
					defer cr.mutex.Unlock()
					if len(cr.rounds) < 5 {
						// log.L().Debug("create round")
						// defer log.L().Debug("new round finished")
						rs, err := cr.newRound()
						if err != nil {
							log.L().Error("Failed to create new round", zap.Error(err))
							return
						}
						ctx, cancel := context.WithCancel(context.Background())
						cr.RunRound(ctx, rs)

						for _, r := range cr.rounds {
							if r.Height() == rs.Height() {
								log.L().Debug("round already exists", zap.Uint64("height", rs.Height()), zap.Uint32("round", rs.RoundNum()))
								cancel()
								return
							}
						}
						cr.rounds = append(cr.rounds, rs)
					}
				}()
			}
		}
	}()
	cr.wg.Add(1)
	go func() {
		defer cr.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-cr.quit:
				return
			case <-ticker.C:
				cr.mutex.RLock()
				fmt.Printf("\nrounds size: %d\n", len(cr.rounds))
				for _, r := range cr.rounds {
					fmt.Printf("round height: %d, round num: %d, state: %+v\n", r.Height(), r.RoundNum(), r.State())
				}
				fmt.Printf("\n")
				cr.mutex.RUnlock()
			}
		}
	}()
	return nil
}

func (cr *ChainedRollDPoS) Stop(ctx context.Context) error {
	close(cr.quit)
	cr.wg.Wait()
	return nil
}

func (cr *ChainedRollDPoS) HandleConsensusMsg(msg *iotextypes.ConsensusMessage) error {
	for _, r := range cr.rounds {
		if r.Height() == msg.Height {
			r.Handle(msg)
		}
	}
	return nil
}

func (cr *ChainedRollDPoS) Calibrate(h uint64) {
	for _, r := range cr.rounds {
		r.Calibrate(h)
	}
}

func (cr *ChainedRollDPoS) ValidateBlockFooter(*block.Block) error {
	return nil
}

func (cr *ChainedRollDPoS) Metrics() (scheme.ConsensusMetrics, error) {
	return scheme.ConsensusMetrics{}, nil
}

func (cr *ChainedRollDPoS) Activate(b bool) {
	cr.active = b
}

func (cr *ChainedRollDPoS) Active() bool {
	return cr.active
}

func (cr *ChainedRollDPoS) removeRound(round Round) {
	for i, r := range cr.rounds {
		if r.Height() == round.Height() {
			cr.rounds = append(cr.rounds[:i], cr.rounds[i+1:]...)
			log.L().Debug("round removed", zap.Uint64("height", round.Height()), zap.Uint32("round", round.RoundNum()))
			break
		}
	}
}

func (cr *ChainedRollDPoS) RunRound(ctx context.Context, r Round) {
	// rolldpos round loop, but ternimate when round status is finalized or invalid
	err := r.Start(ctx)
	if err != nil {
		log.L().Error("Failed to start round", zap.Error(err))
		return
	}
	go func() {
		defer func() {
			err := r.Stop(ctx)
			if err != nil {
				log.L().Error("Failed to stop round", zap.Error(err))
			}
			cr.mutex.Lock()
			cr.removeRound(r)
			cr.mutex.Unlock()
		}()

		var res int
		select {
		case res = <-r.Result():
		case <-ctx.Done():
			log.L().Info("round canceled", zap.Error(ctx.Err()), zap.Uint64("height", r.Height()), zap.Uint32("round", r.RoundNum()))
			return
		}
		switch res {
		case 1:
			log.L().Info("round finished", zap.Uint64("height", r.Height()), zap.Uint32("round", r.RoundNum()))
		case 0:
			log.L().Info("round invalid", zap.Uint64("height", r.Height()), zap.Uint32("round", r.RoundNum()))
		default:
			log.L().Info("round terminated", zap.Int("result", res), zap.Uint64("height", r.Height()), zap.Uint32("round", r.RoundNum()))
		}
	}()
}

func (cr *ChainedRollDPoS) newRound() (Round, error) {
	dpos, err := cr.builder.BuildV2()
	if err != nil {
		return nil, err
	}
	return &roundV2{
		dpos: dpos,
		res:  make(chan int, 1),
		quit: make(chan struct{}),
	}, nil
}

type roundV2 struct {
	dpos *RollDPoS
	res  chan int
	quit chan struct{}
	wg   sync.WaitGroup
}

func (r *roundV2) Start(ctx context.Context) error {
	err := r.dpos.Start(ctx)
	if err != nil {
		return err
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.quit:
				return
			case <-ticker.C:
				switch r.dpos.cfsm.CurrentState() {
				case consensusfsm.SFinalized:
					r.res <- 1
					return
				case consensusfsm.SInvalid:
					r.dpos.ctx.Chain().DropDraftBlock(r.Height())
					r.res <- 0
					return
				default:
				}
			}
		}
	}()
	return nil
}

func (r *roundV2) Stop(ctx context.Context) error {
	close(r.quit)
	r.wg.Wait()
	return r.dpos.Stop(ctx)
}

func (r *roundV2) Height() uint64 {
	return r.dpos.ctx.Height()
}

func (r *roundV2) RoundNum() uint32 {
	return r.dpos.ctx.Number()
}

func (r *roundV2) Handle(msg *iotextypes.ConsensusMessage) {
	err := r.dpos.HandleConsensusMsg(msg)
	if err != nil {
		log.L().Error("Failed to handle consensus message", zap.Error(err))
	}
}

func (r *roundV2) Result() chan int {
	return r.res
}

func (r *roundV2) Calibrate(h uint64) {
	r.dpos.Calibrate(h)
}

func (r *roundV2) State() fsm.State {
	return r.dpos.cfsm.CurrentState()
}

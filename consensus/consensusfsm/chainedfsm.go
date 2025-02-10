package consensusfsm

import (
	"time"

	"github.com/facebookgo/clock"
	fsm "github.com/iotexproject/go-fsm"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

const (
	// consensus states
	SFinalized fsm.State = "S_FINALIZED"
	SInvalid   fsm.State = "S_INVALID"
)

type ChainedConsensusFSM struct {
	*ConsensusFSM
}

func NewChainedConsensusFSM(ctx Context, clock clock.Clock) (*ChainedConsensusFSM, error) {
	mm := &ConsensusFSM{
		evtq:  make(chan *ConsensusEvent, ctx.EventChanSize()),
		close: make(chan interface{}),
		ctx:   ctx,
		clock: clock,
	}
	cm := &ChainedConsensusFSM{
		ConsensusFSM: mm,
	}
	b := fsm.NewBuilder().
		AddInitialState(sPrepare).
		AddStates(
			sAcceptBlockProposal,
			sAcceptProposalEndorsement,
			sAcceptLockEndorsement,
			sAcceptPreCommitEndorsement,
			SInvalid,
			SFinalized,
		).
		AddTransition(sPrepare, ePrepare, cm.prepare, []fsm.State{
			sPrepare,
			sAcceptBlockProposal,
			sAcceptPreCommitEndorsement,
			SInvalid,
		}).
		AddTransition(
			sAcceptBlockProposal,
			eReceiveBlock,
			cm.onReceiveBlock,
			[]fsm.State{
				sAcceptBlockProposal,       // proposed block invalid
				sAcceptProposalEndorsement, // receive valid block, jump to next step
			}).
		AddTransition(
			sAcceptBlockProposal,
			eFailedToReceiveBlock,
			cm.onFailedToReceiveBlock,
			[]fsm.State{
				sAcceptProposalEndorsement, // no valid block, jump to next step
			}).
		AddTransition(
			sAcceptProposalEndorsement,
			eReceiveProposalEndorsement,
			cm.onReceiveProposalEndorsementInAcceptProposalEndorsementState,
			[]fsm.State{
				sAcceptProposalEndorsement, // not enough endorsements
				sAcceptLockEndorsement,     // enough endorsements
			}).
		AddTransition(
			sAcceptProposalEndorsement,
			eReceivePreCommitEndorsement,
			cm.onReceiveProposalEndorsementInAcceptProposalEndorsementState,
			[]fsm.State{
				sAcceptProposalEndorsement, // not enough endorsements
				sAcceptLockEndorsement,     // enough endorsements
			}).
		AddTransition(
			sAcceptProposalEndorsement,
			eStopReceivingProposalEndorsement,
			cm.onStopReceivingProposalEndorsement,
			[]fsm.State{
				sAcceptLockEndorsement, // timeout, jump to next step
			}).
		AddTransition(
			sAcceptLockEndorsement,
			eReceiveProposalEndorsement,
			cm.onReceiveProposalEndorsementInAcceptLockEndorsementState,
			[]fsm.State{
				sAcceptLockEndorsement,
			},
		).
		AddTransition(
			sAcceptLockEndorsement,
			eReceiveLockEndorsement,
			cm.onReceiveLockEndorsement,
			[]fsm.State{
				sAcceptLockEndorsement,      // not enough endorsements
				sAcceptPreCommitEndorsement, // reach commit agreement, jump to next step
			}).
		AddTransition(
			sAcceptLockEndorsement,
			eReceivePreCommitEndorsement,
			cm.onReceiveLockEndorsement,
			[]fsm.State{
				sAcceptLockEndorsement,      // not enough endorsements
				sAcceptPreCommitEndorsement, // reach commit agreement, jump to next step
			}).
		AddTransition(
			sAcceptLockEndorsement,
			eStopReceivingLockEndorsement,
			cm.onStopReceivingLockEndorsement,
			[]fsm.State{
				SInvalid, // timeout, invalid
			}).
		AddTransition(
			sAcceptPreCommitEndorsement,
			eBroadcastPreCommitEndorsement,
			cm.onBroadcastPreCommitEndorsement,
			[]fsm.State{
				sAcceptPreCommitEndorsement,
			}).
		AddTransition(
			sAcceptPreCommitEndorsement,
			eStopReceivingPreCommitEndorsement,
			cm.onStopReceivingPreCommitEndorsement,
			[]fsm.State{
				SInvalid,
			}).
		AddTransition(
			sAcceptPreCommitEndorsement,
			eReceivePreCommitEndorsement,
			cm.onReceivePreCommitEndorsement,
			[]fsm.State{
				sAcceptPreCommitEndorsement,
				SFinalized, // reach consensus, start next epoch
			})
	// Add the backdoor transition so that we could unit test the transition from any given state
	for _, state := range consensusStates {
		b = b.AddTransition(state, BackdoorEvent, cm.handleBackdoorEvt, consensusStates)
		if state != sPrepare {
			b = b.AddTransition(state, eCalibrate, cm.calibrate, []fsm.State{sPrepare, state})
		}
	}
	m, err := b.Build()
	if err != nil {
		return nil, errors.Wrap(err, "error when building the FSM")
	}
	cm.fsm = m
	return cm, nil
}

func (m *ChainedConsensusFSM) prepare(evt fsm.Event) (fsm.State, error) {
	log.L().Debug("Handle event", zap.String("event", string(evt.Type())))
	defer log.L().Debug("Handled event", zap.String("event", string(evt.Type())))
	if err := m.ctx.Prepare(); err != nil {
		m.ctx.Logger().Error("Error during prepare", zap.Error(err), zap.Stack("stack"))
		return m.Invalid()
	}
	m.ctx.Logger().Debug("Start a new round", zap.Stack("stack"))
	proposal, err := m.ctx.Proposal()
	if err != nil {
		m.ctx.Logger().Error("failed to generate block proposal", zap.Error(err))
		return m.BackToPrepare(100 * time.Millisecond)
	}

	overtime := m.ctx.WaitUntilRoundStart()
	if proposal != nil {
		m.ctx.Broadcast(proposal)
	}
	if !m.ctx.IsDelegate() {
		return m.BackToPrepare(0)
	}
	if proposal != nil {
		m.ProduceReceiveBlockEvent(proposal)
	}

	var h uint64
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		m.ctx.Logger().Panic("failed to convert ConsensusEvent in prepare")
	}
	h = cEvt.Height()
	ttl := m.ctx.AcceptBlockTTL(h) - overtime
	// Setup timeouts
	if preCommitEndorsement := m.ctx.PreCommitEndorsement(); preCommitEndorsement != nil {
		cEvt := m.ctx.NewConsensusEvent(eBroadcastPreCommitEndorsement, preCommitEndorsement)
		m.produce(cEvt, ttl)
		ttl += m.ctx.AcceptProposalEndorsementTTL(cEvt.Height())
		m.produce(cEvt, ttl)
		ttl += m.ctx.AcceptLockEndorsementTTL(cEvt.Height())
		m.produce(cEvt, ttl)
		ttl += m.ctx.CommitTTL(cEvt.Height())
		m.produceConsensusEvent(eStopReceivingPreCommitEndorsement, ttl)
		return sAcceptPreCommitEndorsement, nil
	}
	m.produceConsensusEvent(eFailedToReceiveBlock, ttl)
	ttl += m.ctx.AcceptProposalEndorsementTTL(h)
	m.produceConsensusEvent(eStopReceivingProposalEndorsement, ttl)
	ttl += m.ctx.AcceptLockEndorsementTTL(h)
	m.produceConsensusEvent(eStopReceivingLockEndorsement, ttl)
	ttl += m.ctx.CommitTTL(h)
	m.produceConsensusEvent(eStopReceivingPreCommitEndorsement, ttl)
	return sAcceptBlockProposal, nil
}

func (m *ChainedConsensusFSM) onStopReceivingLockEndorsement(evt fsm.Event) (fsm.State, error) {
	log.L().Debug("Handle event", zap.String("event", string(evt.Type())))
	defer log.L().Debug("Handled event", zap.String("event", string(evt.Type())))
	m.ctx.Logger().Warn("Not enough lock endorsements")

	return m.Invalid()
}

func (m *ChainedConsensusFSM) onStopReceivingPreCommitEndorsement(evt fsm.Event) (fsm.State, error) {
	log.L().Debug("Handle event", zap.String("event", string(evt.Type())))
	defer log.L().Debug("Handled event", zap.String("event", string(evt.Type())))
	m.ctx.Logger().Warn("Not enough pre-commit endorsements")

	return m.Invalid()
}

func (m *ChainedConsensusFSM) onReceivePreCommitEndorsement(evt fsm.Event) (fsm.State, error) {
	log.L().Debug("Handle event", zap.String("event", string(evt.Type())))
	defer log.L().Debug("Handled event", zap.String("event", string(evt.Type())))
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		return sAcceptPreCommitEndorsement, errors.Wrap(ErrEvtCast, "failed to cast to consensus event")
	}
	committed, err := m.ctx.Commit(cEvt.Data())
	if err != nil || !committed {
		return sAcceptPreCommitEndorsement, err
	}
	return SFinalized, nil
}

func (m *ChainedConsensusFSM) Finalize() (fsm.State, error) {
	m.ctx.Logger().Warn("Finalized")
	return SFinalized, nil
}

func (m *ChainedConsensusFSM) Invalid() (fsm.State, error) {
	m.ctx.Logger().Warn("Invalid")
	return SInvalid, nil
}

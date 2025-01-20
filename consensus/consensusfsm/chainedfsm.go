package consensusfsm

import (
	"github.com/facebookgo/clock"
	fsm "github.com/iotexproject/go-fsm"
	"github.com/pkg/errors"
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

func (m *ChainedConsensusFSM) onStopReceivingLockEndorsement(evt fsm.Event) (fsm.State, error) {
	m.ctx.Logger().Warn("Not enough lock endorsements")

	return m.Invalid()
}

func (m *ChainedConsensusFSM) onStopReceivingPreCommitEndorsement(evt fsm.Event) (fsm.State, error) {
	m.ctx.Logger().Warn("Not enough pre-commit endorsements")

	return m.Invalid()
}

func (m *ChainedConsensusFSM) onReceivePreCommitEndorsement(evt fsm.Event) (fsm.State, error) {
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

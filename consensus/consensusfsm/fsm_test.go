// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensusfsm

import (
	"context"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	fsm "github.com/iotexproject/go-fsm"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBackdoorEvt(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockCtx := NewMockContext(ctrl)
	mockCtx.EXPECT().IsFutureEvent(gomock.Any()).Return(false).AnyTimes()
	mockCtx.EXPECT().IsStaleEvent(gomock.Any()).Return(false).AnyTimes()
	mockCtx.EXPECT().Logger().Return(log.L()).AnyTimes()
	mockCtx.EXPECT().Prepare().Return(false, nil, true, false, time.Duration(0), nil).AnyTimes()
	mockCtx.EXPECT().NewConsensusEvent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(eventType fsm.EventType, data interface{}) *ConsensusEvent {
			return &ConsensusEvent{
				eventType: eventType,
				data:      data,
			}
		}).AnyTimes()
	cfsm, err := NewConsensusFSM(Config{
		EventChanSize: 10,
	}, mockCtx, clock.NewMock())
	require.Nil(err)
	require.NotNil(cfsm)
	require.Equal(sPrepare, cfsm.CurrentState())

	require.NoError(cfsm.Start(context.Background()))
	defer require.NoError(cfsm.Stop(context.Background()))

	for _, state := range consensusStates {
		backdoorEvt := &ConsensusEvent{
			eventType: BackdoorEvent,
			data:      state,
		}
		cfsm.produce(backdoorEvt, 0)
		testutil.WaitUntil(10*time.Millisecond, 100*time.Millisecond, func() (bool, error) {
			return state == cfsm.CurrentState(), nil
		})
	}
}

func TestStateTransitionFunctions(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClock := clock.NewMock()
	mockCtx := NewMockContext(ctrl)
	mockCtx.EXPECT().Logger().Return(log.L()).AnyTimes()
	mockCtx.EXPECT().NewConsensusEvent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(eventType fsm.EventType, data interface{}) *ConsensusEvent {
			return &ConsensusEvent{
				eventType: eventType,
				data:      data,
			}
		}).AnyTimes()
	cfsm, err := NewConsensusFSM(Config{
		UnmatchedEventInterval:       100 * time.Millisecond,
		EventChanSize:                10,
		AcceptBlockTTL:               4 * time.Second,
		AcceptProposalEndorsementTTL: 2 * time.Second,
		AcceptLockEndorsementTTL:     2 * time.Second,
		CommitTTL:                    2 * time.Second,
	}, mockCtx, mockClock)
	require.Nil(err)
	require.NotNil(cfsm)
	require.Equal(sPrepare, cfsm.CurrentState())

	t.Run("prepare", func(t *testing.T) {
		t.Run("with-error", func(t *testing.T) {
			mockCtx.EXPECT().Prepare().Return(false, nil, false, false, 10*time.Second, errors.New("some error")).Times(1)
			state, err := cfsm.prepare(nil)
			require.NoError(err)
			require.Equal(sPrepare, state)
			time.Sleep(100 * time.Millisecond)
			mockClock.Add(10 * time.Second)
			evt := <-cfsm.evtq
			require.Equal(ePrepare, evt.Type())
		})
		t.Run("is-not-delegate", func(t *testing.T) {
			mockCtx.EXPECT().Prepare().Return(false, nil, false, false, 10*time.Second, nil).Times(1)
			state, err := cfsm.prepare(nil)
			require.NoError(err)
			require.Equal(sPrepare, state)
			time.Sleep(100 * time.Millisecond)
			mockClock.Add(10 * time.Second)
			evt := <-cfsm.evtq
			require.Equal(ePrepare, evt.Type())
		})
		t.Run("is-delegate", func(t *testing.T) {
			t.Run("is-locked", func(t *testing.T) {
				mockCtx.EXPECT().Prepare().Return(false, nil, true, true, time.Duration(0), nil).Times(1)
				state, err := cfsm.prepare(nil)
				require.NoError(err)
				require.Equal(sAcceptBlockProposal, state)
				evt := <-cfsm.evtq
				require.Equal(eReceiveBlock, evt.Type())
				require.Nil(evt.Data())
				time.Sleep(100 * time.Millisecond)
				// garbage collection
				mockClock.Add(cfsm.cfg.AcceptBlockTTL)
				evt = <-cfsm.evtq
				require.Equal(eFailedToReceiveBlock, evt.Type())
				mockClock.Add(cfsm.cfg.AcceptProposalEndorsementTTL)
				evt = <-cfsm.evtq
				require.Equal(eStopReceivingProposalEndorsement, evt.Type())
				mockClock.Add(cfsm.cfg.AcceptLockEndorsementTTL)
				evt = <-cfsm.evtq
				require.Equal(eStopReceivingLockEndorsement, evt.Type())
			})
			t.Run("is-not-locked", func(t *testing.T) {
				mockCtx.EXPECT().Prepare().Return(false, nil, true, false, time.Duration(0), nil).Times(1)
				state, err := cfsm.prepare(nil)
				require.NoError(err)
				require.Equal(sAcceptBlockProposal, state)
				time.Sleep(100 * time.Millisecond)
				// garbage collection
				mockClock.Add(4 * time.Second)
				evt := <-cfsm.evtq
				require.Equal(eFailedToReceiveBlock, evt.Type())
				mockClock.Add(2 * time.Second)
				evt = <-cfsm.evtq
				require.Equal(eStopReceivingProposalEndorsement, evt.Type())
				mockClock.Add(2 * time.Second)
				evt = <-cfsm.evtq
				require.Equal(eStopReceivingLockEndorsement, evt.Type())
			})
		})
		t.Run("is-proposer", func(t *testing.T) {
			t.Run("fail-to-mint", func(t *testing.T) {
				mockCtx.EXPECT().Prepare().Return(true, nil, true, false, time.Duration(0), errors.New("some error")).Times(1)
				state, err := cfsm.prepare(nil)
				require.NoError(err)
				require.Equal(sPrepare, state)
				evt := <-cfsm.evtq
				require.Equal(ePrepare, evt.Type())
			})
			t.Run("success-to-mint", func(t *testing.T) {
				mockEndorsement := NewMockEndorsement(ctrl)
				mockCtx.EXPECT().Prepare().Return(true, mockEndorsement, true, false, time.Duration(0), nil).Times(1)
				mockCtx.EXPECT().Broadcast(gomock.Any()).Return().Times(1)
				state, err := cfsm.prepare(nil)
				require.NoError(err)
				require.Equal(sAcceptBlockProposal, state)
				evt := <-cfsm.evtq
				require.Equal(eReceiveBlock, evt.Type())
				// garbage collection
				time.Sleep(100 * time.Millisecond)
				mockClock.Add(4 * time.Second)
				evt = <-cfsm.evtq
				require.Equal(eFailedToReceiveBlock, evt.Type())
				mockClock.Add(2 * time.Second)
				evt = <-cfsm.evtq
				require.Equal(eStopReceivingProposalEndorsement, evt.Type())
				mockClock.Add(2 * time.Second)
				evt = <-cfsm.evtq
				require.Equal(eStopReceivingLockEndorsement, evt.Type())
			})
		})
	})
	t.Run("onReceiveBlock", func(t *testing.T) {
		state, err := cfsm.handleBackdoorEvt(
			&ConsensusEvent{eventType: BackdoorEvent, data: sAcceptBlockProposal},
		)
		require.NoError(err)
		require.Equal(sAcceptBlockProposal, state)
		t.Run("invalid-fsm-event", func(t *testing.T) {
			state, err := cfsm.onReceiveBlock(nil)
			require.NoError(err)
			require.Equal(sAcceptBlockProposal, state)
		})
		t.Run("fail-to-new-proposal-vote", func(t *testing.T) {
			mockCtx.EXPECT().NewProposalEndorsement(gomock.Any()).Return(nil, errors.New("some error")).Times(1)
			state, err := cfsm.onReceiveBlock(&ConsensusEvent{data: NewMockEndorsement(ctrl)})
			require.NoError(err)
			require.Equal(sAcceptBlockProposal, state)
		})
		t.Run("success", func(t *testing.T) {
			mockCtx.EXPECT().NewProposalEndorsement(gomock.Any()).Return(NewMockEndorsement(ctrl), nil).Times(1)
			mockCtx.EXPECT().Broadcast(gomock.Any()).Return().Times(1)
			state, err := cfsm.onReceiveBlock(&ConsensusEvent{data: NewMockEndorsement(ctrl)})
			require.NoError(err)
			require.Equal(sAcceptProposalEndorsement, state)
			evt := <-cfsm.evtq
			require.Equal(eReceiveProposalEndorsement, evt.Type())
		})
	})
	t.Run("onFailedToReceiveBlock", func(t *testing.T) {
		mockCtx.EXPECT().NewProposalEndorsement(nil).Return(NewMockEndorsement(ctrl), nil).Times(1)
		mockCtx.EXPECT().Broadcast(gomock.Any()).Return().Times(1)
		state, err := cfsm.onFailedToReceiveBlock(nil)
		require.NoError(err)
		require.Equal(sAcceptProposalEndorsement, state)
		evt := <-cfsm.evtq
		require.Equal(eReceiveProposalEndorsement, evt.Type())
	})
	t.Run("onReceiveProposalEndorsement", func(t *testing.T) {
		t.Run("invalid-fsm-event", func(t *testing.T) {
			state, err := cfsm.onReceiveProposalEndorsement(nil)
			require.Error(err)
			require.Equal(sAcceptProposalEndorsement, state)
		})
		t.Run("fail-to-add-proposal-vote", func(t *testing.T) {
			mockCtx.EXPECT().NewLockEndorsement(gomock.Any()).Return(nil, errors.New("some error")).Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onReceiveProposalEndorsement(&ConsensusEvent{
				eventType: eReceiveProposalEndorsement,
				data:      mockEndorsement,
			})
			require.NoError(err)
			require.Equal(sAcceptProposalEndorsement, state)
		})
		t.Run("is-not-locked", func(t *testing.T) {
			mockCtx.EXPECT().NewLockEndorsement(gomock.Any()).Return(nil, nil).Times(2)
			state, err := cfsm.onReceiveProposalEndorsement(&ConsensusEvent{
				eventType: eReceiveProposalEndorsement,
				data:      NewMockEndorsement(ctrl),
			})
			require.NoError(err)
			require.Equal(sAcceptProposalEndorsement, state)
			state, err = cfsm.onReceiveProposalEndorsement(&ConsensusEvent{
				eventType: eReceivePreCommitEndorsement,
				data:      NewMockEndorsement(ctrl),
			})
			require.NoError(err)
			require.Equal(sAcceptProposalEndorsement, state)
		})
		t.Run("is-locked", func(t *testing.T) {
			mockCtx.EXPECT().NewLockEndorsement(gomock.Any()).Return(NewMockEndorsement(ctrl), nil).Times(2)
			mockCtx.EXPECT().Broadcast(gomock.Any()).Return().Times(2)
			state, err := cfsm.onReceiveProposalEndorsement(&ConsensusEvent{
				eventType: eReceiveProposalEndorsement,
				data:      NewMockEndorsement(ctrl),
			})
			require.NoError(err)
			require.Equal(sAcceptLockEndorsement, state)
			evt := <-cfsm.evtq
			require.Equal(eReceiveLockEndorsement, evt.Type())
			state, err = cfsm.onReceiveProposalEndorsement(&ConsensusEvent{
				eventType: eReceivePreCommitEndorsement,
				data:      NewMockEndorsement(ctrl),
			})
			require.NoError(err)
			require.Equal(sAcceptLockEndorsement, state)
			evt = <-cfsm.evtq
			require.Equal(eReceiveLockEndorsement, evt.Type())
		})
	})
	t.Run("onStopReceivingProposalEndorsement", func(t *testing.T) {
		state, err := cfsm.onStopReceivingProposalEndorsement(nil)
		require.NoError(err)
		require.Equal(sAcceptLockEndorsement, state)
	})
	t.Run("onReceiveLockEndorsement", func(t *testing.T) {
		t.Run("invalid-fsm-event", func(t *testing.T) {
			state, err := cfsm.onReceiveLockEndorsement(nil)
			require.Error(err)
			require.Equal(sAcceptLockEndorsement, state)
		})
		t.Run("fail-to-add-lock-vote", func(t *testing.T) {
			mockCtx.EXPECT().NewPreCommitEndorsement(gomock.Any()).Return(nil, errors.New("some error")).Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onReceiveLockEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      mockEndorsement,
			})
			require.Error(err)
			require.Equal(sAcceptLockEndorsement, state)
		})
		t.Run("not-ready-to-pre-commit", func(t *testing.T) {
			mockCtx.EXPECT().NewPreCommitEndorsement(gomock.Any()).Return(nil, nil).Times(2)
			state, err := cfsm.onReceiveLockEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      NewMockEndorsement(ctrl),
			})
			require.NoError(err)
			require.Equal(sAcceptLockEndorsement, state)
			state, err = cfsm.onReceiveLockEndorsement(&ConsensusEvent{
				eventType: eReceivePreCommitEndorsement,
				data:      NewMockEndorsement(ctrl),
			})
			require.NoError(err)
			require.Equal(sAcceptLockEndorsement, state)
		})
		t.Run("ready-to-pre-commit", func(t *testing.T) {
			mockCtx.EXPECT().NewPreCommitEndorsement(gomock.Any()).Return(NewMockEndorsement(ctrl), nil).Times(1)
			state, err := cfsm.onReceiveLockEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      NewMockEndorsement(ctrl),
			})
			require.NoError(err)
			require.Equal(sAcceptPreCommitEndorsement, state)
			evt := <-cfsm.evtq
			require.Equal(eReceivePreCommitEndorsement, evt.Type())
			evt = <-cfsm.evtq
			require.Equal(eBroadcastPreCommitEndorsement, evt.Type())
		})
	})
	t.Run("onStopReceivingLockEndorsement", func(t *testing.T) {
		state, err := cfsm.onStopReceivingLockEndorsement(nil)
		require.NoError(err)
		require.Equal(sPrepare, state)
		evt := <-cfsm.evtq
		require.Equal(ePrepare, evt.Type())
	})
	t.Run("onBroadcastPreCommitEndorsement", func(t *testing.T) {
		t.Run("invalid-fsm-event", func(t *testing.T) {
			state, err := cfsm.onBroadcastPreCommitEndorsement(nil)
			require.Error(err)
			require.Equal(sAcceptPreCommitEndorsement, state)
		})
		t.Run("success", func(t *testing.T) {
			mockCtx.EXPECT().Broadcast(gomock.Any()).Return().Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onBroadcastPreCommitEndorsement(&ConsensusEvent{
				eventType: eBroadcastPreCommitEndorsement,
				data:      mockEndorsement,
			})
			require.NoError(err)
			require.Equal(sAcceptPreCommitEndorsement, state)
			time.Sleep(100 * time.Millisecond)
			mockClock.Add(cfsm.cfg.CommitTTL)
			evt := <-cfsm.evtq
			require.Equal(eBroadcastPreCommitEndorsement, evt.Type())
		})
	})
	t.Run("onReceivePreCommitEndorsement", func(t *testing.T) {
		t.Run("invalid-fsm-event", func(t *testing.T) {
			state, err := cfsm.onReceivePreCommitEndorsement(nil)
			require.Error(err)
			require.Equal(sAcceptPreCommitEndorsement, state)
		})
		t.Run("fail-to-add-commit-vote", func(t *testing.T) {
			mockCtx.EXPECT().Commit(gomock.Any()).Return(false, errors.New("some error")).Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onReceivePreCommitEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      mockEndorsement,
			})
			require.Error(err)
			require.Equal(sAcceptPreCommitEndorsement, state)
		})
		t.Run("not-enough-commit-vote", func(t *testing.T) {
			mockCtx.EXPECT().Commit(gomock.Any()).Return(false, nil).Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onReceivePreCommitEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      mockEndorsement,
			})
			require.NoError(err)
			require.Equal(sAcceptPreCommitEndorsement, state)
		})
		t.Run("success", func(t *testing.T) {
			mockCtx.EXPECT().Commit(gomock.Any()).Return(true, nil).Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onReceivePreCommitEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      mockEndorsement,
			})
			require.NoError(err)
			require.Equal(sPrepare, state)
			evt := <-cfsm.evtq
			require.Equal(ePrepare, evt.Type())
		})
	})
	t.Run("calibrate", func(t *testing.T) {
		mockCtx.EXPECT().Height().Return(uint64(2)).Times(2)
		_, err := cfsm.calibrate(nil)
		require.Error(err)
		_, err = cfsm.calibrate(&ConsensusEvent{
			eventType: eCalibrate,
			data:      nil,
		})
		require.Error(err)
		_, err = cfsm.calibrate(&ConsensusEvent{
			eventType: eCalibrate,
			data:      uint64(1),
		})
		require.Error(err)
		state, err := cfsm.calibrate(&ConsensusEvent{
			eventType: eCalibrate,
			data:      uint64(2),
		})
		require.NoError(err)
		require.Equal(sPrepare, state)
		evt := <-cfsm.evtq
		require.Equal(ePrepare, evt.Type())
	})
	t.Run("handle", func(t *testing.T) {
		t.Run("is-stale-event", func(t *testing.T) {
			mockCtx.EXPECT().IsStaleEvent(gomock.Any()).Return(true).Times(1)
			require.NoError(cfsm.handle(&ConsensusEvent{
				eventType: ePrepare,
				height:    10,
				round:     2,
			}))
		})
		t.Run("is-future-event", func(t *testing.T) {
			mockCtx.EXPECT().IsStaleEvent(gomock.Any()).Return(false).Times(1)
			mockCtx.EXPECT().IsFutureEvent(gomock.Any()).Return(true).Times(1)
			cEvt := &ConsensusEvent{
				eventType: ePrepare,
				height:    10,
				round:     2,
			}
			require.NoError(cfsm.handle(cEvt))
			time.Sleep(10 * time.Millisecond)
			mockClock.Add(cfsm.cfg.UnmatchedEventInterval)
			evt := <-cfsm.evtq
			require.Equal(cEvt, evt)
		})
		mockCtx.EXPECT().IsStaleEvent(gomock.Any()).Return(false).AnyTimes()
		mockCtx.EXPECT().IsFutureEvent(gomock.Any()).Return(false).AnyTimes()
		t.Run("transition-not-found", func(t *testing.T) {
			cEvt := &ConsensusEvent{
				eventType: eFailedToReceiveBlock,
				height:    10,
				round:     2,
			}
			require.NoError(cfsm.handle(
				&ConsensusEvent{eventType: BackdoorEvent, data: sPrepare},
			))
			t.Run("is-stale-unmatched-event", func(t *testing.T) {
				mockCtx.EXPECT().IsStaleUnmatchedEvent(gomock.Any()).Return(true).Times(1)
				require.NoError(cfsm.handle(cEvt))
			})
			t.Run("not-stale-unmatched-event", func(t *testing.T) {
				mockCtx.EXPECT().IsStaleUnmatchedEvent(gomock.Any()).Return(false).Times(1)
				require.NoError(cfsm.handle(cEvt))
				time.Sleep(10 * time.Millisecond)
				mockClock.Add(cfsm.cfg.UnmatchedEventInterval)
				evt := <-cfsm.evtq
				require.Equal(evt, cEvt)
			})
		})
		t.Run("transition-success", func(t *testing.T) {
			mockCtx.EXPECT().Height().Return(uint64(0)).Times(1)
			require.NoError(cfsm.handle(
				&ConsensusEvent{eventType: BackdoorEvent, data: sAcceptBlockProposal},
			))
			require.Equal(sAcceptBlockProposal, cfsm.CurrentState())
			require.NoError(cfsm.handle(&ConsensusEvent{
				eventType: eCalibrate,
				data:      uint64(1),
			}))
			require.Equal(sPrepare, cfsm.CurrentState())
		})
	})
}

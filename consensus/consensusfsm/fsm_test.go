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
	mockCtx.EXPECT().LoggerWithStats().Return(log.L()).AnyTimes()
	mockCtx.EXPECT().IsDelegate().Return(true).AnyTimes()
	mockCtx.EXPECT().IsProposer().Return(false).AnyTimes()
	mockCtx.EXPECT().Prepare().Return(time.Duration(0), nil).AnyTimes()
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

func TestStateTransitions(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClock := clock.NewMock()
	mockCtx := NewMockContext(ctrl)
	mockCtx.EXPECT().Logger().Return(log.L()).AnyTimes()
	mockCtx.EXPECT().LoggerWithStats().Return(log.L()).AnyTimes()
	mockCtx.EXPECT().NewConsensusEvent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(eventType fsm.EventType, data interface{}) *ConsensusEvent {
			return &ConsensusEvent{
				eventType: eventType,
				data:      data,
			}
		}).AnyTimes()
	cfsm, err := NewConsensusFSM(Config{
		EventChanSize:                10,
		AcceptBlockTTL:               4 * time.Second,
		AcceptProposalEndorsementTTL: 2 * time.Second,
		AcceptLockEndorsementTTL:     2 * time.Second,
		ProposerInterval:             10 * time.Second,
	}, mockCtx, mockClock)
	require.Nil(err)
	require.NotNil(cfsm)
	require.Equal(sPrepare, cfsm.CurrentState())

	t.Run("prepare", func(t *testing.T) {
		t.Run("with-error", func(t *testing.T) {
			mockCtx.EXPECT().Prepare().Return(10*time.Second, errors.New("some error")).Times(1)
			state, err := cfsm.prepare(nil)
			require.Error(err)
			require.Equal(sPrepare, state)
			time.Sleep(100 * time.Millisecond)
			mockClock.Add(10 * time.Second)
			evt := <-cfsm.evtq
			require.Equal(ePrepare, evt.Type())
		})
		t.Run("is-not-delegate", func(t *testing.T) {
			mockCtx.EXPECT().IsDelegate().Return(false).Times(1)
			mockCtx.EXPECT().Prepare().Return(10*time.Second, nil).Times(1)
			state, err := cfsm.prepare(nil)
			require.NoError(err)
			require.Equal(sPrepare, state)
			time.Sleep(100 * time.Millisecond)
			mockClock.Add(10 * time.Second)
			evt := <-cfsm.evtq
			require.Equal(ePrepare, evt.Type())
		})
		t.Run("is-delegate", func(t *testing.T) {
			mockCtx.EXPECT().IsDelegate().Return(true).Times(1)
			mockCtx.EXPECT().IsProposer().Return(false).Times(1)
			mockCtx.EXPECT().Prepare().Return(time.Duration(0), nil).Times(1)
			state, err := cfsm.prepare(nil)
			require.NoError(err)
			require.Equal(sAcceptBlockProposal, state)
			time.Sleep(100 * time.Millisecond)
			mockClock.Add(4 * time.Second)
			evt := <-cfsm.evtq
			require.Equal(eFailedToReceiveBlock, evt.Type())
			mockClock.Add(2 * time.Second)
			evt = <-cfsm.evtq
			require.Equal(eStopReceivingProposalEndorsement, evt.Type())
			mockClock.Add(2 * time.Second)
			evt = <-cfsm.evtq
			require.Equal(eStopReceivingLockEndorsement, evt.Type())
			// garbage collection
			mockClock.Add(2 * time.Second)
			evt = <-cfsm.evtq
			require.Equal(eFailedToReachConsensusInTime, evt.Type())
		})
		t.Run("is-proposer", func(t *testing.T) {
			t.Run("fail-to-mint", func(t *testing.T) {
				mockCtx.EXPECT().IsDelegate().Return(true).Times(1)
				mockCtx.EXPECT().IsProposer().Return(true).Times(1)
				mockCtx.EXPECT().Prepare().Return(time.Duration(0), nil).Times(1)
				mockCtx.EXPECT().MintBlock().Return(nil, errors.New("some error")).Times(1)
				state, err := cfsm.prepare(nil)
				require.Error(err)
				require.Equal(sPrepare, state)
				time.Sleep(100 * time.Millisecond)
				evt := <-cfsm.evtq
				require.Equal(ePrepare, evt.Type())
				// garbage collection
				mockClock.Add(4 * time.Second)
				evt = <-cfsm.evtq
				require.Equal(eFailedToReceiveBlock, evt.Type())
				mockClock.Add(2 * time.Second)
				evt = <-cfsm.evtq
				require.Equal(eStopReceivingProposalEndorsement, evt.Type())
				mockClock.Add(2 * time.Second)
				evt = <-cfsm.evtq
				require.Equal(eStopReceivingLockEndorsement, evt.Type())
				mockClock.Add(2 * time.Second)
				evt = <-cfsm.evtq
				require.Equal(eFailedToReachConsensusInTime, evt.Type())
			})
			t.Run("success-to-mint", func(t *testing.T) {
				mockCtx.EXPECT().IsDelegate().Return(true).Times(1)
				mockCtx.EXPECT().IsProposer().Return(true).Times(1)
				mockCtx.EXPECT().Prepare().Return(time.Duration(0), nil).Times(1)
				mockEndorsement := NewMockEndorsement(ctrl)
				mockEndorsement.EXPECT().Hash().Return([]byte{}).Times(1)
				mockCtx.EXPECT().MintBlock().Return(mockEndorsement, nil).Times(1)
				mockCtx.EXPECT().BroadcastBlockProposal(gomock.Any()).Return().Times(1)
				state, err := cfsm.prepare(nil)
				require.NoError(err)
				require.Equal(sAcceptBlockProposal, state)
				evt := <-cfsm.evtq
				require.Equal(eReceiveBlock, evt.Type())
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
				// garbage collection
				mockClock.Add(2 * time.Second)
				evt = <-cfsm.evtq
				require.Equal(eFailedToReachConsensusInTime, evt.Type())
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
			require.Error(err)
			require.Equal(sAcceptBlockProposal, state)
		})
		t.Run("invalid-data-type", func(t *testing.T) {
			state, err := cfsm.onReceiveBlock(&ConsensusEvent{})
			require.Error(err)
			require.Equal(sAcceptBlockProposal, state)
		})
		t.Run("fail-to-new-proposal-endorsement", func(t *testing.T) {
			mockCtx.EXPECT().NewProposalEndorsement(gomock.Any()).Return(nil, errors.New("some error")).Times(1)
			state, err := cfsm.onReceiveBlock(&ConsensusEvent{data: NewMockEndorsement(ctrl)})
			require.NoError(err)
			require.Equal(sAcceptBlockProposal, state)
		})
		t.Run("success", func(t *testing.T) {
			mockCtx.EXPECT().NewProposalEndorsement(gomock.Any()).Return(NewMockEndorsement(ctrl), nil).Times(1)
			mockCtx.EXPECT().BroadcastEndorsement(gomock.Any()).Return().Times(1)
			state, err := cfsm.onReceiveBlock(&ConsensusEvent{data: NewMockEndorsement(ctrl)})
			require.NoError(err)
			require.Equal(sAcceptProposalEndorsement, state)
			evt := <-cfsm.evtq
			require.Equal(eReceiveProposalEndorsement, evt.Type())
		})
	})
	t.Run("onFailedToReceiveBlock", func(t *testing.T) {
		mockCtx.EXPECT().NewProposalEndorsement(nil).Return(NewMockEndorsement(ctrl), nil).Times(1)
		mockCtx.EXPECT().BroadcastEndorsement(gomock.Any()).Return().Times(1)
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
		t.Run("invalid-data", func(t *testing.T) {
			state, err := cfsm.onReceiveProposalEndorsement(&ConsensusEvent{
				eventType: eReceiveProposalEndorsement,
				data:      nil,
			})
			require.Error(err)
			require.Equal(sAcceptProposalEndorsement, state)
		})
		t.Run("fail-to-add-proposal-endorsement", func(t *testing.T) {
			mockCtx.EXPECT().AddProposalEndorsement(gomock.Any()).Return(errors.New("some error")).Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onReceiveProposalEndorsement(&ConsensusEvent{
				eventType: eReceiveProposalEndorsement,
				data:      mockEndorsement,
			})
			require.NoError(err)
			require.Equal(sAcceptProposalEndorsement, state)
		})
		t.Run("is-not-locked", func(t *testing.T) {
			mockCtx.EXPECT().AddProposalEndorsement(gomock.Any()).Return(nil).Times(1)
			mockCtx.EXPECT().IsLocked().Return(false).Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onReceiveProposalEndorsement(&ConsensusEvent{
				eventType: eReceiveProposalEndorsement,
				data:      mockEndorsement,
			})
			require.NoError(err)
			require.Equal(sAcceptProposalEndorsement, state)
		})
		t.Run("is-locked", func(t *testing.T) {
			t.Run("fail-to-new-lock-endorsement", func(t *testing.T) {
				mockCtx.EXPECT().IsLocked().Return(true).Times(1)
				mockCtx.EXPECT().AddProposalEndorsement(gomock.Any()).Return(nil).Times(1)
				mockCtx.EXPECT().NewLockEndorsement().Return(nil, errors.New("some error")).Times(1)
				state, err := cfsm.onReceiveProposalEndorsement(&ConsensusEvent{
					eventType: eReceiveProposalEndorsement,
					data:      NewMockEndorsement(ctrl),
				})
				require.Error(err)
				require.Equal(sPrepare, state)
				evt := <-cfsm.evtq
				require.Equal(ePrepare, evt.Type())
			})
			t.Run("success", func(t *testing.T) {
				mockCtx.EXPECT().IsLocked().Return(true).Times(1)
				mockCtx.EXPECT().AddProposalEndorsement(gomock.Any()).Return(nil).Times(1)
				mockCtx.EXPECT().NewLockEndorsement().Return(NewMockEndorsement(ctrl), nil).Times(1)
				mockCtx.EXPECT().BroadcastEndorsement(gomock.Any()).Return().Times(1)
				state, err := cfsm.onReceiveProposalEndorsement(&ConsensusEvent{
					eventType: eReceiveProposalEndorsement,
					data:      NewMockEndorsement(ctrl),
				})
				require.NoError(err)
				require.Equal(sAcceptLockEndorsement, state)
				evt := <-cfsm.evtq
				require.Equal(eReceiveLockEndorsement, evt.Type())
			})
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
		t.Run("invalid-data", func(t *testing.T) {
			state, err := cfsm.onReceiveLockEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      nil,
			})
			require.Error(err)
			require.Equal(sAcceptLockEndorsement, state)
		})
		t.Run("fail-to-add-lock-endorsement", func(t *testing.T) {
			mockCtx.EXPECT().AddLockEndorsement(gomock.Any()).Return(errors.New("some error")).Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onReceiveLockEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      mockEndorsement,
			})
			require.Error(err)
			require.Equal(sAcceptLockEndorsement, state)
		})
		t.Run("not-ready-to-pre-commit", func(t *testing.T) {
			mockCtx.EXPECT().ReadyToPreCommit().Return(false).Times(1)
			mockCtx.EXPECT().AddLockEndorsement(gomock.Any()).Return(nil).Times(1)
			state, err := cfsm.onReceiveLockEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      NewMockEndorsement(ctrl),
			})
			require.NoError(err)
			require.Equal(sAcceptLockEndorsement, state)
		})
		t.Run("ready-to-pre-commit", func(t *testing.T) {
			t.Run("fail-to-new-pre-commit-endorsement", func(t *testing.T) {
				mockCtx.EXPECT().ReadyToPreCommit().Return(true).Times(1)
				mockCtx.EXPECT().AddLockEndorsement(gomock.Any()).Return(nil).Times(1)
				mockCtx.EXPECT().NewPreCommitEndorsement().Return(nil, errors.New("some error")).Times(1)
				state, err := cfsm.onReceiveLockEndorsement(&ConsensusEvent{
					eventType: eReceiveLockEndorsement,
					data:      NewMockEndorsement(ctrl),
				})
				require.Error(err)
				require.Equal(sPrepare, state)
				evt := <-cfsm.evtq
				require.Equal(ePrepare, evt.Type())
			})
			t.Run("success", func(t *testing.T) {
				mockCtx.EXPECT().ReadyToPreCommit().Return(true).Times(1)
				mockCtx.EXPECT().AddLockEndorsement(gomock.Any()).Return(nil).Times(1)
				mockCtx.EXPECT().NewPreCommitEndorsement().Return(NewMockEndorsement(ctrl), nil).Times(1)
				mockCtx.EXPECT().BroadcastEndorsement(gomock.Any()).Return().Times(1)
				state, err := cfsm.onReceiveLockEndorsement(&ConsensusEvent{
					eventType: eReceiveLockEndorsement,
					data:      NewMockEndorsement(ctrl),
				})
				require.NoError(err)
				require.Equal(sAcceptPreCommitEndorsement, state)
				evt := <-cfsm.evtq
				require.Equal(eReceivePreCommitEndorsement, evt.Type())
			})
		})
	})
	t.Run("onStopReceivingLockEndorsement", func(t *testing.T) {
		state, err := cfsm.onStopReceivingLockEndorsement(nil)
		require.NoError(err)
		require.Equal(sPrepare, state)
		evt := <-cfsm.evtq
		require.Equal(ePrepare, evt.Type())
	})
	t.Run("onReceivePreCommitEndorsement", func(t *testing.T) {
		t.Run("invalid-fsm-event", func(t *testing.T) {
			state, err := cfsm.onReceivePreCommitEndorsement(nil)
			require.Error(err)
			require.Equal(sAcceptPreCommitEndorsement, state)
		})
		t.Run("invalid-data", func(t *testing.T) {
			state, err := cfsm.onReceivePreCommitEndorsement(&ConsensusEvent{
				eventType: eReceivePreCommitEndorsement,
				data:      nil,
			})
			require.Error(err)
			require.Equal(sAcceptPreCommitEndorsement, state)
		})
		t.Run("fail-to-add-commit-endorsement", func(t *testing.T) {
			mockCtx.EXPECT().AddPreCommitEndorsement(gomock.Any()).Return(errors.New("some error")).Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onReceivePreCommitEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      mockEndorsement,
			})
			require.Error(err)
			require.Equal(sAcceptPreCommitEndorsement, state)
		})
		t.Run("not-enough-commit-endorsement", func(t *testing.T) {
			mockCtx.EXPECT().AddPreCommitEndorsement(gomock.Any()).Return(nil).Times(1)
			mockCtx.EXPECT().ReadyToCommit().Return(false).Times(1)
			mockEndorsement := NewMockEndorsement(ctrl)
			state, err := cfsm.onReceivePreCommitEndorsement(&ConsensusEvent{
				eventType: eReceiveLockEndorsement,
				data:      mockEndorsement,
			})
			require.NoError(err)
			require.Equal(sAcceptPreCommitEndorsement, state)
		})
		t.Run("success", func(t *testing.T) {
			mockCtx.EXPECT().AddPreCommitEndorsement(gomock.Any()).Return(nil).Times(1)
			mockCtx.EXPECT().ReadyToCommit().Return(true).Times(1)
			mockCtx.EXPECT().OnConsensusReached().Return().Times(1)
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
	t.Run("onFailedToReachConsensusInTime", func(t *testing.T) {
		state, err := cfsm.onFailedToReachConsensusInTime(nil)
		require.NoError(err)
		require.Equal(sPrepare, state)
		evt := <-cfsm.evtq
		require.Equal(ePrepare, evt.Type())
	})
}

// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package consensusfsm

import (
	"time"

	fsm "github.com/iotexproject/go-fsm"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
)

// Context defines the context of the fsm
type Context interface {
	lifecycle.StartStopper
	Activate(bool)
	Active() bool
	IsStaleEvent(*ConsensusEvent) bool
	IsFutureEvent(*ConsensusEvent) bool
	IsStaleUnmatchedEvent(*ConsensusEvent) bool

	Logger() *zap.Logger
	Height() uint64
	Number() uint32

	NewConsensusEvent(fsm.EventType, interface{}) *ConsensusEvent
	NewBackdoorEvt(fsm.State) *ConsensusEvent

	Broadcast(interface{})

	Prepare() error
	IsDelegate() bool
	Proposal() (interface{}, error)
	WaitUntilRoundStart() time.Duration
	PreCommitEndorsement() interface{}
	NewProposalEndorsement(interface{}) (interface{}, error)
	NewLockEndorsement(interface{}) (interface{}, error)
	NewPreCommitEndorsement(interface{}) (interface{}, error)
	Commit(interface{}) (bool, error)
	ConsensusConfig
}

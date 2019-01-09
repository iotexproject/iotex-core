// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensusfsm

import (
	"time"

	"github.com/rs/zerolog"
	fsm "github.com/zjshen14/go-fsm"
)

// Context defines the context of the fsm
type Context interface {
	IsStaleEvent(*ConsensusEvent) bool
	IsFutureEvent(*ConsensusEvent) bool
	IsStaleUnmatchedEvent(*ConsensusEvent) bool

	Logger() *zerolog.Logger
	LoggerWithStats() *zerolog.Logger

	NewConsensusEvent(fsm.EventType, interface{}) *ConsensusEvent
	NewBackdoorEvt(fsm.State) *ConsensusEvent

	IsDelegate() bool
	IsProposer() bool

	BroadcastBlockProposal(Endorsement)
	BroadcastEndorsement(Endorsement)

	Prepare() (time.Duration, error)
	MintBlock() (Endorsement, error)
	NewProposalEndorsement(Endorsement) (Endorsement, error)
	NewLockEndorsement() (Endorsement, error)
	NewPreCommitEndorsement() (Endorsement, error)
	OnConsensusReached()

	AddProposalEndorsement(Endorsement) error
	AddLockEndorsement(Endorsement) error
	AddPreCommitEndorsement(Endorsement) error

	HasReceivedBlock() bool
	IsLocked() bool
	ReadyToPreCommit() bool
	ReadyToCommit() bool
}

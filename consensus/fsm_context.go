package consensus

import (
	"time"

	"github.com/iotexproject/go-fsm"
	"go.uber.org/zap"
)

// FSMContext defines the context of the fsm
type FSMContext interface {
	Activate(bool)
	Active() bool
	IsStaleEvent(*Event) bool
	IsFutureEvent(*Event) bool
	IsStaleUnmatchedEvent(*Event) bool

	Logger() *zap.Logger
	Height() uint64

	NewConsensusEvent(fsm.EventType, interface{}) *Event
	NewBackdoorEvt(fsm.State) *Event

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
	FSMConfig
}

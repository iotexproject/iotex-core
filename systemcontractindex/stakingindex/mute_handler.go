package stakingindex

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/pkg/util/abiutil"
)

// eventMuteHandler is a mute handler for staking events
// It mutes all events that are new stakes or will increase the existing bucket's amount or duration
type eventMuteHandler struct {
	*eventHandler
}

func newEventMuteHandler(eventHandler *eventHandler) *eventMuteHandler {
	return &eventMuteHandler{
		eventHandler: eventHandler,
	}
}

func (eh *eventMuteHandler) HandleStakedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}
	delegateParam, err := event.FieldByIDAddress(1)
	if err != nil {
		return err
	}
	amountParam, err := event.FieldByIDUint256(2)
	if err != nil {
		return err
	}
	durationParam, err := event.FieldByIDUint256(3)
	if err != nil {
		return err
	}
	owner, ok := eh.tokenOwner[tokenIDParam.Uint64()]
	if !ok {
		return errors.Errorf("no owner for token id %d", tokenIDParam.Uint64())
	}
	bucket := &Bucket{
		Candidate:                 delegateParam,
		Owner:                     owner,
		StakedAmount:              amountParam,
		StakedDurationBlockNumber: durationParam.Uint64(),
		CreatedAt:                 eh.height,
		UnlockedAt:                maxBlockNumber,
		UnstakedAt:                maxBlockNumber,
		Muted:                     true,
	}
	eh.putBucket(tokenIDParam.Uint64(), bucket)
	return nil
}

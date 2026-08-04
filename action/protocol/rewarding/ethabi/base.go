package ethabi

import (
	"encoding/hex"
	"errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

var (
	errInvalidCallData  = errors.New("invalid call binary data")
	errInvalidCallSig   = errors.New("invalid call sig")
	errConvertBigNumber = errors.New("convert big number error")
	errDecodeFailure    = errors.New("decode data error")
)

// BuildReadStateRequest decode eth_call data to StateContext
func BuildReadStateRequest(data []byte) (protocol.StateContext, error) {
	if len(data) < 4 {
		return nil, errInvalidCallData
	}

	switch methodSig := hex.EncodeToString(data[:4]); methodSig {
	case hex.EncodeToString(_totalBalanceMethod.ID):
		return newTotalBalanceStateContext()
	case hex.EncodeToString(_availableBalanceMethod.ID):
		return newAvailableBalanceStateContext()
	case hex.EncodeToString(_unclaimedBalanceMethod.ID):
		return newUnclaimedBalanceStateContext(data[4:])
	case hex.EncodeToString(_pendingBlockRewardPoolMethod.ID):
		return newPendingBlockRewardPoolStateContext(data[4:])
	case hex.EncodeToString(_pendingBlockRewardPoolIndexMethod.ID):
		return newPendingBlockRewardPoolIndexStateContext()
	case hex.EncodeToString(_eraDrainCursorMethod.ID):
		return newEraDrainCursorStateContext()
	case hex.EncodeToString(_voterRewardDelegateSnapshotMethod.ID):
		return newVoterRewardDelegateSnapshotStateContext(data[4:])
	case hex.EncodeToString(_voterRewardAddressMethod.ID):
		return newVoterRewardAddressStateContext(data[4:])
	case hex.EncodeToString(_voterRewardDestinationMethod.ID):
		return newVoterRewardDestinationStateContext(data[4:])
	case hex.EncodeToString(_voterRewardStatusMethod.ID):
		return newVoterRewardStatusStateContext(data[4:])
	default:
		return nil, errInvalidCallSig
	}
}

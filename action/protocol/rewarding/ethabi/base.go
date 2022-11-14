package ethabi

import (
	"encoding/hex"
	"errors"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

var (
	errInvalidCallData  = errors.New("invalid call binary data")
	errInvalidCallSig   = errors.New("invalid call sig")
	errConvertBigNumber = errors.New("convert big number error")
	errDecodeFailure    = errors.New("decode data error")
)

type (
	// Parameters state request parameters
	Parameters struct {
		MethodName []byte
		Arguments  [][]byte
	}

	// StateContext context for ReadState
	StateContext interface {
		Parameters() *Parameters
		EncodeToEth(*iotexapi.ReadStateResponse) (string, error)
	}

	baseStateContext struct {
		parameters *Parameters
	}
)

func (r *baseStateContext) Parameters() *Parameters {
	return r.parameters
}

// BuildReadStateRequest decode eth_call data to StateContext
func BuildReadStateRequest(data []byte) (StateContext, error) {
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
	default:
		return nil, errInvalidCallSig
	}
}

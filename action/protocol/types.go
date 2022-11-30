package protocol

import "github.com/iotexproject/iotex-proto/golang/iotexapi"

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

	// BaseStateContext base state context
	BaseStateContext struct {
		Parameter *Parameters
	}
)

// Parameters base state parameters
func (r *BaseStateContext) Parameters() *Parameters {
	return r.Parameter
}

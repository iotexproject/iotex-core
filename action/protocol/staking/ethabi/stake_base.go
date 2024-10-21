package ethabi

import (
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
	v1 "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/v1"
	v2 "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/v2"
	v3 "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/v3"
)

// BuildReadStateRequest decode eth_call data to StateContext
func BuildReadStateRequest(data []byte) (protocol.StateContext, error) {
	if len(data) < 4 {
		return nil, stakingComm.ErrInvalidCallData
	}

	methodSig, err := v1.BuildReadStateRequest(data)
	if err == nil {
		return methodSig, nil
	}
	methodSig, err = v2.BuildReadStateRequest(data)
	if err == nil {
		return methodSig, nil
	}
	methodSig, err = v3.BuildReadStateRequest(data)
	if err == nil {
		return methodSig, nil
	}
	return nil, stakingComm.ErrInvalidCallSig
}

package ethabi

import (
	"encoding/hex"
	"math/big"

	"github.com/cockroachdb/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/vote"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

// ProbationInfo represents a probation info in the Ethereum ABI format
type ProbationInfo struct {
	Candidate       common.Address
	OperatorAddress common.Address
	Count           uint32
}

// ProbationList represents a probation list in the Ethereum ABI format
type ProbationList struct {
	ProbationInfo []ProbationInfo
	IntensityRate uint32
}

// NewProbationInfoFromState converts a single probation entry to ethabi.ProbationInfo
func NewProbationInfoFromState(addrStr string, count uint32) (*ProbationInfo, error) {
	// Convert address from IoTeX format to Ethereum format
	addr, err := address.FromString(addrStr)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid probation address %s", addrStr)
	}

	return &ProbationInfo{
		OperatorAddress: common.BytesToAddress(addr.Bytes()),
		Count:           count,
	}, nil
}

// ConvertProbationListFromState converts a vote.ProbationList to ethabi.ProbationList
func ConvertProbationListFromState(stateProbationList *vote.ProbationList) (*ProbationList, error) {
	if stateProbationList == nil {
		return &ProbationList{
			ProbationInfo: []ProbationInfo{},
			IntensityRate: 0,
		}, nil
	}

	probationInfos := make([]ProbationInfo, 0, len(stateProbationList.ProbationInfo))

	for addrStr, count := range stateProbationList.ProbationInfo {
		probationInfo, err := NewProbationInfoFromState(addrStr, count)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert probation info for address %s", addrStr)
		}
		probationInfos = append(probationInfos, *probationInfo)
	}

	return &ProbationList{
		ProbationInfo: probationInfos,
		IntensityRate: stateProbationList.IntensityRate,
	}, nil
}

type ProbationListStateContext struct {
	*protocol.BaseStateContext
}

func newProbationListStateContext(data []byte, methodABI *abi.Method) (*ProbationListStateContext, error) {
	var args [][]byte
	paramsMap := map[string]interface{}{}
	if err := methodABI.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	epoch, ok := paramsMap["epoch"]
	if ok {
		epochBigInt, ok := epoch.(*big.Int)
		if !ok {
			return nil, errors.Errorf("epoch parameter is not of type *big.Int, %T", epoch)
		}
		args = append(args, []byte(epochBigInt.Text(10)))
	}
	return &ProbationListStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: []byte("ProbationListByEpoch"),
				Arguments:  args,
			},
			Method: methodABI,
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *ProbationListStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var stateProbationList vote.ProbationList
	if err := stateProbationList.Deserialize(resp.Data); err != nil {
		return "", errors.Wrap(err, "failed to deserialize probation list")
	}

	// Convert vote.ProbationList to ethabi.ProbationList for ABI encoding
	ethProbationList, err := ConvertProbationListFromState(&stateProbationList)
	if err != nil {
		return "", errors.Wrap(err, "failed to convert probation list")
	}

	data, err := r.Method.Outputs.Pack(ethProbationList, new(big.Int).SetUint64(resp.BlockIdentifier.Height))
	if err != nil {
		return "", errors.Wrap(err, "failed to pack probation list")
	}

	return hex.EncodeToString(data), nil
}

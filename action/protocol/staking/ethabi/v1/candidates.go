package v1

import (
	"encoding/hex"
	"math"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/ethabi/common"
)

// CandidatesStateContext context for Candidates
type CandidatesStateContext struct {
	*protocol.BaseStateContext
}

func NewCandidatesStateContext(data []byte, methodABI *abi.Method, apiMethod iotexapi.ReadStakingDataMethod_Name) (*CandidatesStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := methodABI.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var offset, limit uint32
	if offset, ok = paramsMap["offset"].(uint32); !ok {
		return nil, common.ErrDecodeFailure
	}
	if limit, ok = paramsMap["limit"].(uint32); !ok {
		return nil, common.ErrDecodeFailure
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: apiMethod,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Candidates_{
			Candidates: &iotexapi.ReadStakingDataRequest_Candidates{
				Pagination: &iotexapi.PaginationParam{
					Offset: offset,
					Limit:  limit,
				},
			},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &CandidatesStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
			Method: methodABI,
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *CandidatesStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.CandidateListV2
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	args := make([]common.CandidateEth, len(result.Candidates))
	for i, candidate := range result.Candidates {
		cand, err := encodeCandidateToEth(candidate)
		if err != nil {
			return "", err
		}
		args[i] = *cand
	}
	data, err := r.Method.Outputs.Pack(args)
	if err != nil {
		return "", nil
	}
	return hex.EncodeToString(data), nil
}

func encodeCandidateToEth(candidate *iotextypes.CandidateV2) (*common.CandidateEth, error) {
	if candidate.SelfStakeBucketIdx == math.MaxUint64 {
		// TODO: remove the temporary fix for selfStakeBucketIdx
		// convert max uint64 into max uint32 to avoid overflow in iopay
		// it can be removed after iopay supports uint64
		candidate.SelfStakeBucketIdx = math.MaxUint32
	}
	return common.EncodeCandidateToEth(candidate)
}

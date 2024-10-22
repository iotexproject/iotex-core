package common

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// BucketsByVoterStateContext context for BucketsByVoter
type BucketsByVoterStateContext struct {
	*protocol.BaseStateContext
}

func NewBucketsByVoterStateContext(data []byte, methodABI *abi.Method, apiMethod iotexapi.ReadStakingDataMethod_Name) (*BucketsByVoterStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := methodABI.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var voter common.Address
	if voter, ok = paramsMap["voter"].(common.Address); !ok {
		return nil, ErrDecodeFailure
	}
	voterAddress, err := address.FromBytes(voter[:])
	if err != nil {
		return nil, err
	}
	var offset, limit uint32
	if offset, ok = paramsMap["offset"].(uint32); !ok {
		return nil, ErrDecodeFailure
	}
	if limit, ok = paramsMap["limit"].(uint32); !ok {
		return nil, ErrDecodeFailure
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: apiMethod,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByVoter{
			BucketsByVoter: &iotexapi.ReadStakingDataRequest_VoteBucketsByVoter{
				VoterAddress: voterAddress.String(),
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
	return &BucketsByVoterStateContext{
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
func (r *BucketsByVoterStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.VoteBucketList
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	return EncodeVoteBucketListToEth(r.Method.Outputs, &result)
}

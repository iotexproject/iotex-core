package common

import (
	"encoding/hex"
	"math"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol"
)

// CandidateByAddressStateContext context for candidateByAddress
type CandidateByAddressStateContext struct {
	*protocol.BaseStateContext
	cfg candidateConfig
}

func NewCandidateByAddressStateContext(data []byte, methodABI *abi.Method, apiMethod iotexapi.ReadStakingDataMethod_Name, opts ...OptionCandidate) (*CandidateByAddressStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := methodABI.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var ownerAddress common.Address
	if ownerAddress, ok = paramsMap["ownerAddress"].(common.Address); !ok {
		return nil, ErrDecodeFailure
	}
	owner, err := address.FromBytes(ownerAddress[:])
	if err != nil {
		return nil, err
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: apiMethod,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByAddress_{
			CandidateByAddress: &iotexapi.ReadStakingDataRequest_CandidateByAddress{
				OwnerAddr: owner.String(),
			},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	cfg := &candidateConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return &CandidateByAddressStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
			Method: methodABI,
		},
		*cfg,
	}, nil
}

// EncodeToEth encode proto to eth
func (r *CandidateByAddressStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.CandidateV2
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}
	if r.cfg.noSelfStakeBucketIndexAsMaxUint32 && result.SelfStakeBucketIdx == math.MaxUint64 {
		result.SelfStakeBucketIdx = math.MaxUint32
	}
	cand, err := EncodeCandidateToEth(&result)
	if err != nil {
		return "", err
	}

	data, err := r.Method.Outputs.Pack(cand)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

package common

import (
	"encoding/hex"
	"math"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol"
)

// CandidateByNameStateContext context for CandidateByName
type CandidateByNameStateContext struct {
	*protocol.BaseStateContext
	cfg candidateConfig
}

func NewCandidateByNameStateContext(data []byte, methodABI *abi.Method, apiMethod iotexapi.ReadStakingDataMethod_Name, opts ...OptionCandidate) (*CandidateByNameStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := methodABI.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var candName string
	if candName, ok = paramsMap["candName"].(string); !ok {
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
		Request: &iotexapi.ReadStakingDataRequest_CandidateByName_{
			CandidateByName: &iotexapi.ReadStakingDataRequest_CandidateByName{
				CandName: candName,
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
	return &CandidateByNameStateContext{
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
func (r *CandidateByNameStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
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
		return "", nil
	}
	return hex.EncodeToString(data), nil
}

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

type (
	// CandidatesStateContext context for Candidates
	CandidatesStateContext struct {
		*protocol.BaseStateContext
		cfg candidateConfig
	}
	// OptionCandidate option for candidate
	OptionCandidate func(*candidateConfig)

	candidateConfig struct {
		noSelfStakeBucketIndexAsMaxUint32 bool
	}
)

// WithNoSelfStakeBucketIndexAsMaxUint32 set noSelfStakeBucketIndex as MaxUint32
func WithNoSelfStakeBucketIndexAsMaxUint32() OptionCandidate {
	return func(c *candidateConfig) {
		c.noSelfStakeBucketIndexAsMaxUint32 = true
	}
}

func NewCandidatesStateContext(data []byte, methodABI *abi.Method, apiMethod iotexapi.ReadStakingDataMethod_Name, opts ...OptionCandidate) (*CandidatesStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := methodABI.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
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
	cfg := &candidateConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return &CandidatesStateContext{
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
func (r *CandidatesStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.CandidateListV2
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	args := make([]CandidateEth, len(result.Candidates))
	for i, candidate := range result.Candidates {
		if r.cfg.noSelfStakeBucketIndexAsMaxUint32 && candidate.SelfStakeBucketIdx == math.MaxUint64 {
			candidate.SelfStakeBucketIdx = math.MaxUint32
		}
		cand, err := EncodeCandidateToEth(candidate)
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

package ethabi

import (
	"encoding/hex"
	"math/big"

	"github.com/cockroachdb/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

// Candidate represents a candidate in the Ethereum ABI format
type Candidate struct {
	ID              common.Address `abi:"id"`
	Name            string         `abi:"name"`
	OperatorAddress common.Address `abi:"operatorAddress"`
	RewardAddress   common.Address `abi:"rewardAddress"`
	BlsPubKey       []byte         `abi:"blsPubKey"`
	Votes           *big.Int       `abi:"votes"`
}

// NewCandidateFromState converts a state.Candidate to ethabi.Candidate
func NewCandidateFromState(stateCandidate *state.Candidate) (*Candidate, error) {
	// Convert operator address
	operatorAddr, err := address.FromString(stateCandidate.Address)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid operator address %s", stateCandidate.Address)
	}

	// Convert reward address
	rewardAddr, err := address.FromString(stateCandidate.RewardAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid reward address %s", stateCandidate.RewardAddress)
	}

	return &Candidate{
		Name:            string(stateCandidate.CanName),
		OperatorAddress: common.BytesToAddress(operatorAddr.Bytes()),
		RewardAddress:   common.BytesToAddress(rewardAddr.Bytes()),
		BlsPubKey:       stateCandidate.BLSPubKey,
		Votes:           stateCandidate.Votes,
	}, nil
}

// ConvertCandidateListFromState converts a state.CandidateList to []Candidate
func ConvertCandidateListFromState(stateCandidates state.CandidateList) ([]Candidate, error) {
	ethCandidates := make([]Candidate, len(stateCandidates))
	for i, stateCandidate := range stateCandidates {
		ethCandidate, err := NewCandidateFromState(stateCandidate)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert candidate at index %d", i)
		}
		ethCandidates[i] = *ethCandidate
	}
	return ethCandidates, nil
}

type CandidateListStateContext struct {
	*protocol.BaseStateContext
}

func newCandidateListStateContext(data []byte, methodABI *abi.Method, name string) (*CandidateListStateContext, error) {
	var args [][]byte
	paramsMap := map[string]any{}
	if err := methodABI.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	epoch, ok := paramsMap["epoch"]
	if ok {
		epochBigInt, ok := epoch.(*big.Int)
		if !ok {
			return nil, errors.New("failed to cast epoch to *big.Int")
		}
		args = append(args, []byte(epochBigInt.Text(10)))
	}
	return &CandidateListStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: []byte(name),
				Arguments:  args,
			},
			Method: methodABI,
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *CandidateListStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var cands state.CandidateList
	if err := cands.Deserialize(resp.Data); err != nil {
		return "", err
	}

	// Convert state.CandidateList to []Candidate for ABI encoding
	ethCandidates, err := ConvertCandidateListFromState(cands)
	if err != nil {
		return "", errors.Wrap(err, "failed to convert candidate list")
	}

	data, err := r.Method.Outputs.Pack(ethCandidates, new(big.Int).SetUint64(resp.BlockIdentifier.Height))
	if err != nil {
		return "", errors.Wrap(err, "failed to pack candidate list")
	}

	return hex.EncodeToString(data), nil
}

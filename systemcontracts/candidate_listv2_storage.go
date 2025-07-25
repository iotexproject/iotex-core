package systemcontracts

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type SolidityCandidateV2 struct {
	OwnerAddress       string
	OperatorAddress    string
	RewardAddress      string
	Name               string
	TotalWeightedVotes string
	SelfStakeBucketIdx uint64
	SelfStakingTokens  string
	Id                 string
}

type (
	CandidateListV2StorageContract struct {
		contractAddress common.Address
		backend         ContractBackend
		abi             abi.ABI
	}
)

// NewCandidateListV2StorageContract creates a new CandidateListV2 storage contract instance
func NewCandidateListV2StorageContract(contractAddress common.Address, backend ContractBackend) (*CandidateListV2StorageContract, error) {
	abi, err := abi.JSON(strings.NewReader(CandidateListV2StorageABI))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse CandidateListV2Storage ABI")
	}

	return &CandidateListV2StorageContract{
		contractAddress: contractAddress,
		backend:         backend,
		abi:             abi,
	}, nil
}

// PutCandidates stores a complete candidate list in the contract
func (c *CandidateListV2StorageContract) PutCandidates(candidates []*iotextypes.CandidateV2) error {
	// Convert iotextypes.CandidateV2 to SolidityCandidateV2
	solidityCandidates := make([]SolidityCandidateV2, len(candidates))
	for i, candidate := range candidates {
		solidityCandidates[i] = SolidityCandidateV2{
			OwnerAddress:       candidate.OwnerAddress,
			OperatorAddress:    candidate.OperatorAddress,
			RewardAddress:      candidate.RewardAddress,
			Name:               candidate.Name,
			TotalWeightedVotes: candidate.TotalWeightedVotes,
			SelfStakeBucketIdx: candidate.SelfStakeBucketIdx,
			SelfStakingTokens:  candidate.SelfStakingTokens,
			Id:                 candidate.Id,
		}
	}

	// Pack the function call
	data, err := c.abi.Pack("putCandidates", solidityCandidates)
	if err != nil {
		return errors.Wrap(err, "failed to pack putCandidates call")
	}

	// Execute the transaction
	callMsg := &ethereum.CallMsg{
		To:    &c.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   1000000,
	}

	if err := c.backend.Handle(callMsg); err != nil {
		return errors.Wrap(err, "failed to execute putCandidates")
	}

	log.L().Debug("Successfully stored candidates",
		zap.Int("count", len(candidates)))

	return nil
}

// GetCandidates retrieves candidates with pagination from the contract
func (c *CandidateListV2StorageContract) GetCandidates(offset, limit uint64) (*iotextypes.CandidateListV2, error) {
	// Pack the function call
	data, err := c.abi.Pack("getCandidates", new(big.Int).SetUint64(offset), new(big.Int).SetUint64(limit))
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack getCandidates call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		To:    &c.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   1000000,
	}

	result, err := c.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call getCandidates")
	}

	// Unpack the result using UnpackIntoInterface for better type safety
	var getCandidatesResult struct {
		CandidateList []SolidityCandidateV2
		Total         *big.Int
	}

	err = c.abi.UnpackIntoInterface(&getCandidatesResult, "getCandidates", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack getCandidates result into interface")
	}

	solidityCandidates := getCandidatesResult.CandidateList
	total := getCandidatesResult.Total

	// Convert to iotextypes.CandidateV2
	candidates := make([]*iotextypes.CandidateV2, len(solidityCandidates))
	for i, sc := range solidityCandidates {
		candidates[i] = &iotextypes.CandidateV2{
			OwnerAddress:       sc.OwnerAddress,
			OperatorAddress:    sc.OperatorAddress,
			RewardAddress:      sc.RewardAddress,
			Name:               sc.Name,
			TotalWeightedVotes: sc.TotalWeightedVotes,
			SelfStakeBucketIdx: sc.SelfStakeBucketIdx,
			SelfStakingTokens:  sc.SelfStakingTokens,
			Id:                 sc.Id,
		}
	}

	log.L().Debug("Successfully retrieved candidates",
		zap.Uint64("offset", offset),
		zap.Uint64("limit", limit),
		zap.Int("returned", len(candidates)),
		zap.String("total", total.String()))

	return &iotextypes.CandidateListV2{
		Candidates: candidates,
	}, nil
}

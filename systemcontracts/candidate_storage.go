package systemcontracts

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type SolidityCandidate struct {
	CandidateAddress common.Address
	Votes            *big.Int
	RewardAddress    common.Address
	CanName          [32]byte
}

type (
	ContractBackend interface {
		Call(callMsg *ethereum.CallMsg) ([]byte, error)
		Handle(callMsg *ethereum.CallMsg) error
	}
	CandidateStorageContract struct {
		contractAddress common.Address
		backend         ContractBackend
		abi             abi.ABI
	}
)

// NewCandidateStorageContract creates a new candidate storage contract instance
func NewCandidateStorageContract(contractAddress common.Address, backend ContractBackend) (*CandidateStorageContract, error) {
	// Parse the ABI
	parsedABI, err := abi.JSON(strings.NewReader(CandidateStorageABI))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse contract ABI")
	}

	return &CandidateStorageContract{
		contractAddress: contractAddress,
		backend:         backend,
		abi:             parsedABI,
	}, nil
}

func (c *CandidateStorageContract) StoreCandidateList(
	ns string,
	key []byte,
	candidates []SolidityCandidate,
) error {
	// Call the contract function
	input, err := c.abi.Pack("storeCandidateList", ns, key, candidates)
	if err != nil {
		return errors.Wrap(err, "failed to pack input for storeCandidateList")
	}
	tx := &ethereum.CallMsg{
		To:       &c.contractAddress,
		Data:     input,
		Gas:      1000000,
		GasPrice: big.NewInt(0),
		Value:    big.NewInt(0),
	}
	log.L().Info("Storing candidate list contract data", log.Hex("input", input))
	if err := c.backend.Handle(tx); err != nil {
		return errors.Wrap(err, "failed to handle transaction for storeCandidateList")
	}
	return nil
}

func (c *CandidateStorageContract) GetCandidateList(
	ns string,
	key []byte,
) ([]SolidityCandidate, error) {
	// Call the contract function
	input, err := c.abi.Pack("getCandidateList", ns, key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack input for getCandidateList")
	}
	log.L().Info("Retrieving candidate list contract data", log.Hex("input", input))
	output, err := c.backend.Call(&ethereum.CallMsg{
		To:    &c.contractAddress,
		Data:  input,
		Value: big.NewInt(0),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to call getCandidateList")
	}
	log.L().Info("Retrieved candidate list contract data", log.Hex("output", output))
	if len(output) == 0 {
		return nil, nil
	}

	var candidates []SolidityCandidate
	if err := c.abi.UnpackIntoInterface(&candidates, "getCandidateList", output); err != nil {
		return nil, errors.Wrap(err, "failed to unpack output for getCandidateList")
	}
	return candidates, nil
}

package action

import (
	"bytes"
	_ "embed" // import stakingContractABIJSON
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	// CandidateSelfStakeBaseIntrinsicGas represents the base intrinsic gas for CandidateSelfStake
	CandidateSelfStakeBaseIntrinsicGas = uint64(10000)
)

var (
	//go:embed staking_contract.abi
	stakingContractABIJSON string
	stakingContractABI     abi.ABI

	candidateSelfStakeMethod abi.Method
)

// CandidateSelfStake is the action to update a candidate's bucket
type CandidateSelfStake struct {
	AbstractAction

	// bucketID is the bucket index want to be changed to
	bucketID uint64
}

func init() {
	var (
		err error
		ok  bool
	)
	stakingContractABI, err = abi.JSON(strings.NewReader(stakingContractABIJSON))
	if err != nil {
		log.S().Panicf("failed to parse staking contract ABI: %v", err)
	}
	candidateSelfStakeMethod, ok = stakingContractABI.Methods["candidateSelfStake"]
	if !ok {
		log.S().Panic("failed to get candidateSelfStake method")
	}
}

// BucketID returns the bucket index want to be changed to
func (cr *CandidateSelfStake) BucketID() uint64 { return cr.bucketID }

// IntrinsicGas returns the intrinsic gas of a CandidateRegister
func (cr *CandidateSelfStake) IntrinsicGas() (uint64, error) {
	return CandidateSelfStakeBaseIntrinsicGas, nil
}

// Cost returns the total cost of a CandidateRegister
func (cr *CandidateSelfStake) Cost() (*big.Int, error) {
	intrinsicGas, err := cr.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the CandidateRegister creates")
	}
	fee := big.NewInt(0).Mul(cr.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}

func (cr *CandidateSelfStake) encodeABIBinary() ([]byte, error) {
	data, err := candidateSelfStakeMethod.Inputs.Pack(cr.bucketID)
	if err != nil {
		return nil, err
	}
	return append(candidateSelfStakeMethod.ID, data...), nil
}

// ToEthTx returns an Ethereum transaction which corresponds to this action
func (cr *CandidateSelfStake) ToEthTx(_ uint32) (*types.Transaction, error) {
	data, err := cr.encodeABIBinary()
	if err != nil {
		return nil, err
	}
	return types.NewTx(&types.LegacyTx{
		Nonce:    cr.Nonce(),
		GasPrice: cr.GasPrice(),
		Gas:      cr.GasLimit(),
		To:       &_stakingProtocolEthAddr,
		Value:    big.NewInt(0),
		Data:     data,
	}), nil
}

// NewCandidateSelfStake returns a CandidateSelfStake action
func NewCandidateSelfStake(nonce, gasLimit uint64, gasPrice *big.Int, bucketID uint64) *CandidateSelfStake {
	return &CandidateSelfStake{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		bucketID: bucketID,
	}
}

// NewCandidateSelfStakeFromABIBinary parses the smart contract input and creates an action
func NewCandidateSelfStakeFromABIBinary(data []byte) (*CandidateSelfStake, error) {
	var (
		paramsMap = map[string]any{}
		cr        CandidateSelfStake
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(candidateSelfStakeMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := candidateSelfStakeMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	bucketID, ok := paramsMap["bucketIndex"].(uint64)
	if !ok {
		return nil, errDecodeFailure
	}
	cr.bucketID = bucketID
	return &cr, nil
}

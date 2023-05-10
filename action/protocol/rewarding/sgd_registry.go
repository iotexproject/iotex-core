package rewarding

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol/abiutil"
	"github.com/pkg/errors"
)

var (
	_sgdGetContractMethod abi.Method
)

const (
	_sgdGetContractMethodString = "getContract"
	_sgdGetContractInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "_contractAddress",
				"type": "address"
			}
		],
		"name": "getContract",
		"outputs": [
			{
				"components": [
					{
						"internalType": "address",
						"name": "recipient",
						"type": "address"
					},
					{
						"internalType": "bool",
						"name": "isApproved",
						"type": "bool"
					}
				],
				"internalType": "struct IIP15Manager.ContractInfo",
				"name": "",
				"type": "tuple"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`
)

func init() {
	_sgdGetContractMethod = abiutil.MustLoadMethod(_sgdGetContractInterfaceABI, _sgdGetContractMethodString)
}

// SGDRegistry defines the interface of sgd registry
type SGDRegistry interface {
	//CheckContract check if the contract is approved in sgd registry contract, return (receiverAddress, percentage, isApproved, error)
	CheckContract(ctx context.Context, contract string) (address.Address, uint64, bool, error)
}

// ReadContract defines a callback function to read contract
type ReadContract func(context.Context, string, []byte, bool) ([]byte, error)

type sgdRegistry struct {
	percentage         uint64
	sgdContractAddress string
	readContract       ReadContract
}

// NewSGDRegistry creates a new SGDRegistry
func NewSGDRegistry(percentage uint64, sgdContractAddress string, readContract ReadContract) SGDRegistry {
	return &sgdRegistry{
		percentage:         percentage,
		sgdContractAddress: sgdContractAddress,
		readContract:       readContract,
	}
}

// CheckContract check if the contract is approved in sgd registry contract, return (receiverAddress, percentage, isApproved, error)
func (r *sgdRegistry) CheckContract(ctx context.Context, contract string) (address.Address, uint64, bool, error) {
	//if readContract is nil, keep the old behavior
	if r.readContract == nil {
		return nil, 0, false, nil
	}
	receiver, isApproved, err := r.getContract(ctx, contract)
	if err != nil {
		return nil, 0, false, errors.Wrap(err, "failed to GetContract")
	}

	return receiver, r.percentage, isApproved, nil
}

// getContract get contract info in sgd registry contract, return (receiverAddress, isApproved, error)
func (r *sgdRegistry) getContract(ctx context.Context, contract string) (address.Address, bool, error) {
	if r.readContract == nil {
		return nil, false, errors.New("empty read contract callback")
	}
	addr, err := address.FromString(contract)
	if err != nil {
		return nil, false, err
	}
	inputPack, err := _sgdGetContractMethod.Inputs.Pack(common.HexToAddress(addr.Hex()))
	if err != nil {
		return nil, false, err
	}
	inputPack = append(_sgdGetContractMethod.ID, inputPack...)
	data, err := r.readContract(ctx, r.sgdContractAddress, inputPack, true)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to readContract")
	}
	outputs, err := _sgdGetContractMethod.Outputs.UnpackValues(data)
	if err != nil {
		return nil, false, err
	}
	if len(outputs) != 1 {
		return nil, false, errors.Errorf("invalid number of outputs %d", len(outputs))
	}
	type contractInfo struct {
		Recipient  common.Address
		IsApproved bool
	}
	info := abi.ConvertType(outputs[0], new(contractInfo)).(*contractInfo)
	// if recipient is empty, it means the contract is not registered
	if info.Recipient == (common.Address{}) {
		return nil, false, nil
	}
	recipient, _ := address.FromBytes(info.Recipient.Bytes())
	return recipient, info.IsApproved, nil
}

// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// FleetManagementMetaData contains all meta data concerning the FleetManagement contract.
var FleetManagementMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"coordinator\",\"type\":\"address\"}],\"name\":\"CoordinatorSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"creditCenter\",\"type\":\"address\"}],\"name\":\"CreditCenterSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"FeeWithdrawn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"prover\",\"type\":\"address\"}],\"name\":\"ProverStoreSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fee\",\"type\":\"uint256\"}],\"name\":\"RegistrationFeeSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"slasher\",\"type\":\"address\"}],\"name\":\"SlasherSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"msp\",\"type\":\"address\"}],\"name\":\"StakingHubSet\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"coordinator\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"creditCenter\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"epoch\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_proverId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"grant\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_minStake\",\"type\":\"uint256\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_coordinator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"isActiveCoordinator\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"}],\"name\":\"isActiveProver\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"minStake\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_proverId\",\"type\":\"uint256\"}],\"name\":\"ownerOfProver\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"proverStore\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"register\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"registrationFee\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_coordinator\",\"type\":\"address\"}],\"name\":\"setCoordinator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_creditCenter\",\"type\":\"address\"}],\"name\":\"setCreditCenter\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_proverStore\",\"type\":\"address\"}],\"name\":\"setProverStore\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_fee\",\"type\":\"uint256\"}],\"name\":\"setRegistrationFee\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_slasher\",\"type\":\"address\"}],\"name\":\"setSlasher\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_hub\",\"type\":\"address\"}],\"name\":\"setStakingHub\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"slasher\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"stakingHub\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"addresspayable\",\"name\":\"_account\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"withdrawFee\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// FleetManagementABI is the input ABI used to generate the binding from.
// Deprecated: Use FleetManagementMetaData.ABI instead.
var FleetManagementABI = FleetManagementMetaData.ABI

// FleetManagement is an auto generated Go binding around an Ethereum contract.
type FleetManagement struct {
	FleetManagementCaller     // Read-only binding to the contract
	FleetManagementTransactor // Write-only binding to the contract
	FleetManagementFilterer   // Log filterer for contract events
}

// FleetManagementCaller is an auto generated read-only Go binding around an Ethereum contract.
type FleetManagementCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FleetManagementTransactor is an auto generated write-only Go binding around an Ethereum contract.
type FleetManagementTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FleetManagementFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type FleetManagementFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FleetManagementSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type FleetManagementSession struct {
	Contract     *FleetManagement  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// FleetManagementCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type FleetManagementCallerSession struct {
	Contract *FleetManagementCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// FleetManagementTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type FleetManagementTransactorSession struct {
	Contract     *FleetManagementTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// FleetManagementRaw is an auto generated low-level Go binding around an Ethereum contract.
type FleetManagementRaw struct {
	Contract *FleetManagement // Generic contract binding to access the raw methods on
}

// FleetManagementCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type FleetManagementCallerRaw struct {
	Contract *FleetManagementCaller // Generic read-only contract binding to access the raw methods on
}

// FleetManagementTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type FleetManagementTransactorRaw struct {
	Contract *FleetManagementTransactor // Generic write-only contract binding to access the raw methods on
}

// NewFleetManagement creates a new instance of FleetManagement, bound to a specific deployed contract.
func NewFleetManagement(address common.Address, backend bind.ContractBackend) (*FleetManagement, error) {
	contract, err := bindFleetManagement(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &FleetManagement{FleetManagementCaller: FleetManagementCaller{contract: contract}, FleetManagementTransactor: FleetManagementTransactor{contract: contract}, FleetManagementFilterer: FleetManagementFilterer{contract: contract}}, nil
}

// NewFleetManagementCaller creates a new read-only instance of FleetManagement, bound to a specific deployed contract.
func NewFleetManagementCaller(address common.Address, caller bind.ContractCaller) (*FleetManagementCaller, error) {
	contract, err := bindFleetManagement(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &FleetManagementCaller{contract: contract}, nil
}

// NewFleetManagementTransactor creates a new write-only instance of FleetManagement, bound to a specific deployed contract.
func NewFleetManagementTransactor(address common.Address, transactor bind.ContractTransactor) (*FleetManagementTransactor, error) {
	contract, err := bindFleetManagement(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &FleetManagementTransactor{contract: contract}, nil
}

// NewFleetManagementFilterer creates a new log filterer instance of FleetManagement, bound to a specific deployed contract.
func NewFleetManagementFilterer(address common.Address, filterer bind.ContractFilterer) (*FleetManagementFilterer, error) {
	contract, err := bindFleetManagement(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &FleetManagementFilterer{contract: contract}, nil
}

// bindFleetManagement binds a generic wrapper to an already deployed contract.
func bindFleetManagement(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := FleetManagementMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FleetManagement *FleetManagementRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FleetManagement.Contract.FleetManagementCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FleetManagement *FleetManagementRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FleetManagement.Contract.FleetManagementTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FleetManagement *FleetManagementRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FleetManagement.Contract.FleetManagementTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FleetManagement *FleetManagementCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FleetManagement.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FleetManagement *FleetManagementTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FleetManagement.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FleetManagement *FleetManagementTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FleetManagement.Contract.contract.Transact(opts, method, params...)
}

// Coordinator is a free data retrieval call binding the contract method 0x0a009097.
//
// Solidity: function coordinator() view returns(address)
func (_FleetManagement *FleetManagementCaller) Coordinator(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "coordinator")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Coordinator is a free data retrieval call binding the contract method 0x0a009097.
//
// Solidity: function coordinator() view returns(address)
func (_FleetManagement *FleetManagementSession) Coordinator() (common.Address, error) {
	return _FleetManagement.Contract.Coordinator(&_FleetManagement.CallOpts)
}

// Coordinator is a free data retrieval call binding the contract method 0x0a009097.
//
// Solidity: function coordinator() view returns(address)
func (_FleetManagement *FleetManagementCallerSession) Coordinator() (common.Address, error) {
	return _FleetManagement.Contract.Coordinator(&_FleetManagement.CallOpts)
}

// CreditCenter is a free data retrieval call binding the contract method 0x8c8ffa2e.
//
// Solidity: function creditCenter() view returns(address)
func (_FleetManagement *FleetManagementCaller) CreditCenter(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "creditCenter")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// CreditCenter is a free data retrieval call binding the contract method 0x8c8ffa2e.
//
// Solidity: function creditCenter() view returns(address)
func (_FleetManagement *FleetManagementSession) CreditCenter() (common.Address, error) {
	return _FleetManagement.Contract.CreditCenter(&_FleetManagement.CallOpts)
}

// CreditCenter is a free data retrieval call binding the contract method 0x8c8ffa2e.
//
// Solidity: function creditCenter() view returns(address)
func (_FleetManagement *FleetManagementCallerSession) CreditCenter() (common.Address, error) {
	return _FleetManagement.Contract.CreditCenter(&_FleetManagement.CallOpts)
}

// Epoch is a free data retrieval call binding the contract method 0x900cf0cf.
//
// Solidity: function epoch() view returns(uint256)
func (_FleetManagement *FleetManagementCaller) Epoch(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "epoch")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Epoch is a free data retrieval call binding the contract method 0x900cf0cf.
//
// Solidity: function epoch() view returns(uint256)
func (_FleetManagement *FleetManagementSession) Epoch() (*big.Int, error) {
	return _FleetManagement.Contract.Epoch(&_FleetManagement.CallOpts)
}

// Epoch is a free data retrieval call binding the contract method 0x900cf0cf.
//
// Solidity: function epoch() view returns(uint256)
func (_FleetManagement *FleetManagementCallerSession) Epoch() (*big.Int, error) {
	return _FleetManagement.Contract.Epoch(&_FleetManagement.CallOpts)
}

// IsActiveCoordinator is a free data retrieval call binding the contract method 0xc6b4c7e9.
//
// Solidity: function isActiveCoordinator(address _coordinator, uint256 _projectId) view returns(bool)
func (_FleetManagement *FleetManagementCaller) IsActiveCoordinator(opts *bind.CallOpts, _coordinator common.Address, _projectId *big.Int) (bool, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "isActiveCoordinator", _coordinator, _projectId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsActiveCoordinator is a free data retrieval call binding the contract method 0xc6b4c7e9.
//
// Solidity: function isActiveCoordinator(address _coordinator, uint256 _projectId) view returns(bool)
func (_FleetManagement *FleetManagementSession) IsActiveCoordinator(_coordinator common.Address, _projectId *big.Int) (bool, error) {
	return _FleetManagement.Contract.IsActiveCoordinator(&_FleetManagement.CallOpts, _coordinator, _projectId)
}

// IsActiveCoordinator is a free data retrieval call binding the contract method 0xc6b4c7e9.
//
// Solidity: function isActiveCoordinator(address _coordinator, uint256 _projectId) view returns(bool)
func (_FleetManagement *FleetManagementCallerSession) IsActiveCoordinator(_coordinator common.Address, _projectId *big.Int) (bool, error) {
	return _FleetManagement.Contract.IsActiveCoordinator(&_FleetManagement.CallOpts, _coordinator, _projectId)
}

// IsActiveProver is a free data retrieval call binding the contract method 0x4f8d116a.
//
// Solidity: function isActiveProver(uint256 _id) view returns(bool)
func (_FleetManagement *FleetManagementCaller) IsActiveProver(opts *bind.CallOpts, _id *big.Int) (bool, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "isActiveProver", _id)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsActiveProver is a free data retrieval call binding the contract method 0x4f8d116a.
//
// Solidity: function isActiveProver(uint256 _id) view returns(bool)
func (_FleetManagement *FleetManagementSession) IsActiveProver(_id *big.Int) (bool, error) {
	return _FleetManagement.Contract.IsActiveProver(&_FleetManagement.CallOpts, _id)
}

// IsActiveProver is a free data retrieval call binding the contract method 0x4f8d116a.
//
// Solidity: function isActiveProver(uint256 _id) view returns(bool)
func (_FleetManagement *FleetManagementCallerSession) IsActiveProver(_id *big.Int) (bool, error) {
	return _FleetManagement.Contract.IsActiveProver(&_FleetManagement.CallOpts, _id)
}

// MinStake is a free data retrieval call binding the contract method 0x375b3c0a.
//
// Solidity: function minStake() view returns(uint256)
func (_FleetManagement *FleetManagementCaller) MinStake(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "minStake")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MinStake is a free data retrieval call binding the contract method 0x375b3c0a.
//
// Solidity: function minStake() view returns(uint256)
func (_FleetManagement *FleetManagementSession) MinStake() (*big.Int, error) {
	return _FleetManagement.Contract.MinStake(&_FleetManagement.CallOpts)
}

// MinStake is a free data retrieval call binding the contract method 0x375b3c0a.
//
// Solidity: function minStake() view returns(uint256)
func (_FleetManagement *FleetManagementCallerSession) MinStake() (*big.Int, error) {
	return _FleetManagement.Contract.MinStake(&_FleetManagement.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_FleetManagement *FleetManagementCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_FleetManagement *FleetManagementSession) Owner() (common.Address, error) {
	return _FleetManagement.Contract.Owner(&_FleetManagement.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_FleetManagement *FleetManagementCallerSession) Owner() (common.Address, error) {
	return _FleetManagement.Contract.Owner(&_FleetManagement.CallOpts)
}

// OwnerOfProver is a free data retrieval call binding the contract method 0x6441b1e0.
//
// Solidity: function ownerOfProver(uint256 _proverId) view returns(address)
func (_FleetManagement *FleetManagementCaller) OwnerOfProver(opts *bind.CallOpts, _proverId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "ownerOfProver", _proverId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OwnerOfProver is a free data retrieval call binding the contract method 0x6441b1e0.
//
// Solidity: function ownerOfProver(uint256 _proverId) view returns(address)
func (_FleetManagement *FleetManagementSession) OwnerOfProver(_proverId *big.Int) (common.Address, error) {
	return _FleetManagement.Contract.OwnerOfProver(&_FleetManagement.CallOpts, _proverId)
}

// OwnerOfProver is a free data retrieval call binding the contract method 0x6441b1e0.
//
// Solidity: function ownerOfProver(uint256 _proverId) view returns(address)
func (_FleetManagement *FleetManagementCallerSession) OwnerOfProver(_proverId *big.Int) (common.Address, error) {
	return _FleetManagement.Contract.OwnerOfProver(&_FleetManagement.CallOpts, _proverId)
}

// ProverStore is a free data retrieval call binding the contract method 0x79b851f6.
//
// Solidity: function proverStore() view returns(address)
func (_FleetManagement *FleetManagementCaller) ProverStore(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "proverStore")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ProverStore is a free data retrieval call binding the contract method 0x79b851f6.
//
// Solidity: function proverStore() view returns(address)
func (_FleetManagement *FleetManagementSession) ProverStore() (common.Address, error) {
	return _FleetManagement.Contract.ProverStore(&_FleetManagement.CallOpts)
}

// ProverStore is a free data retrieval call binding the contract method 0x79b851f6.
//
// Solidity: function proverStore() view returns(address)
func (_FleetManagement *FleetManagementCallerSession) ProverStore() (common.Address, error) {
	return _FleetManagement.Contract.ProverStore(&_FleetManagement.CallOpts)
}

// RegistrationFee is a free data retrieval call binding the contract method 0x14c44e09.
//
// Solidity: function registrationFee() view returns(uint256)
func (_FleetManagement *FleetManagementCaller) RegistrationFee(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "registrationFee")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RegistrationFee is a free data retrieval call binding the contract method 0x14c44e09.
//
// Solidity: function registrationFee() view returns(uint256)
func (_FleetManagement *FleetManagementSession) RegistrationFee() (*big.Int, error) {
	return _FleetManagement.Contract.RegistrationFee(&_FleetManagement.CallOpts)
}

// RegistrationFee is a free data retrieval call binding the contract method 0x14c44e09.
//
// Solidity: function registrationFee() view returns(uint256)
func (_FleetManagement *FleetManagementCallerSession) RegistrationFee() (*big.Int, error) {
	return _FleetManagement.Contract.RegistrationFee(&_FleetManagement.CallOpts)
}

// Slasher is a free data retrieval call binding the contract method 0xb1344271.
//
// Solidity: function slasher() view returns(address)
func (_FleetManagement *FleetManagementCaller) Slasher(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "slasher")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Slasher is a free data retrieval call binding the contract method 0xb1344271.
//
// Solidity: function slasher() view returns(address)
func (_FleetManagement *FleetManagementSession) Slasher() (common.Address, error) {
	return _FleetManagement.Contract.Slasher(&_FleetManagement.CallOpts)
}

// Slasher is a free data retrieval call binding the contract method 0xb1344271.
//
// Solidity: function slasher() view returns(address)
func (_FleetManagement *FleetManagementCallerSession) Slasher() (common.Address, error) {
	return _FleetManagement.Contract.Slasher(&_FleetManagement.CallOpts)
}

// StakingHub is a free data retrieval call binding the contract method 0x4213ec0f.
//
// Solidity: function stakingHub() view returns(address)
func (_FleetManagement *FleetManagementCaller) StakingHub(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FleetManagement.contract.Call(opts, &out, "stakingHub")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// StakingHub is a free data retrieval call binding the contract method 0x4213ec0f.
//
// Solidity: function stakingHub() view returns(address)
func (_FleetManagement *FleetManagementSession) StakingHub() (common.Address, error) {
	return _FleetManagement.Contract.StakingHub(&_FleetManagement.CallOpts)
}

// StakingHub is a free data retrieval call binding the contract method 0x4213ec0f.
//
// Solidity: function stakingHub() view returns(address)
func (_FleetManagement *FleetManagementCallerSession) StakingHub() (common.Address, error) {
	return _FleetManagement.Contract.StakingHub(&_FleetManagement.CallOpts)
}

// Grant is a paid mutator transaction binding the contract method 0x59f1c85e.
//
// Solidity: function grant(uint256 _proverId, uint256 _amount) returns()
func (_FleetManagement *FleetManagementTransactor) Grant(opts *bind.TransactOpts, _proverId *big.Int, _amount *big.Int) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "grant", _proverId, _amount)
}

// Grant is a paid mutator transaction binding the contract method 0x59f1c85e.
//
// Solidity: function grant(uint256 _proverId, uint256 _amount) returns()
func (_FleetManagement *FleetManagementSession) Grant(_proverId *big.Int, _amount *big.Int) (*types.Transaction, error) {
	return _FleetManagement.Contract.Grant(&_FleetManagement.TransactOpts, _proverId, _amount)
}

// Grant is a paid mutator transaction binding the contract method 0x59f1c85e.
//
// Solidity: function grant(uint256 _proverId, uint256 _amount) returns()
func (_FleetManagement *FleetManagementTransactorSession) Grant(_proverId *big.Int, _amount *big.Int) (*types.Transaction, error) {
	return _FleetManagement.Contract.Grant(&_FleetManagement.TransactOpts, _proverId, _amount)
}

// Initialize is a paid mutator transaction binding the contract method 0xfe4b84df.
//
// Solidity: function initialize(uint256 _minStake) returns()
func (_FleetManagement *FleetManagementTransactor) Initialize(opts *bind.TransactOpts, _minStake *big.Int) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "initialize", _minStake)
}

// Initialize is a paid mutator transaction binding the contract method 0xfe4b84df.
//
// Solidity: function initialize(uint256 _minStake) returns()
func (_FleetManagement *FleetManagementSession) Initialize(_minStake *big.Int) (*types.Transaction, error) {
	return _FleetManagement.Contract.Initialize(&_FleetManagement.TransactOpts, _minStake)
}

// Initialize is a paid mutator transaction binding the contract method 0xfe4b84df.
//
// Solidity: function initialize(uint256 _minStake) returns()
func (_FleetManagement *FleetManagementTransactorSession) Initialize(_minStake *big.Int) (*types.Transaction, error) {
	return _FleetManagement.Contract.Initialize(&_FleetManagement.TransactOpts, _minStake)
}

// Register is a paid mutator transaction binding the contract method 0x1aa3a008.
//
// Solidity: function register() payable returns(uint256)
func (_FleetManagement *FleetManagementTransactor) Register(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "register")
}

// Register is a paid mutator transaction binding the contract method 0x1aa3a008.
//
// Solidity: function register() payable returns(uint256)
func (_FleetManagement *FleetManagementSession) Register() (*types.Transaction, error) {
	return _FleetManagement.Contract.Register(&_FleetManagement.TransactOpts)
}

// Register is a paid mutator transaction binding the contract method 0x1aa3a008.
//
// Solidity: function register() payable returns(uint256)
func (_FleetManagement *FleetManagementTransactorSession) Register() (*types.Transaction, error) {
	return _FleetManagement.Contract.Register(&_FleetManagement.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_FleetManagement *FleetManagementTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_FleetManagement *FleetManagementSession) RenounceOwnership() (*types.Transaction, error) {
	return _FleetManagement.Contract.RenounceOwnership(&_FleetManagement.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_FleetManagement *FleetManagementTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _FleetManagement.Contract.RenounceOwnership(&_FleetManagement.TransactOpts)
}

// SetCoordinator is a paid mutator transaction binding the contract method 0x8ea98117.
//
// Solidity: function setCoordinator(address _coordinator) returns()
func (_FleetManagement *FleetManagementTransactor) SetCoordinator(opts *bind.TransactOpts, _coordinator common.Address) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "setCoordinator", _coordinator)
}

// SetCoordinator is a paid mutator transaction binding the contract method 0x8ea98117.
//
// Solidity: function setCoordinator(address _coordinator) returns()
func (_FleetManagement *FleetManagementSession) SetCoordinator(_coordinator common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetCoordinator(&_FleetManagement.TransactOpts, _coordinator)
}

// SetCoordinator is a paid mutator transaction binding the contract method 0x8ea98117.
//
// Solidity: function setCoordinator(address _coordinator) returns()
func (_FleetManagement *FleetManagementTransactorSession) SetCoordinator(_coordinator common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetCoordinator(&_FleetManagement.TransactOpts, _coordinator)
}

// SetCreditCenter is a paid mutator transaction binding the contract method 0x3d625dc2.
//
// Solidity: function setCreditCenter(address _creditCenter) returns()
func (_FleetManagement *FleetManagementTransactor) SetCreditCenter(opts *bind.TransactOpts, _creditCenter common.Address) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "setCreditCenter", _creditCenter)
}

// SetCreditCenter is a paid mutator transaction binding the contract method 0x3d625dc2.
//
// Solidity: function setCreditCenter(address _creditCenter) returns()
func (_FleetManagement *FleetManagementSession) SetCreditCenter(_creditCenter common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetCreditCenter(&_FleetManagement.TransactOpts, _creditCenter)
}

// SetCreditCenter is a paid mutator transaction binding the contract method 0x3d625dc2.
//
// Solidity: function setCreditCenter(address _creditCenter) returns()
func (_FleetManagement *FleetManagementTransactorSession) SetCreditCenter(_creditCenter common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetCreditCenter(&_FleetManagement.TransactOpts, _creditCenter)
}

// SetProverStore is a paid mutator transaction binding the contract method 0x7037e334.
//
// Solidity: function setProverStore(address _proverStore) returns()
func (_FleetManagement *FleetManagementTransactor) SetProverStore(opts *bind.TransactOpts, _proverStore common.Address) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "setProverStore", _proverStore)
}

// SetProverStore is a paid mutator transaction binding the contract method 0x7037e334.
//
// Solidity: function setProverStore(address _proverStore) returns()
func (_FleetManagement *FleetManagementSession) SetProverStore(_proverStore common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetProverStore(&_FleetManagement.TransactOpts, _proverStore)
}

// SetProverStore is a paid mutator transaction binding the contract method 0x7037e334.
//
// Solidity: function setProverStore(address _proverStore) returns()
func (_FleetManagement *FleetManagementTransactorSession) SetProverStore(_proverStore common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetProverStore(&_FleetManagement.TransactOpts, _proverStore)
}

// SetRegistrationFee is a paid mutator transaction binding the contract method 0xc320c727.
//
// Solidity: function setRegistrationFee(uint256 _fee) returns()
func (_FleetManagement *FleetManagementTransactor) SetRegistrationFee(opts *bind.TransactOpts, _fee *big.Int) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "setRegistrationFee", _fee)
}

// SetRegistrationFee is a paid mutator transaction binding the contract method 0xc320c727.
//
// Solidity: function setRegistrationFee(uint256 _fee) returns()
func (_FleetManagement *FleetManagementSession) SetRegistrationFee(_fee *big.Int) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetRegistrationFee(&_FleetManagement.TransactOpts, _fee)
}

// SetRegistrationFee is a paid mutator transaction binding the contract method 0xc320c727.
//
// Solidity: function setRegistrationFee(uint256 _fee) returns()
func (_FleetManagement *FleetManagementTransactorSession) SetRegistrationFee(_fee *big.Int) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetRegistrationFee(&_FleetManagement.TransactOpts, _fee)
}

// SetSlasher is a paid mutator transaction binding the contract method 0xaabc2496.
//
// Solidity: function setSlasher(address _slasher) returns()
func (_FleetManagement *FleetManagementTransactor) SetSlasher(opts *bind.TransactOpts, _slasher common.Address) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "setSlasher", _slasher)
}

// SetSlasher is a paid mutator transaction binding the contract method 0xaabc2496.
//
// Solidity: function setSlasher(address _slasher) returns()
func (_FleetManagement *FleetManagementSession) SetSlasher(_slasher common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetSlasher(&_FleetManagement.TransactOpts, _slasher)
}

// SetSlasher is a paid mutator transaction binding the contract method 0xaabc2496.
//
// Solidity: function setSlasher(address _slasher) returns()
func (_FleetManagement *FleetManagementTransactorSession) SetSlasher(_slasher common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetSlasher(&_FleetManagement.TransactOpts, _slasher)
}

// SetStakingHub is a paid mutator transaction binding the contract method 0x5bee43c7.
//
// Solidity: function setStakingHub(address _hub) returns()
func (_FleetManagement *FleetManagementTransactor) SetStakingHub(opts *bind.TransactOpts, _hub common.Address) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "setStakingHub", _hub)
}

// SetStakingHub is a paid mutator transaction binding the contract method 0x5bee43c7.
//
// Solidity: function setStakingHub(address _hub) returns()
func (_FleetManagement *FleetManagementSession) SetStakingHub(_hub common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetStakingHub(&_FleetManagement.TransactOpts, _hub)
}

// SetStakingHub is a paid mutator transaction binding the contract method 0x5bee43c7.
//
// Solidity: function setStakingHub(address _hub) returns()
func (_FleetManagement *FleetManagementTransactorSession) SetStakingHub(_hub common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.SetStakingHub(&_FleetManagement.TransactOpts, _hub)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_FleetManagement *FleetManagementTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_FleetManagement *FleetManagementSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.TransferOwnership(&_FleetManagement.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_FleetManagement *FleetManagementTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _FleetManagement.Contract.TransferOwnership(&_FleetManagement.TransactOpts, newOwner)
}

// WithdrawFee is a paid mutator transaction binding the contract method 0xfd9be522.
//
// Solidity: function withdrawFee(address _account, uint256 _amount) returns()
func (_FleetManagement *FleetManagementTransactor) WithdrawFee(opts *bind.TransactOpts, _account common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _FleetManagement.contract.Transact(opts, "withdrawFee", _account, _amount)
}

// WithdrawFee is a paid mutator transaction binding the contract method 0xfd9be522.
//
// Solidity: function withdrawFee(address _account, uint256 _amount) returns()
func (_FleetManagement *FleetManagementSession) WithdrawFee(_account common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _FleetManagement.Contract.WithdrawFee(&_FleetManagement.TransactOpts, _account, _amount)
}

// WithdrawFee is a paid mutator transaction binding the contract method 0xfd9be522.
//
// Solidity: function withdrawFee(address _account, uint256 _amount) returns()
func (_FleetManagement *FleetManagementTransactorSession) WithdrawFee(_account common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _FleetManagement.Contract.WithdrawFee(&_FleetManagement.TransactOpts, _account, _amount)
}

// FleetManagementCoordinatorSetIterator is returned from FilterCoordinatorSet and is used to iterate over the raw logs and unpacked data for CoordinatorSet events raised by the FleetManagement contract.
type FleetManagementCoordinatorSetIterator struct {
	Event *FleetManagementCoordinatorSet // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FleetManagementCoordinatorSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FleetManagementCoordinatorSet)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FleetManagementCoordinatorSet)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FleetManagementCoordinatorSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FleetManagementCoordinatorSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FleetManagementCoordinatorSet represents a CoordinatorSet event raised by the FleetManagement contract.
type FleetManagementCoordinatorSet struct {
	Coordinator common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterCoordinatorSet is a free log retrieval operation binding the contract event 0xd1a6a14209a385a964d036e404cb5cfb71f4000cdb03c9366292430787261be6.
//
// Solidity: event CoordinatorSet(address indexed coordinator)
func (_FleetManagement *FleetManagementFilterer) FilterCoordinatorSet(opts *bind.FilterOpts, coordinator []common.Address) (*FleetManagementCoordinatorSetIterator, error) {

	var coordinatorRule []interface{}
	for _, coordinatorItem := range coordinator {
		coordinatorRule = append(coordinatorRule, coordinatorItem)
	}

	logs, sub, err := _FleetManagement.contract.FilterLogs(opts, "CoordinatorSet", coordinatorRule)
	if err != nil {
		return nil, err
	}
	return &FleetManagementCoordinatorSetIterator{contract: _FleetManagement.contract, event: "CoordinatorSet", logs: logs, sub: sub}, nil
}

// WatchCoordinatorSet is a free log subscription operation binding the contract event 0xd1a6a14209a385a964d036e404cb5cfb71f4000cdb03c9366292430787261be6.
//
// Solidity: event CoordinatorSet(address indexed coordinator)
func (_FleetManagement *FleetManagementFilterer) WatchCoordinatorSet(opts *bind.WatchOpts, sink chan<- *FleetManagementCoordinatorSet, coordinator []common.Address) (event.Subscription, error) {

	var coordinatorRule []interface{}
	for _, coordinatorItem := range coordinator {
		coordinatorRule = append(coordinatorRule, coordinatorItem)
	}

	logs, sub, err := _FleetManagement.contract.WatchLogs(opts, "CoordinatorSet", coordinatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FleetManagementCoordinatorSet)
				if err := _FleetManagement.contract.UnpackLog(event, "CoordinatorSet", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCoordinatorSet is a log parse operation binding the contract event 0xd1a6a14209a385a964d036e404cb5cfb71f4000cdb03c9366292430787261be6.
//
// Solidity: event CoordinatorSet(address indexed coordinator)
func (_FleetManagement *FleetManagementFilterer) ParseCoordinatorSet(log types.Log) (*FleetManagementCoordinatorSet, error) {
	event := new(FleetManagementCoordinatorSet)
	if err := _FleetManagement.contract.UnpackLog(event, "CoordinatorSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FleetManagementCreditCenterSetIterator is returned from FilterCreditCenterSet and is used to iterate over the raw logs and unpacked data for CreditCenterSet events raised by the FleetManagement contract.
type FleetManagementCreditCenterSetIterator struct {
	Event *FleetManagementCreditCenterSet // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FleetManagementCreditCenterSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FleetManagementCreditCenterSet)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FleetManagementCreditCenterSet)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FleetManagementCreditCenterSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FleetManagementCreditCenterSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FleetManagementCreditCenterSet represents a CreditCenterSet event raised by the FleetManagement contract.
type FleetManagementCreditCenterSet struct {
	CreditCenter common.Address
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterCreditCenterSet is a free log retrieval operation binding the contract event 0xb139440c94b411ec448cfab90b78cc0c0a5b864ec2c00f0372f29c8234ffe6eb.
//
// Solidity: event CreditCenterSet(address indexed creditCenter)
func (_FleetManagement *FleetManagementFilterer) FilterCreditCenterSet(opts *bind.FilterOpts, creditCenter []common.Address) (*FleetManagementCreditCenterSetIterator, error) {

	var creditCenterRule []interface{}
	for _, creditCenterItem := range creditCenter {
		creditCenterRule = append(creditCenterRule, creditCenterItem)
	}

	logs, sub, err := _FleetManagement.contract.FilterLogs(opts, "CreditCenterSet", creditCenterRule)
	if err != nil {
		return nil, err
	}
	return &FleetManagementCreditCenterSetIterator{contract: _FleetManagement.contract, event: "CreditCenterSet", logs: logs, sub: sub}, nil
}

// WatchCreditCenterSet is a free log subscription operation binding the contract event 0xb139440c94b411ec448cfab90b78cc0c0a5b864ec2c00f0372f29c8234ffe6eb.
//
// Solidity: event CreditCenterSet(address indexed creditCenter)
func (_FleetManagement *FleetManagementFilterer) WatchCreditCenterSet(opts *bind.WatchOpts, sink chan<- *FleetManagementCreditCenterSet, creditCenter []common.Address) (event.Subscription, error) {

	var creditCenterRule []interface{}
	for _, creditCenterItem := range creditCenter {
		creditCenterRule = append(creditCenterRule, creditCenterItem)
	}

	logs, sub, err := _FleetManagement.contract.WatchLogs(opts, "CreditCenterSet", creditCenterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FleetManagementCreditCenterSet)
				if err := _FleetManagement.contract.UnpackLog(event, "CreditCenterSet", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCreditCenterSet is a log parse operation binding the contract event 0xb139440c94b411ec448cfab90b78cc0c0a5b864ec2c00f0372f29c8234ffe6eb.
//
// Solidity: event CreditCenterSet(address indexed creditCenter)
func (_FleetManagement *FleetManagementFilterer) ParseCreditCenterSet(log types.Log) (*FleetManagementCreditCenterSet, error) {
	event := new(FleetManagementCreditCenterSet)
	if err := _FleetManagement.contract.UnpackLog(event, "CreditCenterSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FleetManagementFeeWithdrawnIterator is returned from FilterFeeWithdrawn and is used to iterate over the raw logs and unpacked data for FeeWithdrawn events raised by the FleetManagement contract.
type FleetManagementFeeWithdrawnIterator struct {
	Event *FleetManagementFeeWithdrawn // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FleetManagementFeeWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FleetManagementFeeWithdrawn)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FleetManagementFeeWithdrawn)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FleetManagementFeeWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FleetManagementFeeWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FleetManagementFeeWithdrawn represents a FeeWithdrawn event raised by the FleetManagement contract.
type FleetManagementFeeWithdrawn struct {
	Account common.Address
	Amount  *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterFeeWithdrawn is a free log retrieval operation binding the contract event 0x78473f3f373f7673597f4f0fa5873cb4d375fea6d4339ad6b56dbd411513cb3f.
//
// Solidity: event FeeWithdrawn(address indexed account, uint256 amount)
func (_FleetManagement *FleetManagementFilterer) FilterFeeWithdrawn(opts *bind.FilterOpts, account []common.Address) (*FleetManagementFeeWithdrawnIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _FleetManagement.contract.FilterLogs(opts, "FeeWithdrawn", accountRule)
	if err != nil {
		return nil, err
	}
	return &FleetManagementFeeWithdrawnIterator{contract: _FleetManagement.contract, event: "FeeWithdrawn", logs: logs, sub: sub}, nil
}

// WatchFeeWithdrawn is a free log subscription operation binding the contract event 0x78473f3f373f7673597f4f0fa5873cb4d375fea6d4339ad6b56dbd411513cb3f.
//
// Solidity: event FeeWithdrawn(address indexed account, uint256 amount)
func (_FleetManagement *FleetManagementFilterer) WatchFeeWithdrawn(opts *bind.WatchOpts, sink chan<- *FleetManagementFeeWithdrawn, account []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _FleetManagement.contract.WatchLogs(opts, "FeeWithdrawn", accountRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FleetManagementFeeWithdrawn)
				if err := _FleetManagement.contract.UnpackLog(event, "FeeWithdrawn", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFeeWithdrawn is a log parse operation binding the contract event 0x78473f3f373f7673597f4f0fa5873cb4d375fea6d4339ad6b56dbd411513cb3f.
//
// Solidity: event FeeWithdrawn(address indexed account, uint256 amount)
func (_FleetManagement *FleetManagementFilterer) ParseFeeWithdrawn(log types.Log) (*FleetManagementFeeWithdrawn, error) {
	event := new(FleetManagementFeeWithdrawn)
	if err := _FleetManagement.contract.UnpackLog(event, "FeeWithdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FleetManagementInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the FleetManagement contract.
type FleetManagementInitializedIterator struct {
	Event *FleetManagementInitialized // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FleetManagementInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FleetManagementInitialized)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FleetManagementInitialized)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FleetManagementInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FleetManagementInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FleetManagementInitialized represents a Initialized event raised by the FleetManagement contract.
type FleetManagementInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_FleetManagement *FleetManagementFilterer) FilterInitialized(opts *bind.FilterOpts) (*FleetManagementInitializedIterator, error) {

	logs, sub, err := _FleetManagement.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &FleetManagementInitializedIterator{contract: _FleetManagement.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_FleetManagement *FleetManagementFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *FleetManagementInitialized) (event.Subscription, error) {

	logs, sub, err := _FleetManagement.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FleetManagementInitialized)
				if err := _FleetManagement.contract.UnpackLog(event, "Initialized", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseInitialized is a log parse operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_FleetManagement *FleetManagementFilterer) ParseInitialized(log types.Log) (*FleetManagementInitialized, error) {
	event := new(FleetManagementInitialized)
	if err := _FleetManagement.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FleetManagementOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the FleetManagement contract.
type FleetManagementOwnershipTransferredIterator struct {
	Event *FleetManagementOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FleetManagementOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FleetManagementOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FleetManagementOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FleetManagementOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FleetManagementOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FleetManagementOwnershipTransferred represents a OwnershipTransferred event raised by the FleetManagement contract.
type FleetManagementOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_FleetManagement *FleetManagementFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*FleetManagementOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _FleetManagement.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &FleetManagementOwnershipTransferredIterator{contract: _FleetManagement.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_FleetManagement *FleetManagementFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *FleetManagementOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _FleetManagement.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FleetManagementOwnershipTransferred)
				if err := _FleetManagement.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_FleetManagement *FleetManagementFilterer) ParseOwnershipTransferred(log types.Log) (*FleetManagementOwnershipTransferred, error) {
	event := new(FleetManagementOwnershipTransferred)
	if err := _FleetManagement.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FleetManagementProverStoreSetIterator is returned from FilterProverStoreSet and is used to iterate over the raw logs and unpacked data for ProverStoreSet events raised by the FleetManagement contract.
type FleetManagementProverStoreSetIterator struct {
	Event *FleetManagementProverStoreSet // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FleetManagementProverStoreSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FleetManagementProverStoreSet)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FleetManagementProverStoreSet)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FleetManagementProverStoreSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FleetManagementProverStoreSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FleetManagementProverStoreSet represents a ProverStoreSet event raised by the FleetManagement contract.
type FleetManagementProverStoreSet struct {
	Prover common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterProverStoreSet is a free log retrieval operation binding the contract event 0xa548f7571b682f75eb09b6255cee4e7010a2136f2535aa8449ea00ad1483c641.
//
// Solidity: event ProverStoreSet(address indexed prover)
func (_FleetManagement *FleetManagementFilterer) FilterProverStoreSet(opts *bind.FilterOpts, prover []common.Address) (*FleetManagementProverStoreSetIterator, error) {

	var proverRule []interface{}
	for _, proverItem := range prover {
		proverRule = append(proverRule, proverItem)
	}

	logs, sub, err := _FleetManagement.contract.FilterLogs(opts, "ProverStoreSet", proverRule)
	if err != nil {
		return nil, err
	}
	return &FleetManagementProverStoreSetIterator{contract: _FleetManagement.contract, event: "ProverStoreSet", logs: logs, sub: sub}, nil
}

// WatchProverStoreSet is a free log subscription operation binding the contract event 0xa548f7571b682f75eb09b6255cee4e7010a2136f2535aa8449ea00ad1483c641.
//
// Solidity: event ProverStoreSet(address indexed prover)
func (_FleetManagement *FleetManagementFilterer) WatchProverStoreSet(opts *bind.WatchOpts, sink chan<- *FleetManagementProverStoreSet, prover []common.Address) (event.Subscription, error) {

	var proverRule []interface{}
	for _, proverItem := range prover {
		proverRule = append(proverRule, proverItem)
	}

	logs, sub, err := _FleetManagement.contract.WatchLogs(opts, "ProverStoreSet", proverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FleetManagementProverStoreSet)
				if err := _FleetManagement.contract.UnpackLog(event, "ProverStoreSet", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseProverStoreSet is a log parse operation binding the contract event 0xa548f7571b682f75eb09b6255cee4e7010a2136f2535aa8449ea00ad1483c641.
//
// Solidity: event ProverStoreSet(address indexed prover)
func (_FleetManagement *FleetManagementFilterer) ParseProverStoreSet(log types.Log) (*FleetManagementProverStoreSet, error) {
	event := new(FleetManagementProverStoreSet)
	if err := _FleetManagement.contract.UnpackLog(event, "ProverStoreSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FleetManagementRegistrationFeeSetIterator is returned from FilterRegistrationFeeSet and is used to iterate over the raw logs and unpacked data for RegistrationFeeSet events raised by the FleetManagement contract.
type FleetManagementRegistrationFeeSetIterator struct {
	Event *FleetManagementRegistrationFeeSet // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FleetManagementRegistrationFeeSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FleetManagementRegistrationFeeSet)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FleetManagementRegistrationFeeSet)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FleetManagementRegistrationFeeSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FleetManagementRegistrationFeeSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FleetManagementRegistrationFeeSet represents a RegistrationFeeSet event raised by the FleetManagement contract.
type FleetManagementRegistrationFeeSet struct {
	Fee *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterRegistrationFeeSet is a free log retrieval operation binding the contract event 0x3836764761eb705a9815e9070541f9a604757da847d45d157a28f14b58b5f5e6.
//
// Solidity: event RegistrationFeeSet(uint256 fee)
func (_FleetManagement *FleetManagementFilterer) FilterRegistrationFeeSet(opts *bind.FilterOpts) (*FleetManagementRegistrationFeeSetIterator, error) {

	logs, sub, err := _FleetManagement.contract.FilterLogs(opts, "RegistrationFeeSet")
	if err != nil {
		return nil, err
	}
	return &FleetManagementRegistrationFeeSetIterator{contract: _FleetManagement.contract, event: "RegistrationFeeSet", logs: logs, sub: sub}, nil
}

// WatchRegistrationFeeSet is a free log subscription operation binding the contract event 0x3836764761eb705a9815e9070541f9a604757da847d45d157a28f14b58b5f5e6.
//
// Solidity: event RegistrationFeeSet(uint256 fee)
func (_FleetManagement *FleetManagementFilterer) WatchRegistrationFeeSet(opts *bind.WatchOpts, sink chan<- *FleetManagementRegistrationFeeSet) (event.Subscription, error) {

	logs, sub, err := _FleetManagement.contract.WatchLogs(opts, "RegistrationFeeSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FleetManagementRegistrationFeeSet)
				if err := _FleetManagement.contract.UnpackLog(event, "RegistrationFeeSet", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRegistrationFeeSet is a log parse operation binding the contract event 0x3836764761eb705a9815e9070541f9a604757da847d45d157a28f14b58b5f5e6.
//
// Solidity: event RegistrationFeeSet(uint256 fee)
func (_FleetManagement *FleetManagementFilterer) ParseRegistrationFeeSet(log types.Log) (*FleetManagementRegistrationFeeSet, error) {
	event := new(FleetManagementRegistrationFeeSet)
	if err := _FleetManagement.contract.UnpackLog(event, "RegistrationFeeSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FleetManagementSlasherSetIterator is returned from FilterSlasherSet and is used to iterate over the raw logs and unpacked data for SlasherSet events raised by the FleetManagement contract.
type FleetManagementSlasherSetIterator struct {
	Event *FleetManagementSlasherSet // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FleetManagementSlasherSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FleetManagementSlasherSet)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FleetManagementSlasherSet)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FleetManagementSlasherSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FleetManagementSlasherSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FleetManagementSlasherSet represents a SlasherSet event raised by the FleetManagement contract.
type FleetManagementSlasherSet struct {
	Slasher common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterSlasherSet is a free log retrieval operation binding the contract event 0x93984378289228ac490c326f8dbcc3f9b2c703753c272e7c572949e115da985e.
//
// Solidity: event SlasherSet(address indexed slasher)
func (_FleetManagement *FleetManagementFilterer) FilterSlasherSet(opts *bind.FilterOpts, slasher []common.Address) (*FleetManagementSlasherSetIterator, error) {

	var slasherRule []interface{}
	for _, slasherItem := range slasher {
		slasherRule = append(slasherRule, slasherItem)
	}

	logs, sub, err := _FleetManagement.contract.FilterLogs(opts, "SlasherSet", slasherRule)
	if err != nil {
		return nil, err
	}
	return &FleetManagementSlasherSetIterator{contract: _FleetManagement.contract, event: "SlasherSet", logs: logs, sub: sub}, nil
}

// WatchSlasherSet is a free log subscription operation binding the contract event 0x93984378289228ac490c326f8dbcc3f9b2c703753c272e7c572949e115da985e.
//
// Solidity: event SlasherSet(address indexed slasher)
func (_FleetManagement *FleetManagementFilterer) WatchSlasherSet(opts *bind.WatchOpts, sink chan<- *FleetManagementSlasherSet, slasher []common.Address) (event.Subscription, error) {

	var slasherRule []interface{}
	for _, slasherItem := range slasher {
		slasherRule = append(slasherRule, slasherItem)
	}

	logs, sub, err := _FleetManagement.contract.WatchLogs(opts, "SlasherSet", slasherRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FleetManagementSlasherSet)
				if err := _FleetManagement.contract.UnpackLog(event, "SlasherSet", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSlasherSet is a log parse operation binding the contract event 0x93984378289228ac490c326f8dbcc3f9b2c703753c272e7c572949e115da985e.
//
// Solidity: event SlasherSet(address indexed slasher)
func (_FleetManagement *FleetManagementFilterer) ParseSlasherSet(log types.Log) (*FleetManagementSlasherSet, error) {
	event := new(FleetManagementSlasherSet)
	if err := _FleetManagement.contract.UnpackLog(event, "SlasherSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FleetManagementStakingHubSetIterator is returned from FilterStakingHubSet and is used to iterate over the raw logs and unpacked data for StakingHubSet events raised by the FleetManagement contract.
type FleetManagementStakingHubSetIterator struct {
	Event *FleetManagementStakingHubSet // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FleetManagementStakingHubSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FleetManagementStakingHubSet)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FleetManagementStakingHubSet)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FleetManagementStakingHubSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FleetManagementStakingHubSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FleetManagementStakingHubSet represents a StakingHubSet event raised by the FleetManagement contract.
type FleetManagementStakingHubSet struct {
	Msp common.Address
	Raw types.Log // Blockchain specific contextual infos
}

// FilterStakingHubSet is a free log retrieval operation binding the contract event 0x4f6353da0125ceacadbf12ecb33e3bfbc7f14b1922bf5783daecf3bc7042bf7e.
//
// Solidity: event StakingHubSet(address indexed msp)
func (_FleetManagement *FleetManagementFilterer) FilterStakingHubSet(opts *bind.FilterOpts, msp []common.Address) (*FleetManagementStakingHubSetIterator, error) {

	var mspRule []interface{}
	for _, mspItem := range msp {
		mspRule = append(mspRule, mspItem)
	}

	logs, sub, err := _FleetManagement.contract.FilterLogs(opts, "StakingHubSet", mspRule)
	if err != nil {
		return nil, err
	}
	return &FleetManagementStakingHubSetIterator{contract: _FleetManagement.contract, event: "StakingHubSet", logs: logs, sub: sub}, nil
}

// WatchStakingHubSet is a free log subscription operation binding the contract event 0x4f6353da0125ceacadbf12ecb33e3bfbc7f14b1922bf5783daecf3bc7042bf7e.
//
// Solidity: event StakingHubSet(address indexed msp)
func (_FleetManagement *FleetManagementFilterer) WatchStakingHubSet(opts *bind.WatchOpts, sink chan<- *FleetManagementStakingHubSet, msp []common.Address) (event.Subscription, error) {

	var mspRule []interface{}
	for _, mspItem := range msp {
		mspRule = append(mspRule, mspItem)
	}

	logs, sub, err := _FleetManagement.contract.WatchLogs(opts, "StakingHubSet", mspRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FleetManagementStakingHubSet)
				if err := _FleetManagement.contract.UnpackLog(event, "StakingHubSet", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseStakingHubSet is a log parse operation binding the contract event 0x4f6353da0125ceacadbf12ecb33e3bfbc7f14b1922bf5783daecf3bc7042bf7e.
//
// Solidity: event StakingHubSet(address indexed msp)
func (_FleetManagement *FleetManagementFilterer) ParseStakingHubSet(log types.Log) (*FleetManagementStakingHubSet, error) {
	event := new(FleetManagementStakingHubSet)
	if err := _FleetManagement.contract.UnpackLog(event, "StakingHubSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

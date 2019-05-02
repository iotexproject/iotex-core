// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package detc

import (
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
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = abi.U256
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// DetcGenABI is the input ABI used to generate the binding from.
const DetcGenABI = "[{\"constant\":false,\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"isOwner\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"author\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"diffId\",\"type\":\"uint256\"}],\"name\":\"DiffCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"author\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"diffId\",\"type\":\"uint256\"}],\"name\":\"DiffMerged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"reviewer\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"diffId\",\"type\":\"uint256\"}],\"name\":\"DiffApproved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"author\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"diffId\",\"type\":\"uint256\"}],\"name\":\"DiffAbandoned\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"quorum\",\"type\":\"uint256\"}],\"name\":\"QuorumUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"reviewer\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"weight\",\"type\":\"uint256\"}],\"name\":\"ReviewerCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"reviewer\",\"type\":\"address\"}],\"name\":\"ReviewerDeleted\",\"type\":\"event\"},{\"constant\":true,\"inputs\":[{\"name\":\"_key\",\"type\":\"string\"}],\"name\":\"getConfig\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes\"},{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_diffId\",\"type\":\"uint256\"}],\"name\":\"getDiff\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"},{\"name\":\"\",\"type\":\"bytes\"},{\"name\":\"\",\"type\":\"uint64\"},{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"address[]\"},{\"name\":\"status\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_key\",\"type\":\"string\"},{\"name\":\"_value\",\"type\":\"bytes\"},{\"name\":\"_baseVersion\",\"type\":\"uint64\"}],\"name\":\"proposeDiff\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_diffId\",\"type\":\"uint256\"}],\"name\":\"approveDiff\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_diffId\",\"type\":\"uint256\"}],\"name\":\"mergeDiff\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_diffId\",\"type\":\"uint256\"}],\"name\":\"abandonDiff\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getQuorum\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_reviewer\",\"type\":\"address\"}],\"name\":\"getReviewer\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_reviewer\",\"type\":\"address\"},{\"name\":\"_weight\",\"type\":\"uint256\"}],\"name\":\"addReviewer\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_reviewer\",\"type\":\"address\"}],\"name\":\"removeReviewer\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_quorum\",\"type\":\"uint256\"}],\"name\":\"setQuorum\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// DetcGen is an auto generated Go binding around an Ethereum contract.
type DetcGen struct {
	DetcGenCaller     // Read-only binding to the contract
	DetcGenTransactor // Write-only binding to the contract
	DetcGenFilterer   // Log filterer for contract events
}

// DetcGenCaller is an auto generated read-only Go binding around an Ethereum contract.
type DetcGenCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DetcGenTransactor is an auto generated write-only Go binding around an Ethereum contract.
type DetcGenTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DetcGenFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type DetcGenFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DetcGenSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type DetcGenSession struct {
	Contract     *DetcGen          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// DetcGenCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type DetcGenCallerSession struct {
	Contract *DetcGenCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// DetcGenTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type DetcGenTransactorSession struct {
	Contract     *DetcGenTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// DetcGenRaw is an auto generated low-level Go binding around an Ethereum contract.
type DetcGenRaw struct {
	Contract *DetcGen // Generic contract binding to access the raw methods on
}

// DetcGenCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type DetcGenCallerRaw struct {
	Contract *DetcGenCaller // Generic read-only contract binding to access the raw methods on
}

// DetcGenTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type DetcGenTransactorRaw struct {
	Contract *DetcGenTransactor // Generic write-only contract binding to access the raw methods on
}

// NewDetcGen creates a new instance of DetcGen, bound to a specific deployed contract.
func NewDetcGen(address common.Address, backend bind.ContractBackend) (*DetcGen, error) {
	contract, err := bindDetcGen(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &DetcGen{DetcGenCaller: DetcGenCaller{contract: contract}, DetcGenTransactor: DetcGenTransactor{contract: contract}, DetcGenFilterer: DetcGenFilterer{contract: contract}}, nil
}

// NewDetcGenCaller creates a new read-only instance of DetcGen, bound to a specific deployed contract.
func NewDetcGenCaller(address common.Address, caller bind.ContractCaller) (*DetcGenCaller, error) {
	contract, err := bindDetcGen(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &DetcGenCaller{contract: contract}, nil
}

// NewDetcGenTransactor creates a new write-only instance of DetcGen, bound to a specific deployed contract.
func NewDetcGenTransactor(address common.Address, transactor bind.ContractTransactor) (*DetcGenTransactor, error) {
	contract, err := bindDetcGen(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &DetcGenTransactor{contract: contract}, nil
}

// NewDetcGenFilterer creates a new log filterer instance of DetcGen, bound to a specific deployed contract.
func NewDetcGenFilterer(address common.Address, filterer bind.ContractFilterer) (*DetcGenFilterer, error) {
	contract, err := bindDetcGen(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &DetcGenFilterer{contract: contract}, nil
}

// bindDetcGen binds a generic wrapper to an already deployed contract.
func bindDetcGen(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(DetcGenABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_DetcGen *DetcGenRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _DetcGen.Contract.DetcGenCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_DetcGen *DetcGenRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DetcGen.Contract.DetcGenTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_DetcGen *DetcGenRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _DetcGen.Contract.DetcGenTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_DetcGen *DetcGenCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _DetcGen.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_DetcGen *DetcGenTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DetcGen.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_DetcGen *DetcGenTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _DetcGen.Contract.contract.Transact(opts, method, params...)
}

// GetConfig is a free data retrieval call binding the contract method 0xb44bd51d.
//
// Solidity: function getConfig(string _key) constant returns(bytes, uint256)
func (_DetcGen *DetcGenCaller) GetConfig(opts *bind.CallOpts, _key string) ([]byte, *big.Int, error) {
	var (
		ret0 = new([]byte)
		ret1 = new(*big.Int)
	)
	out := &[]interface{}{
		ret0,
		ret1,
	}
	err := _DetcGen.contract.Call(opts, out, "getConfig", _key)
	return *ret0, *ret1, err
}

// GetConfig is a free data retrieval call binding the contract method 0xb44bd51d.
//
// Solidity: function getConfig(string _key) constant returns(bytes, uint256)
func (_DetcGen *DetcGenSession) GetConfig(_key string) ([]byte, *big.Int, error) {
	return _DetcGen.Contract.GetConfig(&_DetcGen.CallOpts, _key)
}

// GetConfig is a free data retrieval call binding the contract method 0xb44bd51d.
//
// Solidity: function getConfig(string _key) constant returns(bytes, uint256)
func (_DetcGen *DetcGenCallerSession) GetConfig(_key string) ([]byte, *big.Int, error) {
	return _DetcGen.Contract.GetConfig(&_DetcGen.CallOpts, _key)
}

// GetDiff is a free data retrieval call binding the contract method 0xff76b13c.
//
// Solidity: function getDiff(uint256 _diffId) constant returns(string, bytes, uint64, address, address[], uint8 status)
func (_DetcGen *DetcGenCaller) GetDiff(opts *bind.CallOpts, _diffId *big.Int) (string, []byte, uint64, common.Address, []common.Address, uint8, error) {
	var (
		ret0 = new(string)
		ret1 = new([]byte)
		ret2 = new(uint64)
		ret3 = new(common.Address)
		ret4 = new([]common.Address)
		ret5 = new(uint8)
	)
	out := &[]interface{}{
		ret0,
		ret1,
		ret2,
		ret3,
		ret4,
		ret5,
	}
	err := _DetcGen.contract.Call(opts, out, "getDiff", _diffId)
	return *ret0, *ret1, *ret2, *ret3, *ret4, *ret5, err
}

// GetDiff is a free data retrieval call binding the contract method 0xff76b13c.
//
// Solidity: function getDiff(uint256 _diffId) constant returns(string, bytes, uint64, address, address[], uint8 status)
func (_DetcGen *DetcGenSession) GetDiff(_diffId *big.Int) (string, []byte, uint64, common.Address, []common.Address, uint8, error) {
	return _DetcGen.Contract.GetDiff(&_DetcGen.CallOpts, _diffId)
}

// GetDiff is a free data retrieval call binding the contract method 0xff76b13c.
//
// Solidity: function getDiff(uint256 _diffId) constant returns(string, bytes, uint64, address, address[], uint8 status)
func (_DetcGen *DetcGenCallerSession) GetDiff(_diffId *big.Int) (string, []byte, uint64, common.Address, []common.Address, uint8, error) {
	return _DetcGen.Contract.GetDiff(&_DetcGen.CallOpts, _diffId)
}

// GetQuorum is a free data retrieval call binding the contract method 0xc26c12eb.
//
// Solidity: function getQuorum() constant returns(uint256)
func (_DetcGen *DetcGenCaller) GetQuorum(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _DetcGen.contract.Call(opts, out, "getQuorum")
	return *ret0, err
}

// GetQuorum is a free data retrieval call binding the contract method 0xc26c12eb.
//
// Solidity: function getQuorum() constant returns(uint256)
func (_DetcGen *DetcGenSession) GetQuorum() (*big.Int, error) {
	return _DetcGen.Contract.GetQuorum(&_DetcGen.CallOpts)
}

// GetQuorum is a free data retrieval call binding the contract method 0xc26c12eb.
//
// Solidity: function getQuorum() constant returns(uint256)
func (_DetcGen *DetcGenCallerSession) GetQuorum() (*big.Int, error) {
	return _DetcGen.Contract.GetQuorum(&_DetcGen.CallOpts)
}

// GetReviewer is a free data retrieval call binding the contract method 0x8979d901.
//
// Solidity: function getReviewer(address _reviewer) constant returns(uint256)
func (_DetcGen *DetcGenCaller) GetReviewer(opts *bind.CallOpts, _reviewer common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _DetcGen.contract.Call(opts, out, "getReviewer", _reviewer)
	return *ret0, err
}

// GetReviewer is a free data retrieval call binding the contract method 0x8979d901.
//
// Solidity: function getReviewer(address _reviewer) constant returns(uint256)
func (_DetcGen *DetcGenSession) GetReviewer(_reviewer common.Address) (*big.Int, error) {
	return _DetcGen.Contract.GetReviewer(&_DetcGen.CallOpts, _reviewer)
}

// GetReviewer is a free data retrieval call binding the contract method 0x8979d901.
//
// Solidity: function getReviewer(address _reviewer) constant returns(uint256)
func (_DetcGen *DetcGenCallerSession) GetReviewer(_reviewer common.Address) (*big.Int, error) {
	return _DetcGen.Contract.GetReviewer(&_DetcGen.CallOpts, _reviewer)
}

// IsOwner is a free data retrieval call binding the contract method 0x8f32d59b.
//
// Solidity: function isOwner() constant returns(bool)
func (_DetcGen *DetcGenCaller) IsOwner(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _DetcGen.contract.Call(opts, out, "isOwner")
	return *ret0, err
}

// IsOwner is a free data retrieval call binding the contract method 0x8f32d59b.
//
// Solidity: function isOwner() constant returns(bool)
func (_DetcGen *DetcGenSession) IsOwner() (bool, error) {
	return _DetcGen.Contract.IsOwner(&_DetcGen.CallOpts)
}

// IsOwner is a free data retrieval call binding the contract method 0x8f32d59b.
//
// Solidity: function isOwner() constant returns(bool)
func (_DetcGen *DetcGenCallerSession) IsOwner() (bool, error) {
	return _DetcGen.Contract.IsOwner(&_DetcGen.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_DetcGen *DetcGenCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _DetcGen.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_DetcGen *DetcGenSession) Owner() (common.Address, error) {
	return _DetcGen.Contract.Owner(&_DetcGen.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_DetcGen *DetcGenCallerSession) Owner() (common.Address, error) {
	return _DetcGen.Contract.Owner(&_DetcGen.CallOpts)
}

// AbandonDiff is a paid mutator transaction binding the contract method 0xa1ee7b55.
//
// Solidity: function abandonDiff(uint256 _diffId) returns()
func (_DetcGen *DetcGenTransactor) AbandonDiff(opts *bind.TransactOpts, _diffId *big.Int) (*types.Transaction, error) {
	return _DetcGen.contract.Transact(opts, "abandonDiff", _diffId)
}

// AbandonDiff is a paid mutator transaction binding the contract method 0xa1ee7b55.
//
// Solidity: function abandonDiff(uint256 _diffId) returns()
func (_DetcGen *DetcGenSession) AbandonDiff(_diffId *big.Int) (*types.Transaction, error) {
	return _DetcGen.Contract.AbandonDiff(&_DetcGen.TransactOpts, _diffId)
}

// AbandonDiff is a paid mutator transaction binding the contract method 0xa1ee7b55.
//
// Solidity: function abandonDiff(uint256 _diffId) returns()
func (_DetcGen *DetcGenTransactorSession) AbandonDiff(_diffId *big.Int) (*types.Transaction, error) {
	return _DetcGen.Contract.AbandonDiff(&_DetcGen.TransactOpts, _diffId)
}

// AddReviewer is a paid mutator transaction binding the contract method 0x81fecb4f.
//
// Solidity: function addReviewer(address _reviewer, uint256 _weight) returns()
func (_DetcGen *DetcGenTransactor) AddReviewer(opts *bind.TransactOpts, _reviewer common.Address, _weight *big.Int) (*types.Transaction, error) {
	return _DetcGen.contract.Transact(opts, "addReviewer", _reviewer, _weight)
}

// AddReviewer is a paid mutator transaction binding the contract method 0x81fecb4f.
//
// Solidity: function addReviewer(address _reviewer, uint256 _weight) returns()
func (_DetcGen *DetcGenSession) AddReviewer(_reviewer common.Address, _weight *big.Int) (*types.Transaction, error) {
	return _DetcGen.Contract.AddReviewer(&_DetcGen.TransactOpts, _reviewer, _weight)
}

// AddReviewer is a paid mutator transaction binding the contract method 0x81fecb4f.
//
// Solidity: function addReviewer(address _reviewer, uint256 _weight) returns()
func (_DetcGen *DetcGenTransactorSession) AddReviewer(_reviewer common.Address, _weight *big.Int) (*types.Transaction, error) {
	return _DetcGen.Contract.AddReviewer(&_DetcGen.TransactOpts, _reviewer, _weight)
}

// ApproveDiff is a paid mutator transaction binding the contract method 0xa5c96586.
//
// Solidity: function approveDiff(uint256 _diffId) returns()
func (_DetcGen *DetcGenTransactor) ApproveDiff(opts *bind.TransactOpts, _diffId *big.Int) (*types.Transaction, error) {
	return _DetcGen.contract.Transact(opts, "approveDiff", _diffId)
}

// ApproveDiff is a paid mutator transaction binding the contract method 0xa5c96586.
//
// Solidity: function approveDiff(uint256 _diffId) returns()
func (_DetcGen *DetcGenSession) ApproveDiff(_diffId *big.Int) (*types.Transaction, error) {
	return _DetcGen.Contract.ApproveDiff(&_DetcGen.TransactOpts, _diffId)
}

// ApproveDiff is a paid mutator transaction binding the contract method 0xa5c96586.
//
// Solidity: function approveDiff(uint256 _diffId) returns()
func (_DetcGen *DetcGenTransactorSession) ApproveDiff(_diffId *big.Int) (*types.Transaction, error) {
	return _DetcGen.Contract.ApproveDiff(&_DetcGen.TransactOpts, _diffId)
}

// MergeDiff is a paid mutator transaction binding the contract method 0xb90ba87e.
//
// Solidity: function mergeDiff(uint256 _diffId) returns()
func (_DetcGen *DetcGenTransactor) MergeDiff(opts *bind.TransactOpts, _diffId *big.Int) (*types.Transaction, error) {
	return _DetcGen.contract.Transact(opts, "mergeDiff", _diffId)
}

// MergeDiff is a paid mutator transaction binding the contract method 0xb90ba87e.
//
// Solidity: function mergeDiff(uint256 _diffId) returns()
func (_DetcGen *DetcGenSession) MergeDiff(_diffId *big.Int) (*types.Transaction, error) {
	return _DetcGen.Contract.MergeDiff(&_DetcGen.TransactOpts, _diffId)
}

// MergeDiff is a paid mutator transaction binding the contract method 0xb90ba87e.
//
// Solidity: function mergeDiff(uint256 _diffId) returns()
func (_DetcGen *DetcGenTransactorSession) MergeDiff(_diffId *big.Int) (*types.Transaction, error) {
	return _DetcGen.Contract.MergeDiff(&_DetcGen.TransactOpts, _diffId)
}

// ProposeDiff is a paid mutator transaction binding the contract method 0x8cc2732e.
//
// Solidity: function proposeDiff(string _key, bytes _value, uint64 _baseVersion) returns()
func (_DetcGen *DetcGenTransactor) ProposeDiff(opts *bind.TransactOpts, _key string, _value []byte, _baseVersion uint64) (*types.Transaction, error) {
	return _DetcGen.contract.Transact(opts, "proposeDiff", _key, _value, _baseVersion)
}

// ProposeDiff is a paid mutator transaction binding the contract method 0x8cc2732e.
//
// Solidity: function proposeDiff(string _key, bytes _value, uint64 _baseVersion) returns()
func (_DetcGen *DetcGenSession) ProposeDiff(_key string, _value []byte, _baseVersion uint64) (*types.Transaction, error) {
	return _DetcGen.Contract.ProposeDiff(&_DetcGen.TransactOpts, _key, _value, _baseVersion)
}

// ProposeDiff is a paid mutator transaction binding the contract method 0x8cc2732e.
//
// Solidity: function proposeDiff(string _key, bytes _value, uint64 _baseVersion) returns()
func (_DetcGen *DetcGenTransactorSession) ProposeDiff(_key string, _value []byte, _baseVersion uint64) (*types.Transaction, error) {
	return _DetcGen.Contract.ProposeDiff(&_DetcGen.TransactOpts, _key, _value, _baseVersion)
}

// RemoveReviewer is a paid mutator transaction binding the contract method 0xc9442dac.
//
// Solidity: function removeReviewer(address _reviewer) returns()
func (_DetcGen *DetcGenTransactor) RemoveReviewer(opts *bind.TransactOpts, _reviewer common.Address) (*types.Transaction, error) {
	return _DetcGen.contract.Transact(opts, "removeReviewer", _reviewer)
}

// RemoveReviewer is a paid mutator transaction binding the contract method 0xc9442dac.
//
// Solidity: function removeReviewer(address _reviewer) returns()
func (_DetcGen *DetcGenSession) RemoveReviewer(_reviewer common.Address) (*types.Transaction, error) {
	return _DetcGen.Contract.RemoveReviewer(&_DetcGen.TransactOpts, _reviewer)
}

// RemoveReviewer is a paid mutator transaction binding the contract method 0xc9442dac.
//
// Solidity: function removeReviewer(address _reviewer) returns()
func (_DetcGen *DetcGenTransactorSession) RemoveReviewer(_reviewer common.Address) (*types.Transaction, error) {
	return _DetcGen.Contract.RemoveReviewer(&_DetcGen.TransactOpts, _reviewer)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_DetcGen *DetcGenTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DetcGen.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_DetcGen *DetcGenSession) RenounceOwnership() (*types.Transaction, error) {
	return _DetcGen.Contract.RenounceOwnership(&_DetcGen.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_DetcGen *DetcGenTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _DetcGen.Contract.RenounceOwnership(&_DetcGen.TransactOpts)
}

// SetQuorum is a paid mutator transaction binding the contract method 0xc1ba4e59.
//
// Solidity: function setQuorum(uint256 _quorum) returns()
func (_DetcGen *DetcGenTransactor) SetQuorum(opts *bind.TransactOpts, _quorum *big.Int) (*types.Transaction, error) {
	return _DetcGen.contract.Transact(opts, "setQuorum", _quorum)
}

// SetQuorum is a paid mutator transaction binding the contract method 0xc1ba4e59.
//
// Solidity: function setQuorum(uint256 _quorum) returns()
func (_DetcGen *DetcGenSession) SetQuorum(_quorum *big.Int) (*types.Transaction, error) {
	return _DetcGen.Contract.SetQuorum(&_DetcGen.TransactOpts, _quorum)
}

// SetQuorum is a paid mutator transaction binding the contract method 0xc1ba4e59.
//
// Solidity: function setQuorum(uint256 _quorum) returns()
func (_DetcGen *DetcGenTransactorSession) SetQuorum(_quorum *big.Int) (*types.Transaction, error) {
	return _DetcGen.Contract.SetQuorum(&_DetcGen.TransactOpts, _quorum)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_DetcGen *DetcGenTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _DetcGen.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_DetcGen *DetcGenSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _DetcGen.Contract.TransferOwnership(&_DetcGen.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_DetcGen *DetcGenTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _DetcGen.Contract.TransferOwnership(&_DetcGen.TransactOpts, newOwner)
}

// DetcGenDiffAbandonedIterator is returned from FilterDiffAbandoned and is used to iterate over the raw logs and unpacked data for DiffAbandoned events raised by the DetcGen contract.
type DetcGenDiffAbandonedIterator struct {
	Event *DetcGenDiffAbandoned // Event containing the contract specifics and raw log

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
func (it *DetcGenDiffAbandonedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DetcGenDiffAbandoned)
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
		it.Event = new(DetcGenDiffAbandoned)
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
func (it *DetcGenDiffAbandonedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DetcGenDiffAbandonedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DetcGenDiffAbandoned represents a DiffAbandoned event raised by the DetcGen contract.
type DetcGenDiffAbandoned struct {
	Author common.Address
	DiffId *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterDiffAbandoned is a free log retrieval operation binding the contract event 0x770fad293206b895131c6ab1912ac36e03aadbb2a5b168668be4d2b0d5bb19ff.
//
// Solidity: event DiffAbandoned(address author, uint256 diffId)
func (_DetcGen *DetcGenFilterer) FilterDiffAbandoned(opts *bind.FilterOpts) (*DetcGenDiffAbandonedIterator, error) {

	logs, sub, err := _DetcGen.contract.FilterLogs(opts, "DiffAbandoned")
	if err != nil {
		return nil, err
	}
	return &DetcGenDiffAbandonedIterator{contract: _DetcGen.contract, event: "DiffAbandoned", logs: logs, sub: sub}, nil
}

// WatchDiffAbandoned is a free log subscription operation binding the contract event 0x770fad293206b895131c6ab1912ac36e03aadbb2a5b168668be4d2b0d5bb19ff.
//
// Solidity: event DiffAbandoned(address author, uint256 diffId)
func (_DetcGen *DetcGenFilterer) WatchDiffAbandoned(opts *bind.WatchOpts, sink chan<- *DetcGenDiffAbandoned) (event.Subscription, error) {

	logs, sub, err := _DetcGen.contract.WatchLogs(opts, "DiffAbandoned")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DetcGenDiffAbandoned)
				if err := _DetcGen.contract.UnpackLog(event, "DiffAbandoned", log); err != nil {
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

// DetcGenDiffApprovedIterator is returned from FilterDiffApproved and is used to iterate over the raw logs and unpacked data for DiffApproved events raised by the DetcGen contract.
type DetcGenDiffApprovedIterator struct {
	Event *DetcGenDiffApproved // Event containing the contract specifics and raw log

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
func (it *DetcGenDiffApprovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DetcGenDiffApproved)
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
		it.Event = new(DetcGenDiffApproved)
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
func (it *DetcGenDiffApprovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DetcGenDiffApprovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DetcGenDiffApproved represents a DiffApproved event raised by the DetcGen contract.
type DetcGenDiffApproved struct {
	Reviewer common.Address
	DiffId   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterDiffApproved is a free log retrieval operation binding the contract event 0x53a28d15ce16b127f1805a9b7d3784c39db5967f2317e38cd804c9d87e1f0144.
//
// Solidity: event DiffApproved(address reviewer, uint256 diffId)
func (_DetcGen *DetcGenFilterer) FilterDiffApproved(opts *bind.FilterOpts) (*DetcGenDiffApprovedIterator, error) {

	logs, sub, err := _DetcGen.contract.FilterLogs(opts, "DiffApproved")
	if err != nil {
		return nil, err
	}
	return &DetcGenDiffApprovedIterator{contract: _DetcGen.contract, event: "DiffApproved", logs: logs, sub: sub}, nil
}

// WatchDiffApproved is a free log subscription operation binding the contract event 0x53a28d15ce16b127f1805a9b7d3784c39db5967f2317e38cd804c9d87e1f0144.
//
// Solidity: event DiffApproved(address reviewer, uint256 diffId)
func (_DetcGen *DetcGenFilterer) WatchDiffApproved(opts *bind.WatchOpts, sink chan<- *DetcGenDiffApproved) (event.Subscription, error) {

	logs, sub, err := _DetcGen.contract.WatchLogs(opts, "DiffApproved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DetcGenDiffApproved)
				if err := _DetcGen.contract.UnpackLog(event, "DiffApproved", log); err != nil {
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

// DetcGenDiffCreatedIterator is returned from FilterDiffCreated and is used to iterate over the raw logs and unpacked data for DiffCreated events raised by the DetcGen contract.
type DetcGenDiffCreatedIterator struct {
	Event *DetcGenDiffCreated // Event containing the contract specifics and raw log

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
func (it *DetcGenDiffCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DetcGenDiffCreated)
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
		it.Event = new(DetcGenDiffCreated)
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
func (it *DetcGenDiffCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DetcGenDiffCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DetcGenDiffCreated represents a DiffCreated event raised by the DetcGen contract.
type DetcGenDiffCreated struct {
	Author common.Address
	DiffId *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterDiffCreated is a free log retrieval operation binding the contract event 0xf1b140fca7f636627683b2e70a3fd2b0ba3eb1fa9efba802694b5d53fbb42708.
//
// Solidity: event DiffCreated(address author, uint256 diffId)
func (_DetcGen *DetcGenFilterer) FilterDiffCreated(opts *bind.FilterOpts) (*DetcGenDiffCreatedIterator, error) {

	logs, sub, err := _DetcGen.contract.FilterLogs(opts, "DiffCreated")
	if err != nil {
		return nil, err
	}
	return &DetcGenDiffCreatedIterator{contract: _DetcGen.contract, event: "DiffCreated", logs: logs, sub: sub}, nil
}

// WatchDiffCreated is a free log subscription operation binding the contract event 0xf1b140fca7f636627683b2e70a3fd2b0ba3eb1fa9efba802694b5d53fbb42708.
//
// Solidity: event DiffCreated(address author, uint256 diffId)
func (_DetcGen *DetcGenFilterer) WatchDiffCreated(opts *bind.WatchOpts, sink chan<- *DetcGenDiffCreated) (event.Subscription, error) {

	logs, sub, err := _DetcGen.contract.WatchLogs(opts, "DiffCreated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DetcGenDiffCreated)
				if err := _DetcGen.contract.UnpackLog(event, "DiffCreated", log); err != nil {
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

// DetcGenDiffMergedIterator is returned from FilterDiffMerged and is used to iterate over the raw logs and unpacked data for DiffMerged events raised by the DetcGen contract.
type DetcGenDiffMergedIterator struct {
	Event *DetcGenDiffMerged // Event containing the contract specifics and raw log

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
func (it *DetcGenDiffMergedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DetcGenDiffMerged)
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
		it.Event = new(DetcGenDiffMerged)
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
func (it *DetcGenDiffMergedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DetcGenDiffMergedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DetcGenDiffMerged represents a DiffMerged event raised by the DetcGen contract.
type DetcGenDiffMerged struct {
	Author common.Address
	DiffId *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterDiffMerged is a free log retrieval operation binding the contract event 0x0155513f03b2096c9a2707c60ca2123fac686c9330c0545daeb8e6e9ab842be0.
//
// Solidity: event DiffMerged(address author, uint256 diffId)
func (_DetcGen *DetcGenFilterer) FilterDiffMerged(opts *bind.FilterOpts) (*DetcGenDiffMergedIterator, error) {

	logs, sub, err := _DetcGen.contract.FilterLogs(opts, "DiffMerged")
	if err != nil {
		return nil, err
	}
	return &DetcGenDiffMergedIterator{contract: _DetcGen.contract, event: "DiffMerged", logs: logs, sub: sub}, nil
}

// WatchDiffMerged is a free log subscription operation binding the contract event 0x0155513f03b2096c9a2707c60ca2123fac686c9330c0545daeb8e6e9ab842be0.
//
// Solidity: event DiffMerged(address author, uint256 diffId)
func (_DetcGen *DetcGenFilterer) WatchDiffMerged(opts *bind.WatchOpts, sink chan<- *DetcGenDiffMerged) (event.Subscription, error) {

	logs, sub, err := _DetcGen.contract.WatchLogs(opts, "DiffMerged")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DetcGenDiffMerged)
				if err := _DetcGen.contract.UnpackLog(event, "DiffMerged", log); err != nil {
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

// DetcGenOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the DetcGen contract.
type DetcGenOwnershipTransferredIterator struct {
	Event *DetcGenOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *DetcGenOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DetcGenOwnershipTransferred)
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
		it.Event = new(DetcGenOwnershipTransferred)
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
func (it *DetcGenOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DetcGenOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DetcGenOwnershipTransferred represents a OwnershipTransferred event raised by the DetcGen contract.
type DetcGenOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_DetcGen *DetcGenFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*DetcGenOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _DetcGen.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &DetcGenOwnershipTransferredIterator{contract: _DetcGen.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_DetcGen *DetcGenFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *DetcGenOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _DetcGen.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DetcGenOwnershipTransferred)
				if err := _DetcGen.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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

// DetcGenQuorumUpdatedIterator is returned from FilterQuorumUpdated and is used to iterate over the raw logs and unpacked data for QuorumUpdated events raised by the DetcGen contract.
type DetcGenQuorumUpdatedIterator struct {
	Event *DetcGenQuorumUpdated // Event containing the contract specifics and raw log

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
func (it *DetcGenQuorumUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DetcGenQuorumUpdated)
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
		it.Event = new(DetcGenQuorumUpdated)
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
func (it *DetcGenQuorumUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DetcGenQuorumUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DetcGenQuorumUpdated represents a QuorumUpdated event raised by the DetcGen contract.
type DetcGenQuorumUpdated struct {
	Quorum *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterQuorumUpdated is a free log retrieval operation binding the contract event 0xf18f88786aae85a652aadb99a82462616489a33370c9bcc7b245906812ef7cd1.
//
// Solidity: event QuorumUpdated(uint256 quorum)
func (_DetcGen *DetcGenFilterer) FilterQuorumUpdated(opts *bind.FilterOpts) (*DetcGenQuorumUpdatedIterator, error) {

	logs, sub, err := _DetcGen.contract.FilterLogs(opts, "QuorumUpdated")
	if err != nil {
		return nil, err
	}
	return &DetcGenQuorumUpdatedIterator{contract: _DetcGen.contract, event: "QuorumUpdated", logs: logs, sub: sub}, nil
}

// WatchQuorumUpdated is a free log subscription operation binding the contract event 0xf18f88786aae85a652aadb99a82462616489a33370c9bcc7b245906812ef7cd1.
//
// Solidity: event QuorumUpdated(uint256 quorum)
func (_DetcGen *DetcGenFilterer) WatchQuorumUpdated(opts *bind.WatchOpts, sink chan<- *DetcGenQuorumUpdated) (event.Subscription, error) {

	logs, sub, err := _DetcGen.contract.WatchLogs(opts, "QuorumUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DetcGenQuorumUpdated)
				if err := _DetcGen.contract.UnpackLog(event, "QuorumUpdated", log); err != nil {
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

// DetcGenReviewerCreatedIterator is returned from FilterReviewerCreated and is used to iterate over the raw logs and unpacked data for ReviewerCreated events raised by the DetcGen contract.
type DetcGenReviewerCreatedIterator struct {
	Event *DetcGenReviewerCreated // Event containing the contract specifics and raw log

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
func (it *DetcGenReviewerCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DetcGenReviewerCreated)
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
		it.Event = new(DetcGenReviewerCreated)
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
func (it *DetcGenReviewerCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DetcGenReviewerCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DetcGenReviewerCreated represents a ReviewerCreated event raised by the DetcGen contract.
type DetcGenReviewerCreated struct {
	Reviewer common.Address
	Weight   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterReviewerCreated is a free log retrieval operation binding the contract event 0xfbb1f16d39b0b5e5ec47018150a5362c0eeec1b4045285b3ce568ac74281c52f.
//
// Solidity: event ReviewerCreated(address reviewer, uint256 weight)
func (_DetcGen *DetcGenFilterer) FilterReviewerCreated(opts *bind.FilterOpts) (*DetcGenReviewerCreatedIterator, error) {

	logs, sub, err := _DetcGen.contract.FilterLogs(opts, "ReviewerCreated")
	if err != nil {
		return nil, err
	}
	return &DetcGenReviewerCreatedIterator{contract: _DetcGen.contract, event: "ReviewerCreated", logs: logs, sub: sub}, nil
}

// WatchReviewerCreated is a free log subscription operation binding the contract event 0xfbb1f16d39b0b5e5ec47018150a5362c0eeec1b4045285b3ce568ac74281c52f.
//
// Solidity: event ReviewerCreated(address reviewer, uint256 weight)
func (_DetcGen *DetcGenFilterer) WatchReviewerCreated(opts *bind.WatchOpts, sink chan<- *DetcGenReviewerCreated) (event.Subscription, error) {

	logs, sub, err := _DetcGen.contract.WatchLogs(opts, "ReviewerCreated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DetcGenReviewerCreated)
				if err := _DetcGen.contract.UnpackLog(event, "ReviewerCreated", log); err != nil {
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

// DetcGenReviewerDeletedIterator is returned from FilterReviewerDeleted and is used to iterate over the raw logs and unpacked data for ReviewerDeleted events raised by the DetcGen contract.
type DetcGenReviewerDeletedIterator struct {
	Event *DetcGenReviewerDeleted // Event containing the contract specifics and raw log

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
func (it *DetcGenReviewerDeletedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DetcGenReviewerDeleted)
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
		it.Event = new(DetcGenReviewerDeleted)
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
func (it *DetcGenReviewerDeletedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DetcGenReviewerDeletedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DetcGenReviewerDeleted represents a ReviewerDeleted event raised by the DetcGen contract.
type DetcGenReviewerDeleted struct {
	Reviewer common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterReviewerDeleted is a free log retrieval operation binding the contract event 0xd7dd76966e5ba2d3ac07925672c045eb49d6af0fbeb8a94083fe2af3597f3e5d.
//
// Solidity: event ReviewerDeleted(address reviewer)
func (_DetcGen *DetcGenFilterer) FilterReviewerDeleted(opts *bind.FilterOpts) (*DetcGenReviewerDeletedIterator, error) {

	logs, sub, err := _DetcGen.contract.FilterLogs(opts, "ReviewerDeleted")
	if err != nil {
		return nil, err
	}
	return &DetcGenReviewerDeletedIterator{contract: _DetcGen.contract, event: "ReviewerDeleted", logs: logs, sub: sub}, nil
}

// WatchReviewerDeleted is a free log subscription operation binding the contract event 0xd7dd76966e5ba2d3ac07925672c045eb49d6af0fbeb8a94083fe2af3597f3e5d.
//
// Solidity: event ReviewerDeleted(address reviewer)
func (_DetcGen *DetcGenFilterer) WatchReviewerDeleted(opts *bind.WatchOpts, sink chan<- *DetcGenReviewerDeleted) (event.Subscription, error) {

	logs, sub, err := _DetcGen.contract.WatchLogs(opts, "ReviewerDeleted")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DetcGenReviewerDeleted)
				if err := _DetcGen.contract.UnpackLog(event, "ReviewerDeleted", log); err != nil {
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

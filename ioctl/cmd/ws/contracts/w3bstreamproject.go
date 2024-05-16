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

// W3bstreamProjectProjectConfig is an auto generated low-level Go binding around an user-defined struct.
type W3bstreamProjectProjectConfig struct {
	Uri  string
	Hash [32]byte
}

// W3bstreamProjectMetaData contains all meta data concerning the W3bstreamProject contract.
var W3bstreamProjectMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"key\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"value\",\"type\":\"bytes\"}],\"name\":\"AttributeSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"binder\",\"type\":\"address\"}],\"name\":\"BinderSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"}],\"name\":\"ProjectBinded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"uri\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"hash\",\"type\":\"bytes32\"}],\"name\":\"ProjectConfigUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"}],\"name\":\"ProjectPaused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"}],\"name\":\"ProjectResumed\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"_name\",\"type\":\"bytes32\"}],\"name\":\"attribute\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"attributes\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"bytes32[]\",\"name\":\"_keys\",\"type\":\"bytes32[]\"}],\"name\":\"attributesOf\",\"outputs\":[{\"internalType\":\"bytes[]\",\"name\":\"values_\",\"type\":\"bytes[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"bind\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"binder\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"config\",\"outputs\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"uri\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"hash\",\"type\":\"bytes32\"}],\"internalType\":\"structW3bstreamProject.ProjectConfig\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"count\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_project\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"isPaused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"isValidProject\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"ownerOf\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"pause\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"project\",\"outputs\":[{\"internalType\":\"contractIERC721\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"resume\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"bytes32[]\",\"name\":\"_keys\",\"type\":\"bytes32[]\"},{\"internalType\":\"bytes[]\",\"name\":\"_values\",\"type\":\"bytes[]\"}],\"name\":\"setAttributes\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_binder\",\"type\":\"address\"}],\"name\":\"setBinder\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"_uri\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"_hash\",\"type\":\"bytes32\"}],\"name\":\"updateConfig\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// W3bstreamProjectABI is the input ABI used to generate the binding from.
// Deprecated: Use W3bstreamProjectMetaData.ABI instead.
var W3bstreamProjectABI = W3bstreamProjectMetaData.ABI

// W3bstreamProject is an auto generated Go binding around an Ethereum contract.
type W3bstreamProject struct {
	W3bstreamProjectCaller     // Read-only binding to the contract
	W3bstreamProjectTransactor // Write-only binding to the contract
	W3bstreamProjectFilterer   // Log filterer for contract events
}

// W3bstreamProjectCaller is an auto generated read-only Go binding around an Ethereum contract.
type W3bstreamProjectCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamProjectTransactor is an auto generated write-only Go binding around an Ethereum contract.
type W3bstreamProjectTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamProjectFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type W3bstreamProjectFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamProjectSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type W3bstreamProjectSession struct {
	Contract     *W3bstreamProject // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// W3bstreamProjectCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type W3bstreamProjectCallerSession struct {
	Contract *W3bstreamProjectCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// W3bstreamProjectTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type W3bstreamProjectTransactorSession struct {
	Contract     *W3bstreamProjectTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// W3bstreamProjectRaw is an auto generated low-level Go binding around an Ethereum contract.
type W3bstreamProjectRaw struct {
	Contract *W3bstreamProject // Generic contract binding to access the raw methods on
}

// W3bstreamProjectCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type W3bstreamProjectCallerRaw struct {
	Contract *W3bstreamProjectCaller // Generic read-only contract binding to access the raw methods on
}

// W3bstreamProjectTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type W3bstreamProjectTransactorRaw struct {
	Contract *W3bstreamProjectTransactor // Generic write-only contract binding to access the raw methods on
}

// NewW3bstreamProject creates a new instance of W3bstreamProject, bound to a specific deployed contract.
func NewW3bstreamProject(address common.Address, backend bind.ContractBackend) (*W3bstreamProject, error) {
	contract, err := bindW3bstreamProject(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProject{W3bstreamProjectCaller: W3bstreamProjectCaller{contract: contract}, W3bstreamProjectTransactor: W3bstreamProjectTransactor{contract: contract}, W3bstreamProjectFilterer: W3bstreamProjectFilterer{contract: contract}}, nil
}

// NewW3bstreamProjectCaller creates a new read-only instance of W3bstreamProject, bound to a specific deployed contract.
func NewW3bstreamProjectCaller(address common.Address, caller bind.ContractCaller) (*W3bstreamProjectCaller, error) {
	contract, err := bindW3bstreamProject(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectCaller{contract: contract}, nil
}

// NewW3bstreamProjectTransactor creates a new write-only instance of W3bstreamProject, bound to a specific deployed contract.
func NewW3bstreamProjectTransactor(address common.Address, transactor bind.ContractTransactor) (*W3bstreamProjectTransactor, error) {
	contract, err := bindW3bstreamProject(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectTransactor{contract: contract}, nil
}

// NewW3bstreamProjectFilterer creates a new log filterer instance of W3bstreamProject, bound to a specific deployed contract.
func NewW3bstreamProjectFilterer(address common.Address, filterer bind.ContractFilterer) (*W3bstreamProjectFilterer, error) {
	contract, err := bindW3bstreamProject(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectFilterer{contract: contract}, nil
}

// bindW3bstreamProject binds a generic wrapper to an already deployed contract.
func bindW3bstreamProject(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := W3bstreamProjectMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_W3bstreamProject *W3bstreamProjectRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _W3bstreamProject.Contract.W3bstreamProjectCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_W3bstreamProject *W3bstreamProjectRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.W3bstreamProjectTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_W3bstreamProject *W3bstreamProjectRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.W3bstreamProjectTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_W3bstreamProject *W3bstreamProjectCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _W3bstreamProject.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_W3bstreamProject *W3bstreamProjectTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_W3bstreamProject *W3bstreamProjectTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.contract.Transact(opts, method, params...)
}

// Attribute is a free data retrieval call binding the contract method 0x40341e4b.
//
// Solidity: function attribute(uint256 _projectId, bytes32 _name) view returns(bytes)
func (_W3bstreamProject *W3bstreamProjectCaller) Attribute(opts *bind.CallOpts, _projectId *big.Int, _name [32]byte) ([]byte, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "attribute", _projectId, _name)

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// Attribute is a free data retrieval call binding the contract method 0x40341e4b.
//
// Solidity: function attribute(uint256 _projectId, bytes32 _name) view returns(bytes)
func (_W3bstreamProject *W3bstreamProjectSession) Attribute(_projectId *big.Int, _name [32]byte) ([]byte, error) {
	return _W3bstreamProject.Contract.Attribute(&_W3bstreamProject.CallOpts, _projectId, _name)
}

// Attribute is a free data retrieval call binding the contract method 0x40341e4b.
//
// Solidity: function attribute(uint256 _projectId, bytes32 _name) view returns(bytes)
func (_W3bstreamProject *W3bstreamProjectCallerSession) Attribute(_projectId *big.Int, _name [32]byte) ([]byte, error) {
	return _W3bstreamProject.Contract.Attribute(&_W3bstreamProject.CallOpts, _projectId, _name)
}

// Attributes is a free data retrieval call binding the contract method 0x73c773db.
//
// Solidity: function attributes(uint256 , bytes32 ) view returns(bytes)
func (_W3bstreamProject *W3bstreamProjectCaller) Attributes(opts *bind.CallOpts, arg0 *big.Int, arg1 [32]byte) ([]byte, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "attributes", arg0, arg1)

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// Attributes is a free data retrieval call binding the contract method 0x73c773db.
//
// Solidity: function attributes(uint256 , bytes32 ) view returns(bytes)
func (_W3bstreamProject *W3bstreamProjectSession) Attributes(arg0 *big.Int, arg1 [32]byte) ([]byte, error) {
	return _W3bstreamProject.Contract.Attributes(&_W3bstreamProject.CallOpts, arg0, arg1)
}

// Attributes is a free data retrieval call binding the contract method 0x73c773db.
//
// Solidity: function attributes(uint256 , bytes32 ) view returns(bytes)
func (_W3bstreamProject *W3bstreamProjectCallerSession) Attributes(arg0 *big.Int, arg1 [32]byte) ([]byte, error) {
	return _W3bstreamProject.Contract.Attributes(&_W3bstreamProject.CallOpts, arg0, arg1)
}

// AttributesOf is a free data retrieval call binding the contract method 0xe76d621c.
//
// Solidity: function attributesOf(uint256 _projectId, bytes32[] _keys) view returns(bytes[] values_)
func (_W3bstreamProject *W3bstreamProjectCaller) AttributesOf(opts *bind.CallOpts, _projectId *big.Int, _keys [][32]byte) ([][]byte, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "attributesOf", _projectId, _keys)

	if err != nil {
		return *new([][]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([][]byte)).(*[][]byte)

	return out0, err

}

// AttributesOf is a free data retrieval call binding the contract method 0xe76d621c.
//
// Solidity: function attributesOf(uint256 _projectId, bytes32[] _keys) view returns(bytes[] values_)
func (_W3bstreamProject *W3bstreamProjectSession) AttributesOf(_projectId *big.Int, _keys [][32]byte) ([][]byte, error) {
	return _W3bstreamProject.Contract.AttributesOf(&_W3bstreamProject.CallOpts, _projectId, _keys)
}

// AttributesOf is a free data retrieval call binding the contract method 0xe76d621c.
//
// Solidity: function attributesOf(uint256 _projectId, bytes32[] _keys) view returns(bytes[] values_)
func (_W3bstreamProject *W3bstreamProjectCallerSession) AttributesOf(_projectId *big.Int, _keys [][32]byte) ([][]byte, error) {
	return _W3bstreamProject.Contract.AttributesOf(&_W3bstreamProject.CallOpts, _projectId, _keys)
}

// Binder is a free data retrieval call binding the contract method 0xacba7b42.
//
// Solidity: function binder() view returns(address)
func (_W3bstreamProject *W3bstreamProjectCaller) Binder(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "binder")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Binder is a free data retrieval call binding the contract method 0xacba7b42.
//
// Solidity: function binder() view returns(address)
func (_W3bstreamProject *W3bstreamProjectSession) Binder() (common.Address, error) {
	return _W3bstreamProject.Contract.Binder(&_W3bstreamProject.CallOpts)
}

// Binder is a free data retrieval call binding the contract method 0xacba7b42.
//
// Solidity: function binder() view returns(address)
func (_W3bstreamProject *W3bstreamProjectCallerSession) Binder() (common.Address, error) {
	return _W3bstreamProject.Contract.Binder(&_W3bstreamProject.CallOpts)
}

// Config is a free data retrieval call binding the contract method 0x84691767.
//
// Solidity: function config(uint256 _projectId) view returns((string,bytes32))
func (_W3bstreamProject *W3bstreamProjectCaller) Config(opts *bind.CallOpts, _projectId *big.Int) (W3bstreamProjectProjectConfig, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "config", _projectId)

	if err != nil {
		return *new(W3bstreamProjectProjectConfig), err
	}

	out0 := *abi.ConvertType(out[0], new(W3bstreamProjectProjectConfig)).(*W3bstreamProjectProjectConfig)

	return out0, err

}

// Config is a free data retrieval call binding the contract method 0x84691767.
//
// Solidity: function config(uint256 _projectId) view returns((string,bytes32))
func (_W3bstreamProject *W3bstreamProjectSession) Config(_projectId *big.Int) (W3bstreamProjectProjectConfig, error) {
	return _W3bstreamProject.Contract.Config(&_W3bstreamProject.CallOpts, _projectId)
}

// Config is a free data retrieval call binding the contract method 0x84691767.
//
// Solidity: function config(uint256 _projectId) view returns((string,bytes32))
func (_W3bstreamProject *W3bstreamProjectCallerSession) Config(_projectId *big.Int) (W3bstreamProjectProjectConfig, error) {
	return _W3bstreamProject.Contract.Config(&_W3bstreamProject.CallOpts, _projectId)
}

// Count is a free data retrieval call binding the contract method 0x06661abd.
//
// Solidity: function count() view returns(uint256)
func (_W3bstreamProject *W3bstreamProjectCaller) Count(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "count")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Count is a free data retrieval call binding the contract method 0x06661abd.
//
// Solidity: function count() view returns(uint256)
func (_W3bstreamProject *W3bstreamProjectSession) Count() (*big.Int, error) {
	return _W3bstreamProject.Contract.Count(&_W3bstreamProject.CallOpts)
}

// Count is a free data retrieval call binding the contract method 0x06661abd.
//
// Solidity: function count() view returns(uint256)
func (_W3bstreamProject *W3bstreamProjectCallerSession) Count() (*big.Int, error) {
	return _W3bstreamProject.Contract.Count(&_W3bstreamProject.CallOpts)
}

// IsPaused is a free data retrieval call binding the contract method 0xbdf2a43c.
//
// Solidity: function isPaused(uint256 _projectId) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectCaller) IsPaused(opts *bind.CallOpts, _projectId *big.Int) (bool, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "isPaused", _projectId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsPaused is a free data retrieval call binding the contract method 0xbdf2a43c.
//
// Solidity: function isPaused(uint256 _projectId) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectSession) IsPaused(_projectId *big.Int) (bool, error) {
	return _W3bstreamProject.Contract.IsPaused(&_W3bstreamProject.CallOpts, _projectId)
}

// IsPaused is a free data retrieval call binding the contract method 0xbdf2a43c.
//
// Solidity: function isPaused(uint256 _projectId) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectCallerSession) IsPaused(_projectId *big.Int) (bool, error) {
	return _W3bstreamProject.Contract.IsPaused(&_W3bstreamProject.CallOpts, _projectId)
}

// IsValidProject is a free data retrieval call binding the contract method 0x2c7caaf1.
//
// Solidity: function isValidProject(uint256 _projectId) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectCaller) IsValidProject(opts *bind.CallOpts, _projectId *big.Int) (bool, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "isValidProject", _projectId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValidProject is a free data retrieval call binding the contract method 0x2c7caaf1.
//
// Solidity: function isValidProject(uint256 _projectId) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectSession) IsValidProject(_projectId *big.Int) (bool, error) {
	return _W3bstreamProject.Contract.IsValidProject(&_W3bstreamProject.CallOpts, _projectId)
}

// IsValidProject is a free data retrieval call binding the contract method 0x2c7caaf1.
//
// Solidity: function isValidProject(uint256 _projectId) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectCallerSession) IsValidProject(_projectId *big.Int) (bool, error) {
	return _W3bstreamProject.Contract.IsValidProject(&_W3bstreamProject.CallOpts, _projectId)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_W3bstreamProject *W3bstreamProjectCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_W3bstreamProject *W3bstreamProjectSession) Owner() (common.Address, error) {
	return _W3bstreamProject.Contract.Owner(&_W3bstreamProject.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_W3bstreamProject *W3bstreamProjectCallerSession) Owner() (common.Address, error) {
	return _W3bstreamProject.Contract.Owner(&_W3bstreamProject.CallOpts)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 _projectId) view returns(address)
func (_W3bstreamProject *W3bstreamProjectCaller) OwnerOf(opts *bind.CallOpts, _projectId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "ownerOf", _projectId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 _projectId) view returns(address)
func (_W3bstreamProject *W3bstreamProjectSession) OwnerOf(_projectId *big.Int) (common.Address, error) {
	return _W3bstreamProject.Contract.OwnerOf(&_W3bstreamProject.CallOpts, _projectId)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 _projectId) view returns(address)
func (_W3bstreamProject *W3bstreamProjectCallerSession) OwnerOf(_projectId *big.Int) (common.Address, error) {
	return _W3bstreamProject.Contract.OwnerOf(&_W3bstreamProject.CallOpts, _projectId)
}

// Project is a free data retrieval call binding the contract method 0xf60ca60d.
//
// Solidity: function project() view returns(address)
func (_W3bstreamProject *W3bstreamProjectCaller) Project(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "project")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Project is a free data retrieval call binding the contract method 0xf60ca60d.
//
// Solidity: function project() view returns(address)
func (_W3bstreamProject *W3bstreamProjectSession) Project() (common.Address, error) {
	return _W3bstreamProject.Contract.Project(&_W3bstreamProject.CallOpts)
}

// Project is a free data retrieval call binding the contract method 0xf60ca60d.
//
// Solidity: function project() view returns(address)
func (_W3bstreamProject *W3bstreamProjectCallerSession) Project() (common.Address, error) {
	return _W3bstreamProject.Contract.Project(&_W3bstreamProject.CallOpts)
}

// Bind is a paid mutator transaction binding the contract method 0x1fb8b00e.
//
// Solidity: function bind(uint256 _projectId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) Bind(opts *bind.TransactOpts, _projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "bind", _projectId)
}

// Bind is a paid mutator transaction binding the contract method 0x1fb8b00e.
//
// Solidity: function bind(uint256 _projectId) returns()
func (_W3bstreamProject *W3bstreamProjectSession) Bind(_projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Bind(&_W3bstreamProject.TransactOpts, _projectId)
}

// Bind is a paid mutator transaction binding the contract method 0x1fb8b00e.
//
// Solidity: function bind(uint256 _projectId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) Bind(_projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Bind(&_W3bstreamProject.TransactOpts, _projectId)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _project) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) Initialize(opts *bind.TransactOpts, _project common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "initialize", _project)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _project) returns()
func (_W3bstreamProject *W3bstreamProjectSession) Initialize(_project common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Initialize(&_W3bstreamProject.TransactOpts, _project)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _project) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) Initialize(_project common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Initialize(&_W3bstreamProject.TransactOpts, _project)
}

// Pause is a paid mutator transaction binding the contract method 0x136439dd.
//
// Solidity: function pause(uint256 _projectId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) Pause(opts *bind.TransactOpts, _projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "pause", _projectId)
}

// Pause is a paid mutator transaction binding the contract method 0x136439dd.
//
// Solidity: function pause(uint256 _projectId) returns()
func (_W3bstreamProject *W3bstreamProjectSession) Pause(_projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Pause(&_W3bstreamProject.TransactOpts, _projectId)
}

// Pause is a paid mutator transaction binding the contract method 0x136439dd.
//
// Solidity: function pause(uint256 _projectId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) Pause(_projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Pause(&_W3bstreamProject.TransactOpts, _projectId)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_W3bstreamProject *W3bstreamProjectSession) RenounceOwnership() (*types.Transaction, error) {
	return _W3bstreamProject.Contract.RenounceOwnership(&_W3bstreamProject.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _W3bstreamProject.Contract.RenounceOwnership(&_W3bstreamProject.TransactOpts)
}

// Resume is a paid mutator transaction binding the contract method 0x414000b5.
//
// Solidity: function resume(uint256 _projectId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) Resume(opts *bind.TransactOpts, _projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "resume", _projectId)
}

// Resume is a paid mutator transaction binding the contract method 0x414000b5.
//
// Solidity: function resume(uint256 _projectId) returns()
func (_W3bstreamProject *W3bstreamProjectSession) Resume(_projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Resume(&_W3bstreamProject.TransactOpts, _projectId)
}

// Resume is a paid mutator transaction binding the contract method 0x414000b5.
//
// Solidity: function resume(uint256 _projectId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) Resume(_projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Resume(&_W3bstreamProject.TransactOpts, _projectId)
}

// SetAttributes is a paid mutator transaction binding the contract method 0xa122effb.
//
// Solidity: function setAttributes(uint256 _projectId, bytes32[] _keys, bytes[] _values) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) SetAttributes(opts *bind.TransactOpts, _projectId *big.Int, _keys [][32]byte, _values [][]byte) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "setAttributes", _projectId, _keys, _values)
}

// SetAttributes is a paid mutator transaction binding the contract method 0xa122effb.
//
// Solidity: function setAttributes(uint256 _projectId, bytes32[] _keys, bytes[] _values) returns()
func (_W3bstreamProject *W3bstreamProjectSession) SetAttributes(_projectId *big.Int, _keys [][32]byte, _values [][]byte) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SetAttributes(&_W3bstreamProject.TransactOpts, _projectId, _keys, _values)
}

// SetAttributes is a paid mutator transaction binding the contract method 0xa122effb.
//
// Solidity: function setAttributes(uint256 _projectId, bytes32[] _keys, bytes[] _values) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) SetAttributes(_projectId *big.Int, _keys [][32]byte, _values [][]byte) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SetAttributes(&_W3bstreamProject.TransactOpts, _projectId, _keys, _values)
}

// SetBinder is a paid mutator transaction binding the contract method 0x536c54fa.
//
// Solidity: function setBinder(address _binder) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) SetBinder(opts *bind.TransactOpts, _binder common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "setBinder", _binder)
}

// SetBinder is a paid mutator transaction binding the contract method 0x536c54fa.
//
// Solidity: function setBinder(address _binder) returns()
func (_W3bstreamProject *W3bstreamProjectSession) SetBinder(_binder common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SetBinder(&_W3bstreamProject.TransactOpts, _binder)
}

// SetBinder is a paid mutator transaction binding the contract method 0x536c54fa.
//
// Solidity: function setBinder(address _binder) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) SetBinder(_binder common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SetBinder(&_W3bstreamProject.TransactOpts, _binder)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_W3bstreamProject *W3bstreamProjectSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.TransferOwnership(&_W3bstreamProject.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.TransferOwnership(&_W3bstreamProject.TransactOpts, newOwner)
}

// UpdateConfig is a paid mutator transaction binding the contract method 0x4b597aa3.
//
// Solidity: function updateConfig(uint256 _projectId, string _uri, bytes32 _hash) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) UpdateConfig(opts *bind.TransactOpts, _projectId *big.Int, _uri string, _hash [32]byte) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "updateConfig", _projectId, _uri, _hash)
}

// UpdateConfig is a paid mutator transaction binding the contract method 0x4b597aa3.
//
// Solidity: function updateConfig(uint256 _projectId, string _uri, bytes32 _hash) returns()
func (_W3bstreamProject *W3bstreamProjectSession) UpdateConfig(_projectId *big.Int, _uri string, _hash [32]byte) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.UpdateConfig(&_W3bstreamProject.TransactOpts, _projectId, _uri, _hash)
}

// UpdateConfig is a paid mutator transaction binding the contract method 0x4b597aa3.
//
// Solidity: function updateConfig(uint256 _projectId, string _uri, bytes32 _hash) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) UpdateConfig(_projectId *big.Int, _uri string, _hash [32]byte) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.UpdateConfig(&_W3bstreamProject.TransactOpts, _projectId, _uri, _hash)
}

// W3bstreamProjectAttributeSetIterator is returned from FilterAttributeSet and is used to iterate over the raw logs and unpacked data for AttributeSet events raised by the W3bstreamProject contract.
type W3bstreamProjectAttributeSetIterator struct {
	Event *W3bstreamProjectAttributeSet // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectAttributeSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectAttributeSet)
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
		it.Event = new(W3bstreamProjectAttributeSet)
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
func (it *W3bstreamProjectAttributeSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectAttributeSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectAttributeSet represents a AttributeSet event raised by the W3bstreamProject contract.
type W3bstreamProjectAttributeSet struct {
	ProjectId *big.Int
	Key       [32]byte
	Value     []byte
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterAttributeSet is a free log retrieval operation binding the contract event 0x840db4c564ec8ec61fd9377b125346993b20659d558d2e066e33c588b60f9fc3.
//
// Solidity: event AttributeSet(uint256 indexed projectId, bytes32 indexed key, bytes value)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterAttributeSet(opts *bind.FilterOpts, projectId []*big.Int, key [][32]byte) (*W3bstreamProjectAttributeSetIterator, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}
	var keyRule []interface{}
	for _, keyItem := range key {
		keyRule = append(keyRule, keyItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "AttributeSet", projectIdRule, keyRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectAttributeSetIterator{contract: _W3bstreamProject.contract, event: "AttributeSet", logs: logs, sub: sub}, nil
}

// WatchAttributeSet is a free log subscription operation binding the contract event 0x840db4c564ec8ec61fd9377b125346993b20659d558d2e066e33c588b60f9fc3.
//
// Solidity: event AttributeSet(uint256 indexed projectId, bytes32 indexed key, bytes value)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchAttributeSet(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectAttributeSet, projectId []*big.Int, key [][32]byte) (event.Subscription, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}
	var keyRule []interface{}
	for _, keyItem := range key {
		keyRule = append(keyRule, keyItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "AttributeSet", projectIdRule, keyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectAttributeSet)
				if err := _W3bstreamProject.contract.UnpackLog(event, "AttributeSet", log); err != nil {
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

// ParseAttributeSet is a log parse operation binding the contract event 0x840db4c564ec8ec61fd9377b125346993b20659d558d2e066e33c588b60f9fc3.
//
// Solidity: event AttributeSet(uint256 indexed projectId, bytes32 indexed key, bytes value)
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseAttributeSet(log types.Log) (*W3bstreamProjectAttributeSet, error) {
	event := new(W3bstreamProjectAttributeSet)
	if err := _W3bstreamProject.contract.UnpackLog(event, "AttributeSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProjectBinderSetIterator is returned from FilterBinderSet and is used to iterate over the raw logs and unpacked data for BinderSet events raised by the W3bstreamProject contract.
type W3bstreamProjectBinderSetIterator struct {
	Event *W3bstreamProjectBinderSet // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectBinderSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectBinderSet)
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
		it.Event = new(W3bstreamProjectBinderSet)
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
func (it *W3bstreamProjectBinderSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectBinderSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectBinderSet represents a BinderSet event raised by the W3bstreamProject contract.
type W3bstreamProjectBinderSet struct {
	Binder common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterBinderSet is a free log retrieval operation binding the contract event 0xb19637e660a60813c298ad7ddbf824f445fc38044f48e8ad24dc0ac64646f29b.
//
// Solidity: event BinderSet(address indexed binder)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterBinderSet(opts *bind.FilterOpts, binder []common.Address) (*W3bstreamProjectBinderSetIterator, error) {

	var binderRule []interface{}
	for _, binderItem := range binder {
		binderRule = append(binderRule, binderItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "BinderSet", binderRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectBinderSetIterator{contract: _W3bstreamProject.contract, event: "BinderSet", logs: logs, sub: sub}, nil
}

// WatchBinderSet is a free log subscription operation binding the contract event 0xb19637e660a60813c298ad7ddbf824f445fc38044f48e8ad24dc0ac64646f29b.
//
// Solidity: event BinderSet(address indexed binder)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchBinderSet(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectBinderSet, binder []common.Address) (event.Subscription, error) {

	var binderRule []interface{}
	for _, binderItem := range binder {
		binderRule = append(binderRule, binderItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "BinderSet", binderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectBinderSet)
				if err := _W3bstreamProject.contract.UnpackLog(event, "BinderSet", log); err != nil {
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

// ParseBinderSet is a log parse operation binding the contract event 0xb19637e660a60813c298ad7ddbf824f445fc38044f48e8ad24dc0ac64646f29b.
//
// Solidity: event BinderSet(address indexed binder)
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseBinderSet(log types.Log) (*W3bstreamProjectBinderSet, error) {
	event := new(W3bstreamProjectBinderSet)
	if err := _W3bstreamProject.contract.UnpackLog(event, "BinderSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProjectInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the W3bstreamProject contract.
type W3bstreamProjectInitializedIterator struct {
	Event *W3bstreamProjectInitialized // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectInitialized)
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
		it.Event = new(W3bstreamProjectInitialized)
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
func (it *W3bstreamProjectInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectInitialized represents a Initialized event raised by the W3bstreamProject contract.
type W3bstreamProjectInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterInitialized(opts *bind.FilterOpts) (*W3bstreamProjectInitializedIterator, error) {

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectInitializedIterator{contract: _W3bstreamProject.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectInitialized) (event.Subscription, error) {

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectInitialized)
				if err := _W3bstreamProject.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseInitialized(log types.Log) (*W3bstreamProjectInitialized, error) {
	event := new(W3bstreamProjectInitialized)
	if err := _W3bstreamProject.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProjectOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the W3bstreamProject contract.
type W3bstreamProjectOwnershipTransferredIterator struct {
	Event *W3bstreamProjectOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectOwnershipTransferred)
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
		it.Event = new(W3bstreamProjectOwnershipTransferred)
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
func (it *W3bstreamProjectOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectOwnershipTransferred represents a OwnershipTransferred event raised by the W3bstreamProject contract.
type W3bstreamProjectOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*W3bstreamProjectOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectOwnershipTransferredIterator{contract: _W3bstreamProject.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectOwnershipTransferred)
				if err := _W3bstreamProject.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseOwnershipTransferred(log types.Log) (*W3bstreamProjectOwnershipTransferred, error) {
	event := new(W3bstreamProjectOwnershipTransferred)
	if err := _W3bstreamProject.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProjectProjectBindedIterator is returned from FilterProjectBinded and is used to iterate over the raw logs and unpacked data for ProjectBinded events raised by the W3bstreamProject contract.
type W3bstreamProjectProjectBindedIterator struct {
	Event *W3bstreamProjectProjectBinded // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectProjectBindedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectProjectBinded)
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
		it.Event = new(W3bstreamProjectProjectBinded)
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
func (it *W3bstreamProjectProjectBindedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectProjectBindedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectProjectBinded represents a ProjectBinded event raised by the W3bstreamProject contract.
type W3bstreamProjectProjectBinded struct {
	ProjectId *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterProjectBinded is a free log retrieval operation binding the contract event 0x333b64ed2eccc26c75dc26694a75281557fd03abd4b4f13cc7399ab89ad0761a.
//
// Solidity: event ProjectBinded(uint256 indexed projectId)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterProjectBinded(opts *bind.FilterOpts, projectId []*big.Int) (*W3bstreamProjectProjectBindedIterator, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "ProjectBinded", projectIdRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectProjectBindedIterator{contract: _W3bstreamProject.contract, event: "ProjectBinded", logs: logs, sub: sub}, nil
}

// WatchProjectBinded is a free log subscription operation binding the contract event 0x333b64ed2eccc26c75dc26694a75281557fd03abd4b4f13cc7399ab89ad0761a.
//
// Solidity: event ProjectBinded(uint256 indexed projectId)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchProjectBinded(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectProjectBinded, projectId []*big.Int) (event.Subscription, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "ProjectBinded", projectIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectProjectBinded)
				if err := _W3bstreamProject.contract.UnpackLog(event, "ProjectBinded", log); err != nil {
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

// ParseProjectBinded is a log parse operation binding the contract event 0x333b64ed2eccc26c75dc26694a75281557fd03abd4b4f13cc7399ab89ad0761a.
//
// Solidity: event ProjectBinded(uint256 indexed projectId)
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseProjectBinded(log types.Log) (*W3bstreamProjectProjectBinded, error) {
	event := new(W3bstreamProjectProjectBinded)
	if err := _W3bstreamProject.contract.UnpackLog(event, "ProjectBinded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProjectProjectConfigUpdatedIterator is returned from FilterProjectConfigUpdated and is used to iterate over the raw logs and unpacked data for ProjectConfigUpdated events raised by the W3bstreamProject contract.
type W3bstreamProjectProjectConfigUpdatedIterator struct {
	Event *W3bstreamProjectProjectConfigUpdated // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectProjectConfigUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectProjectConfigUpdated)
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
		it.Event = new(W3bstreamProjectProjectConfigUpdated)
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
func (it *W3bstreamProjectProjectConfigUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectProjectConfigUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectProjectConfigUpdated represents a ProjectConfigUpdated event raised by the W3bstreamProject contract.
type W3bstreamProjectProjectConfigUpdated struct {
	ProjectId *big.Int
	Uri       string
	Hash      [32]byte
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterProjectConfigUpdated is a free log retrieval operation binding the contract event 0xa9ee0c223bc138bec6ebb21e09d00d5423fc3bbc210bdb6aef9d190b0641aecb.
//
// Solidity: event ProjectConfigUpdated(uint256 indexed projectId, string uri, bytes32 hash)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterProjectConfigUpdated(opts *bind.FilterOpts, projectId []*big.Int) (*W3bstreamProjectProjectConfigUpdatedIterator, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "ProjectConfigUpdated", projectIdRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectProjectConfigUpdatedIterator{contract: _W3bstreamProject.contract, event: "ProjectConfigUpdated", logs: logs, sub: sub}, nil
}

// WatchProjectConfigUpdated is a free log subscription operation binding the contract event 0xa9ee0c223bc138bec6ebb21e09d00d5423fc3bbc210bdb6aef9d190b0641aecb.
//
// Solidity: event ProjectConfigUpdated(uint256 indexed projectId, string uri, bytes32 hash)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchProjectConfigUpdated(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectProjectConfigUpdated, projectId []*big.Int) (event.Subscription, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "ProjectConfigUpdated", projectIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectProjectConfigUpdated)
				if err := _W3bstreamProject.contract.UnpackLog(event, "ProjectConfigUpdated", log); err != nil {
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

// ParseProjectConfigUpdated is a log parse operation binding the contract event 0xa9ee0c223bc138bec6ebb21e09d00d5423fc3bbc210bdb6aef9d190b0641aecb.
//
// Solidity: event ProjectConfigUpdated(uint256 indexed projectId, string uri, bytes32 hash)
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseProjectConfigUpdated(log types.Log) (*W3bstreamProjectProjectConfigUpdated, error) {
	event := new(W3bstreamProjectProjectConfigUpdated)
	if err := _W3bstreamProject.contract.UnpackLog(event, "ProjectConfigUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProjectProjectPausedIterator is returned from FilterProjectPaused and is used to iterate over the raw logs and unpacked data for ProjectPaused events raised by the W3bstreamProject contract.
type W3bstreamProjectProjectPausedIterator struct {
	Event *W3bstreamProjectProjectPaused // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectProjectPausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectProjectPaused)
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
		it.Event = new(W3bstreamProjectProjectPaused)
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
func (it *W3bstreamProjectProjectPausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectProjectPausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectProjectPaused represents a ProjectPaused event raised by the W3bstreamProject contract.
type W3bstreamProjectProjectPaused struct {
	ProjectId *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterProjectPaused is a free log retrieval operation binding the contract event 0x9f505f325627bdd7f5a6dd8bcceecdc48a989f647561427d61d35b7a50703f79.
//
// Solidity: event ProjectPaused(uint256 indexed projectId)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterProjectPaused(opts *bind.FilterOpts, projectId []*big.Int) (*W3bstreamProjectProjectPausedIterator, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "ProjectPaused", projectIdRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectProjectPausedIterator{contract: _W3bstreamProject.contract, event: "ProjectPaused", logs: logs, sub: sub}, nil
}

// WatchProjectPaused is a free log subscription operation binding the contract event 0x9f505f325627bdd7f5a6dd8bcceecdc48a989f647561427d61d35b7a50703f79.
//
// Solidity: event ProjectPaused(uint256 indexed projectId)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchProjectPaused(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectProjectPaused, projectId []*big.Int) (event.Subscription, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "ProjectPaused", projectIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectProjectPaused)
				if err := _W3bstreamProject.contract.UnpackLog(event, "ProjectPaused", log); err != nil {
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

// ParseProjectPaused is a log parse operation binding the contract event 0x9f505f325627bdd7f5a6dd8bcceecdc48a989f647561427d61d35b7a50703f79.
//
// Solidity: event ProjectPaused(uint256 indexed projectId)
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseProjectPaused(log types.Log) (*W3bstreamProjectProjectPaused, error) {
	event := new(W3bstreamProjectProjectPaused)
	if err := _W3bstreamProject.contract.UnpackLog(event, "ProjectPaused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProjectProjectResumedIterator is returned from FilterProjectResumed and is used to iterate over the raw logs and unpacked data for ProjectResumed events raised by the W3bstreamProject contract.
type W3bstreamProjectProjectResumedIterator struct {
	Event *W3bstreamProjectProjectResumed // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectProjectResumedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectProjectResumed)
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
		it.Event = new(W3bstreamProjectProjectResumed)
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
func (it *W3bstreamProjectProjectResumedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectProjectResumedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectProjectResumed represents a ProjectResumed event raised by the W3bstreamProject contract.
type W3bstreamProjectProjectResumed struct {
	ProjectId *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterProjectResumed is a free log retrieval operation binding the contract event 0x8c936416fd11c0291d9c7f69aad7e8847c5228b15a3969a1ac6a1c7bf394cd75.
//
// Solidity: event ProjectResumed(uint256 indexed projectId)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterProjectResumed(opts *bind.FilterOpts, projectId []*big.Int) (*W3bstreamProjectProjectResumedIterator, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "ProjectResumed", projectIdRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectProjectResumedIterator{contract: _W3bstreamProject.contract, event: "ProjectResumed", logs: logs, sub: sub}, nil
}

// WatchProjectResumed is a free log subscription operation binding the contract event 0x8c936416fd11c0291d9c7f69aad7e8847c5228b15a3969a1ac6a1c7bf394cd75.
//
// Solidity: event ProjectResumed(uint256 indexed projectId)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchProjectResumed(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectProjectResumed, projectId []*big.Int) (event.Subscription, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "ProjectResumed", projectIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectProjectResumed)
				if err := _W3bstreamProject.contract.UnpackLog(event, "ProjectResumed", log); err != nil {
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

// ParseProjectResumed is a log parse operation binding the contract event 0x8c936416fd11c0291d9c7f69aad7e8847c5228b15a3969a1ac6a1c7bf394cd75.
//
// Solidity: event ProjectResumed(uint256 indexed projectId)
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseProjectResumed(log types.Log) (*W3bstreamProjectProjectResumed, error) {
	event := new(W3bstreamProjectProjectResumed)
	if err := _W3bstreamProject.contract.UnpackLog(event, "ProjectResumed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

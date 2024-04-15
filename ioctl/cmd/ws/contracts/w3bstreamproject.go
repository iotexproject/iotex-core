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
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"approved\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"Approval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"approved\",\"type\":\"bool\"}],\"name\":\"ApprovalForAll\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"key\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"value\",\"type\":\"bytes\"}],\"name\":\"AttributeSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"minter\",\"type\":\"address\"}],\"name\":\"MinterSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"uri\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"hash\",\"type\":\"bytes32\"}],\"name\":\"ProjectConfigUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"}],\"name\":\"ProjectPaused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"}],\"name\":\"ProjectResumed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"_name\",\"type\":\"bytes32\"}],\"name\":\"attribute\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"attributes\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"bytes32[]\",\"name\":\"_keys\",\"type\":\"bytes32[]\"}],\"name\":\"attributesOf\",\"outputs\":[{\"internalType\":\"bytes[]\",\"name\":\"values_\",\"type\":\"bytes[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"config\",\"outputs\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"uri\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"hash\",\"type\":\"bytes32\"}],\"internalType\":\"structW3bstreamProject.ProjectConfig\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"count\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"getApproved\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"_name\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"_symbol\",\"type\":\"string\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isApprovedForAll\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"isPaused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_owner\",\"type\":\"address\"}],\"name\":\"mint\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"projectId_\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"minter\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"ownerOf\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"pause\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"resume\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"safeTransferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"safeTransferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"approved\",\"type\":\"bool\"}],\"name\":\"setApprovalForAll\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"_projectId\",\"type\":\"uint64\"},{\"internalType\":\"bytes32[]\",\"name\":\"_keys\",\"type\":\"bytes32[]\"},{\"internalType\":\"bytes[]\",\"name\":\"_values\",\"type\":\"bytes[]\"}],\"name\":\"setAttributes\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_minter\",\"type\":\"address\"}],\"name\":\"setMinter\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes4\",\"name\":\"interfaceId\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"tokenURI\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"transferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"_uri\",\"type\":\"string\"},{\"internalType\":\"bytes32\",\"name\":\"_hash\",\"type\":\"bytes32\"}],\"name\":\"updateConfig\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
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

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_W3bstreamProject *W3bstreamProjectCaller) BalanceOf(opts *bind.CallOpts, owner common.Address) (*big.Int, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "balanceOf", owner)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_W3bstreamProject *W3bstreamProjectSession) BalanceOf(owner common.Address) (*big.Int, error) {
	return _W3bstreamProject.Contract.BalanceOf(&_W3bstreamProject.CallOpts, owner)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_W3bstreamProject *W3bstreamProjectCallerSession) BalanceOf(owner common.Address) (*big.Int, error) {
	return _W3bstreamProject.Contract.BalanceOf(&_W3bstreamProject.CallOpts, owner)
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

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_W3bstreamProject *W3bstreamProjectCaller) GetApproved(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "getApproved", tokenId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_W3bstreamProject *W3bstreamProjectSession) GetApproved(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamProject.Contract.GetApproved(&_W3bstreamProject.CallOpts, tokenId)
}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_W3bstreamProject *W3bstreamProjectCallerSession) GetApproved(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamProject.Contract.GetApproved(&_W3bstreamProject.CallOpts, tokenId)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectCaller) IsApprovedForAll(opts *bind.CallOpts, owner common.Address, operator common.Address) (bool, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "isApprovedForAll", owner, operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectSession) IsApprovedForAll(owner common.Address, operator common.Address) (bool, error) {
	return _W3bstreamProject.Contract.IsApprovedForAll(&_W3bstreamProject.CallOpts, owner, operator)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectCallerSession) IsApprovedForAll(owner common.Address, operator common.Address) (bool, error) {
	return _W3bstreamProject.Contract.IsApprovedForAll(&_W3bstreamProject.CallOpts, owner, operator)
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

// Minter is a free data retrieval call binding the contract method 0x07546172.
//
// Solidity: function minter() view returns(address)
func (_W3bstreamProject *W3bstreamProjectCaller) Minter(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "minter")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Minter is a free data retrieval call binding the contract method 0x07546172.
//
// Solidity: function minter() view returns(address)
func (_W3bstreamProject *W3bstreamProjectSession) Minter() (common.Address, error) {
	return _W3bstreamProject.Contract.Minter(&_W3bstreamProject.CallOpts)
}

// Minter is a free data retrieval call binding the contract method 0x07546172.
//
// Solidity: function minter() view returns(address)
func (_W3bstreamProject *W3bstreamProjectCallerSession) Minter() (common.Address, error) {
	return _W3bstreamProject.Contract.Minter(&_W3bstreamProject.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_W3bstreamProject *W3bstreamProjectCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_W3bstreamProject *W3bstreamProjectSession) Name() (string, error) {
	return _W3bstreamProject.Contract.Name(&_W3bstreamProject.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_W3bstreamProject *W3bstreamProjectCallerSession) Name() (string, error) {
	return _W3bstreamProject.Contract.Name(&_W3bstreamProject.CallOpts)
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
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_W3bstreamProject *W3bstreamProjectCaller) OwnerOf(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "ownerOf", tokenId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_W3bstreamProject *W3bstreamProjectSession) OwnerOf(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamProject.Contract.OwnerOf(&_W3bstreamProject.CallOpts, tokenId)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_W3bstreamProject *W3bstreamProjectCallerSession) OwnerOf(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamProject.Contract.OwnerOf(&_W3bstreamProject.CallOpts, tokenId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _W3bstreamProject.Contract.SupportsInterface(&_W3bstreamProject.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_W3bstreamProject *W3bstreamProjectCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _W3bstreamProject.Contract.SupportsInterface(&_W3bstreamProject.CallOpts, interfaceId)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_W3bstreamProject *W3bstreamProjectCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_W3bstreamProject *W3bstreamProjectSession) Symbol() (string, error) {
	return _W3bstreamProject.Contract.Symbol(&_W3bstreamProject.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_W3bstreamProject *W3bstreamProjectCallerSession) Symbol() (string, error) {
	return _W3bstreamProject.Contract.Symbol(&_W3bstreamProject.CallOpts)
}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_W3bstreamProject *W3bstreamProjectCaller) TokenURI(opts *bind.CallOpts, tokenId *big.Int) (string, error) {
	var out []interface{}
	err := _W3bstreamProject.contract.Call(opts, &out, "tokenURI", tokenId)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_W3bstreamProject *W3bstreamProjectSession) TokenURI(tokenId *big.Int) (string, error) {
	return _W3bstreamProject.Contract.TokenURI(&_W3bstreamProject.CallOpts, tokenId)
}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_W3bstreamProject *W3bstreamProjectCallerSession) TokenURI(tokenId *big.Int) (string, error) {
	return _W3bstreamProject.Contract.TokenURI(&_W3bstreamProject.CallOpts, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) Approve(opts *bind.TransactOpts, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "approve", to, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_W3bstreamProject *W3bstreamProjectSession) Approve(to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Approve(&_W3bstreamProject.TransactOpts, to, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) Approve(to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Approve(&_W3bstreamProject.TransactOpts, to, tokenId)
}

// Initialize is a paid mutator transaction binding the contract method 0x4cd88b76.
//
// Solidity: function initialize(string _name, string _symbol) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) Initialize(opts *bind.TransactOpts, _name string, _symbol string) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "initialize", _name, _symbol)
}

// Initialize is a paid mutator transaction binding the contract method 0x4cd88b76.
//
// Solidity: function initialize(string _name, string _symbol) returns()
func (_W3bstreamProject *W3bstreamProjectSession) Initialize(_name string, _symbol string) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Initialize(&_W3bstreamProject.TransactOpts, _name, _symbol)
}

// Initialize is a paid mutator transaction binding the contract method 0x4cd88b76.
//
// Solidity: function initialize(string _name, string _symbol) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) Initialize(_name string, _symbol string) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Initialize(&_W3bstreamProject.TransactOpts, _name, _symbol)
}

// Mint is a paid mutator transaction binding the contract method 0x6a627842.
//
// Solidity: function mint(address _owner) returns(uint256 projectId_)
func (_W3bstreamProject *W3bstreamProjectTransactor) Mint(opts *bind.TransactOpts, _owner common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "mint", _owner)
}

// Mint is a paid mutator transaction binding the contract method 0x6a627842.
//
// Solidity: function mint(address _owner) returns(uint256 projectId_)
func (_W3bstreamProject *W3bstreamProjectSession) Mint(_owner common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Mint(&_W3bstreamProject.TransactOpts, _owner)
}

// Mint is a paid mutator transaction binding the contract method 0x6a627842.
//
// Solidity: function mint(address _owner) returns(uint256 projectId_)
func (_W3bstreamProject *W3bstreamProjectTransactorSession) Mint(_owner common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.Mint(&_W3bstreamProject.TransactOpts, _owner)
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

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) SafeTransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "safeTransferFrom", from, to, tokenId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProject *W3bstreamProjectSession) SafeTransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SafeTransferFrom(&_W3bstreamProject.TransactOpts, from, to, tokenId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) SafeTransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SafeTransferFrom(&_W3bstreamProject.TransactOpts, from, to, tokenId)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) SafeTransferFrom0(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "safeTransferFrom0", from, to, tokenId, data)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_W3bstreamProject *W3bstreamProjectSession) SafeTransferFrom0(from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SafeTransferFrom0(&_W3bstreamProject.TransactOpts, from, to, tokenId, data)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) SafeTransferFrom0(from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SafeTransferFrom0(&_W3bstreamProject.TransactOpts, from, to, tokenId, data)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) SetApprovalForAll(opts *bind.TransactOpts, operator common.Address, approved bool) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "setApprovalForAll", operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_W3bstreamProject *W3bstreamProjectSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SetApprovalForAll(&_W3bstreamProject.TransactOpts, operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SetApprovalForAll(&_W3bstreamProject.TransactOpts, operator, approved)
}

// SetAttributes is a paid mutator transaction binding the contract method 0xe24ce5b4.
//
// Solidity: function setAttributes(uint64 _projectId, bytes32[] _keys, bytes[] _values) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) SetAttributes(opts *bind.TransactOpts, _projectId uint64, _keys [][32]byte, _values [][]byte) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "setAttributes", _projectId, _keys, _values)
}

// SetAttributes is a paid mutator transaction binding the contract method 0xe24ce5b4.
//
// Solidity: function setAttributes(uint64 _projectId, bytes32[] _keys, bytes[] _values) returns()
func (_W3bstreamProject *W3bstreamProjectSession) SetAttributes(_projectId uint64, _keys [][32]byte, _values [][]byte) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SetAttributes(&_W3bstreamProject.TransactOpts, _projectId, _keys, _values)
}

// SetAttributes is a paid mutator transaction binding the contract method 0xe24ce5b4.
//
// Solidity: function setAttributes(uint64 _projectId, bytes32[] _keys, bytes[] _values) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) SetAttributes(_projectId uint64, _keys [][32]byte, _values [][]byte) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SetAttributes(&_W3bstreamProject.TransactOpts, _projectId, _keys, _values)
}

// SetMinter is a paid mutator transaction binding the contract method 0xfca3b5aa.
//
// Solidity: function setMinter(address _minter) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) SetMinter(opts *bind.TransactOpts, _minter common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "setMinter", _minter)
}

// SetMinter is a paid mutator transaction binding the contract method 0xfca3b5aa.
//
// Solidity: function setMinter(address _minter) returns()
func (_W3bstreamProject *W3bstreamProjectSession) SetMinter(_minter common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SetMinter(&_W3bstreamProject.TransactOpts, _minter)
}

// SetMinter is a paid mutator transaction binding the contract method 0xfca3b5aa.
//
// Solidity: function setMinter(address _minter) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) SetMinter(_minter common.Address) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.SetMinter(&_W3bstreamProject.TransactOpts, _minter)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactor) TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.contract.Transact(opts, "transferFrom", from, to, tokenId)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProject *W3bstreamProjectSession) TransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.TransferFrom(&_W3bstreamProject.TransactOpts, from, to, tokenId)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProject *W3bstreamProjectTransactorSession) TransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProject.Contract.TransferFrom(&_W3bstreamProject.TransactOpts, from, to, tokenId)
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

// W3bstreamProjectApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the W3bstreamProject contract.
type W3bstreamProjectApprovalIterator struct {
	Event *W3bstreamProjectApproval // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectApproval)
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
		it.Event = new(W3bstreamProjectApproval)
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
func (it *W3bstreamProjectApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectApproval represents a Approval event raised by the W3bstreamProject contract.
type W3bstreamProjectApproval struct {
	Owner    common.Address
	Approved common.Address
	TokenId  *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterApproval(opts *bind.FilterOpts, owner []common.Address, approved []common.Address, tokenId []*big.Int) (*W3bstreamProjectApprovalIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var approvedRule []interface{}
	for _, approvedItem := range approved {
		approvedRule = append(approvedRule, approvedItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "Approval", ownerRule, approvedRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectApprovalIterator{contract: _W3bstreamProject.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectApproval, owner []common.Address, approved []common.Address, tokenId []*big.Int) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var approvedRule []interface{}
	for _, approvedItem := range approved {
		approvedRule = append(approvedRule, approvedItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "Approval", ownerRule, approvedRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectApproval)
				if err := _W3bstreamProject.contract.UnpackLog(event, "Approval", log); err != nil {
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

// ParseApproval is a log parse operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseApproval(log types.Log) (*W3bstreamProjectApproval, error) {
	event := new(W3bstreamProjectApproval)
	if err := _W3bstreamProject.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProjectApprovalForAllIterator is returned from FilterApprovalForAll and is used to iterate over the raw logs and unpacked data for ApprovalForAll events raised by the W3bstreamProject contract.
type W3bstreamProjectApprovalForAllIterator struct {
	Event *W3bstreamProjectApprovalForAll // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectApprovalForAllIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectApprovalForAll)
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
		it.Event = new(W3bstreamProjectApprovalForAll)
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
func (it *W3bstreamProjectApprovalForAllIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectApprovalForAllIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectApprovalForAll represents a ApprovalForAll event raised by the W3bstreamProject contract.
type W3bstreamProjectApprovalForAll struct {
	Owner    common.Address
	Operator common.Address
	Approved bool
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApprovalForAll is a free log retrieval operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterApprovalForAll(opts *bind.FilterOpts, owner []common.Address, operator []common.Address) (*W3bstreamProjectApprovalForAllIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "ApprovalForAll", ownerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectApprovalForAllIterator{contract: _W3bstreamProject.contract, event: "ApprovalForAll", logs: logs, sub: sub}, nil
}

// WatchApprovalForAll is a free log subscription operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchApprovalForAll(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectApprovalForAll, owner []common.Address, operator []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "ApprovalForAll", ownerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectApprovalForAll)
				if err := _W3bstreamProject.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
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

// ParseApprovalForAll is a log parse operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseApprovalForAll(log types.Log) (*W3bstreamProjectApprovalForAll, error) {
	event := new(W3bstreamProjectApprovalForAll)
	if err := _W3bstreamProject.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
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

// W3bstreamProjectMinterSetIterator is returned from FilterMinterSet and is used to iterate over the raw logs and unpacked data for MinterSet events raised by the W3bstreamProject contract.
type W3bstreamProjectMinterSetIterator struct {
	Event *W3bstreamProjectMinterSet // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectMinterSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectMinterSet)
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
		it.Event = new(W3bstreamProjectMinterSet)
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
func (it *W3bstreamProjectMinterSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectMinterSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectMinterSet represents a MinterSet event raised by the W3bstreamProject contract.
type W3bstreamProjectMinterSet struct {
	Minter common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterMinterSet is a free log retrieval operation binding the contract event 0x726b590ef91a8c76ad05bbe91a57ef84605276528f49cd47d787f558a4e755b6.
//
// Solidity: event MinterSet(address indexed minter)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterMinterSet(opts *bind.FilterOpts, minter []common.Address) (*W3bstreamProjectMinterSetIterator, error) {

	var minterRule []interface{}
	for _, minterItem := range minter {
		minterRule = append(minterRule, minterItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "MinterSet", minterRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectMinterSetIterator{contract: _W3bstreamProject.contract, event: "MinterSet", logs: logs, sub: sub}, nil
}

// WatchMinterSet is a free log subscription operation binding the contract event 0x726b590ef91a8c76ad05bbe91a57ef84605276528f49cd47d787f558a4e755b6.
//
// Solidity: event MinterSet(address indexed minter)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchMinterSet(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectMinterSet, minter []common.Address) (event.Subscription, error) {

	var minterRule []interface{}
	for _, minterItem := range minter {
		minterRule = append(minterRule, minterItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "MinterSet", minterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectMinterSet)
				if err := _W3bstreamProject.contract.UnpackLog(event, "MinterSet", log); err != nil {
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

// ParseMinterSet is a log parse operation binding the contract event 0x726b590ef91a8c76ad05bbe91a57ef84605276528f49cd47d787f558a4e755b6.
//
// Solidity: event MinterSet(address indexed minter)
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseMinterSet(log types.Log) (*W3bstreamProjectMinterSet, error) {
	event := new(W3bstreamProjectMinterSet)
	if err := _W3bstreamProject.contract.UnpackLog(event, "MinterSet", log); err != nil {
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

// W3bstreamProjectTransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the W3bstreamProject contract.
type W3bstreamProjectTransferIterator struct {
	Event *W3bstreamProjectTransfer // Event containing the contract specifics and raw log

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
func (it *W3bstreamProjectTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProjectTransfer)
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
		it.Event = new(W3bstreamProjectTransfer)
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
func (it *W3bstreamProjectTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProjectTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProjectTransfer represents a Transfer event raised by the W3bstreamProject contract.
type W3bstreamProjectTransfer struct {
	From    common.Address
	To      common.Address
	TokenId *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_W3bstreamProject *W3bstreamProjectFilterer) FilterTransfer(opts *bind.FilterOpts, from []common.Address, to []common.Address, tokenId []*big.Int) (*W3bstreamProjectTransferIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.FilterLogs(opts, "Transfer", fromRule, toRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProjectTransferIterator{contract: _W3bstreamProject.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_W3bstreamProject *W3bstreamProjectFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *W3bstreamProjectTransfer, from []common.Address, to []common.Address, tokenId []*big.Int) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _W3bstreamProject.contract.WatchLogs(opts, "Transfer", fromRule, toRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProjectTransfer)
				if err := _W3bstreamProject.contract.UnpackLog(event, "Transfer", log); err != nil {
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

// ParseTransfer is a log parse operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_W3bstreamProject *W3bstreamProjectFilterer) ParseTransfer(log types.Log) (*W3bstreamProjectTransfer, error) {
	event := new(W3bstreamProjectTransfer)
	if err := _W3bstreamProject.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

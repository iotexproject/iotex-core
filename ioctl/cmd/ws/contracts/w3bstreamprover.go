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

// W3bstreamProverMetaData contains all meta data concerning the W3bstreamProver contract.
var W3bstreamProverMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"approved\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"Approval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"approved\",\"type\":\"bool\"}],\"name\":\"ApprovalForAll\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"minter\",\"type\":\"address\"}],\"name\":\"MinterSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"OperatorSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"ProverPaused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"ProverResumed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"typ\",\"type\":\"uint256\"}],\"name\":\"VMTypeAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"typ\",\"type\":\"uint256\"}],\"name\":\"VMTypeDeleted\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_type\",\"type\":\"uint256\"}],\"name\":\"addVMType\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"_operator\",\"type\":\"address\"}],\"name\":\"changeOperator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"count\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_type\",\"type\":\"uint256\"}],\"name\":\"delVMType\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"getApproved\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"_name\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"_symbol\",\"type\":\"string\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isApprovedForAll\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"}],\"name\":\"isPaused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_type\",\"type\":\"uint256\"}],\"name\":\"isVMTypeSupported\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_account\",\"type\":\"address\"}],\"name\":\"mint\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"id_\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"minter\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"}],\"name\":\"operator\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"ownerOf\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_operator\",\"type\":\"address\"}],\"name\":\"ownerOfOperator\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"}],\"name\":\"pause\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"}],\"name\":\"prover\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"}],\"name\":\"resume\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"safeTransferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"safeTransferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"approved\",\"type\":\"bool\"}],\"name\":\"setApprovalForAll\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_minter\",\"type\":\"address\"}],\"name\":\"setMinter\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes4\",\"name\":\"interfaceId\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"tokenURI\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"transferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// W3bstreamProverABI is the input ABI used to generate the binding from.
// Deprecated: Use W3bstreamProverMetaData.ABI instead.
var W3bstreamProverABI = W3bstreamProverMetaData.ABI

// W3bstreamProver is an auto generated Go binding around an Ethereum contract.
type W3bstreamProver struct {
	W3bstreamProverCaller     // Read-only binding to the contract
	W3bstreamProverTransactor // Write-only binding to the contract
	W3bstreamProverFilterer   // Log filterer for contract events
}

// W3bstreamProverCaller is an auto generated read-only Go binding around an Ethereum contract.
type W3bstreamProverCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamProverTransactor is an auto generated write-only Go binding around an Ethereum contract.
type W3bstreamProverTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamProverFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type W3bstreamProverFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamProverSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type W3bstreamProverSession struct {
	Contract     *W3bstreamProver  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// W3bstreamProverCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type W3bstreamProverCallerSession struct {
	Contract *W3bstreamProverCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// W3bstreamProverTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type W3bstreamProverTransactorSession struct {
	Contract     *W3bstreamProverTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// W3bstreamProverRaw is an auto generated low-level Go binding around an Ethereum contract.
type W3bstreamProverRaw struct {
	Contract *W3bstreamProver // Generic contract binding to access the raw methods on
}

// W3bstreamProverCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type W3bstreamProverCallerRaw struct {
	Contract *W3bstreamProverCaller // Generic read-only contract binding to access the raw methods on
}

// W3bstreamProverTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type W3bstreamProverTransactorRaw struct {
	Contract *W3bstreamProverTransactor // Generic write-only contract binding to access the raw methods on
}

// NewW3bstreamProver creates a new instance of W3bstreamProver, bound to a specific deployed contract.
func NewW3bstreamProver(address common.Address, backend bind.ContractBackend) (*W3bstreamProver, error) {
	contract, err := bindW3bstreamProver(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProver{W3bstreamProverCaller: W3bstreamProverCaller{contract: contract}, W3bstreamProverTransactor: W3bstreamProverTransactor{contract: contract}, W3bstreamProverFilterer: W3bstreamProverFilterer{contract: contract}}, nil
}

// NewW3bstreamProverCaller creates a new read-only instance of W3bstreamProver, bound to a specific deployed contract.
func NewW3bstreamProverCaller(address common.Address, caller bind.ContractCaller) (*W3bstreamProverCaller, error) {
	contract, err := bindW3bstreamProver(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverCaller{contract: contract}, nil
}

// NewW3bstreamProverTransactor creates a new write-only instance of W3bstreamProver, bound to a specific deployed contract.
func NewW3bstreamProverTransactor(address common.Address, transactor bind.ContractTransactor) (*W3bstreamProverTransactor, error) {
	contract, err := bindW3bstreamProver(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverTransactor{contract: contract}, nil
}

// NewW3bstreamProverFilterer creates a new log filterer instance of W3bstreamProver, bound to a specific deployed contract.
func NewW3bstreamProverFilterer(address common.Address, filterer bind.ContractFilterer) (*W3bstreamProverFilterer, error) {
	contract, err := bindW3bstreamProver(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverFilterer{contract: contract}, nil
}

// bindW3bstreamProver binds a generic wrapper to an already deployed contract.
func bindW3bstreamProver(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := W3bstreamProverMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_W3bstreamProver *W3bstreamProverRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _W3bstreamProver.Contract.W3bstreamProverCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_W3bstreamProver *W3bstreamProverRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.W3bstreamProverTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_W3bstreamProver *W3bstreamProverRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.W3bstreamProverTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_W3bstreamProver *W3bstreamProverCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _W3bstreamProver.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_W3bstreamProver *W3bstreamProverTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_W3bstreamProver *W3bstreamProverTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.contract.Transact(opts, method, params...)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_W3bstreamProver *W3bstreamProverCaller) BalanceOf(opts *bind.CallOpts, owner common.Address) (*big.Int, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "balanceOf", owner)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_W3bstreamProver *W3bstreamProverSession) BalanceOf(owner common.Address) (*big.Int, error) {
	return _W3bstreamProver.Contract.BalanceOf(&_W3bstreamProver.CallOpts, owner)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_W3bstreamProver *W3bstreamProverCallerSession) BalanceOf(owner common.Address) (*big.Int, error) {
	return _W3bstreamProver.Contract.BalanceOf(&_W3bstreamProver.CallOpts, owner)
}

// Count is a free data retrieval call binding the contract method 0x06661abd.
//
// Solidity: function count() view returns(uint256)
func (_W3bstreamProver *W3bstreamProverCaller) Count(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "count")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Count is a free data retrieval call binding the contract method 0x06661abd.
//
// Solidity: function count() view returns(uint256)
func (_W3bstreamProver *W3bstreamProverSession) Count() (*big.Int, error) {
	return _W3bstreamProver.Contract.Count(&_W3bstreamProver.CallOpts)
}

// Count is a free data retrieval call binding the contract method 0x06661abd.
//
// Solidity: function count() view returns(uint256)
func (_W3bstreamProver *W3bstreamProverCallerSession) Count() (*big.Int, error) {
	return _W3bstreamProver.Contract.Count(&_W3bstreamProver.CallOpts)
}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_W3bstreamProver *W3bstreamProverCaller) GetApproved(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "getApproved", tokenId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_W3bstreamProver *W3bstreamProverSession) GetApproved(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamProver.Contract.GetApproved(&_W3bstreamProver.CallOpts, tokenId)
}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_W3bstreamProver *W3bstreamProverCallerSession) GetApproved(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamProver.Contract.GetApproved(&_W3bstreamProver.CallOpts, tokenId)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_W3bstreamProver *W3bstreamProverCaller) IsApprovedForAll(opts *bind.CallOpts, owner common.Address, operator common.Address) (bool, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "isApprovedForAll", owner, operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_W3bstreamProver *W3bstreamProverSession) IsApprovedForAll(owner common.Address, operator common.Address) (bool, error) {
	return _W3bstreamProver.Contract.IsApprovedForAll(&_W3bstreamProver.CallOpts, owner, operator)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_W3bstreamProver *W3bstreamProverCallerSession) IsApprovedForAll(owner common.Address, operator common.Address) (bool, error) {
	return _W3bstreamProver.Contract.IsApprovedForAll(&_W3bstreamProver.CallOpts, owner, operator)
}

// IsPaused is a free data retrieval call binding the contract method 0xbdf2a43c.
//
// Solidity: function isPaused(uint256 _id) view returns(bool)
func (_W3bstreamProver *W3bstreamProverCaller) IsPaused(opts *bind.CallOpts, _id *big.Int) (bool, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "isPaused", _id)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsPaused is a free data retrieval call binding the contract method 0xbdf2a43c.
//
// Solidity: function isPaused(uint256 _id) view returns(bool)
func (_W3bstreamProver *W3bstreamProverSession) IsPaused(_id *big.Int) (bool, error) {
	return _W3bstreamProver.Contract.IsPaused(&_W3bstreamProver.CallOpts, _id)
}

// IsPaused is a free data retrieval call binding the contract method 0xbdf2a43c.
//
// Solidity: function isPaused(uint256 _id) view returns(bool)
func (_W3bstreamProver *W3bstreamProverCallerSession) IsPaused(_id *big.Int) (bool, error) {
	return _W3bstreamProver.Contract.IsPaused(&_W3bstreamProver.CallOpts, _id)
}

// IsVMTypeSupported is a free data retrieval call binding the contract method 0x6d968739.
//
// Solidity: function isVMTypeSupported(uint256 _id, uint256 _type) view returns(bool)
func (_W3bstreamProver *W3bstreamProverCaller) IsVMTypeSupported(opts *bind.CallOpts, _id *big.Int, _type *big.Int) (bool, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "isVMTypeSupported", _id, _type)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsVMTypeSupported is a free data retrieval call binding the contract method 0x6d968739.
//
// Solidity: function isVMTypeSupported(uint256 _id, uint256 _type) view returns(bool)
func (_W3bstreamProver *W3bstreamProverSession) IsVMTypeSupported(_id *big.Int, _type *big.Int) (bool, error) {
	return _W3bstreamProver.Contract.IsVMTypeSupported(&_W3bstreamProver.CallOpts, _id, _type)
}

// IsVMTypeSupported is a free data retrieval call binding the contract method 0x6d968739.
//
// Solidity: function isVMTypeSupported(uint256 _id, uint256 _type) view returns(bool)
func (_W3bstreamProver *W3bstreamProverCallerSession) IsVMTypeSupported(_id *big.Int, _type *big.Int) (bool, error) {
	return _W3bstreamProver.Contract.IsVMTypeSupported(&_W3bstreamProver.CallOpts, _id, _type)
}

// Minter is a free data retrieval call binding the contract method 0x07546172.
//
// Solidity: function minter() view returns(address)
func (_W3bstreamProver *W3bstreamProverCaller) Minter(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "minter")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Minter is a free data retrieval call binding the contract method 0x07546172.
//
// Solidity: function minter() view returns(address)
func (_W3bstreamProver *W3bstreamProverSession) Minter() (common.Address, error) {
	return _W3bstreamProver.Contract.Minter(&_W3bstreamProver.CallOpts)
}

// Minter is a free data retrieval call binding the contract method 0x07546172.
//
// Solidity: function minter() view returns(address)
func (_W3bstreamProver *W3bstreamProverCallerSession) Minter() (common.Address, error) {
	return _W3bstreamProver.Contract.Minter(&_W3bstreamProver.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_W3bstreamProver *W3bstreamProverCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_W3bstreamProver *W3bstreamProverSession) Name() (string, error) {
	return _W3bstreamProver.Contract.Name(&_W3bstreamProver.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_W3bstreamProver *W3bstreamProverCallerSession) Name() (string, error) {
	return _W3bstreamProver.Contract.Name(&_W3bstreamProver.CallOpts)
}

// Operator is a free data retrieval call binding the contract method 0xab3d047f.
//
// Solidity: function operator(uint256 _id) view returns(address)
func (_W3bstreamProver *W3bstreamProverCaller) Operator(opts *bind.CallOpts, _id *big.Int) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "operator", _id)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Operator is a free data retrieval call binding the contract method 0xab3d047f.
//
// Solidity: function operator(uint256 _id) view returns(address)
func (_W3bstreamProver *W3bstreamProverSession) Operator(_id *big.Int) (common.Address, error) {
	return _W3bstreamProver.Contract.Operator(&_W3bstreamProver.CallOpts, _id)
}

// Operator is a free data retrieval call binding the contract method 0xab3d047f.
//
// Solidity: function operator(uint256 _id) view returns(address)
func (_W3bstreamProver *W3bstreamProverCallerSession) Operator(_id *big.Int) (common.Address, error) {
	return _W3bstreamProver.Contract.Operator(&_W3bstreamProver.CallOpts, _id)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_W3bstreamProver *W3bstreamProverCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_W3bstreamProver *W3bstreamProverSession) Owner() (common.Address, error) {
	return _W3bstreamProver.Contract.Owner(&_W3bstreamProver.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_W3bstreamProver *W3bstreamProverCallerSession) Owner() (common.Address, error) {
	return _W3bstreamProver.Contract.Owner(&_W3bstreamProver.CallOpts)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_W3bstreamProver *W3bstreamProverCaller) OwnerOf(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "ownerOf", tokenId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_W3bstreamProver *W3bstreamProverSession) OwnerOf(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamProver.Contract.OwnerOf(&_W3bstreamProver.CallOpts, tokenId)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_W3bstreamProver *W3bstreamProverCallerSession) OwnerOf(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamProver.Contract.OwnerOf(&_W3bstreamProver.CallOpts, tokenId)
}

// OwnerOfOperator is a free data retrieval call binding the contract method 0xaeb44406.
//
// Solidity: function ownerOfOperator(address _operator) view returns(uint256, address)
func (_W3bstreamProver *W3bstreamProverCaller) OwnerOfOperator(opts *bind.CallOpts, _operator common.Address) (*big.Int, common.Address, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "ownerOfOperator", _operator)

	if err != nil {
		return *new(*big.Int), *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	out1 := *abi.ConvertType(out[1], new(common.Address)).(*common.Address)

	return out0, out1, err

}

// OwnerOfOperator is a free data retrieval call binding the contract method 0xaeb44406.
//
// Solidity: function ownerOfOperator(address _operator) view returns(uint256, address)
func (_W3bstreamProver *W3bstreamProverSession) OwnerOfOperator(_operator common.Address) (*big.Int, common.Address, error) {
	return _W3bstreamProver.Contract.OwnerOfOperator(&_W3bstreamProver.CallOpts, _operator)
}

// OwnerOfOperator is a free data retrieval call binding the contract method 0xaeb44406.
//
// Solidity: function ownerOfOperator(address _operator) view returns(uint256, address)
func (_W3bstreamProver *W3bstreamProverCallerSession) OwnerOfOperator(_operator common.Address) (*big.Int, common.Address, error) {
	return _W3bstreamProver.Contract.OwnerOfOperator(&_W3bstreamProver.CallOpts, _operator)
}

// Prover is a free data retrieval call binding the contract method 0x2becbd3e.
//
// Solidity: function prover(uint256 _id) view returns(address)
func (_W3bstreamProver *W3bstreamProverCaller) Prover(opts *bind.CallOpts, _id *big.Int) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "prover", _id)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Prover is a free data retrieval call binding the contract method 0x2becbd3e.
//
// Solidity: function prover(uint256 _id) view returns(address)
func (_W3bstreamProver *W3bstreamProverSession) Prover(_id *big.Int) (common.Address, error) {
	return _W3bstreamProver.Contract.Prover(&_W3bstreamProver.CallOpts, _id)
}

// Prover is a free data retrieval call binding the contract method 0x2becbd3e.
//
// Solidity: function prover(uint256 _id) view returns(address)
func (_W3bstreamProver *W3bstreamProverCallerSession) Prover(_id *big.Int) (common.Address, error) {
	return _W3bstreamProver.Contract.Prover(&_W3bstreamProver.CallOpts, _id)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_W3bstreamProver *W3bstreamProverCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_W3bstreamProver *W3bstreamProverSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _W3bstreamProver.Contract.SupportsInterface(&_W3bstreamProver.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_W3bstreamProver *W3bstreamProverCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _W3bstreamProver.Contract.SupportsInterface(&_W3bstreamProver.CallOpts, interfaceId)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_W3bstreamProver *W3bstreamProverCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_W3bstreamProver *W3bstreamProverSession) Symbol() (string, error) {
	return _W3bstreamProver.Contract.Symbol(&_W3bstreamProver.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_W3bstreamProver *W3bstreamProverCallerSession) Symbol() (string, error) {
	return _W3bstreamProver.Contract.Symbol(&_W3bstreamProver.CallOpts)
}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_W3bstreamProver *W3bstreamProverCaller) TokenURI(opts *bind.CallOpts, tokenId *big.Int) (string, error) {
	var out []interface{}
	err := _W3bstreamProver.contract.Call(opts, &out, "tokenURI", tokenId)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_W3bstreamProver *W3bstreamProverSession) TokenURI(tokenId *big.Int) (string, error) {
	return _W3bstreamProver.Contract.TokenURI(&_W3bstreamProver.CallOpts, tokenId)
}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_W3bstreamProver *W3bstreamProverCallerSession) TokenURI(tokenId *big.Int) (string, error) {
	return _W3bstreamProver.Contract.TokenURI(&_W3bstreamProver.CallOpts, tokenId)
}

// AddVMType is a paid mutator transaction binding the contract method 0x5ba87ef4.
//
// Solidity: function addVMType(uint256 _id, uint256 _type) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) AddVMType(opts *bind.TransactOpts, _id *big.Int, _type *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "addVMType", _id, _type)
}

// AddVMType is a paid mutator transaction binding the contract method 0x5ba87ef4.
//
// Solidity: function addVMType(uint256 _id, uint256 _type) returns()
func (_W3bstreamProver *W3bstreamProverSession) AddVMType(_id *big.Int, _type *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.AddVMType(&_W3bstreamProver.TransactOpts, _id, _type)
}

// AddVMType is a paid mutator transaction binding the contract method 0x5ba87ef4.
//
// Solidity: function addVMType(uint256 _id, uint256 _type) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) AddVMType(_id *big.Int, _type *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.AddVMType(&_W3bstreamProver.TransactOpts, _id, _type)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) Approve(opts *bind.TransactOpts, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "approve", to, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_W3bstreamProver *W3bstreamProverSession) Approve(to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.Approve(&_W3bstreamProver.TransactOpts, to, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) Approve(to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.Approve(&_W3bstreamProver.TransactOpts, to, tokenId)
}

// ChangeOperator is a paid mutator transaction binding the contract method 0x7a3bc817.
//
// Solidity: function changeOperator(uint256 _id, address _operator) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) ChangeOperator(opts *bind.TransactOpts, _id *big.Int, _operator common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "changeOperator", _id, _operator)
}

// ChangeOperator is a paid mutator transaction binding the contract method 0x7a3bc817.
//
// Solidity: function changeOperator(uint256 _id, address _operator) returns()
func (_W3bstreamProver *W3bstreamProverSession) ChangeOperator(_id *big.Int, _operator common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.ChangeOperator(&_W3bstreamProver.TransactOpts, _id, _operator)
}

// ChangeOperator is a paid mutator transaction binding the contract method 0x7a3bc817.
//
// Solidity: function changeOperator(uint256 _id, address _operator) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) ChangeOperator(_id *big.Int, _operator common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.ChangeOperator(&_W3bstreamProver.TransactOpts, _id, _operator)
}

// DelVMType is a paid mutator transaction binding the contract method 0x572529b7.
//
// Solidity: function delVMType(uint256 _id, uint256 _type) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) DelVMType(opts *bind.TransactOpts, _id *big.Int, _type *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "delVMType", _id, _type)
}

// DelVMType is a paid mutator transaction binding the contract method 0x572529b7.
//
// Solidity: function delVMType(uint256 _id, uint256 _type) returns()
func (_W3bstreamProver *W3bstreamProverSession) DelVMType(_id *big.Int, _type *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.DelVMType(&_W3bstreamProver.TransactOpts, _id, _type)
}

// DelVMType is a paid mutator transaction binding the contract method 0x572529b7.
//
// Solidity: function delVMType(uint256 _id, uint256 _type) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) DelVMType(_id *big.Int, _type *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.DelVMType(&_W3bstreamProver.TransactOpts, _id, _type)
}

// Initialize is a paid mutator transaction binding the contract method 0x4cd88b76.
//
// Solidity: function initialize(string _name, string _symbol) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) Initialize(opts *bind.TransactOpts, _name string, _symbol string) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "initialize", _name, _symbol)
}

// Initialize is a paid mutator transaction binding the contract method 0x4cd88b76.
//
// Solidity: function initialize(string _name, string _symbol) returns()
func (_W3bstreamProver *W3bstreamProverSession) Initialize(_name string, _symbol string) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.Initialize(&_W3bstreamProver.TransactOpts, _name, _symbol)
}

// Initialize is a paid mutator transaction binding the contract method 0x4cd88b76.
//
// Solidity: function initialize(string _name, string _symbol) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) Initialize(_name string, _symbol string) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.Initialize(&_W3bstreamProver.TransactOpts, _name, _symbol)
}

// Mint is a paid mutator transaction binding the contract method 0x6a627842.
//
// Solidity: function mint(address _account) returns(uint256 id_)
func (_W3bstreamProver *W3bstreamProverTransactor) Mint(opts *bind.TransactOpts, _account common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "mint", _account)
}

// Mint is a paid mutator transaction binding the contract method 0x6a627842.
//
// Solidity: function mint(address _account) returns(uint256 id_)
func (_W3bstreamProver *W3bstreamProverSession) Mint(_account common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.Mint(&_W3bstreamProver.TransactOpts, _account)
}

// Mint is a paid mutator transaction binding the contract method 0x6a627842.
//
// Solidity: function mint(address _account) returns(uint256 id_)
func (_W3bstreamProver *W3bstreamProverTransactorSession) Mint(_account common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.Mint(&_W3bstreamProver.TransactOpts, _account)
}

// Pause is a paid mutator transaction binding the contract method 0x136439dd.
//
// Solidity: function pause(uint256 _id) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) Pause(opts *bind.TransactOpts, _id *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "pause", _id)
}

// Pause is a paid mutator transaction binding the contract method 0x136439dd.
//
// Solidity: function pause(uint256 _id) returns()
func (_W3bstreamProver *W3bstreamProverSession) Pause(_id *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.Pause(&_W3bstreamProver.TransactOpts, _id)
}

// Pause is a paid mutator transaction binding the contract method 0x136439dd.
//
// Solidity: function pause(uint256 _id) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) Pause(_id *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.Pause(&_W3bstreamProver.TransactOpts, _id)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_W3bstreamProver *W3bstreamProverTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_W3bstreamProver *W3bstreamProverSession) RenounceOwnership() (*types.Transaction, error) {
	return _W3bstreamProver.Contract.RenounceOwnership(&_W3bstreamProver.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _W3bstreamProver.Contract.RenounceOwnership(&_W3bstreamProver.TransactOpts)
}

// Resume is a paid mutator transaction binding the contract method 0x414000b5.
//
// Solidity: function resume(uint256 _id) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) Resume(opts *bind.TransactOpts, _id *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "resume", _id)
}

// Resume is a paid mutator transaction binding the contract method 0x414000b5.
//
// Solidity: function resume(uint256 _id) returns()
func (_W3bstreamProver *W3bstreamProverSession) Resume(_id *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.Resume(&_W3bstreamProver.TransactOpts, _id)
}

// Resume is a paid mutator transaction binding the contract method 0x414000b5.
//
// Solidity: function resume(uint256 _id) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) Resume(_id *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.Resume(&_W3bstreamProver.TransactOpts, _id)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) SafeTransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "safeTransferFrom", from, to, tokenId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProver *W3bstreamProverSession) SafeTransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.SafeTransferFrom(&_W3bstreamProver.TransactOpts, from, to, tokenId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) SafeTransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.SafeTransferFrom(&_W3bstreamProver.TransactOpts, from, to, tokenId)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) SafeTransferFrom0(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "safeTransferFrom0", from, to, tokenId, data)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_W3bstreamProver *W3bstreamProverSession) SafeTransferFrom0(from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.SafeTransferFrom0(&_W3bstreamProver.TransactOpts, from, to, tokenId, data)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) SafeTransferFrom0(from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.SafeTransferFrom0(&_W3bstreamProver.TransactOpts, from, to, tokenId, data)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) SetApprovalForAll(opts *bind.TransactOpts, operator common.Address, approved bool) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "setApprovalForAll", operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_W3bstreamProver *W3bstreamProverSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.SetApprovalForAll(&_W3bstreamProver.TransactOpts, operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.SetApprovalForAll(&_W3bstreamProver.TransactOpts, operator, approved)
}

// SetMinter is a paid mutator transaction binding the contract method 0xfca3b5aa.
//
// Solidity: function setMinter(address _minter) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) SetMinter(opts *bind.TransactOpts, _minter common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "setMinter", _minter)
}

// SetMinter is a paid mutator transaction binding the contract method 0xfca3b5aa.
//
// Solidity: function setMinter(address _minter) returns()
func (_W3bstreamProver *W3bstreamProverSession) SetMinter(_minter common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.SetMinter(&_W3bstreamProver.TransactOpts, _minter)
}

// SetMinter is a paid mutator transaction binding the contract method 0xfca3b5aa.
//
// Solidity: function setMinter(address _minter) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) SetMinter(_minter common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.SetMinter(&_W3bstreamProver.TransactOpts, _minter)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "transferFrom", from, to, tokenId)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProver *W3bstreamProverSession) TransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.TransferFrom(&_W3bstreamProver.TransactOpts, from, to, tokenId)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) TransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.TransferFrom(&_W3bstreamProver.TransactOpts, from, to, tokenId)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_W3bstreamProver *W3bstreamProverTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_W3bstreamProver *W3bstreamProverSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.TransferOwnership(&_W3bstreamProver.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_W3bstreamProver *W3bstreamProverTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _W3bstreamProver.Contract.TransferOwnership(&_W3bstreamProver.TransactOpts, newOwner)
}

// W3bstreamProverApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the W3bstreamProver contract.
type W3bstreamProverApprovalIterator struct {
	Event *W3bstreamProverApproval // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverApproval)
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
		it.Event = new(W3bstreamProverApproval)
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
func (it *W3bstreamProverApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverApproval represents a Approval event raised by the W3bstreamProver contract.
type W3bstreamProverApproval struct {
	Owner    common.Address
	Approved common.Address
	TokenId  *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterApproval(opts *bind.FilterOpts, owner []common.Address, approved []common.Address, tokenId []*big.Int) (*W3bstreamProverApprovalIterator, error) {

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

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "Approval", ownerRule, approvedRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverApprovalIterator{contract: _W3bstreamProver.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *W3bstreamProverApproval, owner []common.Address, approved []common.Address, tokenId []*big.Int) (event.Subscription, error) {

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

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "Approval", ownerRule, approvedRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverApproval)
				if err := _W3bstreamProver.contract.UnpackLog(event, "Approval", log); err != nil {
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
func (_W3bstreamProver *W3bstreamProverFilterer) ParseApproval(log types.Log) (*W3bstreamProverApproval, error) {
	event := new(W3bstreamProverApproval)
	if err := _W3bstreamProver.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProverApprovalForAllIterator is returned from FilterApprovalForAll and is used to iterate over the raw logs and unpacked data for ApprovalForAll events raised by the W3bstreamProver contract.
type W3bstreamProverApprovalForAllIterator struct {
	Event *W3bstreamProverApprovalForAll // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverApprovalForAllIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverApprovalForAll)
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
		it.Event = new(W3bstreamProverApprovalForAll)
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
func (it *W3bstreamProverApprovalForAllIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverApprovalForAllIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverApprovalForAll represents a ApprovalForAll event raised by the W3bstreamProver contract.
type W3bstreamProverApprovalForAll struct {
	Owner    common.Address
	Operator common.Address
	Approved bool
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApprovalForAll is a free log retrieval operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterApprovalForAll(opts *bind.FilterOpts, owner []common.Address, operator []common.Address) (*W3bstreamProverApprovalForAllIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "ApprovalForAll", ownerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverApprovalForAllIterator{contract: _W3bstreamProver.contract, event: "ApprovalForAll", logs: logs, sub: sub}, nil
}

// WatchApprovalForAll is a free log subscription operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchApprovalForAll(opts *bind.WatchOpts, sink chan<- *W3bstreamProverApprovalForAll, owner []common.Address, operator []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "ApprovalForAll", ownerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverApprovalForAll)
				if err := _W3bstreamProver.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
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
func (_W3bstreamProver *W3bstreamProverFilterer) ParseApprovalForAll(log types.Log) (*W3bstreamProverApprovalForAll, error) {
	event := new(W3bstreamProverApprovalForAll)
	if err := _W3bstreamProver.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProverInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the W3bstreamProver contract.
type W3bstreamProverInitializedIterator struct {
	Event *W3bstreamProverInitialized // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverInitialized)
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
		it.Event = new(W3bstreamProverInitialized)
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
func (it *W3bstreamProverInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverInitialized represents a Initialized event raised by the W3bstreamProver contract.
type W3bstreamProverInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterInitialized(opts *bind.FilterOpts) (*W3bstreamProverInitializedIterator, error) {

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverInitializedIterator{contract: _W3bstreamProver.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *W3bstreamProverInitialized) (event.Subscription, error) {

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverInitialized)
				if err := _W3bstreamProver.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_W3bstreamProver *W3bstreamProverFilterer) ParseInitialized(log types.Log) (*W3bstreamProverInitialized, error) {
	event := new(W3bstreamProverInitialized)
	if err := _W3bstreamProver.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProverMinterSetIterator is returned from FilterMinterSet and is used to iterate over the raw logs and unpacked data for MinterSet events raised by the W3bstreamProver contract.
type W3bstreamProverMinterSetIterator struct {
	Event *W3bstreamProverMinterSet // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverMinterSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverMinterSet)
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
		it.Event = new(W3bstreamProverMinterSet)
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
func (it *W3bstreamProverMinterSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverMinterSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverMinterSet represents a MinterSet event raised by the W3bstreamProver contract.
type W3bstreamProverMinterSet struct {
	Minter common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterMinterSet is a free log retrieval operation binding the contract event 0x726b590ef91a8c76ad05bbe91a57ef84605276528f49cd47d787f558a4e755b6.
//
// Solidity: event MinterSet(address minter)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterMinterSet(opts *bind.FilterOpts) (*W3bstreamProverMinterSetIterator, error) {

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "MinterSet")
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverMinterSetIterator{contract: _W3bstreamProver.contract, event: "MinterSet", logs: logs, sub: sub}, nil
}

// WatchMinterSet is a free log subscription operation binding the contract event 0x726b590ef91a8c76ad05bbe91a57ef84605276528f49cd47d787f558a4e755b6.
//
// Solidity: event MinterSet(address minter)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchMinterSet(opts *bind.WatchOpts, sink chan<- *W3bstreamProverMinterSet) (event.Subscription, error) {

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "MinterSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverMinterSet)
				if err := _W3bstreamProver.contract.UnpackLog(event, "MinterSet", log); err != nil {
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
// Solidity: event MinterSet(address minter)
func (_W3bstreamProver *W3bstreamProverFilterer) ParseMinterSet(log types.Log) (*W3bstreamProverMinterSet, error) {
	event := new(W3bstreamProverMinterSet)
	if err := _W3bstreamProver.contract.UnpackLog(event, "MinterSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProverOperatorSetIterator is returned from FilterOperatorSet and is used to iterate over the raw logs and unpacked data for OperatorSet events raised by the W3bstreamProver contract.
type W3bstreamProverOperatorSetIterator struct {
	Event *W3bstreamProverOperatorSet // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverOperatorSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverOperatorSet)
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
		it.Event = new(W3bstreamProverOperatorSet)
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
func (it *W3bstreamProverOperatorSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverOperatorSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverOperatorSet represents a OperatorSet event raised by the W3bstreamProver contract.
type W3bstreamProverOperatorSet struct {
	Id       *big.Int
	Operator common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterOperatorSet is a free log retrieval operation binding the contract event 0x712369dba77e7931b9ec3bd57319108256b9f79ea5b5255122e3c06117421593.
//
// Solidity: event OperatorSet(uint256 indexed id, address indexed operator)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterOperatorSet(opts *bind.FilterOpts, id []*big.Int, operator []common.Address) (*W3bstreamProverOperatorSetIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "OperatorSet", idRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverOperatorSetIterator{contract: _W3bstreamProver.contract, event: "OperatorSet", logs: logs, sub: sub}, nil
}

// WatchOperatorSet is a free log subscription operation binding the contract event 0x712369dba77e7931b9ec3bd57319108256b9f79ea5b5255122e3c06117421593.
//
// Solidity: event OperatorSet(uint256 indexed id, address indexed operator)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchOperatorSet(opts *bind.WatchOpts, sink chan<- *W3bstreamProverOperatorSet, id []*big.Int, operator []common.Address) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "OperatorSet", idRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverOperatorSet)
				if err := _W3bstreamProver.contract.UnpackLog(event, "OperatorSet", log); err != nil {
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

// ParseOperatorSet is a log parse operation binding the contract event 0x712369dba77e7931b9ec3bd57319108256b9f79ea5b5255122e3c06117421593.
//
// Solidity: event OperatorSet(uint256 indexed id, address indexed operator)
func (_W3bstreamProver *W3bstreamProverFilterer) ParseOperatorSet(log types.Log) (*W3bstreamProverOperatorSet, error) {
	event := new(W3bstreamProverOperatorSet)
	if err := _W3bstreamProver.contract.UnpackLog(event, "OperatorSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProverOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the W3bstreamProver contract.
type W3bstreamProverOwnershipTransferredIterator struct {
	Event *W3bstreamProverOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverOwnershipTransferred)
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
		it.Event = new(W3bstreamProverOwnershipTransferred)
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
func (it *W3bstreamProverOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverOwnershipTransferred represents a OwnershipTransferred event raised by the W3bstreamProver contract.
type W3bstreamProverOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*W3bstreamProverOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverOwnershipTransferredIterator{contract: _W3bstreamProver.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *W3bstreamProverOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverOwnershipTransferred)
				if err := _W3bstreamProver.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_W3bstreamProver *W3bstreamProverFilterer) ParseOwnershipTransferred(log types.Log) (*W3bstreamProverOwnershipTransferred, error) {
	event := new(W3bstreamProverOwnershipTransferred)
	if err := _W3bstreamProver.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProverProverPausedIterator is returned from FilterProverPaused and is used to iterate over the raw logs and unpacked data for ProverPaused events raised by the W3bstreamProver contract.
type W3bstreamProverProverPausedIterator struct {
	Event *W3bstreamProverProverPaused // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverProverPausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverProverPaused)
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
		it.Event = new(W3bstreamProverProverPaused)
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
func (it *W3bstreamProverProverPausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverProverPausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverProverPaused represents a ProverPaused event raised by the W3bstreamProver contract.
type W3bstreamProverProverPaused struct {
	Id  *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterProverPaused is a free log retrieval operation binding the contract event 0x09c10a851184c6f4c4f912c821413d9b27d48061ecf90d270551f40a23131a88.
//
// Solidity: event ProverPaused(uint256 indexed id)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterProverPaused(opts *bind.FilterOpts, id []*big.Int) (*W3bstreamProverProverPausedIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "ProverPaused", idRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverProverPausedIterator{contract: _W3bstreamProver.contract, event: "ProverPaused", logs: logs, sub: sub}, nil
}

// WatchProverPaused is a free log subscription operation binding the contract event 0x09c10a851184c6f4c4f912c821413d9b27d48061ecf90d270551f40a23131a88.
//
// Solidity: event ProverPaused(uint256 indexed id)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchProverPaused(opts *bind.WatchOpts, sink chan<- *W3bstreamProverProverPaused, id []*big.Int) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "ProverPaused", idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverProverPaused)
				if err := _W3bstreamProver.contract.UnpackLog(event, "ProverPaused", log); err != nil {
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

// ParseProverPaused is a log parse operation binding the contract event 0x09c10a851184c6f4c4f912c821413d9b27d48061ecf90d270551f40a23131a88.
//
// Solidity: event ProverPaused(uint256 indexed id)
func (_W3bstreamProver *W3bstreamProverFilterer) ParseProverPaused(log types.Log) (*W3bstreamProverProverPaused, error) {
	event := new(W3bstreamProverProverPaused)
	if err := _W3bstreamProver.contract.UnpackLog(event, "ProverPaused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProverProverResumedIterator is returned from FilterProverResumed and is used to iterate over the raw logs and unpacked data for ProverResumed events raised by the W3bstreamProver contract.
type W3bstreamProverProverResumedIterator struct {
	Event *W3bstreamProverProverResumed // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverProverResumedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverProverResumed)
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
		it.Event = new(W3bstreamProverProverResumed)
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
func (it *W3bstreamProverProverResumedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverProverResumedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverProverResumed represents a ProverResumed event raised by the W3bstreamProver contract.
type W3bstreamProverProverResumed struct {
	Id  *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterProverResumed is a free log retrieval operation binding the contract event 0xd5c12038aca4e36d3193c55c06f70eee8f829f1165a9e383c70b00d28e3bfdb9.
//
// Solidity: event ProverResumed(uint256 indexed id)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterProverResumed(opts *bind.FilterOpts, id []*big.Int) (*W3bstreamProverProverResumedIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "ProverResumed", idRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverProverResumedIterator{contract: _W3bstreamProver.contract, event: "ProverResumed", logs: logs, sub: sub}, nil
}

// WatchProverResumed is a free log subscription operation binding the contract event 0xd5c12038aca4e36d3193c55c06f70eee8f829f1165a9e383c70b00d28e3bfdb9.
//
// Solidity: event ProverResumed(uint256 indexed id)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchProverResumed(opts *bind.WatchOpts, sink chan<- *W3bstreamProverProverResumed, id []*big.Int) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "ProverResumed", idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverProverResumed)
				if err := _W3bstreamProver.contract.UnpackLog(event, "ProverResumed", log); err != nil {
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

// ParseProverResumed is a log parse operation binding the contract event 0xd5c12038aca4e36d3193c55c06f70eee8f829f1165a9e383c70b00d28e3bfdb9.
//
// Solidity: event ProverResumed(uint256 indexed id)
func (_W3bstreamProver *W3bstreamProverFilterer) ParseProverResumed(log types.Log) (*W3bstreamProverProverResumed, error) {
	event := new(W3bstreamProverProverResumed)
	if err := _W3bstreamProver.contract.UnpackLog(event, "ProverResumed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProverTransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the W3bstreamProver contract.
type W3bstreamProverTransferIterator struct {
	Event *W3bstreamProverTransfer // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverTransfer)
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
		it.Event = new(W3bstreamProverTransfer)
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
func (it *W3bstreamProverTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverTransfer represents a Transfer event raised by the W3bstreamProver contract.
type W3bstreamProverTransfer struct {
	From    common.Address
	To      common.Address
	TokenId *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterTransfer(opts *bind.FilterOpts, from []common.Address, to []common.Address, tokenId []*big.Int) (*W3bstreamProverTransferIterator, error) {

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

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "Transfer", fromRule, toRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverTransferIterator{contract: _W3bstreamProver.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *W3bstreamProverTransfer, from []common.Address, to []common.Address, tokenId []*big.Int) (event.Subscription, error) {

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

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "Transfer", fromRule, toRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverTransfer)
				if err := _W3bstreamProver.contract.UnpackLog(event, "Transfer", log); err != nil {
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
func (_W3bstreamProver *W3bstreamProverFilterer) ParseTransfer(log types.Log) (*W3bstreamProverTransfer, error) {
	event := new(W3bstreamProverTransfer)
	if err := _W3bstreamProver.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProverVMTypeAddedIterator is returned from FilterVMTypeAdded and is used to iterate over the raw logs and unpacked data for VMTypeAdded events raised by the W3bstreamProver contract.
type W3bstreamProverVMTypeAddedIterator struct {
	Event *W3bstreamProverVMTypeAdded // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverVMTypeAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverVMTypeAdded)
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
		it.Event = new(W3bstreamProverVMTypeAdded)
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
func (it *W3bstreamProverVMTypeAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverVMTypeAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverVMTypeAdded represents a VMTypeAdded event raised by the W3bstreamProver contract.
type W3bstreamProverVMTypeAdded struct {
	Id  *big.Int
	Typ *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterVMTypeAdded is a free log retrieval operation binding the contract event 0x31d3cf7e64f8a54ff46174ebf94989d43bf8db12f6b2668940bc5653fea13ab4.
//
// Solidity: event VMTypeAdded(uint256 indexed id, uint256 typ)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterVMTypeAdded(opts *bind.FilterOpts, id []*big.Int) (*W3bstreamProverVMTypeAddedIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "VMTypeAdded", idRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverVMTypeAddedIterator{contract: _W3bstreamProver.contract, event: "VMTypeAdded", logs: logs, sub: sub}, nil
}

// WatchVMTypeAdded is a free log subscription operation binding the contract event 0x31d3cf7e64f8a54ff46174ebf94989d43bf8db12f6b2668940bc5653fea13ab4.
//
// Solidity: event VMTypeAdded(uint256 indexed id, uint256 typ)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchVMTypeAdded(opts *bind.WatchOpts, sink chan<- *W3bstreamProverVMTypeAdded, id []*big.Int) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "VMTypeAdded", idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverVMTypeAdded)
				if err := _W3bstreamProver.contract.UnpackLog(event, "VMTypeAdded", log); err != nil {
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

// ParseVMTypeAdded is a log parse operation binding the contract event 0x31d3cf7e64f8a54ff46174ebf94989d43bf8db12f6b2668940bc5653fea13ab4.
//
// Solidity: event VMTypeAdded(uint256 indexed id, uint256 typ)
func (_W3bstreamProver *W3bstreamProverFilterer) ParseVMTypeAdded(log types.Log) (*W3bstreamProverVMTypeAdded, error) {
	event := new(W3bstreamProverVMTypeAdded)
	if err := _W3bstreamProver.contract.UnpackLog(event, "VMTypeAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamProverVMTypeDeletedIterator is returned from FilterVMTypeDeleted and is used to iterate over the raw logs and unpacked data for VMTypeDeleted events raised by the W3bstreamProver contract.
type W3bstreamProverVMTypeDeletedIterator struct {
	Event *W3bstreamProverVMTypeDeleted // Event containing the contract specifics and raw log

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
func (it *W3bstreamProverVMTypeDeletedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamProverVMTypeDeleted)
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
		it.Event = new(W3bstreamProverVMTypeDeleted)
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
func (it *W3bstreamProverVMTypeDeletedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamProverVMTypeDeletedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamProverVMTypeDeleted represents a VMTypeDeleted event raised by the W3bstreamProver contract.
type W3bstreamProverVMTypeDeleted struct {
	Id  *big.Int
	Typ *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterVMTypeDeleted is a free log retrieval operation binding the contract event 0xa52940e84e8bab7c0aef4e1446a2893c726b08d30eba0a03d1ba969f0852cf46.
//
// Solidity: event VMTypeDeleted(uint256 indexed id, uint256 typ)
func (_W3bstreamProver *W3bstreamProverFilterer) FilterVMTypeDeleted(opts *bind.FilterOpts, id []*big.Int) (*W3bstreamProverVMTypeDeletedIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamProver.contract.FilterLogs(opts, "VMTypeDeleted", idRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamProverVMTypeDeletedIterator{contract: _W3bstreamProver.contract, event: "VMTypeDeleted", logs: logs, sub: sub}, nil
}

// WatchVMTypeDeleted is a free log subscription operation binding the contract event 0xa52940e84e8bab7c0aef4e1446a2893c726b08d30eba0a03d1ba969f0852cf46.
//
// Solidity: event VMTypeDeleted(uint256 indexed id, uint256 typ)
func (_W3bstreamProver *W3bstreamProverFilterer) WatchVMTypeDeleted(opts *bind.WatchOpts, sink chan<- *W3bstreamProverVMTypeDeleted, id []*big.Int) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamProver.contract.WatchLogs(opts, "VMTypeDeleted", idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamProverVMTypeDeleted)
				if err := _W3bstreamProver.contract.UnpackLog(event, "VMTypeDeleted", log); err != nil {
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

// ParseVMTypeDeleted is a log parse operation binding the contract event 0xa52940e84e8bab7c0aef4e1446a2893c726b08d30eba0a03d1ba969f0852cf46.
//
// Solidity: event VMTypeDeleted(uint256 indexed id, uint256 typ)
func (_W3bstreamProver *W3bstreamProverFilterer) ParseVMTypeDeleted(log types.Log) (*W3bstreamProverVMTypeDeleted, error) {
	event := new(W3bstreamProverVMTypeDeleted)
	if err := _W3bstreamProver.contract.UnpackLog(event, "VMTypeDeleted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

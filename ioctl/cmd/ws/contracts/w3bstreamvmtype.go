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

// W3bstreamVMTypeMetaData contains all meta data concerning the W3bstreamVMType contract.
var W3bstreamVMTypeMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"approved\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"Approval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"approved\",\"type\":\"bool\"}],\"name\":\"ApprovalForAll\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"VMTypePaused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"VMTypeResumed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"VMTypeSet\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"count\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"getApproved\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"_name\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"_symbol\",\"type\":\"string\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isApprovedForAll\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"}],\"name\":\"isPaused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"_name\",\"type\":\"string\"}],\"name\":\"mint\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"id_\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"ownerOf\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"}],\"name\":\"pause\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"}],\"name\":\"resume\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"safeTransferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"safeTransferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"approved\",\"type\":\"bool\"}],\"name\":\"setApprovalForAll\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes4\",\"name\":\"interfaceId\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"tokenURI\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"transferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"}],\"name\":\"vmTypeName\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// W3bstreamVMTypeABI is the input ABI used to generate the binding from.
// Deprecated: Use W3bstreamVMTypeMetaData.ABI instead.
var W3bstreamVMTypeABI = W3bstreamVMTypeMetaData.ABI

// W3bstreamVMType is an auto generated Go binding around an Ethereum contract.
type W3bstreamVMType struct {
	W3bstreamVMTypeCaller     // Read-only binding to the contract
	W3bstreamVMTypeTransactor // Write-only binding to the contract
	W3bstreamVMTypeFilterer   // Log filterer for contract events
}

// W3bstreamVMTypeCaller is an auto generated read-only Go binding around an Ethereum contract.
type W3bstreamVMTypeCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamVMTypeTransactor is an auto generated write-only Go binding around an Ethereum contract.
type W3bstreamVMTypeTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamVMTypeFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type W3bstreamVMTypeFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamVMTypeSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type W3bstreamVMTypeSession struct {
	Contract     *W3bstreamVMType  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// W3bstreamVMTypeCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type W3bstreamVMTypeCallerSession struct {
	Contract *W3bstreamVMTypeCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// W3bstreamVMTypeTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type W3bstreamVMTypeTransactorSession struct {
	Contract     *W3bstreamVMTypeTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// W3bstreamVMTypeRaw is an auto generated low-level Go binding around an Ethereum contract.
type W3bstreamVMTypeRaw struct {
	Contract *W3bstreamVMType // Generic contract binding to access the raw methods on
}

// W3bstreamVMTypeCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type W3bstreamVMTypeCallerRaw struct {
	Contract *W3bstreamVMTypeCaller // Generic read-only contract binding to access the raw methods on
}

// W3bstreamVMTypeTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type W3bstreamVMTypeTransactorRaw struct {
	Contract *W3bstreamVMTypeTransactor // Generic write-only contract binding to access the raw methods on
}

// NewW3bstreamVMType creates a new instance of W3bstreamVMType, bound to a specific deployed contract.
func NewW3bstreamVMType(address common.Address, backend bind.ContractBackend) (*W3bstreamVMType, error) {
	contract, err := bindW3bstreamVMType(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMType{W3bstreamVMTypeCaller: W3bstreamVMTypeCaller{contract: contract}, W3bstreamVMTypeTransactor: W3bstreamVMTypeTransactor{contract: contract}, W3bstreamVMTypeFilterer: W3bstreamVMTypeFilterer{contract: contract}}, nil
}

// NewW3bstreamVMTypeCaller creates a new read-only instance of W3bstreamVMType, bound to a specific deployed contract.
func NewW3bstreamVMTypeCaller(address common.Address, caller bind.ContractCaller) (*W3bstreamVMTypeCaller, error) {
	contract, err := bindW3bstreamVMType(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeCaller{contract: contract}, nil
}

// NewW3bstreamVMTypeTransactor creates a new write-only instance of W3bstreamVMType, bound to a specific deployed contract.
func NewW3bstreamVMTypeTransactor(address common.Address, transactor bind.ContractTransactor) (*W3bstreamVMTypeTransactor, error) {
	contract, err := bindW3bstreamVMType(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeTransactor{contract: contract}, nil
}

// NewW3bstreamVMTypeFilterer creates a new log filterer instance of W3bstreamVMType, bound to a specific deployed contract.
func NewW3bstreamVMTypeFilterer(address common.Address, filterer bind.ContractFilterer) (*W3bstreamVMTypeFilterer, error) {
	contract, err := bindW3bstreamVMType(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeFilterer{contract: contract}, nil
}

// bindW3bstreamVMType binds a generic wrapper to an already deployed contract.
func bindW3bstreamVMType(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := W3bstreamVMTypeMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_W3bstreamVMType *W3bstreamVMTypeRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _W3bstreamVMType.Contract.W3bstreamVMTypeCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_W3bstreamVMType *W3bstreamVMTypeRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.W3bstreamVMTypeTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_W3bstreamVMType *W3bstreamVMTypeRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.W3bstreamVMTypeTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_W3bstreamVMType *W3bstreamVMTypeCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _W3bstreamVMType.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_W3bstreamVMType *W3bstreamVMTypeTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_W3bstreamVMType *W3bstreamVMTypeTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.contract.Transact(opts, method, params...)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) BalanceOf(opts *bind.CallOpts, owner common.Address) (*big.Int, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "balanceOf", owner)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_W3bstreamVMType *W3bstreamVMTypeSession) BalanceOf(owner common.Address) (*big.Int, error) {
	return _W3bstreamVMType.Contract.BalanceOf(&_W3bstreamVMType.CallOpts, owner)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) BalanceOf(owner common.Address) (*big.Int, error) {
	return _W3bstreamVMType.Contract.BalanceOf(&_W3bstreamVMType.CallOpts, owner)
}

// Count is a free data retrieval call binding the contract method 0x06661abd.
//
// Solidity: function count() view returns(uint256)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) Count(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "count")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Count is a free data retrieval call binding the contract method 0x06661abd.
//
// Solidity: function count() view returns(uint256)
func (_W3bstreamVMType *W3bstreamVMTypeSession) Count() (*big.Int, error) {
	return _W3bstreamVMType.Contract.Count(&_W3bstreamVMType.CallOpts)
}

// Count is a free data retrieval call binding the contract method 0x06661abd.
//
// Solidity: function count() view returns(uint256)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) Count() (*big.Int, error) {
	return _W3bstreamVMType.Contract.Count(&_W3bstreamVMType.CallOpts)
}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) GetApproved(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "getApproved", tokenId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_W3bstreamVMType *W3bstreamVMTypeSession) GetApproved(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamVMType.Contract.GetApproved(&_W3bstreamVMType.CallOpts, tokenId)
}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) GetApproved(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamVMType.Contract.GetApproved(&_W3bstreamVMType.CallOpts, tokenId)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) IsApprovedForAll(opts *bind.CallOpts, owner common.Address, operator common.Address) (bool, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "isApprovedForAll", owner, operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_W3bstreamVMType *W3bstreamVMTypeSession) IsApprovedForAll(owner common.Address, operator common.Address) (bool, error) {
	return _W3bstreamVMType.Contract.IsApprovedForAll(&_W3bstreamVMType.CallOpts, owner, operator)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) IsApprovedForAll(owner common.Address, operator common.Address) (bool, error) {
	return _W3bstreamVMType.Contract.IsApprovedForAll(&_W3bstreamVMType.CallOpts, owner, operator)
}

// IsPaused is a free data retrieval call binding the contract method 0xbdf2a43c.
//
// Solidity: function isPaused(uint256 _id) view returns(bool)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) IsPaused(opts *bind.CallOpts, _id *big.Int) (bool, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "isPaused", _id)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsPaused is a free data retrieval call binding the contract method 0xbdf2a43c.
//
// Solidity: function isPaused(uint256 _id) view returns(bool)
func (_W3bstreamVMType *W3bstreamVMTypeSession) IsPaused(_id *big.Int) (bool, error) {
	return _W3bstreamVMType.Contract.IsPaused(&_W3bstreamVMType.CallOpts, _id)
}

// IsPaused is a free data retrieval call binding the contract method 0xbdf2a43c.
//
// Solidity: function isPaused(uint256 _id) view returns(bool)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) IsPaused(_id *big.Int) (bool, error) {
	return _W3bstreamVMType.Contract.IsPaused(&_W3bstreamVMType.CallOpts, _id)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeSession) Name() (string, error) {
	return _W3bstreamVMType.Contract.Name(&_W3bstreamVMType.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) Name() (string, error) {
	return _W3bstreamVMType.Contract.Name(&_W3bstreamVMType.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_W3bstreamVMType *W3bstreamVMTypeSession) Owner() (common.Address, error) {
	return _W3bstreamVMType.Contract.Owner(&_W3bstreamVMType.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) Owner() (common.Address, error) {
	return _W3bstreamVMType.Contract.Owner(&_W3bstreamVMType.CallOpts)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) OwnerOf(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "ownerOf", tokenId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_W3bstreamVMType *W3bstreamVMTypeSession) OwnerOf(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamVMType.Contract.OwnerOf(&_W3bstreamVMType.CallOpts, tokenId)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) OwnerOf(tokenId *big.Int) (common.Address, error) {
	return _W3bstreamVMType.Contract.OwnerOf(&_W3bstreamVMType.CallOpts, tokenId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_W3bstreamVMType *W3bstreamVMTypeSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _W3bstreamVMType.Contract.SupportsInterface(&_W3bstreamVMType.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _W3bstreamVMType.Contract.SupportsInterface(&_W3bstreamVMType.CallOpts, interfaceId)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeSession) Symbol() (string, error) {
	return _W3bstreamVMType.Contract.Symbol(&_W3bstreamVMType.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) Symbol() (string, error) {
	return _W3bstreamVMType.Contract.Symbol(&_W3bstreamVMType.CallOpts)
}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) TokenURI(opts *bind.CallOpts, tokenId *big.Int) (string, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "tokenURI", tokenId)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeSession) TokenURI(tokenId *big.Int) (string, error) {
	return _W3bstreamVMType.Contract.TokenURI(&_W3bstreamVMType.CallOpts, tokenId)
}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) TokenURI(tokenId *big.Int) (string, error) {
	return _W3bstreamVMType.Contract.TokenURI(&_W3bstreamVMType.CallOpts, tokenId)
}

// VmTypeName is a free data retrieval call binding the contract method 0xfad40382.
//
// Solidity: function vmTypeName(uint256 _id) view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeCaller) VmTypeName(opts *bind.CallOpts, _id *big.Int) (string, error) {
	var out []interface{}
	err := _W3bstreamVMType.contract.Call(opts, &out, "vmTypeName", _id)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// VmTypeName is a free data retrieval call binding the contract method 0xfad40382.
//
// Solidity: function vmTypeName(uint256 _id) view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeSession) VmTypeName(_id *big.Int) (string, error) {
	return _W3bstreamVMType.Contract.VmTypeName(&_W3bstreamVMType.CallOpts, _id)
}

// VmTypeName is a free data retrieval call binding the contract method 0xfad40382.
//
// Solidity: function vmTypeName(uint256 _id) view returns(string)
func (_W3bstreamVMType *W3bstreamVMTypeCallerSession) VmTypeName(_id *big.Int) (string, error) {
	return _W3bstreamVMType.Contract.VmTypeName(&_W3bstreamVMType.CallOpts, _id)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) Approve(opts *bind.TransactOpts, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "approve", to, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_W3bstreamVMType *W3bstreamVMTypeSession) Approve(to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.Approve(&_W3bstreamVMType.TransactOpts, to, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) Approve(to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.Approve(&_W3bstreamVMType.TransactOpts, to, tokenId)
}

// Initialize is a paid mutator transaction binding the contract method 0x4cd88b76.
//
// Solidity: function initialize(string _name, string _symbol) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) Initialize(opts *bind.TransactOpts, _name string, _symbol string) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "initialize", _name, _symbol)
}

// Initialize is a paid mutator transaction binding the contract method 0x4cd88b76.
//
// Solidity: function initialize(string _name, string _symbol) returns()
func (_W3bstreamVMType *W3bstreamVMTypeSession) Initialize(_name string, _symbol string) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.Initialize(&_W3bstreamVMType.TransactOpts, _name, _symbol)
}

// Initialize is a paid mutator transaction binding the contract method 0x4cd88b76.
//
// Solidity: function initialize(string _name, string _symbol) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) Initialize(_name string, _symbol string) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.Initialize(&_W3bstreamVMType.TransactOpts, _name, _symbol)
}

// Mint is a paid mutator transaction binding the contract method 0xd85d3d27.
//
// Solidity: function mint(string _name) returns(uint256 id_)
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) Mint(opts *bind.TransactOpts, _name string) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "mint", _name)
}

// Mint is a paid mutator transaction binding the contract method 0xd85d3d27.
//
// Solidity: function mint(string _name) returns(uint256 id_)
func (_W3bstreamVMType *W3bstreamVMTypeSession) Mint(_name string) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.Mint(&_W3bstreamVMType.TransactOpts, _name)
}

// Mint is a paid mutator transaction binding the contract method 0xd85d3d27.
//
// Solidity: function mint(string _name) returns(uint256 id_)
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) Mint(_name string) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.Mint(&_W3bstreamVMType.TransactOpts, _name)
}

// Pause is a paid mutator transaction binding the contract method 0x136439dd.
//
// Solidity: function pause(uint256 _id) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) Pause(opts *bind.TransactOpts, _id *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "pause", _id)
}

// Pause is a paid mutator transaction binding the contract method 0x136439dd.
//
// Solidity: function pause(uint256 _id) returns()
func (_W3bstreamVMType *W3bstreamVMTypeSession) Pause(_id *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.Pause(&_W3bstreamVMType.TransactOpts, _id)
}

// Pause is a paid mutator transaction binding the contract method 0x136439dd.
//
// Solidity: function pause(uint256 _id) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) Pause(_id *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.Pause(&_W3bstreamVMType.TransactOpts, _id)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_W3bstreamVMType *W3bstreamVMTypeSession) RenounceOwnership() (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.RenounceOwnership(&_W3bstreamVMType.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.RenounceOwnership(&_W3bstreamVMType.TransactOpts)
}

// Resume is a paid mutator transaction binding the contract method 0x414000b5.
//
// Solidity: function resume(uint256 _id) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) Resume(opts *bind.TransactOpts, _id *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "resume", _id)
}

// Resume is a paid mutator transaction binding the contract method 0x414000b5.
//
// Solidity: function resume(uint256 _id) returns()
func (_W3bstreamVMType *W3bstreamVMTypeSession) Resume(_id *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.Resume(&_W3bstreamVMType.TransactOpts, _id)
}

// Resume is a paid mutator transaction binding the contract method 0x414000b5.
//
// Solidity: function resume(uint256 _id) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) Resume(_id *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.Resume(&_W3bstreamVMType.TransactOpts, _id)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) SafeTransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "safeTransferFrom", from, to, tokenId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamVMType *W3bstreamVMTypeSession) SafeTransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.SafeTransferFrom(&_W3bstreamVMType.TransactOpts, from, to, tokenId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) SafeTransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.SafeTransferFrom(&_W3bstreamVMType.TransactOpts, from, to, tokenId)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) SafeTransferFrom0(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "safeTransferFrom0", from, to, tokenId, data)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_W3bstreamVMType *W3bstreamVMTypeSession) SafeTransferFrom0(from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.SafeTransferFrom0(&_W3bstreamVMType.TransactOpts, from, to, tokenId, data)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) SafeTransferFrom0(from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.SafeTransferFrom0(&_W3bstreamVMType.TransactOpts, from, to, tokenId, data)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) SetApprovalForAll(opts *bind.TransactOpts, operator common.Address, approved bool) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "setApprovalForAll", operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_W3bstreamVMType *W3bstreamVMTypeSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.SetApprovalForAll(&_W3bstreamVMType.TransactOpts, operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.SetApprovalForAll(&_W3bstreamVMType.TransactOpts, operator, approved)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "transferFrom", from, to, tokenId)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamVMType *W3bstreamVMTypeSession) TransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.TransferFrom(&_W3bstreamVMType.TransactOpts, from, to, tokenId)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) TransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.TransferFrom(&_W3bstreamVMType.TransactOpts, from, to, tokenId)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _W3bstreamVMType.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_W3bstreamVMType *W3bstreamVMTypeSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.TransferOwnership(&_W3bstreamVMType.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_W3bstreamVMType *W3bstreamVMTypeTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _W3bstreamVMType.Contract.TransferOwnership(&_W3bstreamVMType.TransactOpts, newOwner)
}

// W3bstreamVMTypeApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the W3bstreamVMType contract.
type W3bstreamVMTypeApprovalIterator struct {
	Event *W3bstreamVMTypeApproval // Event containing the contract specifics and raw log

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
func (it *W3bstreamVMTypeApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamVMTypeApproval)
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
		it.Event = new(W3bstreamVMTypeApproval)
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
func (it *W3bstreamVMTypeApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamVMTypeApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamVMTypeApproval represents a Approval event raised by the W3bstreamVMType contract.
type W3bstreamVMTypeApproval struct {
	Owner    common.Address
	Approved common.Address
	TokenId  *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) FilterApproval(opts *bind.FilterOpts, owner []common.Address, approved []common.Address, tokenId []*big.Int) (*W3bstreamVMTypeApprovalIterator, error) {

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

	logs, sub, err := _W3bstreamVMType.contract.FilterLogs(opts, "Approval", ownerRule, approvedRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeApprovalIterator{contract: _W3bstreamVMType.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *W3bstreamVMTypeApproval, owner []common.Address, approved []common.Address, tokenId []*big.Int) (event.Subscription, error) {

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

	logs, sub, err := _W3bstreamVMType.contract.WatchLogs(opts, "Approval", ownerRule, approvedRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamVMTypeApproval)
				if err := _W3bstreamVMType.contract.UnpackLog(event, "Approval", log); err != nil {
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
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) ParseApproval(log types.Log) (*W3bstreamVMTypeApproval, error) {
	event := new(W3bstreamVMTypeApproval)
	if err := _W3bstreamVMType.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamVMTypeApprovalForAllIterator is returned from FilterApprovalForAll and is used to iterate over the raw logs and unpacked data for ApprovalForAll events raised by the W3bstreamVMType contract.
type W3bstreamVMTypeApprovalForAllIterator struct {
	Event *W3bstreamVMTypeApprovalForAll // Event containing the contract specifics and raw log

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
func (it *W3bstreamVMTypeApprovalForAllIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamVMTypeApprovalForAll)
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
		it.Event = new(W3bstreamVMTypeApprovalForAll)
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
func (it *W3bstreamVMTypeApprovalForAllIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamVMTypeApprovalForAllIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamVMTypeApprovalForAll represents a ApprovalForAll event raised by the W3bstreamVMType contract.
type W3bstreamVMTypeApprovalForAll struct {
	Owner    common.Address
	Operator common.Address
	Approved bool
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApprovalForAll is a free log retrieval operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) FilterApprovalForAll(opts *bind.FilterOpts, owner []common.Address, operator []common.Address) (*W3bstreamVMTypeApprovalForAllIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamVMType.contract.FilterLogs(opts, "ApprovalForAll", ownerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeApprovalForAllIterator{contract: _W3bstreamVMType.contract, event: "ApprovalForAll", logs: logs, sub: sub}, nil
}

// WatchApprovalForAll is a free log subscription operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) WatchApprovalForAll(opts *bind.WatchOpts, sink chan<- *W3bstreamVMTypeApprovalForAll, owner []common.Address, operator []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamVMType.contract.WatchLogs(opts, "ApprovalForAll", ownerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamVMTypeApprovalForAll)
				if err := _W3bstreamVMType.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
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
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) ParseApprovalForAll(log types.Log) (*W3bstreamVMTypeApprovalForAll, error) {
	event := new(W3bstreamVMTypeApprovalForAll)
	if err := _W3bstreamVMType.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamVMTypeInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the W3bstreamVMType contract.
type W3bstreamVMTypeInitializedIterator struct {
	Event *W3bstreamVMTypeInitialized // Event containing the contract specifics and raw log

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
func (it *W3bstreamVMTypeInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamVMTypeInitialized)
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
		it.Event = new(W3bstreamVMTypeInitialized)
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
func (it *W3bstreamVMTypeInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamVMTypeInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamVMTypeInitialized represents a Initialized event raised by the W3bstreamVMType contract.
type W3bstreamVMTypeInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) FilterInitialized(opts *bind.FilterOpts) (*W3bstreamVMTypeInitializedIterator, error) {

	logs, sub, err := _W3bstreamVMType.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeInitializedIterator{contract: _W3bstreamVMType.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *W3bstreamVMTypeInitialized) (event.Subscription, error) {

	logs, sub, err := _W3bstreamVMType.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamVMTypeInitialized)
				if err := _W3bstreamVMType.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) ParseInitialized(log types.Log) (*W3bstreamVMTypeInitialized, error) {
	event := new(W3bstreamVMTypeInitialized)
	if err := _W3bstreamVMType.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamVMTypeOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the W3bstreamVMType contract.
type W3bstreamVMTypeOwnershipTransferredIterator struct {
	Event *W3bstreamVMTypeOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *W3bstreamVMTypeOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamVMTypeOwnershipTransferred)
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
		it.Event = new(W3bstreamVMTypeOwnershipTransferred)
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
func (it *W3bstreamVMTypeOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamVMTypeOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamVMTypeOwnershipTransferred represents a OwnershipTransferred event raised by the W3bstreamVMType contract.
type W3bstreamVMTypeOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*W3bstreamVMTypeOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _W3bstreamVMType.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeOwnershipTransferredIterator{contract: _W3bstreamVMType.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *W3bstreamVMTypeOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _W3bstreamVMType.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamVMTypeOwnershipTransferred)
				if err := _W3bstreamVMType.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) ParseOwnershipTransferred(log types.Log) (*W3bstreamVMTypeOwnershipTransferred, error) {
	event := new(W3bstreamVMTypeOwnershipTransferred)
	if err := _W3bstreamVMType.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamVMTypeTransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the W3bstreamVMType contract.
type W3bstreamVMTypeTransferIterator struct {
	Event *W3bstreamVMTypeTransfer // Event containing the contract specifics and raw log

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
func (it *W3bstreamVMTypeTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamVMTypeTransfer)
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
		it.Event = new(W3bstreamVMTypeTransfer)
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
func (it *W3bstreamVMTypeTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamVMTypeTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamVMTypeTransfer represents a Transfer event raised by the W3bstreamVMType contract.
type W3bstreamVMTypeTransfer struct {
	From    common.Address
	To      common.Address
	TokenId *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) FilterTransfer(opts *bind.FilterOpts, from []common.Address, to []common.Address, tokenId []*big.Int) (*W3bstreamVMTypeTransferIterator, error) {

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

	logs, sub, err := _W3bstreamVMType.contract.FilterLogs(opts, "Transfer", fromRule, toRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeTransferIterator{contract: _W3bstreamVMType.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *W3bstreamVMTypeTransfer, from []common.Address, to []common.Address, tokenId []*big.Int) (event.Subscription, error) {

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

	logs, sub, err := _W3bstreamVMType.contract.WatchLogs(opts, "Transfer", fromRule, toRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamVMTypeTransfer)
				if err := _W3bstreamVMType.contract.UnpackLog(event, "Transfer", log); err != nil {
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
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) ParseTransfer(log types.Log) (*W3bstreamVMTypeTransfer, error) {
	event := new(W3bstreamVMTypeTransfer)
	if err := _W3bstreamVMType.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamVMTypeVMTypePausedIterator is returned from FilterVMTypePaused and is used to iterate over the raw logs and unpacked data for VMTypePaused events raised by the W3bstreamVMType contract.
type W3bstreamVMTypeVMTypePausedIterator struct {
	Event *W3bstreamVMTypeVMTypePaused // Event containing the contract specifics and raw log

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
func (it *W3bstreamVMTypeVMTypePausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamVMTypeVMTypePaused)
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
		it.Event = new(W3bstreamVMTypeVMTypePaused)
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
func (it *W3bstreamVMTypeVMTypePausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamVMTypeVMTypePausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamVMTypeVMTypePaused represents a VMTypePaused event raised by the W3bstreamVMType contract.
type W3bstreamVMTypeVMTypePaused struct {
	Id  *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterVMTypePaused is a free log retrieval operation binding the contract event 0xd6800ce5ea4bba066159abb6480df7f6484cb48dcf57194f7fbb119182f0be93.
//
// Solidity: event VMTypePaused(uint256 indexed id)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) FilterVMTypePaused(opts *bind.FilterOpts, id []*big.Int) (*W3bstreamVMTypeVMTypePausedIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamVMType.contract.FilterLogs(opts, "VMTypePaused", idRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeVMTypePausedIterator{contract: _W3bstreamVMType.contract, event: "VMTypePaused", logs: logs, sub: sub}, nil
}

// WatchVMTypePaused is a free log subscription operation binding the contract event 0xd6800ce5ea4bba066159abb6480df7f6484cb48dcf57194f7fbb119182f0be93.
//
// Solidity: event VMTypePaused(uint256 indexed id)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) WatchVMTypePaused(opts *bind.WatchOpts, sink chan<- *W3bstreamVMTypeVMTypePaused, id []*big.Int) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamVMType.contract.WatchLogs(opts, "VMTypePaused", idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamVMTypeVMTypePaused)
				if err := _W3bstreamVMType.contract.UnpackLog(event, "VMTypePaused", log); err != nil {
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

// ParseVMTypePaused is a log parse operation binding the contract event 0xd6800ce5ea4bba066159abb6480df7f6484cb48dcf57194f7fbb119182f0be93.
//
// Solidity: event VMTypePaused(uint256 indexed id)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) ParseVMTypePaused(log types.Log) (*W3bstreamVMTypeVMTypePaused, error) {
	event := new(W3bstreamVMTypeVMTypePaused)
	if err := _W3bstreamVMType.contract.UnpackLog(event, "VMTypePaused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamVMTypeVMTypeResumedIterator is returned from FilterVMTypeResumed and is used to iterate over the raw logs and unpacked data for VMTypeResumed events raised by the W3bstreamVMType contract.
type W3bstreamVMTypeVMTypeResumedIterator struct {
	Event *W3bstreamVMTypeVMTypeResumed // Event containing the contract specifics and raw log

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
func (it *W3bstreamVMTypeVMTypeResumedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamVMTypeVMTypeResumed)
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
		it.Event = new(W3bstreamVMTypeVMTypeResumed)
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
func (it *W3bstreamVMTypeVMTypeResumedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamVMTypeVMTypeResumedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamVMTypeVMTypeResumed represents a VMTypeResumed event raised by the W3bstreamVMType contract.
type W3bstreamVMTypeVMTypeResumed struct {
	Id  *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterVMTypeResumed is a free log retrieval operation binding the contract event 0x7b2cfd5a3cf01b5a7a47661508884b5c0bb936c465a58bd9f685e991178c7673.
//
// Solidity: event VMTypeResumed(uint256 indexed id)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) FilterVMTypeResumed(opts *bind.FilterOpts, id []*big.Int) (*W3bstreamVMTypeVMTypeResumedIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamVMType.contract.FilterLogs(opts, "VMTypeResumed", idRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeVMTypeResumedIterator{contract: _W3bstreamVMType.contract, event: "VMTypeResumed", logs: logs, sub: sub}, nil
}

// WatchVMTypeResumed is a free log subscription operation binding the contract event 0x7b2cfd5a3cf01b5a7a47661508884b5c0bb936c465a58bd9f685e991178c7673.
//
// Solidity: event VMTypeResumed(uint256 indexed id)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) WatchVMTypeResumed(opts *bind.WatchOpts, sink chan<- *W3bstreamVMTypeVMTypeResumed, id []*big.Int) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamVMType.contract.WatchLogs(opts, "VMTypeResumed", idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamVMTypeVMTypeResumed)
				if err := _W3bstreamVMType.contract.UnpackLog(event, "VMTypeResumed", log); err != nil {
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

// ParseVMTypeResumed is a log parse operation binding the contract event 0x7b2cfd5a3cf01b5a7a47661508884b5c0bb936c465a58bd9f685e991178c7673.
//
// Solidity: event VMTypeResumed(uint256 indexed id)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) ParseVMTypeResumed(log types.Log) (*W3bstreamVMTypeVMTypeResumed, error) {
	event := new(W3bstreamVMTypeVMTypeResumed)
	if err := _W3bstreamVMType.contract.UnpackLog(event, "VMTypeResumed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamVMTypeVMTypeSetIterator is returned from FilterVMTypeSet and is used to iterate over the raw logs and unpacked data for VMTypeSet events raised by the W3bstreamVMType contract.
type W3bstreamVMTypeVMTypeSetIterator struct {
	Event *W3bstreamVMTypeVMTypeSet // Event containing the contract specifics and raw log

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
func (it *W3bstreamVMTypeVMTypeSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamVMTypeVMTypeSet)
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
		it.Event = new(W3bstreamVMTypeVMTypeSet)
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
func (it *W3bstreamVMTypeVMTypeSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamVMTypeVMTypeSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamVMTypeVMTypeSet represents a VMTypeSet event raised by the W3bstreamVMType contract.
type W3bstreamVMTypeVMTypeSet struct {
	Id  *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterVMTypeSet is a free log retrieval operation binding the contract event 0xbe89461c9f17800f078edad9f41b00b5ce425af848dfe6d4c53a95c0905ce96a.
//
// Solidity: event VMTypeSet(uint256 indexed id)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) FilterVMTypeSet(opts *bind.FilterOpts, id []*big.Int) (*W3bstreamVMTypeVMTypeSetIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamVMType.contract.FilterLogs(opts, "VMTypeSet", idRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamVMTypeVMTypeSetIterator{contract: _W3bstreamVMType.contract, event: "VMTypeSet", logs: logs, sub: sub}, nil
}

// WatchVMTypeSet is a free log subscription operation binding the contract event 0xbe89461c9f17800f078edad9f41b00b5ce425af848dfe6d4c53a95c0905ce96a.
//
// Solidity: event VMTypeSet(uint256 indexed id)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) WatchVMTypeSet(opts *bind.WatchOpts, sink chan<- *W3bstreamVMTypeVMTypeSet, id []*big.Int) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _W3bstreamVMType.contract.WatchLogs(opts, "VMTypeSet", idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamVMTypeVMTypeSet)
				if err := _W3bstreamVMType.contract.UnpackLog(event, "VMTypeSet", log); err != nil {
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

// ParseVMTypeSet is a log parse operation binding the contract event 0xbe89461c9f17800f078edad9f41b00b5ce425af848dfe6d4c53a95c0905ce96a.
//
// Solidity: event VMTypeSet(uint256 indexed id)
func (_W3bstreamVMType *W3bstreamVMTypeFilterer) ParseVMTypeSet(log types.Log) (*W3bstreamVMTypeVMTypeSet, error) {
	event := new(W3bstreamVMTypeVMTypeSet)
	if err := _W3bstreamVMType.contract.UnpackLog(event, "VMTypeSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

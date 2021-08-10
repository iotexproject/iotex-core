// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package poll

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
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// ConsortiumManagementABI is the input ABI used to generate the binding from.
const ConsortiumManagementABI = "[{\"inputs\":[{\"name\":\"orgs\",\"type\":\"address[]\"},{\"name\":\"capacities\",\"type\":\"uint8[]\"},{\"name\":\"delegates\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"id\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"proposer\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"ptype\",\"type\":\"uint8\"},{\"indexed\":false,\"name\":\"member\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"capacity\",\"type\":\"uint8\"}],\"name\":\"Propose\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"id\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"success\",\"type\":\"bool\"}],\"name\":\"SettleProposal\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"id\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"voter\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"approve\",\"type\":\"bool\"}],\"name\":\"Vote\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"member\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"endpoint\",\"type\":\"string\"}],\"name\":\"AddOperator\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"member\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"RemoveOperator\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"member\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"endpoint\",\"type\":\"string\"}],\"name\":\"UpdateOperatorHealthEndpoint\",\"type\":\"event\"},{\"constant\":true,\"inputs\":[],\"name\":\"members\",\"outputs\":[{\"name\":\"\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"isMember\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"operator\",\"type\":\"address\"},{\"name\":\"endpoint\",\"type\":\"string\"}],\"name\":\"addOperator\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"removeOperator\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"healthEndpoint\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"operator\",\"type\":\"address\"},{\"name\":\"endpoint\",\"type\":\"string\"}],\"name\":\"updateHealthEndpoint\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"operators\",\"outputs\":[{\"name\":\"\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"delegates\",\"outputs\":[{\"name\":\"\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"proposal\",\"outputs\":[{\"name\":\"id\",\"type\":\"uint256\"},{\"name\":\"proposer\",\"type\":\"address\"},{\"name\":\"ptype\",\"type\":\"uint8\"},{\"name\":\"member\",\"type\":\"address\"},{\"name\":\"capacity\",\"type\":\"uint8\"},{\"name\":\"approves\",\"type\":\"address[]\"},{\"name\":\"disapproves\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"},{\"name\":\"capacity\",\"type\":\"uint8\"}],\"name\":\"proposeNewMember\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"proposeMemberDeletion\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"},{\"name\":\"capacity\",\"type\":\"uint8\"}],\"name\":\"proposeMemberModification\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"disapprove\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// ConsortiumManagement is an auto generated Go binding around an Ethereum contract.
type ConsortiumManagement struct {
	ConsortiumManagementCaller     // Read-only binding to the contract
	ConsortiumManagementTransactor // Write-only binding to the contract
	ConsortiumManagementFilterer   // Log filterer for contract events
}

// ConsortiumManagementCaller is an auto generated read-only Go binding around an Ethereum contract.
type ConsortiumManagementCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ConsortiumManagementTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ConsortiumManagementTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ConsortiumManagementFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ConsortiumManagementFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ConsortiumManagementSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ConsortiumManagementSession struct {
	Contract     *ConsortiumManagement // Generic contract binding to set the session for
	CallOpts     bind.CallOpts         // Call options to use throughout this session
	TransactOpts bind.TransactOpts     // Transaction auth options to use throughout this session
}

// ConsortiumManagementCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ConsortiumManagementCallerSession struct {
	Contract *ConsortiumManagementCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts               // Call options to use throughout this session
}

// ConsortiumManagementTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ConsortiumManagementTransactorSession struct {
	Contract     *ConsortiumManagementTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts               // Transaction auth options to use throughout this session
}

// ConsortiumManagementRaw is an auto generated low-level Go binding around an Ethereum contract.
type ConsortiumManagementRaw struct {
	Contract *ConsortiumManagement // Generic contract binding to access the raw methods on
}

// ConsortiumManagementCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ConsortiumManagementCallerRaw struct {
	Contract *ConsortiumManagementCaller // Generic read-only contract binding to access the raw methods on
}

// ConsortiumManagementTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ConsortiumManagementTransactorRaw struct {
	Contract *ConsortiumManagementTransactor // Generic write-only contract binding to access the raw methods on
}

// NewConsortiumManagement creates a new instance of ConsortiumManagement, bound to a specific deployed contract.
func NewConsortiumManagement(address common.Address, backend bind.ContractBackend) (*ConsortiumManagement, error) {
	contract, err := bindConsortiumManagement(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ConsortiumManagement{ConsortiumManagementCaller: ConsortiumManagementCaller{contract: contract}, ConsortiumManagementTransactor: ConsortiumManagementTransactor{contract: contract}, ConsortiumManagementFilterer: ConsortiumManagementFilterer{contract: contract}}, nil
}

// NewConsortiumManagementCaller creates a new read-only instance of ConsortiumManagement, bound to a specific deployed contract.
func NewConsortiumManagementCaller(address common.Address, caller bind.ContractCaller) (*ConsortiumManagementCaller, error) {
	contract, err := bindConsortiumManagement(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ConsortiumManagementCaller{contract: contract}, nil
}

// NewConsortiumManagementTransactor creates a new write-only instance of ConsortiumManagement, bound to a specific deployed contract.
func NewConsortiumManagementTransactor(address common.Address, transactor bind.ContractTransactor) (*ConsortiumManagementTransactor, error) {
	contract, err := bindConsortiumManagement(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ConsortiumManagementTransactor{contract: contract}, nil
}

// NewConsortiumManagementFilterer creates a new log filterer instance of ConsortiumManagement, bound to a specific deployed contract.
func NewConsortiumManagementFilterer(address common.Address, filterer bind.ContractFilterer) (*ConsortiumManagementFilterer, error) {
	contract, err := bindConsortiumManagement(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ConsortiumManagementFilterer{contract: contract}, nil
}

// bindConsortiumManagement binds a generic wrapper to an already deployed contract.
func bindConsortiumManagement(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ConsortiumManagementABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ConsortiumManagement *ConsortiumManagementRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ConsortiumManagement.Contract.ConsortiumManagementCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ConsortiumManagement *ConsortiumManagementRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.ConsortiumManagementTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ConsortiumManagement *ConsortiumManagementRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.ConsortiumManagementTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ConsortiumManagement *ConsortiumManagementCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ConsortiumManagement.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ConsortiumManagement *ConsortiumManagementTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ConsortiumManagement *ConsortiumManagementTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.contract.Transact(opts, method, params...)
}

// Delegates is a free data retrieval call binding the contract method 0x6138b19e.
//
// Solidity: function delegates() view returns(address[])
func (_ConsortiumManagement *ConsortiumManagementCaller) Delegates(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _ConsortiumManagement.contract.Call(opts, &out, "delegates")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// Delegates is a free data retrieval call binding the contract method 0x6138b19e.
//
// Solidity: function delegates() view returns(address[])
func (_ConsortiumManagement *ConsortiumManagementSession) Delegates() ([]common.Address, error) {
	return _ConsortiumManagement.Contract.Delegates(&_ConsortiumManagement.CallOpts)
}

// Delegates is a free data retrieval call binding the contract method 0x6138b19e.
//
// Solidity: function delegates() view returns(address[])
func (_ConsortiumManagement *ConsortiumManagementCallerSession) Delegates() ([]common.Address, error) {
	return _ConsortiumManagement.Contract.Delegates(&_ConsortiumManagement.CallOpts)
}

// HealthEndpoint is a free data retrieval call binding the contract method 0xb44d5fc7.
//
// Solidity: function healthEndpoint(address operator) view returns(string)
func (_ConsortiumManagement *ConsortiumManagementCaller) HealthEndpoint(opts *bind.CallOpts, operator common.Address) (string, error) {
	var out []interface{}
	err := _ConsortiumManagement.contract.Call(opts, &out, "healthEndpoint", operator)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// HealthEndpoint is a free data retrieval call binding the contract method 0xb44d5fc7.
//
// Solidity: function healthEndpoint(address operator) view returns(string)
func (_ConsortiumManagement *ConsortiumManagementSession) HealthEndpoint(operator common.Address) (string, error) {
	return _ConsortiumManagement.Contract.HealthEndpoint(&_ConsortiumManagement.CallOpts, operator)
}

// HealthEndpoint is a free data retrieval call binding the contract method 0xb44d5fc7.
//
// Solidity: function healthEndpoint(address operator) view returns(string)
func (_ConsortiumManagement *ConsortiumManagementCallerSession) HealthEndpoint(operator common.Address) (string, error) {
	return _ConsortiumManagement.Contract.HealthEndpoint(&_ConsortiumManagement.CallOpts, operator)
}

// IsMember is a free data retrieval call binding the contract method 0xa230c524.
//
// Solidity: function isMember(address addr) view returns(bool)
func (_ConsortiumManagement *ConsortiumManagementCaller) IsMember(opts *bind.CallOpts, addr common.Address) (bool, error) {
	var out []interface{}
	err := _ConsortiumManagement.contract.Call(opts, &out, "isMember", addr)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsMember is a free data retrieval call binding the contract method 0xa230c524.
//
// Solidity: function isMember(address addr) view returns(bool)
func (_ConsortiumManagement *ConsortiumManagementSession) IsMember(addr common.Address) (bool, error) {
	return _ConsortiumManagement.Contract.IsMember(&_ConsortiumManagement.CallOpts, addr)
}

// IsMember is a free data retrieval call binding the contract method 0xa230c524.
//
// Solidity: function isMember(address addr) view returns(bool)
func (_ConsortiumManagement *ConsortiumManagementCallerSession) IsMember(addr common.Address) (bool, error) {
	return _ConsortiumManagement.Contract.IsMember(&_ConsortiumManagement.CallOpts, addr)
}

// Members is a free data retrieval call binding the contract method 0xbdd4d18d.
//
// Solidity: function members() view returns(address[])
func (_ConsortiumManagement *ConsortiumManagementCaller) Members(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _ConsortiumManagement.contract.Call(opts, &out, "members")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// Members is a free data retrieval call binding the contract method 0xbdd4d18d.
//
// Solidity: function members() view returns(address[])
func (_ConsortiumManagement *ConsortiumManagementSession) Members() ([]common.Address, error) {
	return _ConsortiumManagement.Contract.Members(&_ConsortiumManagement.CallOpts)
}

// Members is a free data retrieval call binding the contract method 0xbdd4d18d.
//
// Solidity: function members() view returns(address[])
func (_ConsortiumManagement *ConsortiumManagementCallerSession) Members() ([]common.Address, error) {
	return _ConsortiumManagement.Contract.Members(&_ConsortiumManagement.CallOpts)
}

// Operators is a free data retrieval call binding the contract method 0x13e7c9d8.
//
// Solidity: function operators(address addr) view returns(address[])
func (_ConsortiumManagement *ConsortiumManagementCaller) Operators(opts *bind.CallOpts, addr common.Address) ([]common.Address, error) {
	var out []interface{}
	err := _ConsortiumManagement.contract.Call(opts, &out, "operators", addr)

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// Operators is a free data retrieval call binding the contract method 0x13e7c9d8.
//
// Solidity: function operators(address addr) view returns(address[])
func (_ConsortiumManagement *ConsortiumManagementSession) Operators(addr common.Address) ([]common.Address, error) {
	return _ConsortiumManagement.Contract.Operators(&_ConsortiumManagement.CallOpts, addr)
}

// Operators is a free data retrieval call binding the contract method 0x13e7c9d8.
//
// Solidity: function operators(address addr) view returns(address[])
func (_ConsortiumManagement *ConsortiumManagementCallerSession) Operators(addr common.Address) ([]common.Address, error) {
	return _ConsortiumManagement.Contract.Operators(&_ConsortiumManagement.CallOpts, addr)
}

// Proposal is a free data retrieval call binding the contract method 0x753ec103.
//
// Solidity: function proposal() view returns(uint256 id, address proposer, uint8 ptype, address member, uint8 capacity, address[] approves, address[] disapproves)
func (_ConsortiumManagement *ConsortiumManagementCaller) Proposal(opts *bind.CallOpts) (struct {
	Id          *big.Int
	Proposer    common.Address
	Ptype       uint8
	Member      common.Address
	Capacity    uint8
	Approves    []common.Address
	Disapproves []common.Address
}, error) {
	var out []interface{}
	err := _ConsortiumManagement.contract.Call(opts, &out, "proposal")

	outstruct := new(struct {
		Id          *big.Int
		Proposer    common.Address
		Ptype       uint8
		Member      common.Address
		Capacity    uint8
		Approves    []common.Address
		Disapproves []common.Address
	})

	outstruct.Id = out[0].(*big.Int)
	outstruct.Proposer = out[1].(common.Address)
	outstruct.Ptype = out[2].(uint8)
	outstruct.Member = out[3].(common.Address)
	outstruct.Capacity = out[4].(uint8)
	outstruct.Approves = out[5].([]common.Address)
	outstruct.Disapproves = out[6].([]common.Address)

	return *outstruct, err

}

// Proposal is a free data retrieval call binding the contract method 0x753ec103.
//
// Solidity: function proposal() view returns(uint256 id, address proposer, uint8 ptype, address member, uint8 capacity, address[] approves, address[] disapproves)
func (_ConsortiumManagement *ConsortiumManagementSession) Proposal() (struct {
	Id          *big.Int
	Proposer    common.Address
	Ptype       uint8
	Member      common.Address
	Capacity    uint8
	Approves    []common.Address
	Disapproves []common.Address
}, error) {
	return _ConsortiumManagement.Contract.Proposal(&_ConsortiumManagement.CallOpts)
}

// Proposal is a free data retrieval call binding the contract method 0x753ec103.
//
// Solidity: function proposal() view returns(uint256 id, address proposer, uint8 ptype, address member, uint8 capacity, address[] approves, address[] disapproves)
func (_ConsortiumManagement *ConsortiumManagementCallerSession) Proposal() (struct {
	Id          *big.Int
	Proposer    common.Address
	Ptype       uint8
	Member      common.Address
	Capacity    uint8
	Approves    []common.Address
	Disapproves []common.Address
}, error) {
	return _ConsortiumManagement.Contract.Proposal(&_ConsortiumManagement.CallOpts)
}

// AddOperator is a paid mutator transaction binding the contract method 0x2430df89.
//
// Solidity: function addOperator(address operator, string endpoint) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactor) AddOperator(opts *bind.TransactOpts, operator common.Address, endpoint string) (*types.Transaction, error) {
	return _ConsortiumManagement.contract.Transact(opts, "addOperator", operator, endpoint)
}

// AddOperator is a paid mutator transaction binding the contract method 0x2430df89.
//
// Solidity: function addOperator(address operator, string endpoint) returns()
func (_ConsortiumManagement *ConsortiumManagementSession) AddOperator(operator common.Address, endpoint string) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.AddOperator(&_ConsortiumManagement.TransactOpts, operator, endpoint)
}

// AddOperator is a paid mutator transaction binding the contract method 0x2430df89.
//
// Solidity: function addOperator(address operator, string endpoint) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactorSession) AddOperator(operator common.Address, endpoint string) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.AddOperator(&_ConsortiumManagement.TransactOpts, operator, endpoint)
}

// Approve is a paid mutator transaction binding the contract method 0xb759f954.
//
// Solidity: function approve(uint256 id) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactor) Approve(opts *bind.TransactOpts, id *big.Int) (*types.Transaction, error) {
	return _ConsortiumManagement.contract.Transact(opts, "approve", id)
}

// Approve is a paid mutator transaction binding the contract method 0xb759f954.
//
// Solidity: function approve(uint256 id) returns()
func (_ConsortiumManagement *ConsortiumManagementSession) Approve(id *big.Int) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.Approve(&_ConsortiumManagement.TransactOpts, id)
}

// Approve is a paid mutator transaction binding the contract method 0xb759f954.
//
// Solidity: function approve(uint256 id) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactorSession) Approve(id *big.Int) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.Approve(&_ConsortiumManagement.TransactOpts, id)
}

// Disapprove is a paid mutator transaction binding the contract method 0x8008df4f.
//
// Solidity: function disapprove(uint256 id) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactor) Disapprove(opts *bind.TransactOpts, id *big.Int) (*types.Transaction, error) {
	return _ConsortiumManagement.contract.Transact(opts, "disapprove", id)
}

// Disapprove is a paid mutator transaction binding the contract method 0x8008df4f.
//
// Solidity: function disapprove(uint256 id) returns()
func (_ConsortiumManagement *ConsortiumManagementSession) Disapprove(id *big.Int) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.Disapprove(&_ConsortiumManagement.TransactOpts, id)
}

// Disapprove is a paid mutator transaction binding the contract method 0x8008df4f.
//
// Solidity: function disapprove(uint256 id) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactorSession) Disapprove(id *big.Int) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.Disapprove(&_ConsortiumManagement.TransactOpts, id)
}

// ProposeMemberDeletion is a paid mutator transaction binding the contract method 0x555ed8bb.
//
// Solidity: function proposeMemberDeletion(address addr) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactor) ProposeMemberDeletion(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _ConsortiumManagement.contract.Transact(opts, "proposeMemberDeletion", addr)
}

// ProposeMemberDeletion is a paid mutator transaction binding the contract method 0x555ed8bb.
//
// Solidity: function proposeMemberDeletion(address addr) returns()
func (_ConsortiumManagement *ConsortiumManagementSession) ProposeMemberDeletion(addr common.Address) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.ProposeMemberDeletion(&_ConsortiumManagement.TransactOpts, addr)
}

// ProposeMemberDeletion is a paid mutator transaction binding the contract method 0x555ed8bb.
//
// Solidity: function proposeMemberDeletion(address addr) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactorSession) ProposeMemberDeletion(addr common.Address) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.ProposeMemberDeletion(&_ConsortiumManagement.TransactOpts, addr)
}

// ProposeMemberModification is a paid mutator transaction binding the contract method 0xa03eb838.
//
// Solidity: function proposeMemberModification(address addr, uint8 capacity) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactor) ProposeMemberModification(opts *bind.TransactOpts, addr common.Address, capacity uint8) (*types.Transaction, error) {
	return _ConsortiumManagement.contract.Transact(opts, "proposeMemberModification", addr, capacity)
}

// ProposeMemberModification is a paid mutator transaction binding the contract method 0xa03eb838.
//
// Solidity: function proposeMemberModification(address addr, uint8 capacity) returns()
func (_ConsortiumManagement *ConsortiumManagementSession) ProposeMemberModification(addr common.Address, capacity uint8) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.ProposeMemberModification(&_ConsortiumManagement.TransactOpts, addr, capacity)
}

// ProposeMemberModification is a paid mutator transaction binding the contract method 0xa03eb838.
//
// Solidity: function proposeMemberModification(address addr, uint8 capacity) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactorSession) ProposeMemberModification(addr common.Address, capacity uint8) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.ProposeMemberModification(&_ConsortiumManagement.TransactOpts, addr, capacity)
}

// ProposeNewMember is a paid mutator transaction binding the contract method 0x0f6d17f9.
//
// Solidity: function proposeNewMember(address addr, uint8 capacity) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactor) ProposeNewMember(opts *bind.TransactOpts, addr common.Address, capacity uint8) (*types.Transaction, error) {
	return _ConsortiumManagement.contract.Transact(opts, "proposeNewMember", addr, capacity)
}

// ProposeNewMember is a paid mutator transaction binding the contract method 0x0f6d17f9.
//
// Solidity: function proposeNewMember(address addr, uint8 capacity) returns()
func (_ConsortiumManagement *ConsortiumManagementSession) ProposeNewMember(addr common.Address, capacity uint8) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.ProposeNewMember(&_ConsortiumManagement.TransactOpts, addr, capacity)
}

// ProposeNewMember is a paid mutator transaction binding the contract method 0x0f6d17f9.
//
// Solidity: function proposeNewMember(address addr, uint8 capacity) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactorSession) ProposeNewMember(addr common.Address, capacity uint8) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.ProposeNewMember(&_ConsortiumManagement.TransactOpts, addr, capacity)
}

// RemoveOperator is a paid mutator transaction binding the contract method 0xac8a584a.
//
// Solidity: function removeOperator(address operator) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactor) RemoveOperator(opts *bind.TransactOpts, operator common.Address) (*types.Transaction, error) {
	return _ConsortiumManagement.contract.Transact(opts, "removeOperator", operator)
}

// RemoveOperator is a paid mutator transaction binding the contract method 0xac8a584a.
//
// Solidity: function removeOperator(address operator) returns()
func (_ConsortiumManagement *ConsortiumManagementSession) RemoveOperator(operator common.Address) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.RemoveOperator(&_ConsortiumManagement.TransactOpts, operator)
}

// RemoveOperator is a paid mutator transaction binding the contract method 0xac8a584a.
//
// Solidity: function removeOperator(address operator) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactorSession) RemoveOperator(operator common.Address) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.RemoveOperator(&_ConsortiumManagement.TransactOpts, operator)
}

// UpdateHealthEndpoint is a paid mutator transaction binding the contract method 0xe1d1f053.
//
// Solidity: function updateHealthEndpoint(address operator, string endpoint) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactor) UpdateHealthEndpoint(opts *bind.TransactOpts, operator common.Address, endpoint string) (*types.Transaction, error) {
	return _ConsortiumManagement.contract.Transact(opts, "updateHealthEndpoint", operator, endpoint)
}

// UpdateHealthEndpoint is a paid mutator transaction binding the contract method 0xe1d1f053.
//
// Solidity: function updateHealthEndpoint(address operator, string endpoint) returns()
func (_ConsortiumManagement *ConsortiumManagementSession) UpdateHealthEndpoint(operator common.Address, endpoint string) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.UpdateHealthEndpoint(&_ConsortiumManagement.TransactOpts, operator, endpoint)
}

// UpdateHealthEndpoint is a paid mutator transaction binding the contract method 0xe1d1f053.
//
// Solidity: function updateHealthEndpoint(address operator, string endpoint) returns()
func (_ConsortiumManagement *ConsortiumManagementTransactorSession) UpdateHealthEndpoint(operator common.Address, endpoint string) (*types.Transaction, error) {
	return _ConsortiumManagement.Contract.UpdateHealthEndpoint(&_ConsortiumManagement.TransactOpts, operator, endpoint)
}

// ConsortiumManagementAddOperatorIterator is returned from FilterAddOperator and is used to iterate over the raw logs and unpacked data for AddOperator events raised by the ConsortiumManagement contract.
type ConsortiumManagementAddOperatorIterator struct {
	Event *ConsortiumManagementAddOperator // Event containing the contract specifics and raw log

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
func (it *ConsortiumManagementAddOperatorIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ConsortiumManagementAddOperator)
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
		it.Event = new(ConsortiumManagementAddOperator)
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
func (it *ConsortiumManagementAddOperatorIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ConsortiumManagementAddOperatorIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ConsortiumManagementAddOperator represents a AddOperator event raised by the ConsortiumManagement contract.
type ConsortiumManagementAddOperator struct {
	Member   common.Address
	Operator common.Address
	Endpoint string
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterAddOperator is a free log retrieval operation binding the contract event 0xc1ad0c93e1873e80f0551582678c47e2e3cd93d9f12f18b1012ed1ad51654d74.
//
// Solidity: event AddOperator(address member, address operator, string endpoint)
func (_ConsortiumManagement *ConsortiumManagementFilterer) FilterAddOperator(opts *bind.FilterOpts) (*ConsortiumManagementAddOperatorIterator, error) {

	logs, sub, err := _ConsortiumManagement.contract.FilterLogs(opts, "AddOperator")
	if err != nil {
		return nil, err
	}
	return &ConsortiumManagementAddOperatorIterator{contract: _ConsortiumManagement.contract, event: "AddOperator", logs: logs, sub: sub}, nil
}

// WatchAddOperator is a free log subscription operation binding the contract event 0xc1ad0c93e1873e80f0551582678c47e2e3cd93d9f12f18b1012ed1ad51654d74.
//
// Solidity: event AddOperator(address member, address operator, string endpoint)
func (_ConsortiumManagement *ConsortiumManagementFilterer) WatchAddOperator(opts *bind.WatchOpts, sink chan<- *ConsortiumManagementAddOperator) (event.Subscription, error) {

	logs, sub, err := _ConsortiumManagement.contract.WatchLogs(opts, "AddOperator")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ConsortiumManagementAddOperator)
				if err := _ConsortiumManagement.contract.UnpackLog(event, "AddOperator", log); err != nil {
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

// ParseAddOperator is a log parse operation binding the contract event 0xc1ad0c93e1873e80f0551582678c47e2e3cd93d9f12f18b1012ed1ad51654d74.
//
// Solidity: event AddOperator(address member, address operator, string endpoint)
func (_ConsortiumManagement *ConsortiumManagementFilterer) ParseAddOperator(log types.Log) (*ConsortiumManagementAddOperator, error) {
	event := new(ConsortiumManagementAddOperator)
	if err := _ConsortiumManagement.contract.UnpackLog(event, "AddOperator", log); err != nil {
		return nil, err
	}
	return event, nil
}

// ConsortiumManagementProposeIterator is returned from FilterPropose and is used to iterate over the raw logs and unpacked data for Propose events raised by the ConsortiumManagement contract.
type ConsortiumManagementProposeIterator struct {
	Event *ConsortiumManagementPropose // Event containing the contract specifics and raw log

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
func (it *ConsortiumManagementProposeIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ConsortiumManagementPropose)
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
		it.Event = new(ConsortiumManagementPropose)
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
func (it *ConsortiumManagementProposeIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ConsortiumManagementProposeIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ConsortiumManagementPropose represents a Propose event raised by the ConsortiumManagement contract.
type ConsortiumManagementPropose struct {
	Id       *big.Int
	Proposer common.Address
	Ptype    uint8
	Member   common.Address
	Capacity uint8
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterPropose is a free log retrieval operation binding the contract event 0x8665d2ae7d62e4273cdfc40af1cbbea2232b315ca2a2e244c833366f2b8ae9eb.
//
// Solidity: event Propose(uint256 id, address proposer, uint8 ptype, address member, uint8 capacity)
func (_ConsortiumManagement *ConsortiumManagementFilterer) FilterPropose(opts *bind.FilterOpts) (*ConsortiumManagementProposeIterator, error) {

	logs, sub, err := _ConsortiumManagement.contract.FilterLogs(opts, "Propose")
	if err != nil {
		return nil, err
	}
	return &ConsortiumManagementProposeIterator{contract: _ConsortiumManagement.contract, event: "Propose", logs: logs, sub: sub}, nil
}

// WatchPropose is a free log subscription operation binding the contract event 0x8665d2ae7d62e4273cdfc40af1cbbea2232b315ca2a2e244c833366f2b8ae9eb.
//
// Solidity: event Propose(uint256 id, address proposer, uint8 ptype, address member, uint8 capacity)
func (_ConsortiumManagement *ConsortiumManagementFilterer) WatchPropose(opts *bind.WatchOpts, sink chan<- *ConsortiumManagementPropose) (event.Subscription, error) {

	logs, sub, err := _ConsortiumManagement.contract.WatchLogs(opts, "Propose")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ConsortiumManagementPropose)
				if err := _ConsortiumManagement.contract.UnpackLog(event, "Propose", log); err != nil {
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

// ParsePropose is a log parse operation binding the contract event 0x8665d2ae7d62e4273cdfc40af1cbbea2232b315ca2a2e244c833366f2b8ae9eb.
//
// Solidity: event Propose(uint256 id, address proposer, uint8 ptype, address member, uint8 capacity)
func (_ConsortiumManagement *ConsortiumManagementFilterer) ParsePropose(log types.Log) (*ConsortiumManagementPropose, error) {
	event := new(ConsortiumManagementPropose)
	if err := _ConsortiumManagement.contract.UnpackLog(event, "Propose", log); err != nil {
		return nil, err
	}
	return event, nil
}

// ConsortiumManagementRemoveOperatorIterator is returned from FilterRemoveOperator and is used to iterate over the raw logs and unpacked data for RemoveOperator events raised by the ConsortiumManagement contract.
type ConsortiumManagementRemoveOperatorIterator struct {
	Event *ConsortiumManagementRemoveOperator // Event containing the contract specifics and raw log

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
func (it *ConsortiumManagementRemoveOperatorIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ConsortiumManagementRemoveOperator)
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
		it.Event = new(ConsortiumManagementRemoveOperator)
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
func (it *ConsortiumManagementRemoveOperatorIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ConsortiumManagementRemoveOperatorIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ConsortiumManagementRemoveOperator represents a RemoveOperator event raised by the ConsortiumManagement contract.
type ConsortiumManagementRemoveOperator struct {
	Member   common.Address
	Operator common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterRemoveOperator is a free log retrieval operation binding the contract event 0xb157cf3e9ae29eb366b3bdda54b41d4738ada5daa73f8d2f1bef6280bb1418e4.
//
// Solidity: event RemoveOperator(address member, address operator)
func (_ConsortiumManagement *ConsortiumManagementFilterer) FilterRemoveOperator(opts *bind.FilterOpts) (*ConsortiumManagementRemoveOperatorIterator, error) {

	logs, sub, err := _ConsortiumManagement.contract.FilterLogs(opts, "RemoveOperator")
	if err != nil {
		return nil, err
	}
	return &ConsortiumManagementRemoveOperatorIterator{contract: _ConsortiumManagement.contract, event: "RemoveOperator", logs: logs, sub: sub}, nil
}

// WatchRemoveOperator is a free log subscription operation binding the contract event 0xb157cf3e9ae29eb366b3bdda54b41d4738ada5daa73f8d2f1bef6280bb1418e4.
//
// Solidity: event RemoveOperator(address member, address operator)
func (_ConsortiumManagement *ConsortiumManagementFilterer) WatchRemoveOperator(opts *bind.WatchOpts, sink chan<- *ConsortiumManagementRemoveOperator) (event.Subscription, error) {

	logs, sub, err := _ConsortiumManagement.contract.WatchLogs(opts, "RemoveOperator")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ConsortiumManagementRemoveOperator)
				if err := _ConsortiumManagement.contract.UnpackLog(event, "RemoveOperator", log); err != nil {
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

// ParseRemoveOperator is a log parse operation binding the contract event 0xb157cf3e9ae29eb366b3bdda54b41d4738ada5daa73f8d2f1bef6280bb1418e4.
//
// Solidity: event RemoveOperator(address member, address operator)
func (_ConsortiumManagement *ConsortiumManagementFilterer) ParseRemoveOperator(log types.Log) (*ConsortiumManagementRemoveOperator, error) {
	event := new(ConsortiumManagementRemoveOperator)
	if err := _ConsortiumManagement.contract.UnpackLog(event, "RemoveOperator", log); err != nil {
		return nil, err
	}
	return event, nil
}

// ConsortiumManagementSettleProposalIterator is returned from FilterSettleProposal and is used to iterate over the raw logs and unpacked data for SettleProposal events raised by the ConsortiumManagement contract.
type ConsortiumManagementSettleProposalIterator struct {
	Event *ConsortiumManagementSettleProposal // Event containing the contract specifics and raw log

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
func (it *ConsortiumManagementSettleProposalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ConsortiumManagementSettleProposal)
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
		it.Event = new(ConsortiumManagementSettleProposal)
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
func (it *ConsortiumManagementSettleProposalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ConsortiumManagementSettleProposalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ConsortiumManagementSettleProposal represents a SettleProposal event raised by the ConsortiumManagement contract.
type ConsortiumManagementSettleProposal struct {
	Id      *big.Int
	Success bool
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterSettleProposal is a free log retrieval operation binding the contract event 0xf412ee5dcd836b3a4fa40ae9d7e90eeea743a32f4e0ba26c91c7cbc8c3d8b44b.
//
// Solidity: event SettleProposal(uint256 id, bool success)
func (_ConsortiumManagement *ConsortiumManagementFilterer) FilterSettleProposal(opts *bind.FilterOpts) (*ConsortiumManagementSettleProposalIterator, error) {

	logs, sub, err := _ConsortiumManagement.contract.FilterLogs(opts, "SettleProposal")
	if err != nil {
		return nil, err
	}
	return &ConsortiumManagementSettleProposalIterator{contract: _ConsortiumManagement.contract, event: "SettleProposal", logs: logs, sub: sub}, nil
}

// WatchSettleProposal is a free log subscription operation binding the contract event 0xf412ee5dcd836b3a4fa40ae9d7e90eeea743a32f4e0ba26c91c7cbc8c3d8b44b.
//
// Solidity: event SettleProposal(uint256 id, bool success)
func (_ConsortiumManagement *ConsortiumManagementFilterer) WatchSettleProposal(opts *bind.WatchOpts, sink chan<- *ConsortiumManagementSettleProposal) (event.Subscription, error) {

	logs, sub, err := _ConsortiumManagement.contract.WatchLogs(opts, "SettleProposal")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ConsortiumManagementSettleProposal)
				if err := _ConsortiumManagement.contract.UnpackLog(event, "SettleProposal", log); err != nil {
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

// ParseSettleProposal is a log parse operation binding the contract event 0xf412ee5dcd836b3a4fa40ae9d7e90eeea743a32f4e0ba26c91c7cbc8c3d8b44b.
//
// Solidity: event SettleProposal(uint256 id, bool success)
func (_ConsortiumManagement *ConsortiumManagementFilterer) ParseSettleProposal(log types.Log) (*ConsortiumManagementSettleProposal, error) {
	event := new(ConsortiumManagementSettleProposal)
	if err := _ConsortiumManagement.contract.UnpackLog(event, "SettleProposal", log); err != nil {
		return nil, err
	}
	return event, nil
}

// ConsortiumManagementUpdateOperatorHealthEndpointIterator is returned from FilterUpdateOperatorHealthEndpoint and is used to iterate over the raw logs and unpacked data for UpdateOperatorHealthEndpoint events raised by the ConsortiumManagement contract.
type ConsortiumManagementUpdateOperatorHealthEndpointIterator struct {
	Event *ConsortiumManagementUpdateOperatorHealthEndpoint // Event containing the contract specifics and raw log

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
func (it *ConsortiumManagementUpdateOperatorHealthEndpointIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ConsortiumManagementUpdateOperatorHealthEndpoint)
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
		it.Event = new(ConsortiumManagementUpdateOperatorHealthEndpoint)
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
func (it *ConsortiumManagementUpdateOperatorHealthEndpointIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ConsortiumManagementUpdateOperatorHealthEndpointIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ConsortiumManagementUpdateOperatorHealthEndpoint represents a UpdateOperatorHealthEndpoint event raised by the ConsortiumManagement contract.
type ConsortiumManagementUpdateOperatorHealthEndpoint struct {
	Member   common.Address
	Operator common.Address
	Endpoint string
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterUpdateOperatorHealthEndpoint is a free log retrieval operation binding the contract event 0x8daf704ffb23b2e479e9849d1bda8860345f6c15d43b68cd9cf8622952564895.
//
// Solidity: event UpdateOperatorHealthEndpoint(address member, address operator, string endpoint)
func (_ConsortiumManagement *ConsortiumManagementFilterer) FilterUpdateOperatorHealthEndpoint(opts *bind.FilterOpts) (*ConsortiumManagementUpdateOperatorHealthEndpointIterator, error) {

	logs, sub, err := _ConsortiumManagement.contract.FilterLogs(opts, "UpdateOperatorHealthEndpoint")
	if err != nil {
		return nil, err
	}
	return &ConsortiumManagementUpdateOperatorHealthEndpointIterator{contract: _ConsortiumManagement.contract, event: "UpdateOperatorHealthEndpoint", logs: logs, sub: sub}, nil
}

// WatchUpdateOperatorHealthEndpoint is a free log subscription operation binding the contract event 0x8daf704ffb23b2e479e9849d1bda8860345f6c15d43b68cd9cf8622952564895.
//
// Solidity: event UpdateOperatorHealthEndpoint(address member, address operator, string endpoint)
func (_ConsortiumManagement *ConsortiumManagementFilterer) WatchUpdateOperatorHealthEndpoint(opts *bind.WatchOpts, sink chan<- *ConsortiumManagementUpdateOperatorHealthEndpoint) (event.Subscription, error) {

	logs, sub, err := _ConsortiumManagement.contract.WatchLogs(opts, "UpdateOperatorHealthEndpoint")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ConsortiumManagementUpdateOperatorHealthEndpoint)
				if err := _ConsortiumManagement.contract.UnpackLog(event, "UpdateOperatorHealthEndpoint", log); err != nil {
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

// ParseUpdateOperatorHealthEndpoint is a log parse operation binding the contract event 0x8daf704ffb23b2e479e9849d1bda8860345f6c15d43b68cd9cf8622952564895.
//
// Solidity: event UpdateOperatorHealthEndpoint(address member, address operator, string endpoint)
func (_ConsortiumManagement *ConsortiumManagementFilterer) ParseUpdateOperatorHealthEndpoint(log types.Log) (*ConsortiumManagementUpdateOperatorHealthEndpoint, error) {
	event := new(ConsortiumManagementUpdateOperatorHealthEndpoint)
	if err := _ConsortiumManagement.contract.UnpackLog(event, "UpdateOperatorHealthEndpoint", log); err != nil {
		return nil, err
	}
	return event, nil
}

// ConsortiumManagementVoteIterator is returned from FilterVote and is used to iterate over the raw logs and unpacked data for Vote events raised by the ConsortiumManagement contract.
type ConsortiumManagementVoteIterator struct {
	Event *ConsortiumManagementVote // Event containing the contract specifics and raw log

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
func (it *ConsortiumManagementVoteIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ConsortiumManagementVote)
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
		it.Event = new(ConsortiumManagementVote)
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
func (it *ConsortiumManagementVoteIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ConsortiumManagementVoteIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ConsortiumManagementVote represents a Vote event raised by the ConsortiumManagement contract.
type ConsortiumManagementVote struct {
	Id      *big.Int
	Voter   common.Address
	Approve bool
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterVote is a free log retrieval operation binding the contract event 0xcfa82ef0390c8f3e57ebe6c0665352a383667e792af012d350d9786ee5173d26.
//
// Solidity: event Vote(uint256 id, address voter, bool approve)
func (_ConsortiumManagement *ConsortiumManagementFilterer) FilterVote(opts *bind.FilterOpts) (*ConsortiumManagementVoteIterator, error) {

	logs, sub, err := _ConsortiumManagement.contract.FilterLogs(opts, "Vote")
	if err != nil {
		return nil, err
	}
	return &ConsortiumManagementVoteIterator{contract: _ConsortiumManagement.contract, event: "Vote", logs: logs, sub: sub}, nil
}

// WatchVote is a free log subscription operation binding the contract event 0xcfa82ef0390c8f3e57ebe6c0665352a383667e792af012d350d9786ee5173d26.
//
// Solidity: event Vote(uint256 id, address voter, bool approve)
func (_ConsortiumManagement *ConsortiumManagementFilterer) WatchVote(opts *bind.WatchOpts, sink chan<- *ConsortiumManagementVote) (event.Subscription, error) {

	logs, sub, err := _ConsortiumManagement.contract.WatchLogs(opts, "Vote")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ConsortiumManagementVote)
				if err := _ConsortiumManagement.contract.UnpackLog(event, "Vote", log); err != nil {
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

// ParseVote is a log parse operation binding the contract event 0xcfa82ef0390c8f3e57ebe6c0665352a383667e792af012d350d9786ee5173d26.
//
// Solidity: event Vote(uint256 id, address voter, bool approve)
func (_ConsortiumManagement *ConsortiumManagementFilterer) ParseVote(log types.Log) (*ConsortiumManagementVote, error) {
	event := new(ConsortiumManagementVote)
	if err := _ConsortiumManagement.contract.UnpackLog(event, "Vote", log); err != nil {
		return nil, err
	}
	return event, nil
}

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

// W3bstreamRouterMetaData contains all meta data concerning the W3bstreamRouter contract.
var W3bstreamRouterMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"dapp\",\"type\":\"address\"}],\"name\":\"DappBound\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"DappUnbound\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"router\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"success\",\"type\":\"bool\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"revertReason\",\"type\":\"string\"}],\"name\":\"DataProcessed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"_dapp\",\"type\":\"address\"}],\"name\":\"bindDapp\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"dapp\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"fleetManagement\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_fleetManagement\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_projectStore\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"projectStore\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_proverId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_taskId\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"route\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"unbindDapp\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// W3bstreamRouterABI is the input ABI used to generate the binding from.
// Deprecated: Use W3bstreamRouterMetaData.ABI instead.
var W3bstreamRouterABI = W3bstreamRouterMetaData.ABI

// W3bstreamRouter is an auto generated Go binding around an Ethereum contract.
type W3bstreamRouter struct {
	W3bstreamRouterCaller     // Read-only binding to the contract
	W3bstreamRouterTransactor // Write-only binding to the contract
	W3bstreamRouterFilterer   // Log filterer for contract events
}

// W3bstreamRouterCaller is an auto generated read-only Go binding around an Ethereum contract.
type W3bstreamRouterCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamRouterTransactor is an auto generated write-only Go binding around an Ethereum contract.
type W3bstreamRouterTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamRouterFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type W3bstreamRouterFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// W3bstreamRouterSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type W3bstreamRouterSession struct {
	Contract     *W3bstreamRouter  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// W3bstreamRouterCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type W3bstreamRouterCallerSession struct {
	Contract *W3bstreamRouterCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// W3bstreamRouterTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type W3bstreamRouterTransactorSession struct {
	Contract     *W3bstreamRouterTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// W3bstreamRouterRaw is an auto generated low-level Go binding around an Ethereum contract.
type W3bstreamRouterRaw struct {
	Contract *W3bstreamRouter // Generic contract binding to access the raw methods on
}

// W3bstreamRouterCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type W3bstreamRouterCallerRaw struct {
	Contract *W3bstreamRouterCaller // Generic read-only contract binding to access the raw methods on
}

// W3bstreamRouterTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type W3bstreamRouterTransactorRaw struct {
	Contract *W3bstreamRouterTransactor // Generic write-only contract binding to access the raw methods on
}

// NewW3bstreamRouter creates a new instance of W3bstreamRouter, bound to a specific deployed contract.
func NewW3bstreamRouter(address common.Address, backend bind.ContractBackend) (*W3bstreamRouter, error) {
	contract, err := bindW3bstreamRouter(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &W3bstreamRouter{W3bstreamRouterCaller: W3bstreamRouterCaller{contract: contract}, W3bstreamRouterTransactor: W3bstreamRouterTransactor{contract: contract}, W3bstreamRouterFilterer: W3bstreamRouterFilterer{contract: contract}}, nil
}

// NewW3bstreamRouterCaller creates a new read-only instance of W3bstreamRouter, bound to a specific deployed contract.
func NewW3bstreamRouterCaller(address common.Address, caller bind.ContractCaller) (*W3bstreamRouterCaller, error) {
	contract, err := bindW3bstreamRouter(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &W3bstreamRouterCaller{contract: contract}, nil
}

// NewW3bstreamRouterTransactor creates a new write-only instance of W3bstreamRouter, bound to a specific deployed contract.
func NewW3bstreamRouterTransactor(address common.Address, transactor bind.ContractTransactor) (*W3bstreamRouterTransactor, error) {
	contract, err := bindW3bstreamRouter(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &W3bstreamRouterTransactor{contract: contract}, nil
}

// NewW3bstreamRouterFilterer creates a new log filterer instance of W3bstreamRouter, bound to a specific deployed contract.
func NewW3bstreamRouterFilterer(address common.Address, filterer bind.ContractFilterer) (*W3bstreamRouterFilterer, error) {
	contract, err := bindW3bstreamRouter(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &W3bstreamRouterFilterer{contract: contract}, nil
}

// bindW3bstreamRouter binds a generic wrapper to an already deployed contract.
func bindW3bstreamRouter(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := W3bstreamRouterMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_W3bstreamRouter *W3bstreamRouterRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _W3bstreamRouter.Contract.W3bstreamRouterCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_W3bstreamRouter *W3bstreamRouterRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.W3bstreamRouterTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_W3bstreamRouter *W3bstreamRouterRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.W3bstreamRouterTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_W3bstreamRouter *W3bstreamRouterCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _W3bstreamRouter.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_W3bstreamRouter *W3bstreamRouterTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_W3bstreamRouter *W3bstreamRouterTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.contract.Transact(opts, method, params...)
}

// Dapp is a free data retrieval call binding the contract method 0x1bf8131f.
//
// Solidity: function dapp(uint256 ) view returns(address)
func (_W3bstreamRouter *W3bstreamRouterCaller) Dapp(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamRouter.contract.Call(opts, &out, "dapp", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Dapp is a free data retrieval call binding the contract method 0x1bf8131f.
//
// Solidity: function dapp(uint256 ) view returns(address)
func (_W3bstreamRouter *W3bstreamRouterSession) Dapp(arg0 *big.Int) (common.Address, error) {
	return _W3bstreamRouter.Contract.Dapp(&_W3bstreamRouter.CallOpts, arg0)
}

// Dapp is a free data retrieval call binding the contract method 0x1bf8131f.
//
// Solidity: function dapp(uint256 ) view returns(address)
func (_W3bstreamRouter *W3bstreamRouterCallerSession) Dapp(arg0 *big.Int) (common.Address, error) {
	return _W3bstreamRouter.Contract.Dapp(&_W3bstreamRouter.CallOpts, arg0)
}

// FleetManagement is a free data retrieval call binding the contract method 0x53ef0542.
//
// Solidity: function fleetManagement() view returns(address)
func (_W3bstreamRouter *W3bstreamRouterCaller) FleetManagement(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamRouter.contract.Call(opts, &out, "fleetManagement")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// FleetManagement is a free data retrieval call binding the contract method 0x53ef0542.
//
// Solidity: function fleetManagement() view returns(address)
func (_W3bstreamRouter *W3bstreamRouterSession) FleetManagement() (common.Address, error) {
	return _W3bstreamRouter.Contract.FleetManagement(&_W3bstreamRouter.CallOpts)
}

// FleetManagement is a free data retrieval call binding the contract method 0x53ef0542.
//
// Solidity: function fleetManagement() view returns(address)
func (_W3bstreamRouter *W3bstreamRouterCallerSession) FleetManagement() (common.Address, error) {
	return _W3bstreamRouter.Contract.FleetManagement(&_W3bstreamRouter.CallOpts)
}

// ProjectStore is a free data retrieval call binding the contract method 0xa0fadaaa.
//
// Solidity: function projectStore() view returns(address)
func (_W3bstreamRouter *W3bstreamRouterCaller) ProjectStore(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _W3bstreamRouter.contract.Call(opts, &out, "projectStore")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ProjectStore is a free data retrieval call binding the contract method 0xa0fadaaa.
//
// Solidity: function projectStore() view returns(address)
func (_W3bstreamRouter *W3bstreamRouterSession) ProjectStore() (common.Address, error) {
	return _W3bstreamRouter.Contract.ProjectStore(&_W3bstreamRouter.CallOpts)
}

// ProjectStore is a free data retrieval call binding the contract method 0xa0fadaaa.
//
// Solidity: function projectStore() view returns(address)
func (_W3bstreamRouter *W3bstreamRouterCallerSession) ProjectStore() (common.Address, error) {
	return _W3bstreamRouter.Contract.ProjectStore(&_W3bstreamRouter.CallOpts)
}

// BindDapp is a paid mutator transaction binding the contract method 0x85a7d275.
//
// Solidity: function bindDapp(uint256 _projectId, address _dapp) returns()
func (_W3bstreamRouter *W3bstreamRouterTransactor) BindDapp(opts *bind.TransactOpts, _projectId *big.Int, _dapp common.Address) (*types.Transaction, error) {
	return _W3bstreamRouter.contract.Transact(opts, "bindDapp", _projectId, _dapp)
}

// BindDapp is a paid mutator transaction binding the contract method 0x85a7d275.
//
// Solidity: function bindDapp(uint256 _projectId, address _dapp) returns()
func (_W3bstreamRouter *W3bstreamRouterSession) BindDapp(_projectId *big.Int, _dapp common.Address) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.BindDapp(&_W3bstreamRouter.TransactOpts, _projectId, _dapp)
}

// BindDapp is a paid mutator transaction binding the contract method 0x85a7d275.
//
// Solidity: function bindDapp(uint256 _projectId, address _dapp) returns()
func (_W3bstreamRouter *W3bstreamRouterTransactorSession) BindDapp(_projectId *big.Int, _dapp common.Address) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.BindDapp(&_W3bstreamRouter.TransactOpts, _projectId, _dapp)
}

// Initialize is a paid mutator transaction binding the contract method 0x485cc955.
//
// Solidity: function initialize(address _fleetManagement, address _projectStore) returns()
func (_W3bstreamRouter *W3bstreamRouterTransactor) Initialize(opts *bind.TransactOpts, _fleetManagement common.Address, _projectStore common.Address) (*types.Transaction, error) {
	return _W3bstreamRouter.contract.Transact(opts, "initialize", _fleetManagement, _projectStore)
}

// Initialize is a paid mutator transaction binding the contract method 0x485cc955.
//
// Solidity: function initialize(address _fleetManagement, address _projectStore) returns()
func (_W3bstreamRouter *W3bstreamRouterSession) Initialize(_fleetManagement common.Address, _projectStore common.Address) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.Initialize(&_W3bstreamRouter.TransactOpts, _fleetManagement, _projectStore)
}

// Initialize is a paid mutator transaction binding the contract method 0x485cc955.
//
// Solidity: function initialize(address _fleetManagement, address _projectStore) returns()
func (_W3bstreamRouter *W3bstreamRouterTransactorSession) Initialize(_fleetManagement common.Address, _projectStore common.Address) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.Initialize(&_W3bstreamRouter.TransactOpts, _fleetManagement, _projectStore)
}

// Route is a paid mutator transaction binding the contract method 0x0f9e7c53.
//
// Solidity: function route(uint256 _projectId, uint256 _proverId, uint256 _taskId, bytes _data) returns()
func (_W3bstreamRouter *W3bstreamRouterTransactor) Route(opts *bind.TransactOpts, _projectId *big.Int, _proverId *big.Int, _taskId *big.Int, _data []byte) (*types.Transaction, error) {
	return _W3bstreamRouter.contract.Transact(opts, "route", _projectId, _proverId, _taskId, _data)
}

// Route is a paid mutator transaction binding the contract method 0x0f9e7c53.
//
// Solidity: function route(uint256 _projectId, uint256 _proverId, uint256 _taskId, bytes _data) returns()
func (_W3bstreamRouter *W3bstreamRouterSession) Route(_projectId *big.Int, _proverId *big.Int, _taskId *big.Int, _data []byte) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.Route(&_W3bstreamRouter.TransactOpts, _projectId, _proverId, _taskId, _data)
}

// Route is a paid mutator transaction binding the contract method 0x0f9e7c53.
//
// Solidity: function route(uint256 _projectId, uint256 _proverId, uint256 _taskId, bytes _data) returns()
func (_W3bstreamRouter *W3bstreamRouterTransactorSession) Route(_projectId *big.Int, _proverId *big.Int, _taskId *big.Int, _data []byte) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.Route(&_W3bstreamRouter.TransactOpts, _projectId, _proverId, _taskId, _data)
}

// UnbindDapp is a paid mutator transaction binding the contract method 0xd869758c.
//
// Solidity: function unbindDapp(uint256 _projectId) returns()
func (_W3bstreamRouter *W3bstreamRouterTransactor) UnbindDapp(opts *bind.TransactOpts, _projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamRouter.contract.Transact(opts, "unbindDapp", _projectId)
}

// UnbindDapp is a paid mutator transaction binding the contract method 0xd869758c.
//
// Solidity: function unbindDapp(uint256 _projectId) returns()
func (_W3bstreamRouter *W3bstreamRouterSession) UnbindDapp(_projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.UnbindDapp(&_W3bstreamRouter.TransactOpts, _projectId)
}

// UnbindDapp is a paid mutator transaction binding the contract method 0xd869758c.
//
// Solidity: function unbindDapp(uint256 _projectId) returns()
func (_W3bstreamRouter *W3bstreamRouterTransactorSession) UnbindDapp(_projectId *big.Int) (*types.Transaction, error) {
	return _W3bstreamRouter.Contract.UnbindDapp(&_W3bstreamRouter.TransactOpts, _projectId)
}

// W3bstreamRouterDappBoundIterator is returned from FilterDappBound and is used to iterate over the raw logs and unpacked data for DappBound events raised by the W3bstreamRouter contract.
type W3bstreamRouterDappBoundIterator struct {
	Event *W3bstreamRouterDappBound // Event containing the contract specifics and raw log

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
func (it *W3bstreamRouterDappBoundIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamRouterDappBound)
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
		it.Event = new(W3bstreamRouterDappBound)
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
func (it *W3bstreamRouterDappBoundIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamRouterDappBoundIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamRouterDappBound represents a DappBound event raised by the W3bstreamRouter contract.
type W3bstreamRouterDappBound struct {
	ProjectId *big.Int
	Operator  common.Address
	Dapp      common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterDappBound is a free log retrieval operation binding the contract event 0xf121fc55c0fd19e108d2d5642aff2967949fb708d9b985093c530a8a1fb97778.
//
// Solidity: event DappBound(uint256 indexed projectId, address indexed operator, address dapp)
func (_W3bstreamRouter *W3bstreamRouterFilterer) FilterDappBound(opts *bind.FilterOpts, projectId []*big.Int, operator []common.Address) (*W3bstreamRouterDappBoundIterator, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamRouter.contract.FilterLogs(opts, "DappBound", projectIdRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamRouterDappBoundIterator{contract: _W3bstreamRouter.contract, event: "DappBound", logs: logs, sub: sub}, nil
}

// WatchDappBound is a free log subscription operation binding the contract event 0xf121fc55c0fd19e108d2d5642aff2967949fb708d9b985093c530a8a1fb97778.
//
// Solidity: event DappBound(uint256 indexed projectId, address indexed operator, address dapp)
func (_W3bstreamRouter *W3bstreamRouterFilterer) WatchDappBound(opts *bind.WatchOpts, sink chan<- *W3bstreamRouterDappBound, projectId []*big.Int, operator []common.Address) (event.Subscription, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamRouter.contract.WatchLogs(opts, "DappBound", projectIdRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamRouterDappBound)
				if err := _W3bstreamRouter.contract.UnpackLog(event, "DappBound", log); err != nil {
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

// ParseDappBound is a log parse operation binding the contract event 0xf121fc55c0fd19e108d2d5642aff2967949fb708d9b985093c530a8a1fb97778.
//
// Solidity: event DappBound(uint256 indexed projectId, address indexed operator, address dapp)
func (_W3bstreamRouter *W3bstreamRouterFilterer) ParseDappBound(log types.Log) (*W3bstreamRouterDappBound, error) {
	event := new(W3bstreamRouterDappBound)
	if err := _W3bstreamRouter.contract.UnpackLog(event, "DappBound", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamRouterDappUnboundIterator is returned from FilterDappUnbound and is used to iterate over the raw logs and unpacked data for DappUnbound events raised by the W3bstreamRouter contract.
type W3bstreamRouterDappUnboundIterator struct {
	Event *W3bstreamRouterDappUnbound // Event containing the contract specifics and raw log

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
func (it *W3bstreamRouterDappUnboundIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamRouterDappUnbound)
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
		it.Event = new(W3bstreamRouterDappUnbound)
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
func (it *W3bstreamRouterDappUnboundIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamRouterDappUnboundIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamRouterDappUnbound represents a DappUnbound event raised by the W3bstreamRouter contract.
type W3bstreamRouterDappUnbound struct {
	ProjectId *big.Int
	Operator  common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterDappUnbound is a free log retrieval operation binding the contract event 0x7019ee8601397d5c4fe244404e2428a9c0b0a4d8679186133186cc01376ee9f1.
//
// Solidity: event DappUnbound(uint256 indexed projectId, address indexed operator)
func (_W3bstreamRouter *W3bstreamRouterFilterer) FilterDappUnbound(opts *bind.FilterOpts, projectId []*big.Int, operator []common.Address) (*W3bstreamRouterDappUnboundIterator, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamRouter.contract.FilterLogs(opts, "DappUnbound", projectIdRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamRouterDappUnboundIterator{contract: _W3bstreamRouter.contract, event: "DappUnbound", logs: logs, sub: sub}, nil
}

// WatchDappUnbound is a free log subscription operation binding the contract event 0x7019ee8601397d5c4fe244404e2428a9c0b0a4d8679186133186cc01376ee9f1.
//
// Solidity: event DappUnbound(uint256 indexed projectId, address indexed operator)
func (_W3bstreamRouter *W3bstreamRouterFilterer) WatchDappUnbound(opts *bind.WatchOpts, sink chan<- *W3bstreamRouterDappUnbound, projectId []*big.Int, operator []common.Address) (event.Subscription, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamRouter.contract.WatchLogs(opts, "DappUnbound", projectIdRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamRouterDappUnbound)
				if err := _W3bstreamRouter.contract.UnpackLog(event, "DappUnbound", log); err != nil {
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

// ParseDappUnbound is a log parse operation binding the contract event 0x7019ee8601397d5c4fe244404e2428a9c0b0a4d8679186133186cc01376ee9f1.
//
// Solidity: event DappUnbound(uint256 indexed projectId, address indexed operator)
func (_W3bstreamRouter *W3bstreamRouterFilterer) ParseDappUnbound(log types.Log) (*W3bstreamRouterDappUnbound, error) {
	event := new(W3bstreamRouterDappUnbound)
	if err := _W3bstreamRouter.contract.UnpackLog(event, "DappUnbound", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamRouterDataProcessedIterator is returned from FilterDataProcessed and is used to iterate over the raw logs and unpacked data for DataProcessed events raised by the W3bstreamRouter contract.
type W3bstreamRouterDataProcessedIterator struct {
	Event *W3bstreamRouterDataProcessed // Event containing the contract specifics and raw log

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
func (it *W3bstreamRouterDataProcessedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamRouterDataProcessed)
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
		it.Event = new(W3bstreamRouterDataProcessed)
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
func (it *W3bstreamRouterDataProcessedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamRouterDataProcessedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamRouterDataProcessed represents a DataProcessed event raised by the W3bstreamRouter contract.
type W3bstreamRouterDataProcessed struct {
	ProjectId    *big.Int
	Router       *big.Int
	Operator     common.Address
	Success      bool
	RevertReason string
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterDataProcessed is a free log retrieval operation binding the contract event 0x1e0d5999e38157a2e2bd253650b56ee404b74b94ada45cca65a8d79222fa9290.
//
// Solidity: event DataProcessed(uint256 indexed projectId, uint256 indexed router, address indexed operator, bool success, string revertReason)
func (_W3bstreamRouter *W3bstreamRouterFilterer) FilterDataProcessed(opts *bind.FilterOpts, projectId []*big.Int, router []*big.Int, operator []common.Address) (*W3bstreamRouterDataProcessedIterator, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}
	var routerRule []interface{}
	for _, routerItem := range router {
		routerRule = append(routerRule, routerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamRouter.contract.FilterLogs(opts, "DataProcessed", projectIdRule, routerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &W3bstreamRouterDataProcessedIterator{contract: _W3bstreamRouter.contract, event: "DataProcessed", logs: logs, sub: sub}, nil
}

// WatchDataProcessed is a free log subscription operation binding the contract event 0x1e0d5999e38157a2e2bd253650b56ee404b74b94ada45cca65a8d79222fa9290.
//
// Solidity: event DataProcessed(uint256 indexed projectId, uint256 indexed router, address indexed operator, bool success, string revertReason)
func (_W3bstreamRouter *W3bstreamRouterFilterer) WatchDataProcessed(opts *bind.WatchOpts, sink chan<- *W3bstreamRouterDataProcessed, projectId []*big.Int, router []*big.Int, operator []common.Address) (event.Subscription, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}
	var routerRule []interface{}
	for _, routerItem := range router {
		routerRule = append(routerRule, routerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _W3bstreamRouter.contract.WatchLogs(opts, "DataProcessed", projectIdRule, routerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamRouterDataProcessed)
				if err := _W3bstreamRouter.contract.UnpackLog(event, "DataProcessed", log); err != nil {
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

// ParseDataProcessed is a log parse operation binding the contract event 0x1e0d5999e38157a2e2bd253650b56ee404b74b94ada45cca65a8d79222fa9290.
//
// Solidity: event DataProcessed(uint256 indexed projectId, uint256 indexed router, address indexed operator, bool success, string revertReason)
func (_W3bstreamRouter *W3bstreamRouterFilterer) ParseDataProcessed(log types.Log) (*W3bstreamRouterDataProcessed, error) {
	event := new(W3bstreamRouterDataProcessed)
	if err := _W3bstreamRouter.contract.UnpackLog(event, "DataProcessed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// W3bstreamRouterInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the W3bstreamRouter contract.
type W3bstreamRouterInitializedIterator struct {
	Event *W3bstreamRouterInitialized // Event containing the contract specifics and raw log

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
func (it *W3bstreamRouterInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(W3bstreamRouterInitialized)
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
		it.Event = new(W3bstreamRouterInitialized)
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
func (it *W3bstreamRouterInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *W3bstreamRouterInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// W3bstreamRouterInitialized represents a Initialized event raised by the W3bstreamRouter contract.
type W3bstreamRouterInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_W3bstreamRouter *W3bstreamRouterFilterer) FilterInitialized(opts *bind.FilterOpts) (*W3bstreamRouterInitializedIterator, error) {

	logs, sub, err := _W3bstreamRouter.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &W3bstreamRouterInitializedIterator{contract: _W3bstreamRouter.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_W3bstreamRouter *W3bstreamRouterFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *W3bstreamRouterInitialized) (event.Subscription, error) {

	logs, sub, err := _W3bstreamRouter.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(W3bstreamRouterInitialized)
				if err := _W3bstreamRouter.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_W3bstreamRouter *W3bstreamRouterFilterer) ParseInitialized(log types.Log) (*W3bstreamRouterInitialized, error) {
	event := new(W3bstreamRouterInitialized)
	if err := _W3bstreamRouter.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

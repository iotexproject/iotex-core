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

// ProjectRegistryMetaData contains all meta data concerning the ProjectRegistry contract.
var ProjectRegistryMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_project\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"project\",\"outputs\":[{\"internalType\":\"contractIProject\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"register\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"_name\",\"type\":\"string\"},{\"internalType\":\"uint8\",\"name\":\"_type\",\"type\":\"uint8\"}],\"name\":\"register\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"_name\",\"type\":\"string\"}],\"name\":\"register\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
}

// ProjectRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use ProjectRegistryMetaData.ABI instead.
var ProjectRegistryABI = ProjectRegistryMetaData.ABI

// ProjectRegistry is an auto generated Go binding around an Ethereum contract.
type ProjectRegistry struct {
	ProjectRegistryCaller     // Read-only binding to the contract
	ProjectRegistryTransactor // Write-only binding to the contract
	ProjectRegistryFilterer   // Log filterer for contract events
}

// ProjectRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type ProjectRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProjectRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ProjectRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProjectRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ProjectRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProjectRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ProjectRegistrySession struct {
	Contract     *ProjectRegistry  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ProjectRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ProjectRegistryCallerSession struct {
	Contract *ProjectRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// ProjectRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ProjectRegistryTransactorSession struct {
	Contract     *ProjectRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// ProjectRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type ProjectRegistryRaw struct {
	Contract *ProjectRegistry // Generic contract binding to access the raw methods on
}

// ProjectRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ProjectRegistryCallerRaw struct {
	Contract *ProjectRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// ProjectRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ProjectRegistryTransactorRaw struct {
	Contract *ProjectRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewProjectRegistry creates a new instance of ProjectRegistry, bound to a specific deployed contract.
func NewProjectRegistry(address common.Address, backend bind.ContractBackend) (*ProjectRegistry, error) {
	contract, err := bindProjectRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistry{ProjectRegistryCaller: ProjectRegistryCaller{contract: contract}, ProjectRegistryTransactor: ProjectRegistryTransactor{contract: contract}, ProjectRegistryFilterer: ProjectRegistryFilterer{contract: contract}}, nil
}

// NewProjectRegistryCaller creates a new read-only instance of ProjectRegistry, bound to a specific deployed contract.
func NewProjectRegistryCaller(address common.Address, caller bind.ContractCaller) (*ProjectRegistryCaller, error) {
	contract, err := bindProjectRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistryCaller{contract: contract}, nil
}

// NewProjectRegistryTransactor creates a new write-only instance of ProjectRegistry, bound to a specific deployed contract.
func NewProjectRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*ProjectRegistryTransactor, error) {
	contract, err := bindProjectRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistryTransactor{contract: contract}, nil
}

// NewProjectRegistryFilterer creates a new log filterer instance of ProjectRegistry, bound to a specific deployed contract.
func NewProjectRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*ProjectRegistryFilterer, error) {
	contract, err := bindProjectRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistryFilterer{contract: contract}, nil
}

// bindProjectRegistry binds a generic wrapper to an already deployed contract.
func bindProjectRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := ProjectRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ProjectRegistry *ProjectRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ProjectRegistry.Contract.ProjectRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ProjectRegistry *ProjectRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ProjectRegistry.Contract.ProjectRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ProjectRegistry *ProjectRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ProjectRegistry.Contract.ProjectRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ProjectRegistry *ProjectRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ProjectRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ProjectRegistry *ProjectRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ProjectRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ProjectRegistry *ProjectRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ProjectRegistry.Contract.contract.Transact(opts, method, params...)
}

// Project is a free data retrieval call binding the contract method 0xf60ca60d.
//
// Solidity: function project() view returns(address)
func (_ProjectRegistry *ProjectRegistryCaller) Project(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ProjectRegistry.contract.Call(opts, &out, "project")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Project is a free data retrieval call binding the contract method 0xf60ca60d.
//
// Solidity: function project() view returns(address)
func (_ProjectRegistry *ProjectRegistrySession) Project() (common.Address, error) {
	return _ProjectRegistry.Contract.Project(&_ProjectRegistry.CallOpts)
}

// Project is a free data retrieval call binding the contract method 0xf60ca60d.
//
// Solidity: function project() view returns(address)
func (_ProjectRegistry *ProjectRegistryCallerSession) Project() (common.Address, error) {
	return _ProjectRegistry.Contract.Project(&_ProjectRegistry.CallOpts)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _project) returns()
func (_ProjectRegistry *ProjectRegistryTransactor) Initialize(opts *bind.TransactOpts, _project common.Address) (*types.Transaction, error) {
	return _ProjectRegistry.contract.Transact(opts, "initialize", _project)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _project) returns()
func (_ProjectRegistry *ProjectRegistrySession) Initialize(_project common.Address) (*types.Transaction, error) {
	return _ProjectRegistry.Contract.Initialize(&_ProjectRegistry.TransactOpts, _project)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _project) returns()
func (_ProjectRegistry *ProjectRegistryTransactorSession) Initialize(_project common.Address) (*types.Transaction, error) {
	return _ProjectRegistry.Contract.Initialize(&_ProjectRegistry.TransactOpts, _project)
}

// Register is a paid mutator transaction binding the contract method 0x1aa3a008.
//
// Solidity: function register() payable returns(uint256)
func (_ProjectRegistry *ProjectRegistryTransactor) Register(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ProjectRegistry.contract.Transact(opts, "register")
}

// Register is a paid mutator transaction binding the contract method 0x1aa3a008.
//
// Solidity: function register() payable returns(uint256)
func (_ProjectRegistry *ProjectRegistrySession) Register() (*types.Transaction, error) {
	return _ProjectRegistry.Contract.Register(&_ProjectRegistry.TransactOpts)
}

// Register is a paid mutator transaction binding the contract method 0x1aa3a008.
//
// Solidity: function register() payable returns(uint256)
func (_ProjectRegistry *ProjectRegistryTransactorSession) Register() (*types.Transaction, error) {
	return _ProjectRegistry.Contract.Register(&_ProjectRegistry.TransactOpts)
}

// Register0 is a paid mutator transaction binding the contract method 0x767b79ed.
//
// Solidity: function register(string _name, uint8 _type) payable returns(uint256)
func (_ProjectRegistry *ProjectRegistryTransactor) Register0(opts *bind.TransactOpts, _name string, _type uint8) (*types.Transaction, error) {
	return _ProjectRegistry.contract.Transact(opts, "register0", _name, _type)
}

// Register0 is a paid mutator transaction binding the contract method 0x767b79ed.
//
// Solidity: function register(string _name, uint8 _type) payable returns(uint256)
func (_ProjectRegistry *ProjectRegistrySession) Register0(_name string, _type uint8) (*types.Transaction, error) {
	return _ProjectRegistry.Contract.Register0(&_ProjectRegistry.TransactOpts, _name, _type)
}

// Register0 is a paid mutator transaction binding the contract method 0x767b79ed.
//
// Solidity: function register(string _name, uint8 _type) payable returns(uint256)
func (_ProjectRegistry *ProjectRegistryTransactorSession) Register0(_name string, _type uint8) (*types.Transaction, error) {
	return _ProjectRegistry.Contract.Register0(&_ProjectRegistry.TransactOpts, _name, _type)
}

// Register1 is a paid mutator transaction binding the contract method 0xf2c298be.
//
// Solidity: function register(string _name) payable returns(uint256)
func (_ProjectRegistry *ProjectRegistryTransactor) Register1(opts *bind.TransactOpts, _name string) (*types.Transaction, error) {
	return _ProjectRegistry.contract.Transact(opts, "register1", _name)
}

// Register1 is a paid mutator transaction binding the contract method 0xf2c298be.
//
// Solidity: function register(string _name) payable returns(uint256)
func (_ProjectRegistry *ProjectRegistrySession) Register1(_name string) (*types.Transaction, error) {
	return _ProjectRegistry.Contract.Register1(&_ProjectRegistry.TransactOpts, _name)
}

// Register1 is a paid mutator transaction binding the contract method 0xf2c298be.
//
// Solidity: function register(string _name) payable returns(uint256)
func (_ProjectRegistry *ProjectRegistryTransactorSession) Register1(_name string) (*types.Transaction, error) {
	return _ProjectRegistry.Contract.Register1(&_ProjectRegistry.TransactOpts, _name)
}

// ProjectRegistryInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the ProjectRegistry contract.
type ProjectRegistryInitializedIterator struct {
	Event *ProjectRegistryInitialized // Event containing the contract specifics and raw log

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
func (it *ProjectRegistryInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProjectRegistryInitialized)
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
		it.Event = new(ProjectRegistryInitialized)
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
func (it *ProjectRegistryInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProjectRegistryInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProjectRegistryInitialized represents a Initialized event raised by the ProjectRegistry contract.
type ProjectRegistryInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_ProjectRegistry *ProjectRegistryFilterer) FilterInitialized(opts *bind.FilterOpts) (*ProjectRegistryInitializedIterator, error) {

	logs, sub, err := _ProjectRegistry.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &ProjectRegistryInitializedIterator{contract: _ProjectRegistry.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_ProjectRegistry *ProjectRegistryFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *ProjectRegistryInitialized) (event.Subscription, error) {

	logs, sub, err := _ProjectRegistry.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProjectRegistryInitialized)
				if err := _ProjectRegistry.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_ProjectRegistry *ProjectRegistryFilterer) ParseInitialized(log types.Log) (*ProjectRegistryInitialized, error) {
	event := new(ProjectRegistryInitialized)
	if err := _ProjectRegistry.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

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

// ProjectRegistrarMetaData contains all meta data concerning the ProjectRegistrar contract.
var ProjectRegistrarMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"FeeWithdrawn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"}],\"name\":\"ProjectRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fee\",\"type\":\"uint256\"}],\"name\":\"RegistrationFeeSet\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_projectStore\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"projectStore\",\"outputs\":[{\"internalType\":\"contractIProjectStore\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"}],\"name\":\"register\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"registrationFee\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_fee\",\"type\":\"uint256\"}],\"name\":\"setRegistrationFee\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"addresspayable\",\"name\":\"_account\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"withdrawFee\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// ProjectRegistrarABI is the input ABI used to generate the binding from.
// Deprecated: Use ProjectRegistrarMetaData.ABI instead.
var ProjectRegistrarABI = ProjectRegistrarMetaData.ABI

// ProjectRegistrar is an auto generated Go binding around an Ethereum contract.
type ProjectRegistrar struct {
	ProjectRegistrarCaller     // Read-only binding to the contract
	ProjectRegistrarTransactor // Write-only binding to the contract
	ProjectRegistrarFilterer   // Log filterer for contract events
}

// ProjectRegistrarCaller is an auto generated read-only Go binding around an Ethereum contract.
type ProjectRegistrarCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProjectRegistrarTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ProjectRegistrarTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProjectRegistrarFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ProjectRegistrarFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProjectRegistrarSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ProjectRegistrarSession struct {
	Contract     *ProjectRegistrar // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ProjectRegistrarCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ProjectRegistrarCallerSession struct {
	Contract *ProjectRegistrarCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// ProjectRegistrarTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ProjectRegistrarTransactorSession struct {
	Contract     *ProjectRegistrarTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// ProjectRegistrarRaw is an auto generated low-level Go binding around an Ethereum contract.
type ProjectRegistrarRaw struct {
	Contract *ProjectRegistrar // Generic contract binding to access the raw methods on
}

// ProjectRegistrarCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ProjectRegistrarCallerRaw struct {
	Contract *ProjectRegistrarCaller // Generic read-only contract binding to access the raw methods on
}

// ProjectRegistrarTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ProjectRegistrarTransactorRaw struct {
	Contract *ProjectRegistrarTransactor // Generic write-only contract binding to access the raw methods on
}

// NewProjectRegistrar creates a new instance of ProjectRegistrar, bound to a specific deployed contract.
func NewProjectRegistrar(address common.Address, backend bind.ContractBackend) (*ProjectRegistrar, error) {
	contract, err := bindProjectRegistrar(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistrar{ProjectRegistrarCaller: ProjectRegistrarCaller{contract: contract}, ProjectRegistrarTransactor: ProjectRegistrarTransactor{contract: contract}, ProjectRegistrarFilterer: ProjectRegistrarFilterer{contract: contract}}, nil
}

// NewProjectRegistrarCaller creates a new read-only instance of ProjectRegistrar, bound to a specific deployed contract.
func NewProjectRegistrarCaller(address common.Address, caller bind.ContractCaller) (*ProjectRegistrarCaller, error) {
	contract, err := bindProjectRegistrar(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistrarCaller{contract: contract}, nil
}

// NewProjectRegistrarTransactor creates a new write-only instance of ProjectRegistrar, bound to a specific deployed contract.
func NewProjectRegistrarTransactor(address common.Address, transactor bind.ContractTransactor) (*ProjectRegistrarTransactor, error) {
	contract, err := bindProjectRegistrar(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistrarTransactor{contract: contract}, nil
}

// NewProjectRegistrarFilterer creates a new log filterer instance of ProjectRegistrar, bound to a specific deployed contract.
func NewProjectRegistrarFilterer(address common.Address, filterer bind.ContractFilterer) (*ProjectRegistrarFilterer, error) {
	contract, err := bindProjectRegistrar(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistrarFilterer{contract: contract}, nil
}

// bindProjectRegistrar binds a generic wrapper to an already deployed contract.
func bindProjectRegistrar(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := ProjectRegistrarMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ProjectRegistrar *ProjectRegistrarRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ProjectRegistrar.Contract.ProjectRegistrarCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ProjectRegistrar *ProjectRegistrarRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.ProjectRegistrarTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ProjectRegistrar *ProjectRegistrarRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.ProjectRegistrarTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ProjectRegistrar *ProjectRegistrarCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ProjectRegistrar.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ProjectRegistrar *ProjectRegistrarTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ProjectRegistrar *ProjectRegistrarTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.contract.Transact(opts, method, params...)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_ProjectRegistrar *ProjectRegistrarCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ProjectRegistrar.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_ProjectRegistrar *ProjectRegistrarSession) Owner() (common.Address, error) {
	return _ProjectRegistrar.Contract.Owner(&_ProjectRegistrar.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_ProjectRegistrar *ProjectRegistrarCallerSession) Owner() (common.Address, error) {
	return _ProjectRegistrar.Contract.Owner(&_ProjectRegistrar.CallOpts)
}

// ProjectStore is a free data retrieval call binding the contract method 0xa0fadaaa.
//
// Solidity: function projectStore() view returns(address)
func (_ProjectRegistrar *ProjectRegistrarCaller) ProjectStore(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ProjectRegistrar.contract.Call(opts, &out, "projectStore")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ProjectStore is a free data retrieval call binding the contract method 0xa0fadaaa.
//
// Solidity: function projectStore() view returns(address)
func (_ProjectRegistrar *ProjectRegistrarSession) ProjectStore() (common.Address, error) {
	return _ProjectRegistrar.Contract.ProjectStore(&_ProjectRegistrar.CallOpts)
}

// ProjectStore is a free data retrieval call binding the contract method 0xa0fadaaa.
//
// Solidity: function projectStore() view returns(address)
func (_ProjectRegistrar *ProjectRegistrarCallerSession) ProjectStore() (common.Address, error) {
	return _ProjectRegistrar.Contract.ProjectStore(&_ProjectRegistrar.CallOpts)
}

// RegistrationFee is a free data retrieval call binding the contract method 0x14c44e09.
//
// Solidity: function registrationFee() view returns(uint256)
func (_ProjectRegistrar *ProjectRegistrarCaller) RegistrationFee(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _ProjectRegistrar.contract.Call(opts, &out, "registrationFee")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RegistrationFee is a free data retrieval call binding the contract method 0x14c44e09.
//
// Solidity: function registrationFee() view returns(uint256)
func (_ProjectRegistrar *ProjectRegistrarSession) RegistrationFee() (*big.Int, error) {
	return _ProjectRegistrar.Contract.RegistrationFee(&_ProjectRegistrar.CallOpts)
}

// RegistrationFee is a free data retrieval call binding the contract method 0x14c44e09.
//
// Solidity: function registrationFee() view returns(uint256)
func (_ProjectRegistrar *ProjectRegistrarCallerSession) RegistrationFee() (*big.Int, error) {
	return _ProjectRegistrar.Contract.RegistrationFee(&_ProjectRegistrar.CallOpts)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _projectStore) returns()
func (_ProjectRegistrar *ProjectRegistrarTransactor) Initialize(opts *bind.TransactOpts, _projectStore common.Address) (*types.Transaction, error) {
	return _ProjectRegistrar.contract.Transact(opts, "initialize", _projectStore)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _projectStore) returns()
func (_ProjectRegistrar *ProjectRegistrarSession) Initialize(_projectStore common.Address) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.Initialize(&_ProjectRegistrar.TransactOpts, _projectStore)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _projectStore) returns()
func (_ProjectRegistrar *ProjectRegistrarTransactorSession) Initialize(_projectStore common.Address) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.Initialize(&_ProjectRegistrar.TransactOpts, _projectStore)
}

// Register is a paid mutator transaction binding the contract method 0xf207564e.
//
// Solidity: function register(uint256 _projectId) payable returns()
func (_ProjectRegistrar *ProjectRegistrarTransactor) Register(opts *bind.TransactOpts, _projectId *big.Int) (*types.Transaction, error) {
	return _ProjectRegistrar.contract.Transact(opts, "register", _projectId)
}

// Register is a paid mutator transaction binding the contract method 0xf207564e.
//
// Solidity: function register(uint256 _projectId) payable returns()
func (_ProjectRegistrar *ProjectRegistrarSession) Register(_projectId *big.Int) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.Register(&_ProjectRegistrar.TransactOpts, _projectId)
}

// Register is a paid mutator transaction binding the contract method 0xf207564e.
//
// Solidity: function register(uint256 _projectId) payable returns()
func (_ProjectRegistrar *ProjectRegistrarTransactorSession) Register(_projectId *big.Int) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.Register(&_ProjectRegistrar.TransactOpts, _projectId)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_ProjectRegistrar *ProjectRegistrarTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ProjectRegistrar.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_ProjectRegistrar *ProjectRegistrarSession) RenounceOwnership() (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.RenounceOwnership(&_ProjectRegistrar.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_ProjectRegistrar *ProjectRegistrarTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.RenounceOwnership(&_ProjectRegistrar.TransactOpts)
}

// SetRegistrationFee is a paid mutator transaction binding the contract method 0xc320c727.
//
// Solidity: function setRegistrationFee(uint256 _fee) returns()
func (_ProjectRegistrar *ProjectRegistrarTransactor) SetRegistrationFee(opts *bind.TransactOpts, _fee *big.Int) (*types.Transaction, error) {
	return _ProjectRegistrar.contract.Transact(opts, "setRegistrationFee", _fee)
}

// SetRegistrationFee is a paid mutator transaction binding the contract method 0xc320c727.
//
// Solidity: function setRegistrationFee(uint256 _fee) returns()
func (_ProjectRegistrar *ProjectRegistrarSession) SetRegistrationFee(_fee *big.Int) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.SetRegistrationFee(&_ProjectRegistrar.TransactOpts, _fee)
}

// SetRegistrationFee is a paid mutator transaction binding the contract method 0xc320c727.
//
// Solidity: function setRegistrationFee(uint256 _fee) returns()
func (_ProjectRegistrar *ProjectRegistrarTransactorSession) SetRegistrationFee(_fee *big.Int) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.SetRegistrationFee(&_ProjectRegistrar.TransactOpts, _fee)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_ProjectRegistrar *ProjectRegistrarTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _ProjectRegistrar.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_ProjectRegistrar *ProjectRegistrarSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.TransferOwnership(&_ProjectRegistrar.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_ProjectRegistrar *ProjectRegistrarTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.TransferOwnership(&_ProjectRegistrar.TransactOpts, newOwner)
}

// WithdrawFee is a paid mutator transaction binding the contract method 0xfd9be522.
//
// Solidity: function withdrawFee(address _account, uint256 _amount) returns()
func (_ProjectRegistrar *ProjectRegistrarTransactor) WithdrawFee(opts *bind.TransactOpts, _account common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _ProjectRegistrar.contract.Transact(opts, "withdrawFee", _account, _amount)
}

// WithdrawFee is a paid mutator transaction binding the contract method 0xfd9be522.
//
// Solidity: function withdrawFee(address _account, uint256 _amount) returns()
func (_ProjectRegistrar *ProjectRegistrarSession) WithdrawFee(_account common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.WithdrawFee(&_ProjectRegistrar.TransactOpts, _account, _amount)
}

// WithdrawFee is a paid mutator transaction binding the contract method 0xfd9be522.
//
// Solidity: function withdrawFee(address _account, uint256 _amount) returns()
func (_ProjectRegistrar *ProjectRegistrarTransactorSession) WithdrawFee(_account common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _ProjectRegistrar.Contract.WithdrawFee(&_ProjectRegistrar.TransactOpts, _account, _amount)
}

// ProjectRegistrarFeeWithdrawnIterator is returned from FilterFeeWithdrawn and is used to iterate over the raw logs and unpacked data for FeeWithdrawn events raised by the ProjectRegistrar contract.
type ProjectRegistrarFeeWithdrawnIterator struct {
	Event *ProjectRegistrarFeeWithdrawn // Event containing the contract specifics and raw log

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
func (it *ProjectRegistrarFeeWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProjectRegistrarFeeWithdrawn)
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
		it.Event = new(ProjectRegistrarFeeWithdrawn)
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
func (it *ProjectRegistrarFeeWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProjectRegistrarFeeWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProjectRegistrarFeeWithdrawn represents a FeeWithdrawn event raised by the ProjectRegistrar contract.
type ProjectRegistrarFeeWithdrawn struct {
	Account common.Address
	Amount  *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterFeeWithdrawn is a free log retrieval operation binding the contract event 0x78473f3f373f7673597f4f0fa5873cb4d375fea6d4339ad6b56dbd411513cb3f.
//
// Solidity: event FeeWithdrawn(address indexed account, uint256 amount)
func (_ProjectRegistrar *ProjectRegistrarFilterer) FilterFeeWithdrawn(opts *bind.FilterOpts, account []common.Address) (*ProjectRegistrarFeeWithdrawnIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _ProjectRegistrar.contract.FilterLogs(opts, "FeeWithdrawn", accountRule)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistrarFeeWithdrawnIterator{contract: _ProjectRegistrar.contract, event: "FeeWithdrawn", logs: logs, sub: sub}, nil
}

// WatchFeeWithdrawn is a free log subscription operation binding the contract event 0x78473f3f373f7673597f4f0fa5873cb4d375fea6d4339ad6b56dbd411513cb3f.
//
// Solidity: event FeeWithdrawn(address indexed account, uint256 amount)
func (_ProjectRegistrar *ProjectRegistrarFilterer) WatchFeeWithdrawn(opts *bind.WatchOpts, sink chan<- *ProjectRegistrarFeeWithdrawn, account []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _ProjectRegistrar.contract.WatchLogs(opts, "FeeWithdrawn", accountRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProjectRegistrarFeeWithdrawn)
				if err := _ProjectRegistrar.contract.UnpackLog(event, "FeeWithdrawn", log); err != nil {
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
func (_ProjectRegistrar *ProjectRegistrarFilterer) ParseFeeWithdrawn(log types.Log) (*ProjectRegistrarFeeWithdrawn, error) {
	event := new(ProjectRegistrarFeeWithdrawn)
	if err := _ProjectRegistrar.contract.UnpackLog(event, "FeeWithdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ProjectRegistrarInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the ProjectRegistrar contract.
type ProjectRegistrarInitializedIterator struct {
	Event *ProjectRegistrarInitialized // Event containing the contract specifics and raw log

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
func (it *ProjectRegistrarInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProjectRegistrarInitialized)
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
		it.Event = new(ProjectRegistrarInitialized)
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
func (it *ProjectRegistrarInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProjectRegistrarInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProjectRegistrarInitialized represents a Initialized event raised by the ProjectRegistrar contract.
type ProjectRegistrarInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_ProjectRegistrar *ProjectRegistrarFilterer) FilterInitialized(opts *bind.FilterOpts) (*ProjectRegistrarInitializedIterator, error) {

	logs, sub, err := _ProjectRegistrar.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &ProjectRegistrarInitializedIterator{contract: _ProjectRegistrar.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_ProjectRegistrar *ProjectRegistrarFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *ProjectRegistrarInitialized) (event.Subscription, error) {

	logs, sub, err := _ProjectRegistrar.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProjectRegistrarInitialized)
				if err := _ProjectRegistrar.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_ProjectRegistrar *ProjectRegistrarFilterer) ParseInitialized(log types.Log) (*ProjectRegistrarInitialized, error) {
	event := new(ProjectRegistrarInitialized)
	if err := _ProjectRegistrar.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ProjectRegistrarOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the ProjectRegistrar contract.
type ProjectRegistrarOwnershipTransferredIterator struct {
	Event *ProjectRegistrarOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *ProjectRegistrarOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProjectRegistrarOwnershipTransferred)
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
		it.Event = new(ProjectRegistrarOwnershipTransferred)
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
func (it *ProjectRegistrarOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProjectRegistrarOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProjectRegistrarOwnershipTransferred represents a OwnershipTransferred event raised by the ProjectRegistrar contract.
type ProjectRegistrarOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_ProjectRegistrar *ProjectRegistrarFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*ProjectRegistrarOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _ProjectRegistrar.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistrarOwnershipTransferredIterator{contract: _ProjectRegistrar.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_ProjectRegistrar *ProjectRegistrarFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *ProjectRegistrarOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _ProjectRegistrar.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProjectRegistrarOwnershipTransferred)
				if err := _ProjectRegistrar.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_ProjectRegistrar *ProjectRegistrarFilterer) ParseOwnershipTransferred(log types.Log) (*ProjectRegistrarOwnershipTransferred, error) {
	event := new(ProjectRegistrarOwnershipTransferred)
	if err := _ProjectRegistrar.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ProjectRegistrarProjectRegisteredIterator is returned from FilterProjectRegistered and is used to iterate over the raw logs and unpacked data for ProjectRegistered events raised by the ProjectRegistrar contract.
type ProjectRegistrarProjectRegisteredIterator struct {
	Event *ProjectRegistrarProjectRegistered // Event containing the contract specifics and raw log

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
func (it *ProjectRegistrarProjectRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProjectRegistrarProjectRegistered)
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
		it.Event = new(ProjectRegistrarProjectRegistered)
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
func (it *ProjectRegistrarProjectRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProjectRegistrarProjectRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProjectRegistrarProjectRegistered represents a ProjectRegistered event raised by the ProjectRegistrar contract.
type ProjectRegistrarProjectRegistered struct {
	ProjectId *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterProjectRegistered is a free log retrieval operation binding the contract event 0xfee73c70d73668319c51a83ac3b19cbe37da12af171bec2e93cadea345ff13bd.
//
// Solidity: event ProjectRegistered(uint256 indexed projectId)
func (_ProjectRegistrar *ProjectRegistrarFilterer) FilterProjectRegistered(opts *bind.FilterOpts, projectId []*big.Int) (*ProjectRegistrarProjectRegisteredIterator, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}

	logs, sub, err := _ProjectRegistrar.contract.FilterLogs(opts, "ProjectRegistered", projectIdRule)
	if err != nil {
		return nil, err
	}
	return &ProjectRegistrarProjectRegisteredIterator{contract: _ProjectRegistrar.contract, event: "ProjectRegistered", logs: logs, sub: sub}, nil
}

// WatchProjectRegistered is a free log subscription operation binding the contract event 0xfee73c70d73668319c51a83ac3b19cbe37da12af171bec2e93cadea345ff13bd.
//
// Solidity: event ProjectRegistered(uint256 indexed projectId)
func (_ProjectRegistrar *ProjectRegistrarFilterer) WatchProjectRegistered(opts *bind.WatchOpts, sink chan<- *ProjectRegistrarProjectRegistered, projectId []*big.Int) (event.Subscription, error) {

	var projectIdRule []interface{}
	for _, projectIdItem := range projectId {
		projectIdRule = append(projectIdRule, projectIdItem)
	}

	logs, sub, err := _ProjectRegistrar.contract.WatchLogs(opts, "ProjectRegistered", projectIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProjectRegistrarProjectRegistered)
				if err := _ProjectRegistrar.contract.UnpackLog(event, "ProjectRegistered", log); err != nil {
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

// ParseProjectRegistered is a log parse operation binding the contract event 0xfee73c70d73668319c51a83ac3b19cbe37da12af171bec2e93cadea345ff13bd.
//
// Solidity: event ProjectRegistered(uint256 indexed projectId)
func (_ProjectRegistrar *ProjectRegistrarFilterer) ParseProjectRegistered(log types.Log) (*ProjectRegistrarProjectRegistered, error) {
	event := new(ProjectRegistrarProjectRegistered)
	if err := _ProjectRegistrar.contract.UnpackLog(event, "ProjectRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ProjectRegistrarRegistrationFeeSetIterator is returned from FilterRegistrationFeeSet and is used to iterate over the raw logs and unpacked data for RegistrationFeeSet events raised by the ProjectRegistrar contract.
type ProjectRegistrarRegistrationFeeSetIterator struct {
	Event *ProjectRegistrarRegistrationFeeSet // Event containing the contract specifics and raw log

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
func (it *ProjectRegistrarRegistrationFeeSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProjectRegistrarRegistrationFeeSet)
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
		it.Event = new(ProjectRegistrarRegistrationFeeSet)
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
func (it *ProjectRegistrarRegistrationFeeSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProjectRegistrarRegistrationFeeSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProjectRegistrarRegistrationFeeSet represents a RegistrationFeeSet event raised by the ProjectRegistrar contract.
type ProjectRegistrarRegistrationFeeSet struct {
	Fee *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterRegistrationFeeSet is a free log retrieval operation binding the contract event 0x3836764761eb705a9815e9070541f9a604757da847d45d157a28f14b58b5f5e6.
//
// Solidity: event RegistrationFeeSet(uint256 fee)
func (_ProjectRegistrar *ProjectRegistrarFilterer) FilterRegistrationFeeSet(opts *bind.FilterOpts) (*ProjectRegistrarRegistrationFeeSetIterator, error) {

	logs, sub, err := _ProjectRegistrar.contract.FilterLogs(opts, "RegistrationFeeSet")
	if err != nil {
		return nil, err
	}
	return &ProjectRegistrarRegistrationFeeSetIterator{contract: _ProjectRegistrar.contract, event: "RegistrationFeeSet", logs: logs, sub: sub}, nil
}

// WatchRegistrationFeeSet is a free log subscription operation binding the contract event 0x3836764761eb705a9815e9070541f9a604757da847d45d157a28f14b58b5f5e6.
//
// Solidity: event RegistrationFeeSet(uint256 fee)
func (_ProjectRegistrar *ProjectRegistrarFilterer) WatchRegistrationFeeSet(opts *bind.WatchOpts, sink chan<- *ProjectRegistrarRegistrationFeeSet) (event.Subscription, error) {

	logs, sub, err := _ProjectRegistrar.contract.WatchLogs(opts, "RegistrationFeeSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProjectRegistrarRegistrationFeeSet)
				if err := _ProjectRegistrar.contract.UnpackLog(event, "RegistrationFeeSet", log); err != nil {
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
func (_ProjectRegistrar *ProjectRegistrarFilterer) ParseRegistrationFeeSet(log types.Log) (*ProjectRegistrarRegistrationFeeSet, error) {
	event := new(ProjectRegistrarRegistrationFeeSet)
	if err := _ProjectRegistrar.contract.UnpackLog(event, "RegistrationFeeSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

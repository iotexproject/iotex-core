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

// ProjectDeviceMetaData contains all meta data concerning the ProjectDevice contract.
var ProjectDeviceMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"device\",\"type\":\"address\"}],\"name\":\"Approve\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"projectId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"device\",\"type\":\"address\"}],\"name\":\"Unapprove\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"address[]\",\"name\":\"_devices\",\"type\":\"address[]\"}],\"name\":\"approve\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"_device\",\"type\":\"address\"}],\"name\":\"approved\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_ioIDRegistry\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_w3bstreamProject\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"ioIDRegistry\",\"outputs\":[{\"internalType\":\"contractIioIDRegistry\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_projectId\",\"type\":\"uint256\"},{\"internalType\":\"address[]\",\"name\":\"_devices\",\"type\":\"address[]\"}],\"name\":\"unapprove\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"w3bstreamProject\",\"outputs\":[{\"internalType\":\"contractIW3bstreamProject\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// ProjectDeviceABI is the input ABI used to generate the binding from.
// Deprecated: Use ProjectDeviceMetaData.ABI instead.
var ProjectDeviceABI = ProjectDeviceMetaData.ABI

// ProjectDevice is an auto generated Go binding around an Ethereum contract.
type ProjectDevice struct {
	ProjectDeviceCaller     // Read-only binding to the contract
	ProjectDeviceTransactor // Write-only binding to the contract
	ProjectDeviceFilterer   // Log filterer for contract events
}

// ProjectDeviceCaller is an auto generated read-only Go binding around an Ethereum contract.
type ProjectDeviceCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProjectDeviceTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ProjectDeviceTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProjectDeviceFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ProjectDeviceFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProjectDeviceSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ProjectDeviceSession struct {
	Contract     *ProjectDevice    // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ProjectDeviceCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ProjectDeviceCallerSession struct {
	Contract *ProjectDeviceCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts        // Call options to use throughout this session
}

// ProjectDeviceTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ProjectDeviceTransactorSession struct {
	Contract     *ProjectDeviceTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// ProjectDeviceRaw is an auto generated low-level Go binding around an Ethereum contract.
type ProjectDeviceRaw struct {
	Contract *ProjectDevice // Generic contract binding to access the raw methods on
}

// ProjectDeviceCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ProjectDeviceCallerRaw struct {
	Contract *ProjectDeviceCaller // Generic read-only contract binding to access the raw methods on
}

// ProjectDeviceTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ProjectDeviceTransactorRaw struct {
	Contract *ProjectDeviceTransactor // Generic write-only contract binding to access the raw methods on
}

// NewProjectDevice creates a new instance of ProjectDevice, bound to a specific deployed contract.
func NewProjectDevice(address common.Address, backend bind.ContractBackend) (*ProjectDevice, error) {
	contract, err := bindProjectDevice(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ProjectDevice{ProjectDeviceCaller: ProjectDeviceCaller{contract: contract}, ProjectDeviceTransactor: ProjectDeviceTransactor{contract: contract}, ProjectDeviceFilterer: ProjectDeviceFilterer{contract: contract}}, nil
}

// NewProjectDeviceCaller creates a new read-only instance of ProjectDevice, bound to a specific deployed contract.
func NewProjectDeviceCaller(address common.Address, caller bind.ContractCaller) (*ProjectDeviceCaller, error) {
	contract, err := bindProjectDevice(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ProjectDeviceCaller{contract: contract}, nil
}

// NewProjectDeviceTransactor creates a new write-only instance of ProjectDevice, bound to a specific deployed contract.
func NewProjectDeviceTransactor(address common.Address, transactor bind.ContractTransactor) (*ProjectDeviceTransactor, error) {
	contract, err := bindProjectDevice(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ProjectDeviceTransactor{contract: contract}, nil
}

// NewProjectDeviceFilterer creates a new log filterer instance of ProjectDevice, bound to a specific deployed contract.
func NewProjectDeviceFilterer(address common.Address, filterer bind.ContractFilterer) (*ProjectDeviceFilterer, error) {
	contract, err := bindProjectDevice(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ProjectDeviceFilterer{contract: contract}, nil
}

// bindProjectDevice binds a generic wrapper to an already deployed contract.
func bindProjectDevice(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := ProjectDeviceMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ProjectDevice *ProjectDeviceRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ProjectDevice.Contract.ProjectDeviceCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ProjectDevice *ProjectDeviceRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ProjectDevice.Contract.ProjectDeviceTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ProjectDevice *ProjectDeviceRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ProjectDevice.Contract.ProjectDeviceTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ProjectDevice *ProjectDeviceCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ProjectDevice.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ProjectDevice *ProjectDeviceTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ProjectDevice.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ProjectDevice *ProjectDeviceTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ProjectDevice.Contract.contract.Transact(opts, method, params...)
}

// Approved is a free data retrieval call binding the contract method 0x8253951a.
//
// Solidity: function approved(uint256 _projectId, address _device) view returns(bool)
func (_ProjectDevice *ProjectDeviceCaller) Approved(opts *bind.CallOpts, _projectId *big.Int, _device common.Address) (bool, error) {
	var out []interface{}
	err := _ProjectDevice.contract.Call(opts, &out, "approved", _projectId, _device)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Approved is a free data retrieval call binding the contract method 0x8253951a.
//
// Solidity: function approved(uint256 _projectId, address _device) view returns(bool)
func (_ProjectDevice *ProjectDeviceSession) Approved(_projectId *big.Int, _device common.Address) (bool, error) {
	return _ProjectDevice.Contract.Approved(&_ProjectDevice.CallOpts, _projectId, _device)
}

// Approved is a free data retrieval call binding the contract method 0x8253951a.
//
// Solidity: function approved(uint256 _projectId, address _device) view returns(bool)
func (_ProjectDevice *ProjectDeviceCallerSession) Approved(_projectId *big.Int, _device common.Address) (bool, error) {
	return _ProjectDevice.Contract.Approved(&_ProjectDevice.CallOpts, _projectId, _device)
}

// IoIDRegistry is a free data retrieval call binding the contract method 0x95cc7086.
//
// Solidity: function ioIDRegistry() view returns(address)
func (_ProjectDevice *ProjectDeviceCaller) IoIDRegistry(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ProjectDevice.contract.Call(opts, &out, "ioIDRegistry")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// IoIDRegistry is a free data retrieval call binding the contract method 0x95cc7086.
//
// Solidity: function ioIDRegistry() view returns(address)
func (_ProjectDevice *ProjectDeviceSession) IoIDRegistry() (common.Address, error) {
	return _ProjectDevice.Contract.IoIDRegistry(&_ProjectDevice.CallOpts)
}

// IoIDRegistry is a free data retrieval call binding the contract method 0x95cc7086.
//
// Solidity: function ioIDRegistry() view returns(address)
func (_ProjectDevice *ProjectDeviceCallerSession) IoIDRegistry() (common.Address, error) {
	return _ProjectDevice.Contract.IoIDRegistry(&_ProjectDevice.CallOpts)
}

// W3bstreamProject is a free data retrieval call binding the contract method 0x561f1fc4.
//
// Solidity: function w3bstreamProject() view returns(address)
func (_ProjectDevice *ProjectDeviceCaller) W3bstreamProject(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ProjectDevice.contract.Call(opts, &out, "w3bstreamProject")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// W3bstreamProject is a free data retrieval call binding the contract method 0x561f1fc4.
//
// Solidity: function w3bstreamProject() view returns(address)
func (_ProjectDevice *ProjectDeviceSession) W3bstreamProject() (common.Address, error) {
	return _ProjectDevice.Contract.W3bstreamProject(&_ProjectDevice.CallOpts)
}

// W3bstreamProject is a free data retrieval call binding the contract method 0x561f1fc4.
//
// Solidity: function w3bstreamProject() view returns(address)
func (_ProjectDevice *ProjectDeviceCallerSession) W3bstreamProject() (common.Address, error) {
	return _ProjectDevice.Contract.W3bstreamProject(&_ProjectDevice.CallOpts)
}

// Approve is a paid mutator transaction binding the contract method 0xd744d8dc.
//
// Solidity: function approve(uint256 _projectId, address[] _devices) returns()
func (_ProjectDevice *ProjectDeviceTransactor) Approve(opts *bind.TransactOpts, _projectId *big.Int, _devices []common.Address) (*types.Transaction, error) {
	return _ProjectDevice.contract.Transact(opts, "approve", _projectId, _devices)
}

// Approve is a paid mutator transaction binding the contract method 0xd744d8dc.
//
// Solidity: function approve(uint256 _projectId, address[] _devices) returns()
func (_ProjectDevice *ProjectDeviceSession) Approve(_projectId *big.Int, _devices []common.Address) (*types.Transaction, error) {
	return _ProjectDevice.Contract.Approve(&_ProjectDevice.TransactOpts, _projectId, _devices)
}

// Approve is a paid mutator transaction binding the contract method 0xd744d8dc.
//
// Solidity: function approve(uint256 _projectId, address[] _devices) returns()
func (_ProjectDevice *ProjectDeviceTransactorSession) Approve(_projectId *big.Int, _devices []common.Address) (*types.Transaction, error) {
	return _ProjectDevice.Contract.Approve(&_ProjectDevice.TransactOpts, _projectId, _devices)
}

// Initialize is a paid mutator transaction binding the contract method 0x485cc955.
//
// Solidity: function initialize(address _ioIDRegistry, address _w3bstreamProject) returns()
func (_ProjectDevice *ProjectDeviceTransactor) Initialize(opts *bind.TransactOpts, _ioIDRegistry common.Address, _w3bstreamProject common.Address) (*types.Transaction, error) {
	return _ProjectDevice.contract.Transact(opts, "initialize", _ioIDRegistry, _w3bstreamProject)
}

// Initialize is a paid mutator transaction binding the contract method 0x485cc955.
//
// Solidity: function initialize(address _ioIDRegistry, address _w3bstreamProject) returns()
func (_ProjectDevice *ProjectDeviceSession) Initialize(_ioIDRegistry common.Address, _w3bstreamProject common.Address) (*types.Transaction, error) {
	return _ProjectDevice.Contract.Initialize(&_ProjectDevice.TransactOpts, _ioIDRegistry, _w3bstreamProject)
}

// Initialize is a paid mutator transaction binding the contract method 0x485cc955.
//
// Solidity: function initialize(address _ioIDRegistry, address _w3bstreamProject) returns()
func (_ProjectDevice *ProjectDeviceTransactorSession) Initialize(_ioIDRegistry common.Address, _w3bstreamProject common.Address) (*types.Transaction, error) {
	return _ProjectDevice.Contract.Initialize(&_ProjectDevice.TransactOpts, _ioIDRegistry, _w3bstreamProject)
}

// Unapprove is a paid mutator transaction binding the contract method 0xfa6b352c.
//
// Solidity: function unapprove(uint256 _projectId, address[] _devices) returns()
func (_ProjectDevice *ProjectDeviceTransactor) Unapprove(opts *bind.TransactOpts, _projectId *big.Int, _devices []common.Address) (*types.Transaction, error) {
	return _ProjectDevice.contract.Transact(opts, "unapprove", _projectId, _devices)
}

// Unapprove is a paid mutator transaction binding the contract method 0xfa6b352c.
//
// Solidity: function unapprove(uint256 _projectId, address[] _devices) returns()
func (_ProjectDevice *ProjectDeviceSession) Unapprove(_projectId *big.Int, _devices []common.Address) (*types.Transaction, error) {
	return _ProjectDevice.Contract.Unapprove(&_ProjectDevice.TransactOpts, _projectId, _devices)
}

// Unapprove is a paid mutator transaction binding the contract method 0xfa6b352c.
//
// Solidity: function unapprove(uint256 _projectId, address[] _devices) returns()
func (_ProjectDevice *ProjectDeviceTransactorSession) Unapprove(_projectId *big.Int, _devices []common.Address) (*types.Transaction, error) {
	return _ProjectDevice.Contract.Unapprove(&_ProjectDevice.TransactOpts, _projectId, _devices)
}

// ProjectDeviceApproveIterator is returned from FilterApprove and is used to iterate over the raw logs and unpacked data for Approve events raised by the ProjectDevice contract.
type ProjectDeviceApproveIterator struct {
	Event *ProjectDeviceApprove // Event containing the contract specifics and raw log

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
func (it *ProjectDeviceApproveIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProjectDeviceApprove)
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
		it.Event = new(ProjectDeviceApprove)
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
func (it *ProjectDeviceApproveIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProjectDeviceApproveIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProjectDeviceApprove represents a Approve event raised by the ProjectDevice contract.
type ProjectDeviceApprove struct {
	ProjectId *big.Int
	Device    common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterApprove is a free log retrieval operation binding the contract event 0x47ad3e4b0f4bdbe3ac08708bbc45053d2ff616911b39e65563fd9bd781909645.
//
// Solidity: event Approve(uint256 projectId, address indexed device)
func (_ProjectDevice *ProjectDeviceFilterer) FilterApprove(opts *bind.FilterOpts, device []common.Address) (*ProjectDeviceApproveIterator, error) {

	var deviceRule []interface{}
	for _, deviceItem := range device {
		deviceRule = append(deviceRule, deviceItem)
	}

	logs, sub, err := _ProjectDevice.contract.FilterLogs(opts, "Approve", deviceRule)
	if err != nil {
		return nil, err
	}
	return &ProjectDeviceApproveIterator{contract: _ProjectDevice.contract, event: "Approve", logs: logs, sub: sub}, nil
}

// WatchApprove is a free log subscription operation binding the contract event 0x47ad3e4b0f4bdbe3ac08708bbc45053d2ff616911b39e65563fd9bd781909645.
//
// Solidity: event Approve(uint256 projectId, address indexed device)
func (_ProjectDevice *ProjectDeviceFilterer) WatchApprove(opts *bind.WatchOpts, sink chan<- *ProjectDeviceApprove, device []common.Address) (event.Subscription, error) {

	var deviceRule []interface{}
	for _, deviceItem := range device {
		deviceRule = append(deviceRule, deviceItem)
	}

	logs, sub, err := _ProjectDevice.contract.WatchLogs(opts, "Approve", deviceRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProjectDeviceApprove)
				if err := _ProjectDevice.contract.UnpackLog(event, "Approve", log); err != nil {
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

// ParseApprove is a log parse operation binding the contract event 0x47ad3e4b0f4bdbe3ac08708bbc45053d2ff616911b39e65563fd9bd781909645.
//
// Solidity: event Approve(uint256 projectId, address indexed device)
func (_ProjectDevice *ProjectDeviceFilterer) ParseApprove(log types.Log) (*ProjectDeviceApprove, error) {
	event := new(ProjectDeviceApprove)
	if err := _ProjectDevice.contract.UnpackLog(event, "Approve", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ProjectDeviceInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the ProjectDevice contract.
type ProjectDeviceInitializedIterator struct {
	Event *ProjectDeviceInitialized // Event containing the contract specifics and raw log

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
func (it *ProjectDeviceInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProjectDeviceInitialized)
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
		it.Event = new(ProjectDeviceInitialized)
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
func (it *ProjectDeviceInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProjectDeviceInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProjectDeviceInitialized represents a Initialized event raised by the ProjectDevice contract.
type ProjectDeviceInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_ProjectDevice *ProjectDeviceFilterer) FilterInitialized(opts *bind.FilterOpts) (*ProjectDeviceInitializedIterator, error) {

	logs, sub, err := _ProjectDevice.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &ProjectDeviceInitializedIterator{contract: _ProjectDevice.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_ProjectDevice *ProjectDeviceFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *ProjectDeviceInitialized) (event.Subscription, error) {

	logs, sub, err := _ProjectDevice.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProjectDeviceInitialized)
				if err := _ProjectDevice.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_ProjectDevice *ProjectDeviceFilterer) ParseInitialized(log types.Log) (*ProjectDeviceInitialized, error) {
	event := new(ProjectDeviceInitialized)
	if err := _ProjectDevice.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ProjectDeviceUnapproveIterator is returned from FilterUnapprove and is used to iterate over the raw logs and unpacked data for Unapprove events raised by the ProjectDevice contract.
type ProjectDeviceUnapproveIterator struct {
	Event *ProjectDeviceUnapprove // Event containing the contract specifics and raw log

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
func (it *ProjectDeviceUnapproveIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProjectDeviceUnapprove)
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
		it.Event = new(ProjectDeviceUnapprove)
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
func (it *ProjectDeviceUnapproveIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProjectDeviceUnapproveIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProjectDeviceUnapprove represents a Unapprove event raised by the ProjectDevice contract.
type ProjectDeviceUnapprove struct {
	ProjectId *big.Int
	Device    common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterUnapprove is a free log retrieval operation binding the contract event 0x958e4c39d74f2f6ff958f2ff19d4cf29448476d9e44e9aab9547f7e30d4cc86b.
//
// Solidity: event Unapprove(uint256 projectId, address indexed device)
func (_ProjectDevice *ProjectDeviceFilterer) FilterUnapprove(opts *bind.FilterOpts, device []common.Address) (*ProjectDeviceUnapproveIterator, error) {

	var deviceRule []interface{}
	for _, deviceItem := range device {
		deviceRule = append(deviceRule, deviceItem)
	}

	logs, sub, err := _ProjectDevice.contract.FilterLogs(opts, "Unapprove", deviceRule)
	if err != nil {
		return nil, err
	}
	return &ProjectDeviceUnapproveIterator{contract: _ProjectDevice.contract, event: "Unapprove", logs: logs, sub: sub}, nil
}

// WatchUnapprove is a free log subscription operation binding the contract event 0x958e4c39d74f2f6ff958f2ff19d4cf29448476d9e44e9aab9547f7e30d4cc86b.
//
// Solidity: event Unapprove(uint256 projectId, address indexed device)
func (_ProjectDevice *ProjectDeviceFilterer) WatchUnapprove(opts *bind.WatchOpts, sink chan<- *ProjectDeviceUnapprove, device []common.Address) (event.Subscription, error) {

	var deviceRule []interface{}
	for _, deviceItem := range device {
		deviceRule = append(deviceRule, deviceItem)
	}

	logs, sub, err := _ProjectDevice.contract.WatchLogs(opts, "Unapprove", deviceRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProjectDeviceUnapprove)
				if err := _ProjectDevice.contract.UnpackLog(event, "Unapprove", log); err != nil {
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

// ParseUnapprove is a log parse operation binding the contract event 0x958e4c39d74f2f6ff958f2ff19d4cf29448476d9e44e9aab9547f7e30d4cc86b.
//
// Solidity: event Unapprove(uint256 projectId, address indexed device)
func (_ProjectDevice *ProjectDeviceFilterer) ParseUnapprove(log types.Log) (*ProjectDeviceUnapprove, error) {
	event := new(ProjectDeviceUnapprove)
	if err := _ProjectDevice.contract.UnpackLog(event, "Unapprove", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

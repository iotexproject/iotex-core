// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

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

// RegisterABI is the input ABI used to generate the binding from.
const RegisterABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"bytes32\"},{\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"ioAddrToIdx\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"isOwner\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_name\",\"type\":\"bytes12\"},{\"name\":\"_weight\",\"type\":\"uint256\"}],\"name\":\"setWeight\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"candidates\",\"outputs\":[{\"name\":\"name\",\"type\":\"bytes12\"},{\"name\":\"addr\",\"type\":\"address\"},{\"name\":\"ioOperatorAddr\",\"type\":\"string\"},{\"name\":\"ioRewardAddr\",\"type\":\"string\"},{\"name\":\"weight\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unpause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"nameRegistrationFee\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_startIndex\",\"type\":\"uint256\"},{\"name\":\"_limit\",\"type\":\"uint256\"}],\"name\":\"getAllCandidates\",\"outputs\":[{\"name\":\"names\",\"type\":\"bytes12[]\"},{\"name\":\"addresses\",\"type\":\"address[]\"},{\"name\":\"ioOperatorAddr\",\"type\":\"bytes32[]\"},{\"name\":\"ioRewardAddr\",\"type\":\"bytes32[]\"},{\"name\":\"weights\",\"type\":\"uint256[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"addrToIdx\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_name\",\"type\":\"bytes12\"},{\"name\":\"_ioOperatorAddr\",\"type\":\"string\"},{\"name\":\"_ioRewardAddr\",\"type\":\"string\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"register\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_fee\",\"type\":\"uint256\"}],\"name\":\"setNameRegistrationFee\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"pause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"setFeeCollector\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"candidateCount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"feeCollector\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"bytes12\"}],\"name\":\"nameToIdx\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_name\",\"type\":\"bytes12\"},{\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"setNameAddress\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"token\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_tokenAddr\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"idx\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"name\",\"type\":\"bytes12\"},{\"indexed\":false,\"name\":\"addr\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"ioOperatorAddr\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"ioRewardAddr\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"Registered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Pause\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Unpause\",\"type\":\"event\"}]"

// Register is an auto generated Go binding around an Ethereum contract.
type Register struct {
	RegisterCaller     // Read-only binding to the contract
	RegisterTransactor // Write-only binding to the contract
	RegisterFilterer   // Log filterer for contract events
}

// RegisterCaller is an auto generated read-only Go binding around an Ethereum contract.
type RegisterCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RegisterTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RegisterTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RegisterFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RegisterFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RegisterSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RegisterSession struct {
	Contract     *Register         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RegisterCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RegisterCallerSession struct {
	Contract *RegisterCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// RegisterTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RegisterTransactorSession struct {
	Contract     *RegisterTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// RegisterRaw is an auto generated low-level Go binding around an Ethereum contract.
type RegisterRaw struct {
	Contract *Register // Generic contract binding to access the raw methods on
}

// RegisterCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RegisterCallerRaw struct {
	Contract *RegisterCaller // Generic read-only contract binding to access the raw methods on
}

// RegisterTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RegisterTransactorRaw struct {
	Contract *RegisterTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRegister creates a new instance of Register, bound to a specific deployed contract.
func NewRegister(address common.Address, backend bind.ContractBackend) (*Register, error) {
	contract, err := bindRegister(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Register{RegisterCaller: RegisterCaller{contract: contract}, RegisterTransactor: RegisterTransactor{contract: contract}, RegisterFilterer: RegisterFilterer{contract: contract}}, nil
}

// NewRegisterCaller creates a new read-only instance of Register, bound to a specific deployed contract.
func NewRegisterCaller(address common.Address, caller bind.ContractCaller) (*RegisterCaller, error) {
	contract, err := bindRegister(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RegisterCaller{contract: contract}, nil
}

// NewRegisterTransactor creates a new write-only instance of Register, bound to a specific deployed contract.
func NewRegisterTransactor(address common.Address, transactor bind.ContractTransactor) (*RegisterTransactor, error) {
	contract, err := bindRegister(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RegisterTransactor{contract: contract}, nil
}

// NewRegisterFilterer creates a new log filterer instance of Register, bound to a specific deployed contract.
func NewRegisterFilterer(address common.Address, filterer bind.ContractFilterer) (*RegisterFilterer, error) {
	contract, err := bindRegister(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RegisterFilterer{contract: contract}, nil
}

// bindRegister binds a generic wrapper to an already deployed contract.
func bindRegister(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(RegisterABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Register *RegisterRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Register.Contract.RegisterCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Register *RegisterRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Register.Contract.RegisterTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Register *RegisterRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Register.Contract.RegisterTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Register *RegisterCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Register.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Register *RegisterTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Register.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Register *RegisterTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Register.Contract.contract.Transact(opts, method, params...)
}

// AddrToIdx is a free data retrieval call binding the contract method 0x5f609695.
//
// Solidity: function addrToIdx(address ) constant returns(uint256)
func (_Register *RegisterCaller) AddrToIdx(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Register.contract.Call(opts, out, "addrToIdx", arg0)
	return *ret0, err
}

// AddrToIdx is a free data retrieval call binding the contract method 0x5f609695.
//
// Solidity: function addrToIdx(address ) constant returns(uint256)
func (_Register *RegisterSession) AddrToIdx(arg0 common.Address) (*big.Int, error) {
	return _Register.Contract.AddrToIdx(&_Register.CallOpts, arg0)
}

// AddrToIdx is a free data retrieval call binding the contract method 0x5f609695.
//
// Solidity: function addrToIdx(address ) constant returns(uint256)
func (_Register *RegisterCallerSession) AddrToIdx(arg0 common.Address) (*big.Int, error) {
	return _Register.Contract.AddrToIdx(&_Register.CallOpts, arg0)
}

// CandidateCount is a free data retrieval call binding the contract method 0xa9a981a3.
//
// Solidity: function candidateCount() constant returns(uint256)
func (_Register *RegisterCaller) CandidateCount(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Register.contract.Call(opts, out, "candidateCount")
	return *ret0, err
}

// CandidateCount is a free data retrieval call binding the contract method 0xa9a981a3.
//
// Solidity: function candidateCount() constant returns(uint256)
func (_Register *RegisterSession) CandidateCount() (*big.Int, error) {
	return _Register.Contract.CandidateCount(&_Register.CallOpts)
}

// CandidateCount is a free data retrieval call binding the contract method 0xa9a981a3.
//
// Solidity: function candidateCount() constant returns(uint256)
func (_Register *RegisterCallerSession) CandidateCount() (*big.Int, error) {
	return _Register.Contract.CandidateCount(&_Register.CallOpts)
}

// Candidates is a free data retrieval call binding the contract method 0x3477ee2e.
//
// Solidity: function candidates(uint256 ) constant returns(bytes12 name, address addr, string ioOperatorAddr, string ioRewardAddr, uint256 weight)
func (_Register *RegisterCaller) Candidates(opts *bind.CallOpts, arg0 *big.Int) (struct {
	Name           [12]byte
	Addr           common.Address
	IoOperatorAddr string
	IoRewardAddr   string
	Weight         *big.Int
}, error) {
	ret := new(struct {
		Name           [12]byte
		Addr           common.Address
		IoOperatorAddr string
		IoRewardAddr   string
		Weight         *big.Int
	})
	out := ret
	err := _Register.contract.Call(opts, out, "candidates", arg0)
	return *ret, err
}

// Candidates is a free data retrieval call binding the contract method 0x3477ee2e.
//
// Solidity: function candidates(uint256 ) constant returns(bytes12 name, address addr, string ioOperatorAddr, string ioRewardAddr, uint256 weight)
func (_Register *RegisterSession) Candidates(arg0 *big.Int) (struct {
	Name           [12]byte
	Addr           common.Address
	IoOperatorAddr string
	IoRewardAddr   string
	Weight         *big.Int
}, error) {
	return _Register.Contract.Candidates(&_Register.CallOpts, arg0)
}

// Candidates is a free data retrieval call binding the contract method 0x3477ee2e.
//
// Solidity: function candidates(uint256 ) constant returns(bytes12 name, address addr, string ioOperatorAddr, string ioRewardAddr, uint256 weight)
func (_Register *RegisterCallerSession) Candidates(arg0 *big.Int) (struct {
	Name           [12]byte
	Addr           common.Address
	IoOperatorAddr string
	IoRewardAddr   string
	Weight         *big.Int
}, error) {
	return _Register.Contract.Candidates(&_Register.CallOpts, arg0)
}

// FeeCollector is a free data retrieval call binding the contract method 0xc415b95c.
//
// Solidity: function feeCollector() constant returns(address)
func (_Register *RegisterCaller) FeeCollector(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Register.contract.Call(opts, out, "feeCollector")
	return *ret0, err
}

// FeeCollector is a free data retrieval call binding the contract method 0xc415b95c.
//
// Solidity: function feeCollector() constant returns(address)
func (_Register *RegisterSession) FeeCollector() (common.Address, error) {
	return _Register.Contract.FeeCollector(&_Register.CallOpts)
}

// FeeCollector is a free data retrieval call binding the contract method 0xc415b95c.
//
// Solidity: function feeCollector() constant returns(address)
func (_Register *RegisterCallerSession) FeeCollector() (common.Address, error) {
	return _Register.Contract.FeeCollector(&_Register.CallOpts)
}

// GetAllCandidates is a free data retrieval call binding the contract method 0x5e924246.
//
// Solidity: function getAllCandidates(uint256 _startIndex, uint256 _limit) constant returns(bytes12[] names, address[] addresses, bytes32[] ioOperatorAddr, bytes32[] ioRewardAddr, uint256[] weights)
func (_Register *RegisterCaller) GetAllCandidates(opts *bind.CallOpts, _startIndex *big.Int, _limit *big.Int) (struct {
	Names          [][12]byte
	Addresses      []common.Address
	IoOperatorAddr [][32]byte
	IoRewardAddr   [][32]byte
	Weights        []*big.Int
}, error) {
	ret := new(struct {
		Names          [][12]byte
		Addresses      []common.Address
		IoOperatorAddr [][32]byte
		IoRewardAddr   [][32]byte
		Weights        []*big.Int
	})
	out := ret
	err := _Register.contract.Call(opts, out, "getAllCandidates", _startIndex, _limit)
	return *ret, err
}

// GetAllCandidates is a free data retrieval call binding the contract method 0x5e924246.
//
// Solidity: function getAllCandidates(uint256 _startIndex, uint256 _limit) constant returns(bytes12[] names, address[] addresses, bytes32[] ioOperatorAddr, bytes32[] ioRewardAddr, uint256[] weights)
func (_Register *RegisterSession) GetAllCandidates(_startIndex *big.Int, _limit *big.Int) (struct {
	Names          [][12]byte
	Addresses      []common.Address
	IoOperatorAddr [][32]byte
	IoRewardAddr   [][32]byte
	Weights        []*big.Int
}, error) {
	return _Register.Contract.GetAllCandidates(&_Register.CallOpts, _startIndex, _limit)
}

// GetAllCandidates is a free data retrieval call binding the contract method 0x5e924246.
//
// Solidity: function getAllCandidates(uint256 _startIndex, uint256 _limit) constant returns(bytes12[] names, address[] addresses, bytes32[] ioOperatorAddr, bytes32[] ioRewardAddr, uint256[] weights)
func (_Register *RegisterCallerSession) GetAllCandidates(_startIndex *big.Int, _limit *big.Int) (struct {
	Names          [][12]byte
	Addresses      []common.Address
	IoOperatorAddr [][32]byte
	IoRewardAddr   [][32]byte
	Weights        []*big.Int
}, error) {
	return _Register.Contract.GetAllCandidates(&_Register.CallOpts, _startIndex, _limit)
}

// IoAddrToIdx is a free data retrieval call binding the contract method 0x12f2a2a9.
//
// Solidity: function ioAddrToIdx(bytes32 , bytes32 ) constant returns(uint256)
func (_Register *RegisterCaller) IoAddrToIdx(opts *bind.CallOpts, arg0 [32]byte, arg1 [32]byte) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Register.contract.Call(opts, out, "ioAddrToIdx", arg0, arg1)
	return *ret0, err
}

// IoAddrToIdx is a free data retrieval call binding the contract method 0x12f2a2a9.
//
// Solidity: function ioAddrToIdx(bytes32 , bytes32 ) constant returns(uint256)
func (_Register *RegisterSession) IoAddrToIdx(arg0 [32]byte, arg1 [32]byte) (*big.Int, error) {
	return _Register.Contract.IoAddrToIdx(&_Register.CallOpts, arg0, arg1)
}

// IoAddrToIdx is a free data retrieval call binding the contract method 0x12f2a2a9.
//
// Solidity: function ioAddrToIdx(bytes32 , bytes32 ) constant returns(uint256)
func (_Register *RegisterCallerSession) IoAddrToIdx(arg0 [32]byte, arg1 [32]byte) (*big.Int, error) {
	return _Register.Contract.IoAddrToIdx(&_Register.CallOpts, arg0, arg1)
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) constant returns(bool)
func (_Register *RegisterCaller) IsOwner(opts *bind.CallOpts, _address common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _Register.contract.Call(opts, out, "isOwner", _address)
	return *ret0, err
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) constant returns(bool)
func (_Register *RegisterSession) IsOwner(_address common.Address) (bool, error) {
	return _Register.Contract.IsOwner(&_Register.CallOpts, _address)
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) constant returns(bool)
func (_Register *RegisterCallerSession) IsOwner(_address common.Address) (bool, error) {
	return _Register.Contract.IsOwner(&_Register.CallOpts, _address)
}

// NameRegistrationFee is a free data retrieval call binding the contract method 0x543ca195.
//
// Solidity: function nameRegistrationFee() constant returns(uint256)
func (_Register *RegisterCaller) NameRegistrationFee(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Register.contract.Call(opts, out, "nameRegistrationFee")
	return *ret0, err
}

// NameRegistrationFee is a free data retrieval call binding the contract method 0x543ca195.
//
// Solidity: function nameRegistrationFee() constant returns(uint256)
func (_Register *RegisterSession) NameRegistrationFee() (*big.Int, error) {
	return _Register.Contract.NameRegistrationFee(&_Register.CallOpts)
}

// NameRegistrationFee is a free data retrieval call binding the contract method 0x543ca195.
//
// Solidity: function nameRegistrationFee() constant returns(uint256)
func (_Register *RegisterCallerSession) NameRegistrationFee() (*big.Int, error) {
	return _Register.Contract.NameRegistrationFee(&_Register.CallOpts)
}

// NameToIdx is a free data retrieval call binding the contract method 0xd64ac4a6.
//
// Solidity: function nameToIdx(bytes12 ) constant returns(uint256)
func (_Register *RegisterCaller) NameToIdx(opts *bind.CallOpts, arg0 [12]byte) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Register.contract.Call(opts, out, "nameToIdx", arg0)
	return *ret0, err
}

// NameToIdx is a free data retrieval call binding the contract method 0xd64ac4a6.
//
// Solidity: function nameToIdx(bytes12 ) constant returns(uint256)
func (_Register *RegisterSession) NameToIdx(arg0 [12]byte) (*big.Int, error) {
	return _Register.Contract.NameToIdx(&_Register.CallOpts, arg0)
}

// NameToIdx is a free data retrieval call binding the contract method 0xd64ac4a6.
//
// Solidity: function nameToIdx(bytes12 ) constant returns(uint256)
func (_Register *RegisterCallerSession) NameToIdx(arg0 [12]byte) (*big.Int, error) {
	return _Register.Contract.NameToIdx(&_Register.CallOpts, arg0)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Register *RegisterCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Register.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Register *RegisterSession) Owner() (common.Address, error) {
	return _Register.Contract.Owner(&_Register.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Register *RegisterCallerSession) Owner() (common.Address, error) {
	return _Register.Contract.Owner(&_Register.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_Register *RegisterCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _Register.contract.Call(opts, out, "paused")
	return *ret0, err
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_Register *RegisterSession) Paused() (bool, error) {
	return _Register.Contract.Paused(&_Register.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_Register *RegisterCallerSession) Paused() (bool, error) {
	return _Register.Contract.Paused(&_Register.CallOpts)
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_Register *RegisterCaller) Token(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Register.contract.Call(opts, out, "token")
	return *ret0, err
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_Register *RegisterSession) Token() (common.Address, error) {
	return _Register.Contract.Token(&_Register.CallOpts)
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_Register *RegisterCallerSession) Token() (common.Address, error) {
	return _Register.Contract.Token(&_Register.CallOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Register *RegisterTransactor) Pause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Register.contract.Transact(opts, "pause")
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Register *RegisterSession) Pause() (*types.Transaction, error) {
	return _Register.Contract.Pause(&_Register.TransactOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Register *RegisterTransactorSession) Pause() (*types.Transaction, error) {
	return _Register.Contract.Pause(&_Register.TransactOpts)
}

// Register is a paid mutator transaction binding the contract method 0x690c1f22.
//
// Solidity: function register(bytes12 _name, string _ioOperatorAddr, string _ioRewardAddr, bytes _data) returns()
func (_Register *RegisterTransactor) Register(opts *bind.TransactOpts, _name [12]byte, _ioOperatorAddr string, _ioRewardAddr string, _data []byte) (*types.Transaction, error) {
	return _Register.contract.Transact(opts, "register", _name, _ioOperatorAddr, _ioRewardAddr, _data)
}

// Register is a paid mutator transaction binding the contract method 0x690c1f22.
//
// Solidity: function register(bytes12 _name, string _ioOperatorAddr, string _ioRewardAddr, bytes _data) returns()
func (_Register *RegisterSession) Register(_name [12]byte, _ioOperatorAddr string, _ioRewardAddr string, _data []byte) (*types.Transaction, error) {
	return _Register.Contract.Register(&_Register.TransactOpts, _name, _ioOperatorAddr, _ioRewardAddr, _data)
}

// Register is a paid mutator transaction binding the contract method 0x690c1f22.
//
// Solidity: function register(bytes12 _name, string _ioOperatorAddr, string _ioRewardAddr, bytes _data) returns()
func (_Register *RegisterTransactorSession) Register(_name [12]byte, _ioOperatorAddr string, _ioRewardAddr string, _data []byte) (*types.Transaction, error) {
	return _Register.Contract.Register(&_Register.TransactOpts, _name, _ioOperatorAddr, _ioRewardAddr, _data)
}

// SetFeeCollector is a paid mutator transaction binding the contract method 0xa42dce80.
//
// Solidity: function setFeeCollector(address _addr) returns()
func (_Register *RegisterTransactor) SetFeeCollector(opts *bind.TransactOpts, _addr common.Address) (*types.Transaction, error) {
	return _Register.contract.Transact(opts, "setFeeCollector", _addr)
}

// SetFeeCollector is a paid mutator transaction binding the contract method 0xa42dce80.
//
// Solidity: function setFeeCollector(address _addr) returns()
func (_Register *RegisterSession) SetFeeCollector(_addr common.Address) (*types.Transaction, error) {
	return _Register.Contract.SetFeeCollector(&_Register.TransactOpts, _addr)
}

// SetFeeCollector is a paid mutator transaction binding the contract method 0xa42dce80.
//
// Solidity: function setFeeCollector(address _addr) returns()
func (_Register *RegisterTransactorSession) SetFeeCollector(_addr common.Address) (*types.Transaction, error) {
	return _Register.Contract.SetFeeCollector(&_Register.TransactOpts, _addr)
}

// SetNameAddress is a paid mutator transaction binding the contract method 0xebfd4148.
//
// Solidity: function setNameAddress(bytes12 _name, address _addr) returns()
func (_Register *RegisterTransactor) SetNameAddress(opts *bind.TransactOpts, _name [12]byte, _addr common.Address) (*types.Transaction, error) {
	return _Register.contract.Transact(opts, "setNameAddress", _name, _addr)
}

// SetNameAddress is a paid mutator transaction binding the contract method 0xebfd4148.
//
// Solidity: function setNameAddress(bytes12 _name, address _addr) returns()
func (_Register *RegisterSession) SetNameAddress(_name [12]byte, _addr common.Address) (*types.Transaction, error) {
	return _Register.Contract.SetNameAddress(&_Register.TransactOpts, _name, _addr)
}

// SetNameAddress is a paid mutator transaction binding the contract method 0xebfd4148.
//
// Solidity: function setNameAddress(bytes12 _name, address _addr) returns()
func (_Register *RegisterTransactorSession) SetNameAddress(_name [12]byte, _addr common.Address) (*types.Transaction, error) {
	return _Register.Contract.SetNameAddress(&_Register.TransactOpts, _name, _addr)
}

// SetNameRegistrationFee is a paid mutator transaction binding the contract method 0x6afee780.
//
// Solidity: function setNameRegistrationFee(uint256 _fee) returns()
func (_Register *RegisterTransactor) SetNameRegistrationFee(opts *bind.TransactOpts, _fee *big.Int) (*types.Transaction, error) {
	return _Register.contract.Transact(opts, "setNameRegistrationFee", _fee)
}

// SetNameRegistrationFee is a paid mutator transaction binding the contract method 0x6afee780.
//
// Solidity: function setNameRegistrationFee(uint256 _fee) returns()
func (_Register *RegisterSession) SetNameRegistrationFee(_fee *big.Int) (*types.Transaction, error) {
	return _Register.Contract.SetNameRegistrationFee(&_Register.TransactOpts, _fee)
}

// SetNameRegistrationFee is a paid mutator transaction binding the contract method 0x6afee780.
//
// Solidity: function setNameRegistrationFee(uint256 _fee) returns()
func (_Register *RegisterTransactorSession) SetNameRegistrationFee(_fee *big.Int) (*types.Transaction, error) {
	return _Register.Contract.SetNameRegistrationFee(&_Register.TransactOpts, _fee)
}

// SetWeight is a paid mutator transaction binding the contract method 0x3207bcf0.
//
// Solidity: function setWeight(bytes12 _name, uint256 _weight) returns()
func (_Register *RegisterTransactor) SetWeight(opts *bind.TransactOpts, _name [12]byte, _weight *big.Int) (*types.Transaction, error) {
	return _Register.contract.Transact(opts, "setWeight", _name, _weight)
}

// SetWeight is a paid mutator transaction binding the contract method 0x3207bcf0.
//
// Solidity: function setWeight(bytes12 _name, uint256 _weight) returns()
func (_Register *RegisterSession) SetWeight(_name [12]byte, _weight *big.Int) (*types.Transaction, error) {
	return _Register.Contract.SetWeight(&_Register.TransactOpts, _name, _weight)
}

// SetWeight is a paid mutator transaction binding the contract method 0x3207bcf0.
//
// Solidity: function setWeight(bytes12 _name, uint256 _weight) returns()
func (_Register *RegisterTransactorSession) SetWeight(_name [12]byte, _weight *big.Int) (*types.Transaction, error) {
	return _Register.Contract.SetWeight(&_Register.TransactOpts, _name, _weight)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_Register *RegisterTransactor) TransferOwnership(opts *bind.TransactOpts, _newOwner common.Address) (*types.Transaction, error) {
	return _Register.contract.Transact(opts, "transferOwnership", _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_Register *RegisterSession) TransferOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _Register.Contract.TransferOwnership(&_Register.TransactOpts, _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_Register *RegisterTransactorSession) TransferOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _Register.Contract.TransferOwnership(&_Register.TransactOpts, _newOwner)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Register *RegisterTransactor) Unpause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Register.contract.Transact(opts, "unpause")
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Register *RegisterSession) Unpause() (*types.Transaction, error) {
	return _Register.Contract.Unpause(&_Register.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Register *RegisterTransactorSession) Unpause() (*types.Transaction, error) {
	return _Register.Contract.Unpause(&_Register.TransactOpts)
}

// RegisterPauseIterator is returned from FilterPause and is used to iterate over the raw logs and unpacked data for Pause events raised by the Register contract.
type RegisterPauseIterator struct {
	Event *RegisterPause // Event containing the contract specifics and raw log

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
func (it *RegisterPauseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegisterPause)
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
		it.Event = new(RegisterPause)
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
func (it *RegisterPauseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegisterPauseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegisterPause represents a Pause event raised by the Register contract.
type RegisterPause struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterPause is a free log retrieval operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: event Pause()
func (_Register *RegisterFilterer) FilterPause(opts *bind.FilterOpts) (*RegisterPauseIterator, error) {

	logs, sub, err := _Register.contract.FilterLogs(opts, "Pause")
	if err != nil {
		return nil, err
	}
	return &RegisterPauseIterator{contract: _Register.contract, event: "Pause", logs: logs, sub: sub}, nil
}

// WatchPause is a free log subscription operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: event Pause()
func (_Register *RegisterFilterer) WatchPause(opts *bind.WatchOpts, sink chan<- *RegisterPause) (event.Subscription, error) {

	logs, sub, err := _Register.contract.WatchLogs(opts, "Pause")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegisterPause)
				if err := _Register.contract.UnpackLog(event, "Pause", log); err != nil {
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

// RegisterRegisteredIterator is returned from FilterRegistered and is used to iterate over the raw logs and unpacked data for Registered events raised by the Register contract.
type RegisterRegisteredIterator struct {
	Event *RegisterRegistered // Event containing the contract specifics and raw log

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
func (it *RegisterRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegisterRegistered)
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
		it.Event = new(RegisterRegistered)
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
func (it *RegisterRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegisterRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegisterRegistered represents a Registered event raised by the Register contract.
type RegisterRegistered struct {
	Idx            *big.Int
	Name           [12]byte
	Addr           common.Address
	IoOperatorAddr string
	IoRewardAddr   string
	Data           []byte
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterRegistered is a free log retrieval operation binding the contract event 0x3340640ab5b6f15b631e9400701305335845b74a8bd5054f7b268608b2b869d4.
//
// Solidity: event Registered(uint256 idx, bytes12 name, address addr, string ioOperatorAddr, string ioRewardAddr, bytes data)
func (_Register *RegisterFilterer) FilterRegistered(opts *bind.FilterOpts) (*RegisterRegisteredIterator, error) {

	logs, sub, err := _Register.contract.FilterLogs(opts, "Registered")
	if err != nil {
		return nil, err
	}
	return &RegisterRegisteredIterator{contract: _Register.contract, event: "Registered", logs: logs, sub: sub}, nil
}

// WatchRegistered is a free log subscription operation binding the contract event 0x3340640ab5b6f15b631e9400701305335845b74a8bd5054f7b268608b2b869d4.
//
// Solidity: event Registered(uint256 idx, bytes12 name, address addr, string ioOperatorAddr, string ioRewardAddr, bytes data)
func (_Register *RegisterFilterer) WatchRegistered(opts *bind.WatchOpts, sink chan<- *RegisterRegistered) (event.Subscription, error) {

	logs, sub, err := _Register.contract.WatchLogs(opts, "Registered")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegisterRegistered)
				if err := _Register.contract.UnpackLog(event, "Registered", log); err != nil {
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

// RegisterUnpauseIterator is returned from FilterUnpause and is used to iterate over the raw logs and unpacked data for Unpause events raised by the Register contract.
type RegisterUnpauseIterator struct {
	Event *RegisterUnpause // Event containing the contract specifics and raw log

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
func (it *RegisterUnpauseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegisterUnpause)
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
		it.Event = new(RegisterUnpause)
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
func (it *RegisterUnpauseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegisterUnpauseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegisterUnpause represents a Unpause event raised by the Register contract.
type RegisterUnpause struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterUnpause is a free log retrieval operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: event Unpause()
func (_Register *RegisterFilterer) FilterUnpause(opts *bind.FilterOpts) (*RegisterUnpauseIterator, error) {

	logs, sub, err := _Register.contract.FilterLogs(opts, "Unpause")
	if err != nil {
		return nil, err
	}
	return &RegisterUnpauseIterator{contract: _Register.contract, event: "Unpause", logs: logs, sub: sub}, nil
}

// WatchUnpause is a free log subscription operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: event Unpause()
func (_Register *RegisterFilterer) WatchUnpause(opts *bind.WatchOpts, sink chan<- *RegisterUnpause) (event.Subscription, error) {

	logs, sub, err := _Register.contract.WatchLogs(opts, "Unpause")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegisterUnpause)
				if err := _Register.contract.UnpackLog(event, "Unpause", log); err != nil {
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

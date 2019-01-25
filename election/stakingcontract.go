// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package election

import (
	"math/big"
	"strings"

	ethereum "github.com/iotexproject/go-ethereum"
	"github.com/iotexproject/go-ethereum/accounts/abi"
	"github.com/iotexproject/go-ethereum/accounts/abi/bind"
	"github.com/iotexproject/go-ethereum/common"
	"github.com/iotexproject/go-ethereum/core/types"
	"github.com/iotexproject/go-ethereum/event"
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

// StakingContractABI is the input ABI used to generate the binding from.
const StakingContractABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"addrs\",\"type\":\"address[]\"}],\"name\":\"addAddressesToWhitelist\",\"outputs\":[{\"name\":\"success\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"addAddressToWhitelist\",\"outputs\":[{\"name\":\"success\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_canPubKey\",\"type\":\"string\"},{\"name\":\"_amount\",\"type\":\"uint256\"},{\"name\":\"_stakeDuration\",\"type\":\"uint256\"},{\"name\":\"_nonDecay\",\"type\":\"bool\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"createBucket\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"pause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addrs\",\"type\":\"address[]\"}],\"name\":\"removeAddressesFromWhitelist\",\"outputs\":[{\"name\":\"success\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"removeAddressFromWhitelist\",\"outputs\":[{\"name\":\"success\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_bucketIndex\",\"type\":\"uint256\"},{\"name\":\"_stakeDuration\",\"type\":\"uint256\"},{\"name\":\"_nonDecay\",\"type\":\"bool\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"restake\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_bucketIndex\",\"type\":\"uint256\"},{\"name\":\"_canPubKey\",\"type\":\"string\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"revote\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_bucketIndex\",\"type\":\"uint256\"},{\"name\":\"_newOwner\",\"type\":\"address\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"setBucketOwner\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unpause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_bucketIndex\",\"type\":\"uint256\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"unstake\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_bucketIndex\",\"type\":\"uint256\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"withdraw\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_stakingTokenAddr\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"bucketIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"canPubKey\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"stakeDuration\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"nonDecay\",\"type\":\"bool\"},{\"indexed\":false,\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"BucketCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"bucketIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"canPubKey\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"stakeDuration\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"stakeStartTime\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"nonDecay\",\"type\":\"bool\"},{\"indexed\":false,\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"BucketUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"bucketIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"canPubKey\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"BucketUnstake\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"bucketIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"canPubKey\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"BucketWithdraw\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"WhitelistedAddressAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"WhitelistedAddressRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Pause\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Unpause\",\"type\":\"event\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"buckets\",\"outputs\":[{\"name\":\"canPubKey\",\"type\":\"string\"},{\"name\":\"stakedAmount\",\"type\":\"uint256\"},{\"name\":\"stakeDuration\",\"type\":\"uint256\"},{\"name\":\"stakeStartTime\",\"type\":\"uint256\"},{\"name\":\"nonDecay\",\"type\":\"bool\"},{\"name\":\"unstakeStartTime\",\"type\":\"uint256\"},{\"name\":\"bucketOwner\",\"type\":\"address\"},{\"name\":\"prev\",\"type\":\"uint256\"},{\"name\":\"next\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_prevIndex\",\"type\":\"uint256\"},{\"name\":\"_limit\",\"type\":\"uint256\"}],\"name\":\"getActiveBuckets\",\"outputs\":[{\"name\":\"count\",\"type\":\"uint256\"},{\"name\":\"indexes\",\"type\":\"uint256[]\"},{\"name\":\"stakeStartTime\",\"type\":\"uint256[]\"},{\"name\":\"stakeDuration\",\"type\":\"uint256[]\"},{\"name\":\"stakedAmount\",\"type\":\"uint256[]\"},{\"name\":\"stakedFor\",\"type\":\"string\"},{\"name\":\"owners\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"getBucketIndexesByAddress\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"isOwner\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"stakeholders\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"token\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"totalStaked\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"whitelist\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"}]"

// StakingContract is an auto generated Go binding around an Ethereum contract.
type StakingContract struct {
	StakingContractCaller     // Read-only binding to the contract
	StakingContractTransactor // Write-only binding to the contract
	StakingContractFilterer   // Log filterer for contract events
}

// StakingContractCaller is an auto generated read-only Go binding around an Ethereum contract.
type StakingContractCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakingContractTransactor is an auto generated write-only Go binding around an Ethereum contract.
type StakingContractTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakingContractFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type StakingContractFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakingContractSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type StakingContractSession struct {
	Contract     *StakingContract  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// StakingContractCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type StakingContractCallerSession struct {
	Contract *StakingContractCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// StakingContractTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type StakingContractTransactorSession struct {
	Contract     *StakingContractTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// StakingContractRaw is an auto generated low-level Go binding around an Ethereum contract.
type StakingContractRaw struct {
	Contract *StakingContract // Generic contract binding to access the raw methods on
}

// StakingContractCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type StakingContractCallerRaw struct {
	Contract *StakingContractCaller // Generic read-only contract binding to access the raw methods on
}

// StakingContractTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type StakingContractTransactorRaw struct {
	Contract *StakingContractTransactor // Generic write-only contract binding to access the raw methods on
}

// NewStakingContract creates a new instance of StakingContract, bound to a specific deployed contract.
func NewStakingContract(address common.Address, backend bind.ContractBackend) (*StakingContract, error) {
	contract, err := bindStakingContract(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &StakingContract{StakingContractCaller: StakingContractCaller{contract: contract}, StakingContractTransactor: StakingContractTransactor{contract: contract}, StakingContractFilterer: StakingContractFilterer{contract: contract}}, nil
}

// NewStakingContractCaller creates a new read-only instance of StakingContract, bound to a specific deployed contract.
func NewStakingContractCaller(address common.Address, caller bind.ContractCaller) (*StakingContractCaller, error) {
	contract, err := bindStakingContract(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &StakingContractCaller{contract: contract}, nil
}

// NewStakingContractTransactor creates a new write-only instance of StakingContract, bound to a specific deployed contract.
func NewStakingContractTransactor(address common.Address, transactor bind.ContractTransactor) (*StakingContractTransactor, error) {
	contract, err := bindStakingContract(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &StakingContractTransactor{contract: contract}, nil
}

// NewStakingContractFilterer creates a new log filterer instance of StakingContract, bound to a specific deployed contract.
func NewStakingContractFilterer(address common.Address, filterer bind.ContractFilterer) (*StakingContractFilterer, error) {
	contract, err := bindStakingContract(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &StakingContractFilterer{contract: contract}, nil
}

// bindStakingContract binds a generic wrapper to an already deployed contract.
func bindStakingContract(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(StakingContractABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_StakingContract *StakingContractRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _StakingContract.Contract.StakingContractCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_StakingContract *StakingContractRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _StakingContract.Contract.StakingContractTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_StakingContract *StakingContractRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _StakingContract.Contract.StakingContractTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_StakingContract *StakingContractCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _StakingContract.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_StakingContract *StakingContractTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _StakingContract.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_StakingContract *StakingContractTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _StakingContract.Contract.contract.Transact(opts, method, params...)
}

// Buckets is a free data retrieval call binding the contract method 0x9b51fb0d.
//
// Solidity: function buckets(uint256 ) constant returns(string canPubKey, uint256 stakedAmount, uint256 stakeDuration, uint256 stakeStartTime, bool nonDecay, uint256 unstakeStartTime, address bucketOwner, uint256 prev, uint256 next)
func (_StakingContract *StakingContractCaller) Buckets(opts *bind.CallOpts, arg0 *big.Int) (struct {
	CanPubKey        string
	StakedAmount     *big.Int
	StakeDuration    *big.Int
	StakeStartTime   *big.Int
	NonDecay         bool
	UnstakeStartTime *big.Int
	BucketOwner      common.Address
	Prev             *big.Int
	Next             *big.Int
}, error) {
	ret := new(struct {
		CanPubKey        string
		StakedAmount     *big.Int
		StakeDuration    *big.Int
		StakeStartTime   *big.Int
		NonDecay         bool
		UnstakeStartTime *big.Int
		BucketOwner      common.Address
		Prev             *big.Int
		Next             *big.Int
	})
	out := ret
	err := _StakingContract.contract.Call(opts, out, "buckets", arg0)
	return *ret, err
}

// Buckets is a free data retrieval call binding the contract method 0x9b51fb0d.
//
// Solidity: function buckets(uint256 ) constant returns(string canPubKey, uint256 stakedAmount, uint256 stakeDuration, uint256 stakeStartTime, bool nonDecay, uint256 unstakeStartTime, address bucketOwner, uint256 prev, uint256 next)
func (_StakingContract *StakingContractSession) Buckets(arg0 *big.Int) (struct {
	CanPubKey        string
	StakedAmount     *big.Int
	StakeDuration    *big.Int
	StakeStartTime   *big.Int
	NonDecay         bool
	UnstakeStartTime *big.Int
	BucketOwner      common.Address
	Prev             *big.Int
	Next             *big.Int
}, error) {
	return _StakingContract.Contract.Buckets(&_StakingContract.CallOpts, arg0)
}

// Buckets is a free data retrieval call binding the contract method 0x9b51fb0d.
//
// Solidity: function buckets(uint256 ) constant returns(string canPubKey, uint256 stakedAmount, uint256 stakeDuration, uint256 stakeStartTime, bool nonDecay, uint256 unstakeStartTime, address bucketOwner, uint256 prev, uint256 next)
func (_StakingContract *StakingContractCallerSession) Buckets(arg0 *big.Int) (struct {
	CanPubKey        string
	StakedAmount     *big.Int
	StakeDuration    *big.Int
	StakeStartTime   *big.Int
	NonDecay         bool
	UnstakeStartTime *big.Int
	BucketOwner      common.Address
	Prev             *big.Int
	Next             *big.Int
}, error) {
	return _StakingContract.Contract.Buckets(&_StakingContract.CallOpts, arg0)
}

// GetActiveBuckets is a free data retrieval call binding the contract method 0x042f95bd.
//
// Solidity: function getActiveBuckets(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes, uint256[] stakeStartTime, uint256[] stakeDuration, uint256[] stakedAmount, string stakedFor, address[] owners)
func (_StakingContract *StakingContractCaller) GetActiveBuckets(opts *bind.CallOpts, _prevIndex *big.Int, _limit *big.Int) (struct {
	Count          *big.Int
	Indexes        []*big.Int
	StakeStartTime []*big.Int
	StakeDuration  []*big.Int
	StakedAmount   []*big.Int
	StakedFor      string
	Owners         []common.Address
}, error) {
	ret := new(struct {
		Count          *big.Int
		Indexes        []*big.Int
		StakeStartTime []*big.Int
		StakeDuration  []*big.Int
		StakedAmount   []*big.Int
		StakedFor      string
		Owners         []common.Address
	})
	out := ret
	err := _StakingContract.contract.Call(opts, out, "getActiveBuckets", _prevIndex, _limit)
	return *ret, err
}

// GetActiveBuckets is a free data retrieval call binding the contract method 0x042f95bd.
//
// Solidity: function getActiveBuckets(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes, uint256[] stakeStartTime, uint256[] stakeDuration, uint256[] stakedAmount, string stakedFor, address[] owners)
func (_StakingContract *StakingContractSession) GetActiveBuckets(_prevIndex *big.Int, _limit *big.Int) (struct {
	Count          *big.Int
	Indexes        []*big.Int
	StakeStartTime []*big.Int
	StakeDuration  []*big.Int
	StakedAmount   []*big.Int
	StakedFor      string
	Owners         []common.Address
}, error) {
	return _StakingContract.Contract.GetActiveBuckets(&_StakingContract.CallOpts, _prevIndex, _limit)
}

// GetActiveBuckets is a free data retrieval call binding the contract method 0x042f95bd.
//
// Solidity: function getActiveBuckets(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes, uint256[] stakeStartTime, uint256[] stakeDuration, uint256[] stakedAmount, string stakedFor, address[] owners)
func (_StakingContract *StakingContractCallerSession) GetActiveBuckets(_prevIndex *big.Int, _limit *big.Int) (struct {
	Count          *big.Int
	Indexes        []*big.Int
	StakeStartTime []*big.Int
	StakeDuration  []*big.Int
	StakedAmount   []*big.Int
	StakedFor      string
	Owners         []common.Address
}, error) {
	return _StakingContract.Contract.GetActiveBuckets(&_StakingContract.CallOpts, _prevIndex, _limit)
}

// GetBucketIndexesByAddress is a free data retrieval call binding the contract method 0x7d0de831.
//
// Solidity: function getBucketIndexesByAddress(address owner) constant returns(uint256[])
func (_StakingContract *StakingContractCaller) GetBucketIndexesByAddress(opts *bind.CallOpts, owner common.Address) ([]*big.Int, error) {
	var (
		ret0 = new([]*big.Int)
	)
	out := ret0
	err := _StakingContract.contract.Call(opts, out, "getBucketIndexesByAddress", owner)
	return *ret0, err
}

// GetBucketIndexesByAddress is a free data retrieval call binding the contract method 0x7d0de831.
//
// Solidity: function getBucketIndexesByAddress(address owner) constant returns(uint256[])
func (_StakingContract *StakingContractSession) GetBucketIndexesByAddress(owner common.Address) ([]*big.Int, error) {
	return _StakingContract.Contract.GetBucketIndexesByAddress(&_StakingContract.CallOpts, owner)
}

// GetBucketIndexesByAddress is a free data retrieval call binding the contract method 0x7d0de831.
//
// Solidity: function getBucketIndexesByAddress(address owner) constant returns(uint256[])
func (_StakingContract *StakingContractCallerSession) GetBucketIndexesByAddress(owner common.Address) ([]*big.Int, error) {
	return _StakingContract.Contract.GetBucketIndexesByAddress(&_StakingContract.CallOpts, owner)
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) constant returns(bool)
func (_StakingContract *StakingContractCaller) IsOwner(opts *bind.CallOpts, _address common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _StakingContract.contract.Call(opts, out, "isOwner", _address)
	return *ret0, err
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) constant returns(bool)
func (_StakingContract *StakingContractSession) IsOwner(_address common.Address) (bool, error) {
	return _StakingContract.Contract.IsOwner(&_StakingContract.CallOpts, _address)
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) constant returns(bool)
func (_StakingContract *StakingContractCallerSession) IsOwner(_address common.Address) (bool, error) {
	return _StakingContract.Contract.IsOwner(&_StakingContract.CallOpts, _address)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_StakingContract *StakingContractCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _StakingContract.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_StakingContract *StakingContractSession) Owner() (common.Address, error) {
	return _StakingContract.Contract.Owner(&_StakingContract.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_StakingContract *StakingContractCallerSession) Owner() (common.Address, error) {
	return _StakingContract.Contract.Owner(&_StakingContract.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_StakingContract *StakingContractCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _StakingContract.contract.Call(opts, out, "paused")
	return *ret0, err
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_StakingContract *StakingContractSession) Paused() (bool, error) {
	return _StakingContract.Contract.Paused(&_StakingContract.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_StakingContract *StakingContractCallerSession) Paused() (bool, error) {
	return _StakingContract.Contract.Paused(&_StakingContract.CallOpts)
}

// Stakeholders is a free data retrieval call binding the contract method 0x423ce1ae.
//
// Solidity: function stakeholders(address , uint256 ) constant returns(uint256)
func (_StakingContract *StakingContractCaller) Stakeholders(opts *bind.CallOpts, arg0 common.Address, arg1 *big.Int) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _StakingContract.contract.Call(opts, out, "stakeholders", arg0, arg1)
	return *ret0, err
}

// Stakeholders is a free data retrieval call binding the contract method 0x423ce1ae.
//
// Solidity: function stakeholders(address , uint256 ) constant returns(uint256)
func (_StakingContract *StakingContractSession) Stakeholders(arg0 common.Address, arg1 *big.Int) (*big.Int, error) {
	return _StakingContract.Contract.Stakeholders(&_StakingContract.CallOpts, arg0, arg1)
}

// Stakeholders is a free data retrieval call binding the contract method 0x423ce1ae.
//
// Solidity: function stakeholders(address , uint256 ) constant returns(uint256)
func (_StakingContract *StakingContractCallerSession) Stakeholders(arg0 common.Address, arg1 *big.Int) (*big.Int, error) {
	return _StakingContract.Contract.Stakeholders(&_StakingContract.CallOpts, arg0, arg1)
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_StakingContract *StakingContractCaller) Token(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _StakingContract.contract.Call(opts, out, "token")
	return *ret0, err
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_StakingContract *StakingContractSession) Token() (common.Address, error) {
	return _StakingContract.Contract.Token(&_StakingContract.CallOpts)
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_StakingContract *StakingContractCallerSession) Token() (common.Address, error) {
	return _StakingContract.Contract.Token(&_StakingContract.CallOpts)
}

// TotalStaked is a free data retrieval call binding the contract method 0x817b1cd2.
//
// Solidity: function totalStaked() constant returns(uint256)
func (_StakingContract *StakingContractCaller) TotalStaked(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _StakingContract.contract.Call(opts, out, "totalStaked")
	return *ret0, err
}

// TotalStaked is a free data retrieval call binding the contract method 0x817b1cd2.
//
// Solidity: function totalStaked() constant returns(uint256)
func (_StakingContract *StakingContractSession) TotalStaked() (*big.Int, error) {
	return _StakingContract.Contract.TotalStaked(&_StakingContract.CallOpts)
}

// TotalStaked is a free data retrieval call binding the contract method 0x817b1cd2.
//
// Solidity: function totalStaked() constant returns(uint256)
func (_StakingContract *StakingContractCallerSession) TotalStaked() (*big.Int, error) {
	return _StakingContract.Contract.TotalStaked(&_StakingContract.CallOpts)
}

// Whitelist is a free data retrieval call binding the contract method 0x9b19251a.
//
// Solidity: function whitelist(address ) constant returns(bool)
func (_StakingContract *StakingContractCaller) Whitelist(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _StakingContract.contract.Call(opts, out, "whitelist", arg0)
	return *ret0, err
}

// Whitelist is a free data retrieval call binding the contract method 0x9b19251a.
//
// Solidity: function whitelist(address ) constant returns(bool)
func (_StakingContract *StakingContractSession) Whitelist(arg0 common.Address) (bool, error) {
	return _StakingContract.Contract.Whitelist(&_StakingContract.CallOpts, arg0)
}

// Whitelist is a free data retrieval call binding the contract method 0x9b19251a.
//
// Solidity: function whitelist(address ) constant returns(bool)
func (_StakingContract *StakingContractCallerSession) Whitelist(arg0 common.Address) (bool, error) {
	return _StakingContract.Contract.Whitelist(&_StakingContract.CallOpts, arg0)
}

// AddAddressToWhitelist is a paid mutator transaction binding the contract method 0x7b9417c8.
//
// Solidity: function addAddressToWhitelist(address addr) returns(bool success)
func (_StakingContract *StakingContractTransactor) AddAddressToWhitelist(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "addAddressToWhitelist", addr)
}

// AddAddressToWhitelist is a paid mutator transaction binding the contract method 0x7b9417c8.
//
// Solidity: function addAddressToWhitelist(address addr) returns(bool success)
func (_StakingContract *StakingContractSession) AddAddressToWhitelist(addr common.Address) (*types.Transaction, error) {
	return _StakingContract.Contract.AddAddressToWhitelist(&_StakingContract.TransactOpts, addr)
}

// AddAddressToWhitelist is a paid mutator transaction binding the contract method 0x7b9417c8.
//
// Solidity: function addAddressToWhitelist(address addr) returns(bool success)
func (_StakingContract *StakingContractTransactorSession) AddAddressToWhitelist(addr common.Address) (*types.Transaction, error) {
	return _StakingContract.Contract.AddAddressToWhitelist(&_StakingContract.TransactOpts, addr)
}

// AddAddressesToWhitelist is a paid mutator transaction binding the contract method 0xe2ec6ec3.
//
// Solidity: function addAddressesToWhitelist(address[] addrs) returns(bool success)
func (_StakingContract *StakingContractTransactor) AddAddressesToWhitelist(opts *bind.TransactOpts, addrs []common.Address) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "addAddressesToWhitelist", addrs)
}

// AddAddressesToWhitelist is a paid mutator transaction binding the contract method 0xe2ec6ec3.
//
// Solidity: function addAddressesToWhitelist(address[] addrs) returns(bool success)
func (_StakingContract *StakingContractSession) AddAddressesToWhitelist(addrs []common.Address) (*types.Transaction, error) {
	return _StakingContract.Contract.AddAddressesToWhitelist(&_StakingContract.TransactOpts, addrs)
}

// AddAddressesToWhitelist is a paid mutator transaction binding the contract method 0xe2ec6ec3.
//
// Solidity: function addAddressesToWhitelist(address[] addrs) returns(bool success)
func (_StakingContract *StakingContractTransactorSession) AddAddressesToWhitelist(addrs []common.Address) (*types.Transaction, error) {
	return _StakingContract.Contract.AddAddressesToWhitelist(&_StakingContract.TransactOpts, addrs)
}

// CreateBucket is a paid mutator transaction binding the contract method 0x38de283c.
//
// Solidity: function createBucket(string _canPubKey, uint256 _amount, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns(uint256)
func (_StakingContract *StakingContractTransactor) CreateBucket(opts *bind.TransactOpts, _canPubKey string, _amount *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "createBucket", _canPubKey, _amount, _stakeDuration, _nonDecay, _data)
}

// CreateBucket is a paid mutator transaction binding the contract method 0x38de283c.
//
// Solidity: function createBucket(string _canPubKey, uint256 _amount, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns(uint256)
func (_StakingContract *StakingContractSession) CreateBucket(_canPubKey string, _amount *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.CreateBucket(&_StakingContract.TransactOpts, _canPubKey, _amount, _stakeDuration, _nonDecay, _data)
}

// CreateBucket is a paid mutator transaction binding the contract method 0x38de283c.
//
// Solidity: function createBucket(string _canPubKey, uint256 _amount, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns(uint256)
func (_StakingContract *StakingContractTransactorSession) CreateBucket(_canPubKey string, _amount *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.CreateBucket(&_StakingContract.TransactOpts, _canPubKey, _amount, _stakeDuration, _nonDecay, _data)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_StakingContract *StakingContractTransactor) Pause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "pause")
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_StakingContract *StakingContractSession) Pause() (*types.Transaction, error) {
	return _StakingContract.Contract.Pause(&_StakingContract.TransactOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_StakingContract *StakingContractTransactorSession) Pause() (*types.Transaction, error) {
	return _StakingContract.Contract.Pause(&_StakingContract.TransactOpts)
}

// RemoveAddressFromWhitelist is a paid mutator transaction binding the contract method 0x286dd3f5.
//
// Solidity: function removeAddressFromWhitelist(address addr) returns(bool success)
func (_StakingContract *StakingContractTransactor) RemoveAddressFromWhitelist(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "removeAddressFromWhitelist", addr)
}

// RemoveAddressFromWhitelist is a paid mutator transaction binding the contract method 0x286dd3f5.
//
// Solidity: function removeAddressFromWhitelist(address addr) returns(bool success)
func (_StakingContract *StakingContractSession) RemoveAddressFromWhitelist(addr common.Address) (*types.Transaction, error) {
	return _StakingContract.Contract.RemoveAddressFromWhitelist(&_StakingContract.TransactOpts, addr)
}

// RemoveAddressFromWhitelist is a paid mutator transaction binding the contract method 0x286dd3f5.
//
// Solidity: function removeAddressFromWhitelist(address addr) returns(bool success)
func (_StakingContract *StakingContractTransactorSession) RemoveAddressFromWhitelist(addr common.Address) (*types.Transaction, error) {
	return _StakingContract.Contract.RemoveAddressFromWhitelist(&_StakingContract.TransactOpts, addr)
}

// RemoveAddressesFromWhitelist is a paid mutator transaction binding the contract method 0x24953eaa.
//
// Solidity: function removeAddressesFromWhitelist(address[] addrs) returns(bool success)
func (_StakingContract *StakingContractTransactor) RemoveAddressesFromWhitelist(opts *bind.TransactOpts, addrs []common.Address) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "removeAddressesFromWhitelist", addrs)
}

// RemoveAddressesFromWhitelist is a paid mutator transaction binding the contract method 0x24953eaa.
//
// Solidity: function removeAddressesFromWhitelist(address[] addrs) returns(bool success)
func (_StakingContract *StakingContractSession) RemoveAddressesFromWhitelist(addrs []common.Address) (*types.Transaction, error) {
	return _StakingContract.Contract.RemoveAddressesFromWhitelist(&_StakingContract.TransactOpts, addrs)
}

// RemoveAddressesFromWhitelist is a paid mutator transaction binding the contract method 0x24953eaa.
//
// Solidity: function removeAddressesFromWhitelist(address[] addrs) returns(bool success)
func (_StakingContract *StakingContractTransactorSession) RemoveAddressesFromWhitelist(addrs []common.Address) (*types.Transaction, error) {
	return _StakingContract.Contract.RemoveAddressesFromWhitelist(&_StakingContract.TransactOpts, addrs)
}

// Restake is a paid mutator transaction binding the contract method 0x7b24a5fd.
//
// Solidity: function restake(uint256 _bucketIndex, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns()
func (_StakingContract *StakingContractTransactor) Restake(opts *bind.TransactOpts, _bucketIndex *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "restake", _bucketIndex, _stakeDuration, _nonDecay, _data)
}

// Restake is a paid mutator transaction binding the contract method 0x7b24a5fd.
//
// Solidity: function restake(uint256 _bucketIndex, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns()
func (_StakingContract *StakingContractSession) Restake(_bucketIndex *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.Restake(&_StakingContract.TransactOpts, _bucketIndex, _stakeDuration, _nonDecay, _data)
}

// Restake is a paid mutator transaction binding the contract method 0x7b24a5fd.
//
// Solidity: function restake(uint256 _bucketIndex, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns()
func (_StakingContract *StakingContractTransactorSession) Restake(_bucketIndex *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.Restake(&_StakingContract.TransactOpts, _bucketIndex, _stakeDuration, _nonDecay, _data)
}

// Revote is a paid mutator transaction binding the contract method 0x44aee33f.
//
// Solidity: function revote(uint256 _bucketIndex, string _canPubKey, bytes _data) returns()
func (_StakingContract *StakingContractTransactor) Revote(opts *bind.TransactOpts, _bucketIndex *big.Int, _canPubKey string, _data []byte) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "revote", _bucketIndex, _canPubKey, _data)
}

// Revote is a paid mutator transaction binding the contract method 0x44aee33f.
//
// Solidity: function revote(uint256 _bucketIndex, string _canPubKey, bytes _data) returns()
func (_StakingContract *StakingContractSession) Revote(_bucketIndex *big.Int, _canPubKey string, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.Revote(&_StakingContract.TransactOpts, _bucketIndex, _canPubKey, _data)
}

// Revote is a paid mutator transaction binding the contract method 0x44aee33f.
//
// Solidity: function revote(uint256 _bucketIndex, string _canPubKey, bytes _data) returns()
func (_StakingContract *StakingContractTransactorSession) Revote(_bucketIndex *big.Int, _canPubKey string, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.Revote(&_StakingContract.TransactOpts, _bucketIndex, _canPubKey, _data)
}

// SetBucketOwner is a paid mutator transaction binding the contract method 0x9cfe3461.
//
// Solidity: function setBucketOwner(uint256 _bucketIndex, address _newOwner, bytes _data) returns()
func (_StakingContract *StakingContractTransactor) SetBucketOwner(opts *bind.TransactOpts, _bucketIndex *big.Int, _newOwner common.Address, _data []byte) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "setBucketOwner", _bucketIndex, _newOwner, _data)
}

// SetBucketOwner is a paid mutator transaction binding the contract method 0x9cfe3461.
//
// Solidity: function setBucketOwner(uint256 _bucketIndex, address _newOwner, bytes _data) returns()
func (_StakingContract *StakingContractSession) SetBucketOwner(_bucketIndex *big.Int, _newOwner common.Address, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.SetBucketOwner(&_StakingContract.TransactOpts, _bucketIndex, _newOwner, _data)
}

// SetBucketOwner is a paid mutator transaction binding the contract method 0x9cfe3461.
//
// Solidity: function setBucketOwner(uint256 _bucketIndex, address _newOwner, bytes _data) returns()
func (_StakingContract *StakingContractTransactorSession) SetBucketOwner(_bucketIndex *big.Int, _newOwner common.Address, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.SetBucketOwner(&_StakingContract.TransactOpts, _bucketIndex, _newOwner, _data)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_StakingContract *StakingContractTransactor) TransferOwnership(opts *bind.TransactOpts, _newOwner common.Address) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "transferOwnership", _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_StakingContract *StakingContractSession) TransferOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _StakingContract.Contract.TransferOwnership(&_StakingContract.TransactOpts, _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_StakingContract *StakingContractTransactorSession) TransferOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _StakingContract.Contract.TransferOwnership(&_StakingContract.TransactOpts, _newOwner)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_StakingContract *StakingContractTransactor) Unpause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "unpause")
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_StakingContract *StakingContractSession) Unpause() (*types.Transaction, error) {
	return _StakingContract.Contract.Unpause(&_StakingContract.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_StakingContract *StakingContractTransactorSession) Unpause() (*types.Transaction, error) {
	return _StakingContract.Contract.Unpause(&_StakingContract.TransactOpts)
}

// Unstake is a paid mutator transaction binding the contract method 0xc8fd6ed0.
//
// Solidity: function unstake(uint256 _bucketIndex, bytes _data) returns()
func (_StakingContract *StakingContractTransactor) Unstake(opts *bind.TransactOpts, _bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "unstake", _bucketIndex, _data)
}

// Unstake is a paid mutator transaction binding the contract method 0xc8fd6ed0.
//
// Solidity: function unstake(uint256 _bucketIndex, bytes _data) returns()
func (_StakingContract *StakingContractSession) Unstake(_bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.Unstake(&_StakingContract.TransactOpts, _bucketIndex, _data)
}

// Unstake is a paid mutator transaction binding the contract method 0xc8fd6ed0.
//
// Solidity: function unstake(uint256 _bucketIndex, bytes _data) returns()
func (_StakingContract *StakingContractTransactorSession) Unstake(_bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.Unstake(&_StakingContract.TransactOpts, _bucketIndex, _data)
}

// Withdraw is a paid mutator transaction binding the contract method 0x030ba25d.
//
// Solidity: function withdraw(uint256 _bucketIndex, bytes _data) returns()
func (_StakingContract *StakingContractTransactor) Withdraw(opts *bind.TransactOpts, _bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _StakingContract.contract.Transact(opts, "withdraw", _bucketIndex, _data)
}

// Withdraw is a paid mutator transaction binding the contract method 0x030ba25d.
//
// Solidity: function withdraw(uint256 _bucketIndex, bytes _data) returns()
func (_StakingContract *StakingContractSession) Withdraw(_bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.Withdraw(&_StakingContract.TransactOpts, _bucketIndex, _data)
}

// Withdraw is a paid mutator transaction binding the contract method 0x030ba25d.
//
// Solidity: function withdraw(uint256 _bucketIndex, bytes _data) returns()
func (_StakingContract *StakingContractTransactorSession) Withdraw(_bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _StakingContract.Contract.Withdraw(&_StakingContract.TransactOpts, _bucketIndex, _data)
}

// StakingContractBucketCreatedIterator is returned from FilterBucketCreated and is used to iterate over the raw logs and unpacked data for BucketCreated events raised by the StakingContract contract.
type StakingContractBucketCreatedIterator struct {
	Event *StakingContractBucketCreated // Event containing the contract specifics and raw log

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
func (it *StakingContractBucketCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingContractBucketCreated)
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
		it.Event = new(StakingContractBucketCreated)
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
func (it *StakingContractBucketCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingContractBucketCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingContractBucketCreated represents a BucketCreated event raised by the StakingContract contract.
type StakingContractBucketCreated struct {
	BucketIndex   *big.Int
	CanPubKey     string
	Amount        *big.Int
	StakeDuration *big.Int
	NonDecay      bool
	Data          []byte
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterBucketCreated is a free log retrieval operation binding the contract event 0x5016942191b1bd2625772749820851828450fa5c40684618b791a01d770574bf.
//
// Solidity: event BucketCreated(uint256 bucketIndex, string canPubKey, uint256 amount, uint256 stakeDuration, bool nonDecay, bytes data)
func (_StakingContract *StakingContractFilterer) FilterBucketCreated(opts *bind.FilterOpts) (*StakingContractBucketCreatedIterator, error) {

	logs, sub, err := _StakingContract.contract.FilterLogs(opts, "BucketCreated")
	if err != nil {
		return nil, err
	}
	return &StakingContractBucketCreatedIterator{contract: _StakingContract.contract, event: "BucketCreated", logs: logs, sub: sub}, nil
}

// WatchBucketCreated is a free log subscription operation binding the contract event 0x5016942191b1bd2625772749820851828450fa5c40684618b791a01d770574bf.
//
// Solidity: event BucketCreated(uint256 bucketIndex, string canPubKey, uint256 amount, uint256 stakeDuration, bool nonDecay, bytes data)
func (_StakingContract *StakingContractFilterer) WatchBucketCreated(opts *bind.WatchOpts, sink chan<- *StakingContractBucketCreated) (event.Subscription, error) {

	logs, sub, err := _StakingContract.contract.WatchLogs(opts, "BucketCreated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingContractBucketCreated)
				if err := _StakingContract.contract.UnpackLog(event, "BucketCreated", log); err != nil {
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

// StakingContractBucketUnstakeIterator is returned from FilterBucketUnstake and is used to iterate over the raw logs and unpacked data for BucketUnstake events raised by the StakingContract contract.
type StakingContractBucketUnstakeIterator struct {
	Event *StakingContractBucketUnstake // Event containing the contract specifics and raw log

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
func (it *StakingContractBucketUnstakeIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingContractBucketUnstake)
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
		it.Event = new(StakingContractBucketUnstake)
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
func (it *StakingContractBucketUnstakeIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingContractBucketUnstakeIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingContractBucketUnstake represents a BucketUnstake event raised by the StakingContract contract.
type StakingContractBucketUnstake struct {
	BucketIndex *big.Int
	CanPubKey   string
	Amount      *big.Int
	Data        []byte
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterBucketUnstake is a free log retrieval operation binding the contract event 0x660182b43f216b0f1a4817b8559ad3757ad47e089d25635aedc331272f30ae4d.
//
// Solidity: event BucketUnstake(uint256 bucketIndex, string canPubKey, uint256 amount, bytes data)
func (_StakingContract *StakingContractFilterer) FilterBucketUnstake(opts *bind.FilterOpts) (*StakingContractBucketUnstakeIterator, error) {

	logs, sub, err := _StakingContract.contract.FilterLogs(opts, "BucketUnstake")
	if err != nil {
		return nil, err
	}
	return &StakingContractBucketUnstakeIterator{contract: _StakingContract.contract, event: "BucketUnstake", logs: logs, sub: sub}, nil
}

// WatchBucketUnstake is a free log subscription operation binding the contract event 0x660182b43f216b0f1a4817b8559ad3757ad47e089d25635aedc331272f30ae4d.
//
// Solidity: event BucketUnstake(uint256 bucketIndex, string canPubKey, uint256 amount, bytes data)
func (_StakingContract *StakingContractFilterer) WatchBucketUnstake(opts *bind.WatchOpts, sink chan<- *StakingContractBucketUnstake) (event.Subscription, error) {

	logs, sub, err := _StakingContract.contract.WatchLogs(opts, "BucketUnstake")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingContractBucketUnstake)
				if err := _StakingContract.contract.UnpackLog(event, "BucketUnstake", log); err != nil {
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

// StakingContractBucketUpdatedIterator is returned from FilterBucketUpdated and is used to iterate over the raw logs and unpacked data for BucketUpdated events raised by the StakingContract contract.
type StakingContractBucketUpdatedIterator struct {
	Event *StakingContractBucketUpdated // Event containing the contract specifics and raw log

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
func (it *StakingContractBucketUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingContractBucketUpdated)
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
		it.Event = new(StakingContractBucketUpdated)
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
func (it *StakingContractBucketUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingContractBucketUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingContractBucketUpdated represents a BucketUpdated event raised by the StakingContract contract.
type StakingContractBucketUpdated struct {
	BucketIndex    *big.Int
	CanPubKey      string
	StakeDuration  *big.Int
	StakeStartTime *big.Int
	NonDecay       bool
	Data           []byte
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterBucketUpdated is a free log retrieval operation binding the contract event 0xb88913b9b8221fabe8346e4467cc5b6ba308192ba32fe916f9b862c3e8570bd0.
//
// Solidity: event BucketUpdated(uint256 bucketIndex, string canPubKey, uint256 stakeDuration, uint256 stakeStartTime, bool nonDecay, bytes data)
func (_StakingContract *StakingContractFilterer) FilterBucketUpdated(opts *bind.FilterOpts) (*StakingContractBucketUpdatedIterator, error) {

	logs, sub, err := _StakingContract.contract.FilterLogs(opts, "BucketUpdated")
	if err != nil {
		return nil, err
	}
	return &StakingContractBucketUpdatedIterator{contract: _StakingContract.contract, event: "BucketUpdated", logs: logs, sub: sub}, nil
}

// WatchBucketUpdated is a free log subscription operation binding the contract event 0xb88913b9b8221fabe8346e4467cc5b6ba308192ba32fe916f9b862c3e8570bd0.
//
// Solidity: event BucketUpdated(uint256 bucketIndex, string canPubKey, uint256 stakeDuration, uint256 stakeStartTime, bool nonDecay, bytes data)
func (_StakingContract *StakingContractFilterer) WatchBucketUpdated(opts *bind.WatchOpts, sink chan<- *StakingContractBucketUpdated) (event.Subscription, error) {

	logs, sub, err := _StakingContract.contract.WatchLogs(opts, "BucketUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingContractBucketUpdated)
				if err := _StakingContract.contract.UnpackLog(event, "BucketUpdated", log); err != nil {
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

// StakingContractBucketWithdrawIterator is returned from FilterBucketWithdraw and is used to iterate over the raw logs and unpacked data for BucketWithdraw events raised by the StakingContract contract.
type StakingContractBucketWithdrawIterator struct {
	Event *StakingContractBucketWithdraw // Event containing the contract specifics and raw log

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
func (it *StakingContractBucketWithdrawIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingContractBucketWithdraw)
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
		it.Event = new(StakingContractBucketWithdraw)
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
func (it *StakingContractBucketWithdrawIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingContractBucketWithdrawIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingContractBucketWithdraw represents a BucketWithdraw event raised by the StakingContract contract.
type StakingContractBucketWithdraw struct {
	BucketIndex *big.Int
	CanPubKey   string
	Amount      *big.Int
	Data        []byte
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterBucketWithdraw is a free log retrieval operation binding the contract event 0x65b4a222929b4322f508031f65d29afe6f6e295c4995b84fb234277dc6889c83.
//
// Solidity: event BucketWithdraw(uint256 bucketIndex, string canPubKey, uint256 amount, bytes data)
func (_StakingContract *StakingContractFilterer) FilterBucketWithdraw(opts *bind.FilterOpts) (*StakingContractBucketWithdrawIterator, error) {

	logs, sub, err := _StakingContract.contract.FilterLogs(opts, "BucketWithdraw")
	if err != nil {
		return nil, err
	}
	return &StakingContractBucketWithdrawIterator{contract: _StakingContract.contract, event: "BucketWithdraw", logs: logs, sub: sub}, nil
}

// WatchBucketWithdraw is a free log subscription operation binding the contract event 0x65b4a222929b4322f508031f65d29afe6f6e295c4995b84fb234277dc6889c83.
//
// Solidity: event BucketWithdraw(uint256 bucketIndex, string canPubKey, uint256 amount, bytes data)
func (_StakingContract *StakingContractFilterer) WatchBucketWithdraw(opts *bind.WatchOpts, sink chan<- *StakingContractBucketWithdraw) (event.Subscription, error) {

	logs, sub, err := _StakingContract.contract.WatchLogs(opts, "BucketWithdraw")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingContractBucketWithdraw)
				if err := _StakingContract.contract.UnpackLog(event, "BucketWithdraw", log); err != nil {
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

// StakingContractPauseIterator is returned from FilterPause and is used to iterate over the raw logs and unpacked data for Pause events raised by the StakingContract contract.
type StakingContractPauseIterator struct {
	Event *StakingContractPause // Event containing the contract specifics and raw log

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
func (it *StakingContractPauseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingContractPause)
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
		it.Event = new(StakingContractPause)
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
func (it *StakingContractPauseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingContractPauseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingContractPause represents a Pause event raised by the StakingContract contract.
type StakingContractPause struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterPause is a free log retrieval operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: event Pause()
func (_StakingContract *StakingContractFilterer) FilterPause(opts *bind.FilterOpts) (*StakingContractPauseIterator, error) {

	logs, sub, err := _StakingContract.contract.FilterLogs(opts, "Pause")
	if err != nil {
		return nil, err
	}
	return &StakingContractPauseIterator{contract: _StakingContract.contract, event: "Pause", logs: logs, sub: sub}, nil
}

// WatchPause is a free log subscription operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: event Pause()
func (_StakingContract *StakingContractFilterer) WatchPause(opts *bind.WatchOpts, sink chan<- *StakingContractPause) (event.Subscription, error) {

	logs, sub, err := _StakingContract.contract.WatchLogs(opts, "Pause")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingContractPause)
				if err := _StakingContract.contract.UnpackLog(event, "Pause", log); err != nil {
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

// StakingContractUnpauseIterator is returned from FilterUnpause and is used to iterate over the raw logs and unpacked data for Unpause events raised by the StakingContract contract.
type StakingContractUnpauseIterator struct {
	Event *StakingContractUnpause // Event containing the contract specifics and raw log

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
func (it *StakingContractUnpauseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingContractUnpause)
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
		it.Event = new(StakingContractUnpause)
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
func (it *StakingContractUnpauseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingContractUnpauseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingContractUnpause represents a Unpause event raised by the StakingContract contract.
type StakingContractUnpause struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterUnpause is a free log retrieval operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: event Unpause()
func (_StakingContract *StakingContractFilterer) FilterUnpause(opts *bind.FilterOpts) (*StakingContractUnpauseIterator, error) {

	logs, sub, err := _StakingContract.contract.FilterLogs(opts, "Unpause")
	if err != nil {
		return nil, err
	}
	return &StakingContractUnpauseIterator{contract: _StakingContract.contract, event: "Unpause", logs: logs, sub: sub}, nil
}

// WatchUnpause is a free log subscription operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: event Unpause()
func (_StakingContract *StakingContractFilterer) WatchUnpause(opts *bind.WatchOpts, sink chan<- *StakingContractUnpause) (event.Subscription, error) {

	logs, sub, err := _StakingContract.contract.WatchLogs(opts, "Unpause")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingContractUnpause)
				if err := _StakingContract.contract.UnpackLog(event, "Unpause", log); err != nil {
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

// StakingContractWhitelistedAddressAddedIterator is returned from FilterWhitelistedAddressAdded and is used to iterate over the raw logs and unpacked data for WhitelistedAddressAdded events raised by the StakingContract contract.
type StakingContractWhitelistedAddressAddedIterator struct {
	Event *StakingContractWhitelistedAddressAdded // Event containing the contract specifics and raw log

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
func (it *StakingContractWhitelistedAddressAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingContractWhitelistedAddressAdded)
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
		it.Event = new(StakingContractWhitelistedAddressAdded)
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
func (it *StakingContractWhitelistedAddressAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingContractWhitelistedAddressAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingContractWhitelistedAddressAdded represents a WhitelistedAddressAdded event raised by the StakingContract contract.
type StakingContractWhitelistedAddressAdded struct {
	Addr common.Address
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterWhitelistedAddressAdded is a free log retrieval operation binding the contract event 0xd1bba68c128cc3f427e5831b3c6f99f480b6efa6b9e80c757768f6124158cc3f.
//
// Solidity: event WhitelistedAddressAdded(address addr)
func (_StakingContract *StakingContractFilterer) FilterWhitelistedAddressAdded(opts *bind.FilterOpts) (*StakingContractWhitelistedAddressAddedIterator, error) {

	logs, sub, err := _StakingContract.contract.FilterLogs(opts, "WhitelistedAddressAdded")
	if err != nil {
		return nil, err
	}
	return &StakingContractWhitelistedAddressAddedIterator{contract: _StakingContract.contract, event: "WhitelistedAddressAdded", logs: logs, sub: sub}, nil
}

// WatchWhitelistedAddressAdded is a free log subscription operation binding the contract event 0xd1bba68c128cc3f427e5831b3c6f99f480b6efa6b9e80c757768f6124158cc3f.
//
// Solidity: event WhitelistedAddressAdded(address addr)
func (_StakingContract *StakingContractFilterer) WatchWhitelistedAddressAdded(opts *bind.WatchOpts, sink chan<- *StakingContractWhitelistedAddressAdded) (event.Subscription, error) {

	logs, sub, err := _StakingContract.contract.WatchLogs(opts, "WhitelistedAddressAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingContractWhitelistedAddressAdded)
				if err := _StakingContract.contract.UnpackLog(event, "WhitelistedAddressAdded", log); err != nil {
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

// StakingContractWhitelistedAddressRemovedIterator is returned from FilterWhitelistedAddressRemoved and is used to iterate over the raw logs and unpacked data for WhitelistedAddressRemoved events raised by the StakingContract contract.
type StakingContractWhitelistedAddressRemovedIterator struct {
	Event *StakingContractWhitelistedAddressRemoved // Event containing the contract specifics and raw log

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
func (it *StakingContractWhitelistedAddressRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingContractWhitelistedAddressRemoved)
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
		it.Event = new(StakingContractWhitelistedAddressRemoved)
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
func (it *StakingContractWhitelistedAddressRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingContractWhitelistedAddressRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingContractWhitelistedAddressRemoved represents a WhitelistedAddressRemoved event raised by the StakingContract contract.
type StakingContractWhitelistedAddressRemoved struct {
	Addr common.Address
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterWhitelistedAddressRemoved is a free log retrieval operation binding the contract event 0xf1abf01a1043b7c244d128e8595cf0c1d10743b022b03a02dffd8ca3bf729f5a.
//
// Solidity: event WhitelistedAddressRemoved(address addr)
func (_StakingContract *StakingContractFilterer) FilterWhitelistedAddressRemoved(opts *bind.FilterOpts) (*StakingContractWhitelistedAddressRemovedIterator, error) {

	logs, sub, err := _StakingContract.contract.FilterLogs(opts, "WhitelistedAddressRemoved")
	if err != nil {
		return nil, err
	}
	return &StakingContractWhitelistedAddressRemovedIterator{contract: _StakingContract.contract, event: "WhitelistedAddressRemoved", logs: logs, sub: sub}, nil
}

// WatchWhitelistedAddressRemoved is a free log subscription operation binding the contract event 0xf1abf01a1043b7c244d128e8595cf0c1d10743b022b03a02dffd8ca3bf729f5a.
//
// Solidity: event WhitelistedAddressRemoved(address addr)
func (_StakingContract *StakingContractFilterer) WatchWhitelistedAddressRemoved(opts *bind.WatchOpts, sink chan<- *StakingContractWhitelistedAddressRemoved) (event.Subscription, error) {

	logs, sub, err := _StakingContract.contract.WatchLogs(opts, "WhitelistedAddressRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingContractWhitelistedAddressRemoved)
				if err := _StakingContract.contract.UnpackLog(event, "WhitelistedAddressRemoved", log); err != nil {
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

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

// StakingABI is the input ABI used to generate the binding from.
const StakingABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"addrs\",\"type\":\"address[]\"}],\"name\":\"addAddressesToWhitelist\",\"outputs\":[{\"name\":\"success\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"addAddressToWhitelist\",\"outputs\":[{\"name\":\"success\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_canName\",\"type\":\"bytes12\"},{\"name\":\"_amount\",\"type\":\"uint256\"},{\"name\":\"_stakeDuration\",\"type\":\"uint256\"},{\"name\":\"_nonDecay\",\"type\":\"bool\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"createBucket\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"pause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addrs\",\"type\":\"address[]\"}],\"name\":\"removeAddressesFromWhitelist\",\"outputs\":[{\"name\":\"success\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"removeAddressFromWhitelist\",\"outputs\":[{\"name\":\"success\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_bucketIndex\",\"type\":\"uint256\"},{\"name\":\"_stakeDuration\",\"type\":\"uint256\"},{\"name\":\"_nonDecay\",\"type\":\"bool\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"restake\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_bucketIndex\",\"type\":\"uint256\"},{\"name\":\"_canName\",\"type\":\"bytes12\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"revote\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_bucketIndex\",\"type\":\"uint256\"},{\"name\":\"_newOwner\",\"type\":\"address\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"setBucketOwner\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unpause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_bucketIndex\",\"type\":\"uint256\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"unstake\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_bucketIndex\",\"type\":\"uint256\"},{\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"withdraw\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_stakingTokenAddr\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"bucketIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"canName\",\"type\":\"bytes12\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"stakeDuration\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"nonDecay\",\"type\":\"bool\"},{\"indexed\":false,\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"BucketCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"bucketIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"canName\",\"type\":\"bytes12\"},{\"indexed\":false,\"name\":\"stakeDuration\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"stakeStartTime\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"nonDecay\",\"type\":\"bool\"},{\"indexed\":false,\"name\":\"bucketOwner\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"BucketUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"bucketIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"canName\",\"type\":\"bytes12\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"BucketUnstake\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"bucketIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"canName\",\"type\":\"bytes12\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"BucketWithdraw\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"WhitelistedAddressAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"WhitelistedAddressRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Pause\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Unpause\",\"type\":\"event\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"buckets\",\"outputs\":[{\"name\":\"canName\",\"type\":\"bytes12\"},{\"name\":\"stakedAmount\",\"type\":\"uint256\"},{\"name\":\"stakeDuration\",\"type\":\"uint256\"},{\"name\":\"stakeStartTime\",\"type\":\"uint256\"},{\"name\":\"nonDecay\",\"type\":\"bool\"},{\"name\":\"unstakeStartTime\",\"type\":\"uint256\"},{\"name\":\"bucketOwner\",\"type\":\"address\"},{\"name\":\"createTime\",\"type\":\"uint256\"},{\"name\":\"prev\",\"type\":\"uint256\"},{\"name\":\"next\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_prevIndex\",\"type\":\"uint256\"},{\"name\":\"_limit\",\"type\":\"uint256\"}],\"name\":\"getActiveBucketCreateTimes\",\"outputs\":[{\"name\":\"count\",\"type\":\"uint256\"},{\"name\":\"indexes\",\"type\":\"uint256[]\"},{\"name\":\"createTimes\",\"type\":\"uint256[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_prevIndex\",\"type\":\"uint256\"},{\"name\":\"_limit\",\"type\":\"uint256\"}],\"name\":\"getActiveBucketIdx\",\"outputs\":[{\"name\":\"count\",\"type\":\"uint256\"},{\"name\":\"indexes\",\"type\":\"uint256[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_prevIndex\",\"type\":\"uint256\"},{\"name\":\"_limit\",\"type\":\"uint256\"}],\"name\":\"getActiveBuckets\",\"outputs\":[{\"name\":\"count\",\"type\":\"uint256\"},{\"name\":\"indexes\",\"type\":\"uint256[]\"},{\"name\":\"stakeStartTimes\",\"type\":\"uint256[]\"},{\"name\":\"stakeDurations\",\"type\":\"uint256[]\"},{\"name\":\"decays\",\"type\":\"bool[]\"},{\"name\":\"stakedAmounts\",\"type\":\"uint256[]\"},{\"name\":\"canNames\",\"type\":\"bytes12[]\"},{\"name\":\"owners\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_owner\",\"type\":\"address\"}],\"name\":\"getBucketIndexesByAddress\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"isOwner\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"maxBucketsPerAddr\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"maxStakeDuration\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"minStakeAmount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"minStakeDuration\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"secondsPerEpoch\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"stakeholders\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"token\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"totalStaked\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"unStakeDuration\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"whitelist\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"}]"

// Staking is an auto generated Go binding around an Ethereum contract.
type Staking struct {
	StakingCaller     // Read-only binding to the contract
	StakingTransactor // Write-only binding to the contract
	StakingFilterer   // Log filterer for contract events
}

// StakingCaller is an auto generated read-only Go binding around an Ethereum contract.
type StakingCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakingTransactor is an auto generated write-only Go binding around an Ethereum contract.
type StakingTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakingFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type StakingFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakingSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type StakingSession struct {
	Contract     *Staking          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// StakingCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type StakingCallerSession struct {
	Contract *StakingCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// StakingTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type StakingTransactorSession struct {
	Contract     *StakingTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// StakingRaw is an auto generated low-level Go binding around an Ethereum contract.
type StakingRaw struct {
	Contract *Staking // Generic contract binding to access the raw methods on
}

// StakingCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type StakingCallerRaw struct {
	Contract *StakingCaller // Generic read-only contract binding to access the raw methods on
}

// StakingTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type StakingTransactorRaw struct {
	Contract *StakingTransactor // Generic write-only contract binding to access the raw methods on
}

// NewStaking creates a new instance of Staking, bound to a specific deployed contract.
func NewStaking(address common.Address, backend bind.ContractBackend) (*Staking, error) {
	contract, err := bindStaking(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Staking{StakingCaller: StakingCaller{contract: contract}, StakingTransactor: StakingTransactor{contract: contract}, StakingFilterer: StakingFilterer{contract: contract}}, nil
}

// NewStakingCaller creates a new read-only instance of Staking, bound to a specific deployed contract.
func NewStakingCaller(address common.Address, caller bind.ContractCaller) (*StakingCaller, error) {
	contract, err := bindStaking(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &StakingCaller{contract: contract}, nil
}

// NewStakingTransactor creates a new write-only instance of Staking, bound to a specific deployed contract.
func NewStakingTransactor(address common.Address, transactor bind.ContractTransactor) (*StakingTransactor, error) {
	contract, err := bindStaking(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &StakingTransactor{contract: contract}, nil
}

// NewStakingFilterer creates a new log filterer instance of Staking, bound to a specific deployed contract.
func NewStakingFilterer(address common.Address, filterer bind.ContractFilterer) (*StakingFilterer, error) {
	contract, err := bindStaking(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &StakingFilterer{contract: contract}, nil
}

// bindStaking binds a generic wrapper to an already deployed contract.
func bindStaking(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(StakingABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Staking *StakingRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Staking.Contract.StakingCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Staking *StakingRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.Contract.StakingTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Staking *StakingRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Staking.Contract.StakingTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Staking *StakingCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Staking.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Staking *StakingTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Staking *StakingTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Staking.Contract.contract.Transact(opts, method, params...)
}

// Buckets is a free data retrieval call binding the contract method 0x9b51fb0d.
//
// Solidity: function buckets(uint256 ) constant returns(bytes12 canName, uint256 stakedAmount, uint256 stakeDuration, uint256 stakeStartTime, bool nonDecay, uint256 unstakeStartTime, address bucketOwner, uint256 createTime, uint256 prev, uint256 next)
func (_Staking *StakingCaller) Buckets(opts *bind.CallOpts, arg0 *big.Int) (struct {
	CanName          [12]byte
	StakedAmount     *big.Int
	StakeDuration    *big.Int
	StakeStartTime   *big.Int
	NonDecay         bool
	UnstakeStartTime *big.Int
	BucketOwner      common.Address
	CreateTime       *big.Int
	Prev             *big.Int
	Next             *big.Int
}, error) {
	ret := new(struct {
		CanName          [12]byte
		StakedAmount     *big.Int
		StakeDuration    *big.Int
		StakeStartTime   *big.Int
		NonDecay         bool
		UnstakeStartTime *big.Int
		BucketOwner      common.Address
		CreateTime       *big.Int
		Prev             *big.Int
		Next             *big.Int
	})
	out := ret
	err := _Staking.contract.Call(opts, out, "buckets", arg0)
	return *ret, err
}

// Buckets is a free data retrieval call binding the contract method 0x9b51fb0d.
//
// Solidity: function buckets(uint256 ) constant returns(bytes12 canName, uint256 stakedAmount, uint256 stakeDuration, uint256 stakeStartTime, bool nonDecay, uint256 unstakeStartTime, address bucketOwner, uint256 createTime, uint256 prev, uint256 next)
func (_Staking *StakingSession) Buckets(arg0 *big.Int) (struct {
	CanName          [12]byte
	StakedAmount     *big.Int
	StakeDuration    *big.Int
	StakeStartTime   *big.Int
	NonDecay         bool
	UnstakeStartTime *big.Int
	BucketOwner      common.Address
	CreateTime       *big.Int
	Prev             *big.Int
	Next             *big.Int
}, error) {
	return _Staking.Contract.Buckets(&_Staking.CallOpts, arg0)
}

// Buckets is a free data retrieval call binding the contract method 0x9b51fb0d.
//
// Solidity: function buckets(uint256 ) constant returns(bytes12 canName, uint256 stakedAmount, uint256 stakeDuration, uint256 stakeStartTime, bool nonDecay, uint256 unstakeStartTime, address bucketOwner, uint256 createTime, uint256 prev, uint256 next)
func (_Staking *StakingCallerSession) Buckets(arg0 *big.Int) (struct {
	CanName          [12]byte
	StakedAmount     *big.Int
	StakeDuration    *big.Int
	StakeStartTime   *big.Int
	NonDecay         bool
	UnstakeStartTime *big.Int
	BucketOwner      common.Address
	CreateTime       *big.Int
	Prev             *big.Int
	Next             *big.Int
}, error) {
	return _Staking.Contract.Buckets(&_Staking.CallOpts, arg0)
}

// GetActiveBucketCreateTimes is a free data retrieval call binding the contract method 0x37130b93.
//
// Solidity: function getActiveBucketCreateTimes(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes, uint256[] createTimes)
func (_Staking *StakingCaller) GetActiveBucketCreateTimes(opts *bind.CallOpts, _prevIndex *big.Int, _limit *big.Int) (struct {
	Count       *big.Int
	Indexes     []*big.Int
	CreateTimes []*big.Int
}, error) {
	ret := new(struct {
		Count       *big.Int
		Indexes     []*big.Int
		CreateTimes []*big.Int
	})
	out := ret
	err := _Staking.contract.Call(opts, out, "getActiveBucketCreateTimes", _prevIndex, _limit)
	return *ret, err
}

// GetActiveBucketCreateTimes is a free data retrieval call binding the contract method 0x37130b93.
//
// Solidity: function getActiveBucketCreateTimes(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes, uint256[] createTimes)
func (_Staking *StakingSession) GetActiveBucketCreateTimes(_prevIndex *big.Int, _limit *big.Int) (struct {
	Count       *big.Int
	Indexes     []*big.Int
	CreateTimes []*big.Int
}, error) {
	return _Staking.Contract.GetActiveBucketCreateTimes(&_Staking.CallOpts, _prevIndex, _limit)
}

// GetActiveBucketCreateTimes is a free data retrieval call binding the contract method 0x37130b93.
//
// Solidity: function getActiveBucketCreateTimes(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes, uint256[] createTimes)
func (_Staking *StakingCallerSession) GetActiveBucketCreateTimes(_prevIndex *big.Int, _limit *big.Int) (struct {
	Count       *big.Int
	Indexes     []*big.Int
	CreateTimes []*big.Int
}, error) {
	return _Staking.Contract.GetActiveBucketCreateTimes(&_Staking.CallOpts, _prevIndex, _limit)
}

// GetActiveBucketIdx is a free data retrieval call binding the contract method 0x8b59c5e0.
//
// Solidity: function getActiveBucketIdx(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes)
func (_Staking *StakingCaller) GetActiveBucketIdx(opts *bind.CallOpts, _prevIndex *big.Int, _limit *big.Int) (struct {
	Count   *big.Int
	Indexes []*big.Int
}, error) {
	ret := new(struct {
		Count   *big.Int
		Indexes []*big.Int
	})
	out := ret
	err := _Staking.contract.Call(opts, out, "getActiveBucketIdx", _prevIndex, _limit)
	return *ret, err
}

// GetActiveBucketIdx is a free data retrieval call binding the contract method 0x8b59c5e0.
//
// Solidity: function getActiveBucketIdx(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes)
func (_Staking *StakingSession) GetActiveBucketIdx(_prevIndex *big.Int, _limit *big.Int) (struct {
	Count   *big.Int
	Indexes []*big.Int
}, error) {
	return _Staking.Contract.GetActiveBucketIdx(&_Staking.CallOpts, _prevIndex, _limit)
}

// GetActiveBucketIdx is a free data retrieval call binding the contract method 0x8b59c5e0.
//
// Solidity: function getActiveBucketIdx(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes)
func (_Staking *StakingCallerSession) GetActiveBucketIdx(_prevIndex *big.Int, _limit *big.Int) (struct {
	Count   *big.Int
	Indexes []*big.Int
}, error) {
	return _Staking.Contract.GetActiveBucketIdx(&_Staking.CallOpts, _prevIndex, _limit)
}

// GetActiveBuckets is a free data retrieval call binding the contract method 0x042f95bd.
//
// Solidity: function getActiveBuckets(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes, uint256[] stakeStartTimes, uint256[] stakeDurations, bool[] decays, uint256[] stakedAmounts, bytes12[] canNames, address[] owners)
func (_Staking *StakingCaller) GetActiveBuckets(opts *bind.CallOpts, _prevIndex *big.Int, _limit *big.Int) (struct {
	Count           *big.Int
	Indexes         []*big.Int
	StakeStartTimes []*big.Int
	StakeDurations  []*big.Int
	Decays          []bool
	StakedAmounts   []*big.Int
	CanNames        [][12]byte
	Owners          []common.Address
}, error) {
	ret := new(struct {
		Count           *big.Int
		Indexes         []*big.Int
		StakeStartTimes []*big.Int
		StakeDurations  []*big.Int
		Decays          []bool
		StakedAmounts   []*big.Int
		CanNames        [][12]byte
		Owners          []common.Address
	})
	out := ret
	err := _Staking.contract.Call(opts, out, "getActiveBuckets", _prevIndex, _limit)
	return *ret, err
}

// GetActiveBuckets is a free data retrieval call binding the contract method 0x042f95bd.
//
// Solidity: function getActiveBuckets(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes, uint256[] stakeStartTimes, uint256[] stakeDurations, bool[] decays, uint256[] stakedAmounts, bytes12[] canNames, address[] owners)
func (_Staking *StakingSession) GetActiveBuckets(_prevIndex *big.Int, _limit *big.Int) (struct {
	Count           *big.Int
	Indexes         []*big.Int
	StakeStartTimes []*big.Int
	StakeDurations  []*big.Int
	Decays          []bool
	StakedAmounts   []*big.Int
	CanNames        [][12]byte
	Owners          []common.Address
}, error) {
	return _Staking.Contract.GetActiveBuckets(&_Staking.CallOpts, _prevIndex, _limit)
}

// GetActiveBuckets is a free data retrieval call binding the contract method 0x042f95bd.
//
// Solidity: function getActiveBuckets(uint256 _prevIndex, uint256 _limit) constant returns(uint256 count, uint256[] indexes, uint256[] stakeStartTimes, uint256[] stakeDurations, bool[] decays, uint256[] stakedAmounts, bytes12[] canNames, address[] owners)
func (_Staking *StakingCallerSession) GetActiveBuckets(_prevIndex *big.Int, _limit *big.Int) (struct {
	Count           *big.Int
	Indexes         []*big.Int
	StakeStartTimes []*big.Int
	StakeDurations  []*big.Int
	Decays          []bool
	StakedAmounts   []*big.Int
	CanNames        [][12]byte
	Owners          []common.Address
}, error) {
	return _Staking.Contract.GetActiveBuckets(&_Staking.CallOpts, _prevIndex, _limit)
}

// GetBucketIndexesByAddress is a free data retrieval call binding the contract method 0x7d0de831.
//
// Solidity: function getBucketIndexesByAddress(address _owner) constant returns(uint256[])
func (_Staking *StakingCaller) GetBucketIndexesByAddress(opts *bind.CallOpts, _owner common.Address) ([]*big.Int, error) {
	var (
		ret0 = new([]*big.Int)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "getBucketIndexesByAddress", _owner)
	return *ret0, err
}

// GetBucketIndexesByAddress is a free data retrieval call binding the contract method 0x7d0de831.
//
// Solidity: function getBucketIndexesByAddress(address _owner) constant returns(uint256[])
func (_Staking *StakingSession) GetBucketIndexesByAddress(_owner common.Address) ([]*big.Int, error) {
	return _Staking.Contract.GetBucketIndexesByAddress(&_Staking.CallOpts, _owner)
}

// GetBucketIndexesByAddress is a free data retrieval call binding the contract method 0x7d0de831.
//
// Solidity: function getBucketIndexesByAddress(address _owner) constant returns(uint256[])
func (_Staking *StakingCallerSession) GetBucketIndexesByAddress(_owner common.Address) ([]*big.Int, error) {
	return _Staking.Contract.GetBucketIndexesByAddress(&_Staking.CallOpts, _owner)
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) constant returns(bool)
func (_Staking *StakingCaller) IsOwner(opts *bind.CallOpts, _address common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "isOwner", _address)
	return *ret0, err
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) constant returns(bool)
func (_Staking *StakingSession) IsOwner(_address common.Address) (bool, error) {
	return _Staking.Contract.IsOwner(&_Staking.CallOpts, _address)
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) constant returns(bool)
func (_Staking *StakingCallerSession) IsOwner(_address common.Address) (bool, error) {
	return _Staking.Contract.IsOwner(&_Staking.CallOpts, _address)
}

// MaxBucketsPerAddr is a free data retrieval call binding the contract method 0xfe8a8b4c.
//
// Solidity: function maxBucketsPerAddr() constant returns(uint256)
func (_Staking *StakingCaller) MaxBucketsPerAddr(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "maxBucketsPerAddr")
	return *ret0, err
}

// MaxBucketsPerAddr is a free data retrieval call binding the contract method 0xfe8a8b4c.
//
// Solidity: function maxBucketsPerAddr() constant returns(uint256)
func (_Staking *StakingSession) MaxBucketsPerAddr() (*big.Int, error) {
	return _Staking.Contract.MaxBucketsPerAddr(&_Staking.CallOpts)
}

// MaxBucketsPerAddr is a free data retrieval call binding the contract method 0xfe8a8b4c.
//
// Solidity: function maxBucketsPerAddr() constant returns(uint256)
func (_Staking *StakingCallerSession) MaxBucketsPerAddr() (*big.Int, error) {
	return _Staking.Contract.MaxBucketsPerAddr(&_Staking.CallOpts)
}

// MaxStakeDuration is a free data retrieval call binding the contract method 0x76f70003.
//
// Solidity: function maxStakeDuration() constant returns(uint256)
func (_Staking *StakingCaller) MaxStakeDuration(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "maxStakeDuration")
	return *ret0, err
}

// MaxStakeDuration is a free data retrieval call binding the contract method 0x76f70003.
//
// Solidity: function maxStakeDuration() constant returns(uint256)
func (_Staking *StakingSession) MaxStakeDuration() (*big.Int, error) {
	return _Staking.Contract.MaxStakeDuration(&_Staking.CallOpts)
}

// MaxStakeDuration is a free data retrieval call binding the contract method 0x76f70003.
//
// Solidity: function maxStakeDuration() constant returns(uint256)
func (_Staking *StakingCallerSession) MaxStakeDuration() (*big.Int, error) {
	return _Staking.Contract.MaxStakeDuration(&_Staking.CallOpts)
}

// MinStakeAmount is a free data retrieval call binding the contract method 0xf1887684.
//
// Solidity: function minStakeAmount() constant returns(uint256)
func (_Staking *StakingCaller) MinStakeAmount(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "minStakeAmount")
	return *ret0, err
}

// MinStakeAmount is a free data retrieval call binding the contract method 0xf1887684.
//
// Solidity: function minStakeAmount() constant returns(uint256)
func (_Staking *StakingSession) MinStakeAmount() (*big.Int, error) {
	return _Staking.Contract.MinStakeAmount(&_Staking.CallOpts)
}

// MinStakeAmount is a free data retrieval call binding the contract method 0xf1887684.
//
// Solidity: function minStakeAmount() constant returns(uint256)
func (_Staking *StakingCallerSession) MinStakeAmount() (*big.Int, error) {
	return _Staking.Contract.MinStakeAmount(&_Staking.CallOpts)
}

// MinStakeDuration is a free data retrieval call binding the contract method 0x5fec5c64.
//
// Solidity: function minStakeDuration() constant returns(uint256)
func (_Staking *StakingCaller) MinStakeDuration(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "minStakeDuration")
	return *ret0, err
}

// MinStakeDuration is a free data retrieval call binding the contract method 0x5fec5c64.
//
// Solidity: function minStakeDuration() constant returns(uint256)
func (_Staking *StakingSession) MinStakeDuration() (*big.Int, error) {
	return _Staking.Contract.MinStakeDuration(&_Staking.CallOpts)
}

// MinStakeDuration is a free data retrieval call binding the contract method 0x5fec5c64.
//
// Solidity: function minStakeDuration() constant returns(uint256)
func (_Staking *StakingCallerSession) MinStakeDuration() (*big.Int, error) {
	return _Staking.Contract.MinStakeDuration(&_Staking.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Staking *StakingCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Staking *StakingSession) Owner() (common.Address, error) {
	return _Staking.Contract.Owner(&_Staking.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Staking *StakingCallerSession) Owner() (common.Address, error) {
	return _Staking.Contract.Owner(&_Staking.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_Staking *StakingCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "paused")
	return *ret0, err
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_Staking *StakingSession) Paused() (bool, error) {
	return _Staking.Contract.Paused(&_Staking.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_Staking *StakingCallerSession) Paused() (bool, error) {
	return _Staking.Contract.Paused(&_Staking.CallOpts)
}

// SecondsPerEpoch is a free data retrieval call binding the contract method 0x580c8f3d.
//
// Solidity: function secondsPerEpoch() constant returns(uint256)
func (_Staking *StakingCaller) SecondsPerEpoch(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "secondsPerEpoch")
	return *ret0, err
}

// SecondsPerEpoch is a free data retrieval call binding the contract method 0x580c8f3d.
//
// Solidity: function secondsPerEpoch() constant returns(uint256)
func (_Staking *StakingSession) SecondsPerEpoch() (*big.Int, error) {
	return _Staking.Contract.SecondsPerEpoch(&_Staking.CallOpts)
}

// SecondsPerEpoch is a free data retrieval call binding the contract method 0x580c8f3d.
//
// Solidity: function secondsPerEpoch() constant returns(uint256)
func (_Staking *StakingCallerSession) SecondsPerEpoch() (*big.Int, error) {
	return _Staking.Contract.SecondsPerEpoch(&_Staking.CallOpts)
}

// Stakeholders is a free data retrieval call binding the contract method 0x423ce1ae.
//
// Solidity: function stakeholders(address , uint256 ) constant returns(uint256)
func (_Staking *StakingCaller) Stakeholders(opts *bind.CallOpts, arg0 common.Address, arg1 *big.Int) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "stakeholders", arg0, arg1)
	return *ret0, err
}

// Stakeholders is a free data retrieval call binding the contract method 0x423ce1ae.
//
// Solidity: function stakeholders(address , uint256 ) constant returns(uint256)
func (_Staking *StakingSession) Stakeholders(arg0 common.Address, arg1 *big.Int) (*big.Int, error) {
	return _Staking.Contract.Stakeholders(&_Staking.CallOpts, arg0, arg1)
}

// Stakeholders is a free data retrieval call binding the contract method 0x423ce1ae.
//
// Solidity: function stakeholders(address , uint256 ) constant returns(uint256)
func (_Staking *StakingCallerSession) Stakeholders(arg0 common.Address, arg1 *big.Int) (*big.Int, error) {
	return _Staking.Contract.Stakeholders(&_Staking.CallOpts, arg0, arg1)
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_Staking *StakingCaller) Token(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "token")
	return *ret0, err
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_Staking *StakingSession) Token() (common.Address, error) {
	return _Staking.Contract.Token(&_Staking.CallOpts)
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_Staking *StakingCallerSession) Token() (common.Address, error) {
	return _Staking.Contract.Token(&_Staking.CallOpts)
}

// TotalStaked is a free data retrieval call binding the contract method 0x817b1cd2.
//
// Solidity: function totalStaked() constant returns(uint256)
func (_Staking *StakingCaller) TotalStaked(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "totalStaked")
	return *ret0, err
}

// TotalStaked is a free data retrieval call binding the contract method 0x817b1cd2.
//
// Solidity: function totalStaked() constant returns(uint256)
func (_Staking *StakingSession) TotalStaked() (*big.Int, error) {
	return _Staking.Contract.TotalStaked(&_Staking.CallOpts)
}

// TotalStaked is a free data retrieval call binding the contract method 0x817b1cd2.
//
// Solidity: function totalStaked() constant returns(uint256)
func (_Staking *StakingCallerSession) TotalStaked() (*big.Int, error) {
	return _Staking.Contract.TotalStaked(&_Staking.CallOpts)
}

// UnStakeDuration is a free data retrieval call binding the contract method 0xc698d495.
//
// Solidity: function unStakeDuration() constant returns(uint256)
func (_Staking *StakingCaller) UnStakeDuration(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "unStakeDuration")
	return *ret0, err
}

// UnStakeDuration is a free data retrieval call binding the contract method 0xc698d495.
//
// Solidity: function unStakeDuration() constant returns(uint256)
func (_Staking *StakingSession) UnStakeDuration() (*big.Int, error) {
	return _Staking.Contract.UnStakeDuration(&_Staking.CallOpts)
}

// UnStakeDuration is a free data retrieval call binding the contract method 0xc698d495.
//
// Solidity: function unStakeDuration() constant returns(uint256)
func (_Staking *StakingCallerSession) UnStakeDuration() (*big.Int, error) {
	return _Staking.Contract.UnStakeDuration(&_Staking.CallOpts)
}

// Whitelist is a free data retrieval call binding the contract method 0x9b19251a.
//
// Solidity: function whitelist(address ) constant returns(bool)
func (_Staking *StakingCaller) Whitelist(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _Staking.contract.Call(opts, out, "whitelist", arg0)
	return *ret0, err
}

// Whitelist is a free data retrieval call binding the contract method 0x9b19251a.
//
// Solidity: function whitelist(address ) constant returns(bool)
func (_Staking *StakingSession) Whitelist(arg0 common.Address) (bool, error) {
	return _Staking.Contract.Whitelist(&_Staking.CallOpts, arg0)
}

// Whitelist is a free data retrieval call binding the contract method 0x9b19251a.
//
// Solidity: function whitelist(address ) constant returns(bool)
func (_Staking *StakingCallerSession) Whitelist(arg0 common.Address) (bool, error) {
	return _Staking.Contract.Whitelist(&_Staking.CallOpts, arg0)
}

// AddAddressToWhitelist is a paid mutator transaction binding the contract method 0x7b9417c8.
//
// Solidity: function addAddressToWhitelist(address addr) returns(bool success)
func (_Staking *StakingTransactor) AddAddressToWhitelist(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "addAddressToWhitelist", addr)
}

// AddAddressToWhitelist is a paid mutator transaction binding the contract method 0x7b9417c8.
//
// Solidity: function addAddressToWhitelist(address addr) returns(bool success)
func (_Staking *StakingSession) AddAddressToWhitelist(addr common.Address) (*types.Transaction, error) {
	return _Staking.Contract.AddAddressToWhitelist(&_Staking.TransactOpts, addr)
}

// AddAddressToWhitelist is a paid mutator transaction binding the contract method 0x7b9417c8.
//
// Solidity: function addAddressToWhitelist(address addr) returns(bool success)
func (_Staking *StakingTransactorSession) AddAddressToWhitelist(addr common.Address) (*types.Transaction, error) {
	return _Staking.Contract.AddAddressToWhitelist(&_Staking.TransactOpts, addr)
}

// AddAddressesToWhitelist is a paid mutator transaction binding the contract method 0xe2ec6ec3.
//
// Solidity: function addAddressesToWhitelist(address[] addrs) returns(bool success)
func (_Staking *StakingTransactor) AddAddressesToWhitelist(opts *bind.TransactOpts, addrs []common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "addAddressesToWhitelist", addrs)
}

// AddAddressesToWhitelist is a paid mutator transaction binding the contract method 0xe2ec6ec3.
//
// Solidity: function addAddressesToWhitelist(address[] addrs) returns(bool success)
func (_Staking *StakingSession) AddAddressesToWhitelist(addrs []common.Address) (*types.Transaction, error) {
	return _Staking.Contract.AddAddressesToWhitelist(&_Staking.TransactOpts, addrs)
}

// AddAddressesToWhitelist is a paid mutator transaction binding the contract method 0xe2ec6ec3.
//
// Solidity: function addAddressesToWhitelist(address[] addrs) returns(bool success)
func (_Staking *StakingTransactorSession) AddAddressesToWhitelist(addrs []common.Address) (*types.Transaction, error) {
	return _Staking.Contract.AddAddressesToWhitelist(&_Staking.TransactOpts, addrs)
}

// CreateBucket is a paid mutator transaction binding the contract method 0xeae20f76.
//
// Solidity: function createBucket(bytes12 _canName, uint256 _amount, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns(uint256)
func (_Staking *StakingTransactor) CreateBucket(opts *bind.TransactOpts, _canName [12]byte, _amount *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "createBucket", _canName, _amount, _stakeDuration, _nonDecay, _data)
}

// CreateBucket is a paid mutator transaction binding the contract method 0xeae20f76.
//
// Solidity: function createBucket(bytes12 _canName, uint256 _amount, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns(uint256)
func (_Staking *StakingSession) CreateBucket(_canName [12]byte, _amount *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.CreateBucket(&_Staking.TransactOpts, _canName, _amount, _stakeDuration, _nonDecay, _data)
}

// CreateBucket is a paid mutator transaction binding the contract method 0xeae20f76.
//
// Solidity: function createBucket(bytes12 _canName, uint256 _amount, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns(uint256)
func (_Staking *StakingTransactorSession) CreateBucket(_canName [12]byte, _amount *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.CreateBucket(&_Staking.TransactOpts, _canName, _amount, _stakeDuration, _nonDecay, _data)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Staking *StakingTransactor) Pause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "pause")
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Staking *StakingSession) Pause() (*types.Transaction, error) {
	return _Staking.Contract.Pause(&_Staking.TransactOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Staking *StakingTransactorSession) Pause() (*types.Transaction, error) {
	return _Staking.Contract.Pause(&_Staking.TransactOpts)
}

// RemoveAddressFromWhitelist is a paid mutator transaction binding the contract method 0x286dd3f5.
//
// Solidity: function removeAddressFromWhitelist(address addr) returns(bool success)
func (_Staking *StakingTransactor) RemoveAddressFromWhitelist(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "removeAddressFromWhitelist", addr)
}

// RemoveAddressFromWhitelist is a paid mutator transaction binding the contract method 0x286dd3f5.
//
// Solidity: function removeAddressFromWhitelist(address addr) returns(bool success)
func (_Staking *StakingSession) RemoveAddressFromWhitelist(addr common.Address) (*types.Transaction, error) {
	return _Staking.Contract.RemoveAddressFromWhitelist(&_Staking.TransactOpts, addr)
}

// RemoveAddressFromWhitelist is a paid mutator transaction binding the contract method 0x286dd3f5.
//
// Solidity: function removeAddressFromWhitelist(address addr) returns(bool success)
func (_Staking *StakingTransactorSession) RemoveAddressFromWhitelist(addr common.Address) (*types.Transaction, error) {
	return _Staking.Contract.RemoveAddressFromWhitelist(&_Staking.TransactOpts, addr)
}

// RemoveAddressesFromWhitelist is a paid mutator transaction binding the contract method 0x24953eaa.
//
// Solidity: function removeAddressesFromWhitelist(address[] addrs) returns(bool success)
func (_Staking *StakingTransactor) RemoveAddressesFromWhitelist(opts *bind.TransactOpts, addrs []common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "removeAddressesFromWhitelist", addrs)
}

// RemoveAddressesFromWhitelist is a paid mutator transaction binding the contract method 0x24953eaa.
//
// Solidity: function removeAddressesFromWhitelist(address[] addrs) returns(bool success)
func (_Staking *StakingSession) RemoveAddressesFromWhitelist(addrs []common.Address) (*types.Transaction, error) {
	return _Staking.Contract.RemoveAddressesFromWhitelist(&_Staking.TransactOpts, addrs)
}

// RemoveAddressesFromWhitelist is a paid mutator transaction binding the contract method 0x24953eaa.
//
// Solidity: function removeAddressesFromWhitelist(address[] addrs) returns(bool success)
func (_Staking *StakingTransactorSession) RemoveAddressesFromWhitelist(addrs []common.Address) (*types.Transaction, error) {
	return _Staking.Contract.RemoveAddressesFromWhitelist(&_Staking.TransactOpts, addrs)
}

// Restake is a paid mutator transaction binding the contract method 0x7b24a5fd.
//
// Solidity: function restake(uint256 _bucketIndex, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns()
func (_Staking *StakingTransactor) Restake(opts *bind.TransactOpts, _bucketIndex *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "restake", _bucketIndex, _stakeDuration, _nonDecay, _data)
}

// Restake is a paid mutator transaction binding the contract method 0x7b24a5fd.
//
// Solidity: function restake(uint256 _bucketIndex, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns()
func (_Staking *StakingSession) Restake(_bucketIndex *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.Restake(&_Staking.TransactOpts, _bucketIndex, _stakeDuration, _nonDecay, _data)
}

// Restake is a paid mutator transaction binding the contract method 0x7b24a5fd.
//
// Solidity: function restake(uint256 _bucketIndex, uint256 _stakeDuration, bool _nonDecay, bytes _data) returns()
func (_Staking *StakingTransactorSession) Restake(_bucketIndex *big.Int, _stakeDuration *big.Int, _nonDecay bool, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.Restake(&_Staking.TransactOpts, _bucketIndex, _stakeDuration, _nonDecay, _data)
}

// Revote is a paid mutator transaction binding the contract method 0xd3e41fd2.
//
// Solidity: function revote(uint256 _bucketIndex, bytes12 _canName, bytes _data) returns()
func (_Staking *StakingTransactor) Revote(opts *bind.TransactOpts, _bucketIndex *big.Int, _canName [12]byte, _data []byte) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "revote", _bucketIndex, _canName, _data)
}

// Revote is a paid mutator transaction binding the contract method 0xd3e41fd2.
//
// Solidity: function revote(uint256 _bucketIndex, bytes12 _canName, bytes _data) returns()
func (_Staking *StakingSession) Revote(_bucketIndex *big.Int, _canName [12]byte, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.Revote(&_Staking.TransactOpts, _bucketIndex, _canName, _data)
}

// Revote is a paid mutator transaction binding the contract method 0xd3e41fd2.
//
// Solidity: function revote(uint256 _bucketIndex, bytes12 _canName, bytes _data) returns()
func (_Staking *StakingTransactorSession) Revote(_bucketIndex *big.Int, _canName [12]byte, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.Revote(&_Staking.TransactOpts, _bucketIndex, _canName, _data)
}

// SetBucketOwner is a paid mutator transaction binding the contract method 0x9cfe3461.
//
// Solidity: function setBucketOwner(uint256 _bucketIndex, address _newOwner, bytes _data) returns()
func (_Staking *StakingTransactor) SetBucketOwner(opts *bind.TransactOpts, _bucketIndex *big.Int, _newOwner common.Address, _data []byte) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "setBucketOwner", _bucketIndex, _newOwner, _data)
}

// SetBucketOwner is a paid mutator transaction binding the contract method 0x9cfe3461.
//
// Solidity: function setBucketOwner(uint256 _bucketIndex, address _newOwner, bytes _data) returns()
func (_Staking *StakingSession) SetBucketOwner(_bucketIndex *big.Int, _newOwner common.Address, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.SetBucketOwner(&_Staking.TransactOpts, _bucketIndex, _newOwner, _data)
}

// SetBucketOwner is a paid mutator transaction binding the contract method 0x9cfe3461.
//
// Solidity: function setBucketOwner(uint256 _bucketIndex, address _newOwner, bytes _data) returns()
func (_Staking *StakingTransactorSession) SetBucketOwner(_bucketIndex *big.Int, _newOwner common.Address, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.SetBucketOwner(&_Staking.TransactOpts, _bucketIndex, _newOwner, _data)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_Staking *StakingTransactor) TransferOwnership(opts *bind.TransactOpts, _newOwner common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "transferOwnership", _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_Staking *StakingSession) TransferOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _Staking.Contract.TransferOwnership(&_Staking.TransactOpts, _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_Staking *StakingTransactorSession) TransferOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _Staking.Contract.TransferOwnership(&_Staking.TransactOpts, _newOwner)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Staking *StakingTransactor) Unpause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "unpause")
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Staking *StakingSession) Unpause() (*types.Transaction, error) {
	return _Staking.Contract.Unpause(&_Staking.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Staking *StakingTransactorSession) Unpause() (*types.Transaction, error) {
	return _Staking.Contract.Unpause(&_Staking.TransactOpts)
}

// Unstake is a paid mutator transaction binding the contract method 0xc8fd6ed0.
//
// Solidity: function unstake(uint256 _bucketIndex, bytes _data) returns()
func (_Staking *StakingTransactor) Unstake(opts *bind.TransactOpts, _bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "unstake", _bucketIndex, _data)
}

// Unstake is a paid mutator transaction binding the contract method 0xc8fd6ed0.
//
// Solidity: function unstake(uint256 _bucketIndex, bytes _data) returns()
func (_Staking *StakingSession) Unstake(_bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.Unstake(&_Staking.TransactOpts, _bucketIndex, _data)
}

// Unstake is a paid mutator transaction binding the contract method 0xc8fd6ed0.
//
// Solidity: function unstake(uint256 _bucketIndex, bytes _data) returns()
func (_Staking *StakingTransactorSession) Unstake(_bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.Unstake(&_Staking.TransactOpts, _bucketIndex, _data)
}

// Withdraw is a paid mutator transaction binding the contract method 0x030ba25d.
//
// Solidity: function withdraw(uint256 _bucketIndex, bytes _data) returns()
func (_Staking *StakingTransactor) Withdraw(opts *bind.TransactOpts, _bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "withdraw", _bucketIndex, _data)
}

// Withdraw is a paid mutator transaction binding the contract method 0x030ba25d.
//
// Solidity: function withdraw(uint256 _bucketIndex, bytes _data) returns()
func (_Staking *StakingSession) Withdraw(_bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.Withdraw(&_Staking.TransactOpts, _bucketIndex, _data)
}

// Withdraw is a paid mutator transaction binding the contract method 0x030ba25d.
//
// Solidity: function withdraw(uint256 _bucketIndex, bytes _data) returns()
func (_Staking *StakingTransactorSession) Withdraw(_bucketIndex *big.Int, _data []byte) (*types.Transaction, error) {
	return _Staking.Contract.Withdraw(&_Staking.TransactOpts, _bucketIndex, _data)
}

// StakingBucketCreatedIterator is returned from FilterBucketCreated and is used to iterate over the raw logs and unpacked data for BucketCreated events raised by the Staking contract.
type StakingBucketCreatedIterator struct {
	Event *StakingBucketCreated // Event containing the contract specifics and raw log

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
func (it *StakingBucketCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingBucketCreated)
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
		it.Event = new(StakingBucketCreated)
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
func (it *StakingBucketCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingBucketCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingBucketCreated represents a BucketCreated event raised by the Staking contract.
type StakingBucketCreated struct {
	BucketIndex   *big.Int
	CanName       [12]byte
	Amount        *big.Int
	StakeDuration *big.Int
	NonDecay      bool
	Data          []byte
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterBucketCreated is a free log retrieval operation binding the contract event 0xbecddf0f61f76a4ac94a507fbc32c036d2fb7c4b466cad82dd9a4a2d76b263fe.
//
// Solidity: event BucketCreated(uint256 bucketIndex, bytes12 canName, uint256 amount, uint256 stakeDuration, bool nonDecay, bytes data)
func (_Staking *StakingFilterer) FilterBucketCreated(opts *bind.FilterOpts) (*StakingBucketCreatedIterator, error) {

	logs, sub, err := _Staking.contract.FilterLogs(opts, "BucketCreated")
	if err != nil {
		return nil, err
	}
	return &StakingBucketCreatedIterator{contract: _Staking.contract, event: "BucketCreated", logs: logs, sub: sub}, nil
}

// WatchBucketCreated is a free log subscription operation binding the contract event 0xbecddf0f61f76a4ac94a507fbc32c036d2fb7c4b466cad82dd9a4a2d76b263fe.
//
// Solidity: event BucketCreated(uint256 bucketIndex, bytes12 canName, uint256 amount, uint256 stakeDuration, bool nonDecay, bytes data)
func (_Staking *StakingFilterer) WatchBucketCreated(opts *bind.WatchOpts, sink chan<- *StakingBucketCreated) (event.Subscription, error) {

	logs, sub, err := _Staking.contract.WatchLogs(opts, "BucketCreated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingBucketCreated)
				if err := _Staking.contract.UnpackLog(event, "BucketCreated", log); err != nil {
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

// StakingBucketUnstakeIterator is returned from FilterBucketUnstake and is used to iterate over the raw logs and unpacked data for BucketUnstake events raised by the Staking contract.
type StakingBucketUnstakeIterator struct {
	Event *StakingBucketUnstake // Event containing the contract specifics and raw log

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
func (it *StakingBucketUnstakeIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingBucketUnstake)
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
		it.Event = new(StakingBucketUnstake)
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
func (it *StakingBucketUnstakeIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingBucketUnstakeIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingBucketUnstake represents a BucketUnstake event raised by the Staking contract.
type StakingBucketUnstake struct {
	BucketIndex *big.Int
	CanName     [12]byte
	Amount      *big.Int
	Data        []byte
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterBucketUnstake is a free log retrieval operation binding the contract event 0xaa192dc938c20fb63756fbd8f4d9f46092c3252f772b2c549c4688c118b6b475.
//
// Solidity: event BucketUnstake(uint256 bucketIndex, bytes12 canName, uint256 amount, bytes data)
func (_Staking *StakingFilterer) FilterBucketUnstake(opts *bind.FilterOpts) (*StakingBucketUnstakeIterator, error) {

	logs, sub, err := _Staking.contract.FilterLogs(opts, "BucketUnstake")
	if err != nil {
		return nil, err
	}
	return &StakingBucketUnstakeIterator{contract: _Staking.contract, event: "BucketUnstake", logs: logs, sub: sub}, nil
}

// WatchBucketUnstake is a free log subscription operation binding the contract event 0xaa192dc938c20fb63756fbd8f4d9f46092c3252f772b2c549c4688c118b6b475.
//
// Solidity: event BucketUnstake(uint256 bucketIndex, bytes12 canName, uint256 amount, bytes data)
func (_Staking *StakingFilterer) WatchBucketUnstake(opts *bind.WatchOpts, sink chan<- *StakingBucketUnstake) (event.Subscription, error) {

	logs, sub, err := _Staking.contract.WatchLogs(opts, "BucketUnstake")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingBucketUnstake)
				if err := _Staking.contract.UnpackLog(event, "BucketUnstake", log); err != nil {
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

// StakingBucketUpdatedIterator is returned from FilterBucketUpdated and is used to iterate over the raw logs and unpacked data for BucketUpdated events raised by the Staking contract.
type StakingBucketUpdatedIterator struct {
	Event *StakingBucketUpdated // Event containing the contract specifics and raw log

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
func (it *StakingBucketUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingBucketUpdated)
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
		it.Event = new(StakingBucketUpdated)
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
func (it *StakingBucketUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingBucketUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingBucketUpdated represents a BucketUpdated event raised by the Staking contract.
type StakingBucketUpdated struct {
	BucketIndex    *big.Int
	CanName        [12]byte
	StakeDuration  *big.Int
	StakeStartTime *big.Int
	NonDecay       bool
	BucketOwner    common.Address
	Data           []byte
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterBucketUpdated is a free log retrieval operation binding the contract event 0x004bbbedd0138c223ffed73fdab05a22a5d22770de54bea694d06661d59d1600.
//
// Solidity: event BucketUpdated(uint256 bucketIndex, bytes12 canName, uint256 stakeDuration, uint256 stakeStartTime, bool nonDecay, address bucketOwner, bytes data)
func (_Staking *StakingFilterer) FilterBucketUpdated(opts *bind.FilterOpts) (*StakingBucketUpdatedIterator, error) {

	logs, sub, err := _Staking.contract.FilterLogs(opts, "BucketUpdated")
	if err != nil {
		return nil, err
	}
	return &StakingBucketUpdatedIterator{contract: _Staking.contract, event: "BucketUpdated", logs: logs, sub: sub}, nil
}

// WatchBucketUpdated is a free log subscription operation binding the contract event 0x004bbbedd0138c223ffed73fdab05a22a5d22770de54bea694d06661d59d1600.
//
// Solidity: event BucketUpdated(uint256 bucketIndex, bytes12 canName, uint256 stakeDuration, uint256 stakeStartTime, bool nonDecay, address bucketOwner, bytes data)
func (_Staking *StakingFilterer) WatchBucketUpdated(opts *bind.WatchOpts, sink chan<- *StakingBucketUpdated) (event.Subscription, error) {

	logs, sub, err := _Staking.contract.WatchLogs(opts, "BucketUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingBucketUpdated)
				if err := _Staking.contract.UnpackLog(event, "BucketUpdated", log); err != nil {
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

// StakingBucketWithdrawIterator is returned from FilterBucketWithdraw and is used to iterate over the raw logs and unpacked data for BucketWithdraw events raised by the Staking contract.
type StakingBucketWithdrawIterator struct {
	Event *StakingBucketWithdraw // Event containing the contract specifics and raw log

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
func (it *StakingBucketWithdrawIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingBucketWithdraw)
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
		it.Event = new(StakingBucketWithdraw)
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
func (it *StakingBucketWithdrawIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingBucketWithdrawIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingBucketWithdraw represents a BucketWithdraw event raised by the Staking contract.
type StakingBucketWithdraw struct {
	BucketIndex *big.Int
	CanName     [12]byte
	Amount      *big.Int
	Data        []byte
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterBucketWithdraw is a free log retrieval operation binding the contract event 0x2a79739690fe6bf5933c5d812824e30c2b95d43b6ddadd96148a4493d3b56540.
//
// Solidity: event BucketWithdraw(uint256 bucketIndex, bytes12 canName, uint256 amount, bytes data)
func (_Staking *StakingFilterer) FilterBucketWithdraw(opts *bind.FilterOpts) (*StakingBucketWithdrawIterator, error) {

	logs, sub, err := _Staking.contract.FilterLogs(opts, "BucketWithdraw")
	if err != nil {
		return nil, err
	}
	return &StakingBucketWithdrawIterator{contract: _Staking.contract, event: "BucketWithdraw", logs: logs, sub: sub}, nil
}

// WatchBucketWithdraw is a free log subscription operation binding the contract event 0x2a79739690fe6bf5933c5d812824e30c2b95d43b6ddadd96148a4493d3b56540.
//
// Solidity: event BucketWithdraw(uint256 bucketIndex, bytes12 canName, uint256 amount, bytes data)
func (_Staking *StakingFilterer) WatchBucketWithdraw(opts *bind.WatchOpts, sink chan<- *StakingBucketWithdraw) (event.Subscription, error) {

	logs, sub, err := _Staking.contract.WatchLogs(opts, "BucketWithdraw")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingBucketWithdraw)
				if err := _Staking.contract.UnpackLog(event, "BucketWithdraw", log); err != nil {
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

// StakingPauseIterator is returned from FilterPause and is used to iterate over the raw logs and unpacked data for Pause events raised by the Staking contract.
type StakingPauseIterator struct {
	Event *StakingPause // Event containing the contract specifics and raw log

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
func (it *StakingPauseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingPause)
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
		it.Event = new(StakingPause)
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
func (it *StakingPauseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingPauseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingPause represents a Pause event raised by the Staking contract.
type StakingPause struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterPause is a free log retrieval operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: event Pause()
func (_Staking *StakingFilterer) FilterPause(opts *bind.FilterOpts) (*StakingPauseIterator, error) {

	logs, sub, err := _Staking.contract.FilterLogs(opts, "Pause")
	if err != nil {
		return nil, err
	}
	return &StakingPauseIterator{contract: _Staking.contract, event: "Pause", logs: logs, sub: sub}, nil
}

// WatchPause is a free log subscription operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: event Pause()
func (_Staking *StakingFilterer) WatchPause(opts *bind.WatchOpts, sink chan<- *StakingPause) (event.Subscription, error) {

	logs, sub, err := _Staking.contract.WatchLogs(opts, "Pause")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingPause)
				if err := _Staking.contract.UnpackLog(event, "Pause", log); err != nil {
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

// StakingUnpauseIterator is returned from FilterUnpause and is used to iterate over the raw logs and unpacked data for Unpause events raised by the Staking contract.
type StakingUnpauseIterator struct {
	Event *StakingUnpause // Event containing the contract specifics and raw log

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
func (it *StakingUnpauseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingUnpause)
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
		it.Event = new(StakingUnpause)
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
func (it *StakingUnpauseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingUnpauseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingUnpause represents a Unpause event raised by the Staking contract.
type StakingUnpause struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterUnpause is a free log retrieval operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: event Unpause()
func (_Staking *StakingFilterer) FilterUnpause(opts *bind.FilterOpts) (*StakingUnpauseIterator, error) {

	logs, sub, err := _Staking.contract.FilterLogs(opts, "Unpause")
	if err != nil {
		return nil, err
	}
	return &StakingUnpauseIterator{contract: _Staking.contract, event: "Unpause", logs: logs, sub: sub}, nil
}

// WatchUnpause is a free log subscription operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: event Unpause()
func (_Staking *StakingFilterer) WatchUnpause(opts *bind.WatchOpts, sink chan<- *StakingUnpause) (event.Subscription, error) {

	logs, sub, err := _Staking.contract.WatchLogs(opts, "Unpause")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingUnpause)
				if err := _Staking.contract.UnpackLog(event, "Unpause", log); err != nil {
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

// StakingWhitelistedAddressAddedIterator is returned from FilterWhitelistedAddressAdded and is used to iterate over the raw logs and unpacked data for WhitelistedAddressAdded events raised by the Staking contract.
type StakingWhitelistedAddressAddedIterator struct {
	Event *StakingWhitelistedAddressAdded // Event containing the contract specifics and raw log

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
func (it *StakingWhitelistedAddressAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingWhitelistedAddressAdded)
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
		it.Event = new(StakingWhitelistedAddressAdded)
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
func (it *StakingWhitelistedAddressAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingWhitelistedAddressAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingWhitelistedAddressAdded represents a WhitelistedAddressAdded event raised by the Staking contract.
type StakingWhitelistedAddressAdded struct {
	Addr common.Address
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterWhitelistedAddressAdded is a free log retrieval operation binding the contract event 0xd1bba68c128cc3f427e5831b3c6f99f480b6efa6b9e80c757768f6124158cc3f.
//
// Solidity: event WhitelistedAddressAdded(address addr)
func (_Staking *StakingFilterer) FilterWhitelistedAddressAdded(opts *bind.FilterOpts) (*StakingWhitelistedAddressAddedIterator, error) {

	logs, sub, err := _Staking.contract.FilterLogs(opts, "WhitelistedAddressAdded")
	if err != nil {
		return nil, err
	}
	return &StakingWhitelistedAddressAddedIterator{contract: _Staking.contract, event: "WhitelistedAddressAdded", logs: logs, sub: sub}, nil
}

// WatchWhitelistedAddressAdded is a free log subscription operation binding the contract event 0xd1bba68c128cc3f427e5831b3c6f99f480b6efa6b9e80c757768f6124158cc3f.
//
// Solidity: event WhitelistedAddressAdded(address addr)
func (_Staking *StakingFilterer) WatchWhitelistedAddressAdded(opts *bind.WatchOpts, sink chan<- *StakingWhitelistedAddressAdded) (event.Subscription, error) {

	logs, sub, err := _Staking.contract.WatchLogs(opts, "WhitelistedAddressAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingWhitelistedAddressAdded)
				if err := _Staking.contract.UnpackLog(event, "WhitelistedAddressAdded", log); err != nil {
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

// StakingWhitelistedAddressRemovedIterator is returned from FilterWhitelistedAddressRemoved and is used to iterate over the raw logs and unpacked data for WhitelistedAddressRemoved events raised by the Staking contract.
type StakingWhitelistedAddressRemovedIterator struct {
	Event *StakingWhitelistedAddressRemoved // Event containing the contract specifics and raw log

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
func (it *StakingWhitelistedAddressRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingWhitelistedAddressRemoved)
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
		it.Event = new(StakingWhitelistedAddressRemoved)
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
func (it *StakingWhitelistedAddressRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingWhitelistedAddressRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingWhitelistedAddressRemoved represents a WhitelistedAddressRemoved event raised by the Staking contract.
type StakingWhitelistedAddressRemoved struct {
	Addr common.Address
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterWhitelistedAddressRemoved is a free log retrieval operation binding the contract event 0xf1abf01a1043b7c244d128e8595cf0c1d10743b022b03a02dffd8ca3bf729f5a.
//
// Solidity: event WhitelistedAddressRemoved(address addr)
func (_Staking *StakingFilterer) FilterWhitelistedAddressRemoved(opts *bind.FilterOpts) (*StakingWhitelistedAddressRemovedIterator, error) {

	logs, sub, err := _Staking.contract.FilterLogs(opts, "WhitelistedAddressRemoved")
	if err != nil {
		return nil, err
	}
	return &StakingWhitelistedAddressRemovedIterator{contract: _Staking.contract, event: "WhitelistedAddressRemoved", logs: logs, sub: sub}, nil
}

// WatchWhitelistedAddressRemoved is a free log subscription operation binding the contract event 0xf1abf01a1043b7c244d128e8595cf0c1d10743b022b03a02dffd8ca3bf729f5a.
//
// Solidity: event WhitelistedAddressRemoved(address addr)
func (_Staking *StakingFilterer) WatchWhitelistedAddressRemoved(opts *bind.WatchOpts, sink chan<- *StakingWhitelistedAddressRemoved) (event.Subscription, error) {

	logs, sub, err := _Staking.contract.WatchLogs(opts, "WhitelistedAddressRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingWhitelistedAddressRemoved)
				if err := _Staking.contract.UnpackLog(event, "WhitelistedAddressRemoved", log); err != nil {
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

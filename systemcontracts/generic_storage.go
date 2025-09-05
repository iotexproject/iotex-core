package systemcontracts

import (
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// GenericValue represents the value structure in the GenericStorage contract
type GenericValue struct {
	PrimaryData   []byte `json:"primaryData"`
	SecondaryData []byte `json:"secondaryData"`
	AuxiliaryData []byte `json:"auxiliaryData"`
}

// BatchGetResult represents the result of a batch get operation
type BatchGetResult struct {
	Values      []GenericValue `json:"values"`
	ExistsFlags []bool         `json:"existsFlags"`
}

// ListResult represents the result of a list operation
type ListResult struct {
	KeyList [][]byte       `json:"keyList"`
	Values  []GenericValue `json:"values"`
	Total   *big.Int       `json:"total"`
}

// ListKeysResult represents the result of a listKeys operation
type ListKeysResult struct {
	KeyList [][]byte `json:"keyList"`
	Total   *big.Int `json:"total"`
}

// GetResult represents the result of a get operation
type GetResult struct {
	Value     GenericValue `json:"value"`
	KeyExists bool         `json:"keyExists"`
}

// GenericStorageContract provides an interface to interact with the GenericStorage smart contract
type GenericStorageContract struct {
	contractAddress common.Address
	backend         ContractBackend
	abi             abi.ABI
}

// NewGenericStorageContract creates a new GenericStorage contract instance
func NewGenericStorageContract(contractAddress common.Address, backend ContractBackend) (*GenericStorageContract, error) {
	abi, err := abi.JSON(strings.NewReader(GenericStorageABI))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse GenericStorage ABI")
	}

	return &GenericStorageContract{
		contractAddress: contractAddress,
		backend:         backend,
		abi:             abi,
	}, nil
}

func (g *GenericStorageContract) Address() common.Address {
	return g.contractAddress
}

// Put stores data with the given key
func (g *GenericStorageContract) Put(key []byte, value GenericValue) error {
	// Validate input
	if len(key) == 0 {
		return errors.New("key cannot be empty")
	}

	// Pack the function call
	data, err := g.abi.Pack("put", key, value)
	if err != nil {
		return errors.Wrap(err, "failed to pack put call")
	}

	// Execute the transaction
	callMsg := &ethereum.CallMsg{
		To:    &g.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	if err := g.backend.Handle(callMsg); err != nil {
		return errors.Wrap(err, "failed to execute put")
	}

	log.L().Debug("Successfully stored data",
		zap.String("key", string(key)),
		zap.Int("primaryDataSize", len(value.PrimaryData)),
		zap.Int("secondaryDataSize", len(value.SecondaryData)),
		zap.Int("auxiliaryDataSize", len(value.AuxiliaryData)))

	return nil
}

// PutSimple stores data with only primary data (convenience method)
func (g *GenericStorageContract) PutSimple(key []byte, data []byte) error {
	value := GenericValue{
		PrimaryData:   data,
		SecondaryData: []byte{},
		AuxiliaryData: []byte{},
	}
	return g.Put(key, value)
}

// Get retrieves data by key
func (g *GenericStorageContract) Get(key []byte) (*GetResult, error) {
	// Validate input
	if len(key) == 0 {
		return nil, errors.New("key cannot be empty")
	}

	// Pack the function call
	data, err := g.abi.Pack("get", key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack get call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		To:    &g.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := g.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call get")
	}

	// Unpack the result
	var getResult struct {
		Value     GenericValue
		KeyExists bool
	}

	err = g.abi.UnpackIntoInterface(&getResult, "get", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack get result")
	}

	log.L().Debug("Successfully retrieved data",
		zap.String("key", string(key)),
		zap.Bool("exists", getResult.KeyExists))

	return &GetResult{
		Value:     getResult.Value,
		KeyExists: getResult.KeyExists,
	}, nil
}

// Remove deletes data by key
func (g *GenericStorageContract) Remove(key []byte) error {
	// Validate input
	if len(key) == 0 {
		return errors.New("key cannot be empty")
	}

	// Pack the function call
	data, err := g.abi.Pack("remove", key)
	if err != nil {
		return errors.Wrap(err, "failed to pack remove call")
	}

	// Execute the transaction
	callMsg := &ethereum.CallMsg{
		To:    &g.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	if err := g.backend.Handle(callMsg); err != nil {
		return errors.Wrap(err, "failed to execute remove")
	}

	log.L().Debug("Successfully removed data",
		zap.String("key", string(key)))

	return nil
}

// BatchGet retrieves multiple values by their keys
func (g *GenericStorageContract) BatchGet(keys [][]byte) (*BatchGetResult, error) {
	// Validate input
	if len(keys) == 0 {
		return &BatchGetResult{
			Values:      []GenericValue{},
			ExistsFlags: []bool{},
		}, nil
	}

	// Pack the function call
	data, err := g.abi.Pack("batchGet", keys)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack batchGet call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		To:    &g.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := g.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call batchGet")
	}

	// Unpack the result
	var batchGetResult struct {
		Values      []GenericValue
		ExistsFlags []bool
	}

	err = g.abi.UnpackIntoInterface(&batchGetResult, "batchGet", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack batchGet result")
	}

	log.L().Debug("Successfully batch retrieved data",
		zap.Int("requestedKeys", len(keys)),
		zap.Int("returnedValues", len(batchGetResult.Values)))

	return &BatchGetResult{
		Values:      batchGetResult.Values,
		ExistsFlags: batchGetResult.ExistsFlags,
	}, nil
}

// List retrieves all stored data with pagination
func (g *GenericStorageContract) List(offset, limit uint64) (*ListResult, error) {
	// Pack the function call
	data, err := g.abi.Pack("list", new(big.Int).SetUint64(offset), new(big.Int).SetUint64(limit))
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack list call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		To:    &g.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := g.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call list")
	}

	// Unpack the result
	var listResult struct {
		KeyList [][]byte
		Values  []GenericValue
		Total   *big.Int
	}

	err = g.abi.UnpackIntoInterface(&listResult, "list", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack list result")
	}

	log.L().Debug("Successfully listed data",
		zap.Uint64("offset", offset),
		zap.Uint64("limit", limit),
		zap.Int("returned", len(listResult.Values)),
		zap.String("total", listResult.Total.String()))

	return &ListResult{
		KeyList: listResult.KeyList,
		Values:  listResult.Values,
		Total:   listResult.Total,
	}, nil
}

// ListKeys retrieves all keys with pagination (lightweight version)
func (g *GenericStorageContract) ListKeys(offset, limit uint64) (*ListKeysResult, error) {
	// Pack the function call
	data, err := g.abi.Pack("listKeys", new(big.Int).SetUint64(offset), new(big.Int).SetUint64(limit))
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack listKeys call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		To:    &g.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := g.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call listKeys")
	}

	// Unpack the result
	var listKeysResult struct {
		KeyList [][]byte
		Total   *big.Int
	}

	err = g.abi.UnpackIntoInterface(&listKeysResult, "listKeys", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack listKeys result")
	}

	log.L().Debug("Successfully listed keys",
		zap.Uint64("offset", offset),
		zap.Uint64("limit", limit),
		zap.Int("returned", len(listKeysResult.KeyList)),
		zap.String("total", listKeysResult.Total.String()))

	return &ListKeysResult{
		KeyList: listKeysResult.KeyList,
		Total:   listKeysResult.Total,
	}, nil
}

// Exists checks if a key exists in the storage
func (g *GenericStorageContract) Exists(key []byte) (bool, error) {
	// Validate input
	if len(key) == 0 {
		return false, errors.New("key cannot be empty")
	}

	// Pack the function call
	data, err := g.abi.Pack("exists", key)
	if err != nil {
		return false, errors.Wrap(err, "failed to pack exists call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		To:    &g.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := g.backend.Call(callMsg)
	if err != nil {
		return false, errors.Wrap(err, "failed to call exists")
	}

	// Unpack the result
	var existsResult struct {
		KeyExists bool
	}

	err = g.abi.UnpackIntoInterface(&existsResult, "exists", result)
	if err != nil {
		return false, errors.Wrap(err, "failed to unpack exists result")
	}

	log.L().Debug("Successfully checked key existence",
		zap.String("key", string(key)),
		zap.Bool("exists", existsResult.KeyExists))

	return existsResult.KeyExists, nil
}

// Count returns the total number of stored items
func (g *GenericStorageContract) Count() (*big.Int, error) {
	// Pack the function call
	data, err := g.abi.Pack("count")
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack count call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		To:    &g.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := g.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call count")
	}

	// Unpack the result
	var countResult struct {
		TotalCount *big.Int
	}

	err = g.abi.UnpackIntoInterface(&countResult, "count", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack count result")
	}

	log.L().Debug("Successfully retrieved count",
		zap.String("count", countResult.TotalCount.String()))

	return countResult.TotalCount, nil
}

// Clear removes all stored data (emergency function)
func (g *GenericStorageContract) Clear() error {
	// Pack the function call
	data, err := g.abi.Pack("clear")
	if err != nil {
		return errors.Wrap(err, "failed to pack clear call")
	}

	// Execute the transaction
	callMsg := &ethereum.CallMsg{
		To:    &g.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	if err := g.backend.Handle(callMsg); err != nil {
		return errors.Wrap(err, "failed to execute clear")
	}

	log.L().Debug("Successfully cleared all data")

	return nil
}

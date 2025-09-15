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

// NamespaceGenericValue represents the value structure in the NamespaceStorage contract
// This is the same as GenericValue but renamed to avoid conflicts
type NamespaceGenericValue = GenericValue

// NamespaceBatchGetResult represents the result of a batch get operation
type NamespaceBatchGetResult = BatchGetResult

// NamespaceListResult represents the result of a list operation
type NamespaceListResult = ListResult

// NamespaceListKeysResult represents the result of a listKeys operation
type NamespaceListKeysResult = ListKeysResult

// NamespaceListNamespacesResult represents the result of a listNamespaces operation
type NamespaceListNamespacesResult struct {
	NamespaceList []string   `json:"namespaceList"`
	Counts        []*big.Int `json:"counts"`
	Total         *big.Int   `json:"total"`
}

// NamespaceGetResult represents the result of a get operation
type NamespaceGetResult = GetResult

// NamespaceStorageContract provides an interface to interact with the NamespaceStorage smart contract
type NamespaceStorageContract struct {
	contractAddress common.Address
	backend         ContractBackend
	abi             abi.ABI
}

// NewNamespaceStorageContract creates a new NamespaceStorage contract instance
func NewNamespaceStorageContract(contractAddress common.Address, backend ContractBackend) (*NamespaceStorageContract, error) {
	abi, err := abi.JSON(strings.NewReader(NamespaceStorageABI))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse NamespaceStorage ABI")
	}

	return &NamespaceStorageContract{
		contractAddress: contractAddress,
		backend:         backend,
		abi:             abi,
	}, nil
}

// Address returns the contract address
func (ns *NamespaceStorageContract) Address() common.Address {
	return ns.contractAddress
}

// Put stores data with the given namespace and key
func (ns *NamespaceStorageContract) Put(namespace string, key []byte, value NamespaceGenericValue) error {
	// Validate input
	if len(namespace) == 0 {
		return errors.New("namespace cannot be empty")
	}
	if len(key) == 0 {
		return errors.New("key cannot be empty")
	}

	// Pack the function call
	data, err := ns.abi.Pack("put", namespace, key, value)
	if err != nil {
		return errors.Wrap(err, "failed to pack put call")
	}

	// Execute the transaction
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	if err := ns.backend.Handle(callMsg); err != nil {
		return errors.Wrap(err, "failed to execute put")
	}

	log.L().Debug("Successfully stored data",
		zap.String("namespace", namespace),
		zap.String("key", string(key)))

	return nil
}

// Get retrieves data by namespace and key
func (ns *NamespaceStorageContract) Get(namespace string, key []byte) (*NamespaceGetResult, error) {
	// Validate input
	if len(namespace) == 0 {
		return nil, errors.New("namespace cannot be empty")
	}
	if len(key) == 0 {
		return nil, errors.New("key cannot be empty")
	}

	// Pack the function call
	data, err := ns.abi.Pack("get", namespace, key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack get call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := ns.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call get")
	}

	// Unpack the result
	var getResult struct {
		Value     NamespaceGenericValue
		KeyExists bool
	}

	err = ns.abi.UnpackIntoInterface(&getResult, "get", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack get result")
	}

	log.L().Debug("Successfully retrieved data",
		zap.String("namespace", namespace),
		zap.String("key", string(key)),
		zap.Bool("exists", getResult.KeyExists))

	return &NamespaceGetResult{
		Value:     getResult.Value,
		KeyExists: getResult.KeyExists,
	}, nil
}

// Remove deletes data by namespace and key
func (ns *NamespaceStorageContract) Remove(namespace string, key []byte) error {
	// Validate input
	if len(namespace) == 0 {
		return errors.New("namespace cannot be empty")
	}
	if len(key) == 0 {
		return errors.New("key cannot be empty")
	}

	// Pack the function call
	data, err := ns.abi.Pack("remove", namespace, key)
	if err != nil {
		return errors.Wrap(err, "failed to pack remove call")
	}

	// Execute the transaction
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	if err := ns.backend.Handle(callMsg); err != nil {
		return errors.Wrap(err, "failed to execute remove")
	}

	log.L().Debug("Successfully removed data",
		zap.String("namespace", namespace),
		zap.String("key", string(key)))

	return nil
}

// Exists checks if a key exists in a namespace
func (ns *NamespaceStorageContract) Exists(namespace string, key []byte) (bool, error) {
	// Validate input
	if len(namespace) == 0 {
		return false, errors.New("namespace cannot be empty")
	}
	if len(key) == 0 {
		return false, errors.New("key cannot be empty")
	}

	// Pack the function call
	data, err := ns.abi.Pack("exists", namespace, key)
	if err != nil {
		return false, errors.Wrap(err, "failed to pack exists call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := ns.backend.Call(callMsg)
	if err != nil {
		return false, errors.Wrap(err, "failed to call exists")
	}

	// Unpack the result
	var keyExists bool
	err = ns.abi.UnpackIntoInterface(&keyExists, "exists", result)
	if err != nil {
		return false, errors.Wrap(err, "failed to unpack exists result")
	}

	return keyExists, nil
}

// BatchGet retrieves multiple values by their keys within a namespace
func (ns *NamespaceStorageContract) BatchGet(namespace string, keys [][]byte) (*NamespaceBatchGetResult, error) {
	// Validate input
	if len(namespace) == 0 {
		return nil, errors.New("namespace cannot be empty")
	}
	if len(keys) == 0 {
		return &NamespaceBatchGetResult{
			Values:      []NamespaceGenericValue{},
			ExistsFlags: []bool{},
		}, nil
	}

	// Pack the function call
	data, err := ns.abi.Pack("batchGet", namespace, keys)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack batchGet call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := ns.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call batchGet")
	}

	// Unpack the result
	var batchResult struct {
		Values      []NamespaceGenericValue
		ExistsFlags []bool
	}

	err = ns.abi.UnpackIntoInterface(&batchResult, "batchGet", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack batchGet result")
	}

	log.L().Debug("Successfully batch retrieved data",
		zap.String("namespace", namespace),
		zap.Int("count", len(keys)))

	return &NamespaceBatchGetResult{
		Values:      batchResult.Values,
		ExistsFlags: batchResult.ExistsFlags,
	}, nil
}

// BatchPut stores multiple key-value pairs in the same namespace
func (ns *NamespaceStorageContract) BatchPut(namespace string, keys [][]byte, values []NamespaceGenericValue) error {
	// Validate input
	if len(namespace) == 0 {
		return errors.New("namespace cannot be empty")
	}
	if len(keys) != len(values) {
		return errors.New("keys and values arrays must have same length")
	}
	if len(keys) == 0 {
		return nil // Nothing to do
	}

	// Pack the function call
	data, err := ns.abi.Pack("batchPut", namespace, keys, values)
	if err != nil {
		return errors.Wrap(err, "failed to pack batchPut call")
	}

	// Execute the transaction
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	if err := ns.backend.Handle(callMsg); err != nil {
		return errors.Wrap(err, "failed to execute batchPut")
	}

	log.L().Debug("Successfully batch stored data",
		zap.String("namespace", namespace),
		zap.Int("count", len(keys)))

	return nil
}

// List retrieves all stored data in a namespace with pagination
func (ns *NamespaceStorageContract) List(namespace string, offset, limit *big.Int) (*NamespaceListResult, error) {
	// Validate input
	if len(namespace) == 0 {
		return nil, errors.New("namespace cannot be empty")
	}
	if offset == nil {
		offset = big.NewInt(0)
	}
	if limit == nil {
		limit = big.NewInt(100) // Default limit
	}

	// Pack the function call
	data, err := ns.abi.Pack("list", namespace, offset, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack list call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := ns.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call list")
	}

	// Unpack the result
	var listResult struct {
		KeyList [][]byte
		Values  []NamespaceGenericValue
		Total   *big.Int
	}

	err = ns.abi.UnpackIntoInterface(&listResult, "list", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack list result")
	}

	log.L().Debug("Successfully listed data",
		zap.String("namespace", namespace),
		zap.String("total", listResult.Total.String()))

	return &NamespaceListResult{
		KeyList: listResult.KeyList,
		Values:  listResult.Values,
		Total:   listResult.Total,
	}, nil
}

// ListKeys retrieves all keys in a namespace with pagination
func (ns *NamespaceStorageContract) ListKeys(namespace string, offset, limit *big.Int) (*NamespaceListKeysResult, error) {
	// Validate input
	if len(namespace) == 0 {
		return nil, errors.New("namespace cannot be empty")
	}
	if offset == nil {
		offset = big.NewInt(0)
	}
	if limit == nil {
		limit = big.NewInt(100) // Default limit
	}

	// Pack the function call
	data, err := ns.abi.Pack("listKeys", namespace, offset, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack listKeys call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := ns.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call listKeys")
	}

	// Unpack the result
	var listKeysResult struct {
		KeyList [][]byte
		Total   *big.Int
	}

	err = ns.abi.UnpackIntoInterface(&listKeysResult, "listKeys", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack listKeys result")
	}

	log.L().Debug("Successfully listed keys",
		zap.String("namespace", namespace),
		zap.String("total", listKeysResult.Total.String()))

	return &NamespaceListKeysResult{
		KeyList: listKeysResult.KeyList,
		Total:   listKeysResult.Total,
	}, nil
}

// ListNamespaces retrieves all namespaces with pagination
func (ns *NamespaceStorageContract) ListNamespaces(offset, limit *big.Int) (*NamespaceListNamespacesResult, error) {
	if offset == nil {
		offset = big.NewInt(0)
	}
	if limit == nil {
		limit = big.NewInt(100) // Default limit
	}

	// Pack the function call
	data, err := ns.abi.Pack("listNamespaces", offset, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack listNamespaces call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := ns.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call listNamespaces")
	}

	// Unpack the result
	var listNamespacesResult struct {
		NamespaceList []string
		Counts        []*big.Int
		Total         *big.Int
	}

	err = ns.abi.UnpackIntoInterface(&listNamespacesResult, "listNamespaces", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack listNamespaces result")
	}

	log.L().Debug("Successfully listed namespaces",
		zap.String("total", listNamespacesResult.Total.String()))

	return &NamespaceListNamespacesResult{
		NamespaceList: listNamespacesResult.NamespaceList,
		Counts:        listNamespacesResult.Counts,
		Total:         listNamespacesResult.Total,
	}, nil
}

// HasNamespace checks if a namespace exists
func (ns *NamespaceStorageContract) HasNamespace(namespace string) (bool, error) {
	// Validate input
	if len(namespace) == 0 {
		return false, errors.New("namespace cannot be empty")
	}

	// Pack the function call
	data, err := ns.abi.Pack("hasNamespace", namespace)
	if err != nil {
		return false, errors.Wrap(err, "failed to pack hasNamespace call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := ns.backend.Call(callMsg)
	if err != nil {
		return false, errors.Wrap(err, "failed to call hasNamespace")
	}

	// Unpack the result
	var nsExists bool
	err = ns.abi.UnpackIntoInterface(&nsExists, "hasNamespace", result)
	if err != nil {
		return false, errors.Wrap(err, "failed to unpack hasNamespace result")
	}

	return nsExists, nil
}

// CountInNamespace returns the number of items in a namespace
func (ns *NamespaceStorageContract) CountInNamespace(namespace string) (*big.Int, error) {
	// Validate input
	if len(namespace) == 0 {
		return nil, errors.New("namespace cannot be empty")
	}

	// Pack the function call
	data, err := ns.abi.Pack("countInNamespace", namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack countInNamespace call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := ns.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call countInNamespace")
	}

	// Unpack the result
	var itemCount *big.Int
	err = ns.abi.UnpackIntoInterface(&itemCount, "countInNamespace", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack countInNamespace result")
	}

	return itemCount, nil
}

// NamespaceCount returns the total number of namespaces
func (ns *NamespaceStorageContract) NamespaceCount() (*big.Int, error) {
	// Pack the function call
	data, err := ns.abi.Pack("namespaceCount")
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack namespaceCount call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := ns.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call namespaceCount")
	}

	// Unpack the result
	var totalNamespaces *big.Int
	err = ns.abi.UnpackIntoInterface(&totalNamespaces, "namespaceCount", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack namespaceCount result")
	}

	return totalNamespaces, nil
}

// TotalCount returns the total number of items across all namespaces
func (ns *NamespaceStorageContract) TotalCount() (*big.Int, error) {
	// Pack the function call
	data, err := ns.abi.Pack("totalCount")
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack totalCount call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	result, err := ns.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call totalCount")
	}

	// Unpack the result
	var totalItems *big.Int
	err = ns.abi.UnpackIntoInterface(&totalItems, "totalCount", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack totalCount result")
	}

	return totalItems, nil
}

// ClearNamespace clears all data in a specific namespace
func (ns *NamespaceStorageContract) ClearNamespace(namespace string) error {
	// Validate input
	if len(namespace) == 0 {
		return errors.New("namespace cannot be empty")
	}

	// Pack the function call
	data, err := ns.abi.Pack("clearNamespace", namespace)
	if err != nil {
		return errors.Wrap(err, "failed to pack clearNamespace call")
	}

	// Execute the transaction
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	if err := ns.backend.Handle(callMsg); err != nil {
		return errors.Wrap(err, "failed to execute clearNamespace")
	}

	log.L().Debug("Successfully cleared namespace",
		zap.String("namespace", namespace))

	return nil
}

// ClearAll clears all stored data across all namespaces (emergency function)
func (ns *NamespaceStorageContract) ClearAll() error {
	// Pack the function call
	data, err := ns.abi.Pack("clearAll")
	if err != nil {
		return errors.Wrap(err, "failed to pack clearAll call")
	}

	// Execute the transaction
	callMsg := &ethereum.CallMsg{
		From:  common.BytesToAddress(systemContractCreatorAddr[:]),
		To:    &ns.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   math.MaxUint64,
	}

	if err := ns.backend.Handle(callMsg); err != nil {
		return errors.Wrap(err, "failed to execute clearAll")
	}

	log.L().Debug("Successfully cleared all data")

	return nil
}

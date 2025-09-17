package systemcontracts

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/pkg/errors"
)

type (

	// GenericValue represents the value structure in the GenericStorage contract
	GenericValue struct {
		PrimaryData   []byte `json:"primaryData"`
		SecondaryData []byte `json:"secondaryData"`
		AuxiliaryData []byte `json:"auxiliaryData"`
	}

	// GenericValueContainer is an interface for objects that can be encoded/decoded to/from GenericValue
	GenericValueContainer interface {
		Decode(data GenericValue) error
		Encode() (GenericValue, error)
	}

	// GenericValueObjectIterator is an iterator for GenericValue objects
	GenericValueObjectIterator struct {
		keys   [][]byte
		values []GenericValue
		exists []bool
		cur    int
	}

	// BatchGetResult represents the result of a batch get operation
	BatchGetResult struct {
		Values      []GenericValue `json:"values"`
		ExistsFlags []bool         `json:"existsFlags"`
	}

	// ListResult represents the result of a list operation
	ListResult struct {
		KeyList [][]byte       `json:"keyList"`
		Values  []GenericValue `json:"values"`
		Total   *big.Int       `json:"total"`
	}

	// ListKeysResult represents the result of a listKeys operation
	ListKeysResult struct {
		KeyList [][]byte `json:"keyList"`
		Total   *big.Int `json:"total"`
	}

	// GetResult represents the result of a get operation
	GetResult struct {
		Value     GenericValue `json:"value"`
		KeyExists bool         `json:"keyExists"`
	}
	// StorageContract provides an interface to interact with the storage smart contract
	StorageContract interface {
		Address() common.Address
		Put(key []byte, value GenericValue) error
		Get(key []byte) (*GetResult, error)
		Remove(key []byte) error
		Exists(key []byte) (bool, error)
		List(uint64, uint64) (*ListResult, error)
		ListKeys(uint64, uint64) (*ListKeysResult, error)
		BatchGet(keys [][]byte) (*BatchGetResult, error)
		Count() (*big.Int, error)
	}
)

// DecodeGenericValue decodes a GenericValue into a specific object
func DecodeGenericValue(o interface{}, data GenericValue) error {
	if o == nil {
		return errors.New("nil object")
	}
	if oo, ok := o.(GenericValueContainer); ok {
		return oo.Decode(data)
	}
	return errors.New("unsupported object type")
}

// NewGenericValueObjectIterator creates a new GenericValueObjectIterator
func NewGenericValueObjectIterator(keys [][]byte, values []GenericValue, exists []bool) (state.Iterator, error) {
	return &GenericValueObjectIterator{
		keys:   keys,
		values: values,
		exists: exists,
		cur:    0,
	}, nil
}

// Size returns the size of the iterator
func (gvoi *GenericValueObjectIterator) Size() int {
	return len(gvoi.values)
}

// Next returns the next key-value pair from the iterator
func (gvoi *GenericValueObjectIterator) Next(o interface{}) ([]byte, error) {
	if gvoi.cur >= len(gvoi.values) {
		return nil, state.ErrOutOfBoundary
	}
	value := gvoi.values[gvoi.cur]
	key := gvoi.keys[gvoi.cur]
	gvoi.cur++
	if gvoi.exists != nil && !gvoi.exists[gvoi.cur] {
		gvoi.cur++
		return key, state.ErrNilValue
	}
	if err := DecodeGenericValue(o, value); err != nil {
		return nil, err
	}
	return key, nil
}

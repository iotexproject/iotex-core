package systemcontracts

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
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
		Remove(key []byte) (bool, error)
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

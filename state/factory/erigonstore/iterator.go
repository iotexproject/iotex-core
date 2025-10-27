package erigonstore

import (
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// GenericValueObjectIterator is an iterator for GenericValue objects
type GenericValueObjectIterator struct {
	keys   [][]byte
	values []systemcontracts.GenericValue
	exists []bool
	cur    int
}

// NewGenericValueObjectIterator creates a new GenericValueObjectIterator
func NewGenericValueObjectIterator(keys [][]byte, values []systemcontracts.GenericValue, exists []bool) (*GenericValueObjectIterator, error) {
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
	if err := systemcontracts.DecodeGenericValue(o, value); err != nil {
		return nil, err
	}
	return key, nil
}

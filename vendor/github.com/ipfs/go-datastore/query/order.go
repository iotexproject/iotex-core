package query

import (
	"bytes"
	"strings"
)

// Order is an object used to order objects
type Order interface {
	Compare(a, b Entry) int
}

// OrderByFunction orders the results based on the result of the given function.
type OrderByFunction func(a, b Entry) int

func (o OrderByFunction) Compare(a, b Entry) int {
	return o(a, b)
}

// OrderByValue is used to signal to datastores they should apply internal
// orderings.
type OrderByValue struct{}

func (o OrderByValue) Compare(a, b Entry) int {
	return bytes.Compare(a.Value, b.Value)
}

// OrderByValueDescending is used to signal to datastores they
// should apply internal orderings.
type OrderByValueDescending struct{}

func (o OrderByValueDescending) Compare(a, b Entry) int {
	return -bytes.Compare(a.Value, b.Value)
}

// OrderByKey
type OrderByKey struct{}

func (o OrderByKey) Compare(a, b Entry) int {
	return strings.Compare(a.Key, b.Key)
}

// OrderByKeyDescending
type OrderByKeyDescending struct{}

func (o OrderByKeyDescending) Compare(a, b Entry) int {
	return -strings.Compare(a.Key, b.Key)
}

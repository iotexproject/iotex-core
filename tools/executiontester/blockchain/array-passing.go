// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

type (
	// ArrayString interface for array-of-strings.sol
	ArrayPassing interface {
		Contract
		GetNum() (ret int64, err error)
	}

	arrayPassing struct {
		Contract
	}
)

// NewArrayPassing creates a new ArrayPassing contract
func NewArrayPassing(exp string) ArrayPassing {
	return &arrayPassing{Contract: NewContract(exp)}
}

// GetNum is calling function makeA() returns (uint256)
func (f *arrayPassing) GetNum() (ret int64, err error) {
	ret, err = f.RunAsOwner().SetAddress(f.Address()).ReadValue(f.Address(), ArrayPassingMakeA, Producer)
	return
}

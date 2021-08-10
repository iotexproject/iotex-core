// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"encoding/hex"
	"math/big"
)

type (
	// ArrayDeletePassing interface for array-delete.sol
	ArrayDeletePassing interface {
		Contract
		GetArray() (ret []*big.Int, err error)
		GetNum() (ret int64, err error)
	}

	arrayDeletePassing struct {
		Contract
	}
)

// NewArrayDelete creates a new ArrayDelete contract
func NewArrayDelete(exp string) ArrayDeletePassing {
	return &arrayDeletePassing{Contract: NewContract(exp)}
}

// MainFunc is function main() returns (uint[])
func (f *arrayDeletePassing) GetArray() (ret []*big.Int, err error) {
	retString, err := f.RunAsOwner().SetAddress(f.Address()).Read(ArrayDeletePassingGetArray, []byte(Producer))
	if err != nil {
		return
	}
	retBytes, err := hex.DecodeString(retString)
	if err != nil {
		return
	}
	len := len(retBytes) / 32
	for i := 2; i < len; i++ {
		b := retBytes[i*32 : (i+1)*32]
		retBig := new(big.Int).SetBytes(b)
		ret = append(ret, retBig)
	}
	return
}

// GetNum is calling function makeA() returns (uint256)
func (f *arrayDeletePassing) GetNum() (ret int64, err error) {
	ret, err = f.RunAsOwner().SetAddress(f.Address()).ReadValue(f.Address(), ArrayDeletePassingMakeA, Producer)
	return
}

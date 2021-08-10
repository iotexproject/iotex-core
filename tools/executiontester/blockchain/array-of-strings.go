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
	// ArrayString interface for array-of-strings.sol
	ArrayString interface {
		Contract
		GetString() (ret string, err error)
	}

	arrayString struct {
		Contract
	}
)

// NewArrayString creates a new ArrayString contract
func NewArrayString(exp string) ArrayString {
	return &arrayString{Contract: NewContract(exp)}
}

// GetString is calling function bar() constant returns(string)
func (f *arrayString) GetString() (ret string, err error) {
	rets, err := f.RunAsOwner().SetAddress(f.Address()).Read(ArrayStringBar, []byte(Producer))
	if err != nil {
		return
	}
	retBytes, err := hex.DecodeString(rets)
	if err != nil {
		return
	}
	len := len(retBytes) / 32
	lenOfString := new(big.Int).SetBytes(retBytes[1*32 : (1+1)*32])

	for i := 2; i < len; i++ {
		b := retBytes[i*32 : (i+1)*32]
		ret += string(b[:lenOfString.Uint64()])
	}
	return
}

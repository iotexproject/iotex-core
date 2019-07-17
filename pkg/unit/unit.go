// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package unit

import (
	"math/big"
)

const (
	// Rau is the smallest non-fungible token unit
	Rau int64 = 1
	// KRau is 1000 Rau
	KRau = Rau * 1000
	// MRau is 1000 KRau
	MRau = KRau * 1000
	// GRau is 1000 MRau
	GRau = MRau * 1000
	// Qev is 1000 GRau
	Qev = GRau * 1000
	// Jin is 1000 Qev
	Jin = Qev * 1000
	// Iotx is 1000 Jin, which should be fit into int64
	Iotx = Jin * 1000
)

// ConvertIotxToRau converts an Iotx to Rau
func ConvertIotxToRau(iotx int64) *big.Int {
	itx := big.NewInt(iotx)
	return itx.Mul(itx, big.NewInt(1e18))
}

// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trx

import (
	"bytes"
)

// TxInSorter is TxInput sorter that implements Sort interface
type TxInSorter []*TxInput

// TxOutSorter is TxOutput sorter that implements Sort interface
type TxOutSorter []*TxOutput

// Sort Interface implementation for *TxInput slice
func (sorter TxInSorter) Len() int {
	return len(sorter)
}

func (sorter TxInSorter) Swap(i, j int) {
	sorter[i], sorter[j] = sorter[j], sorter[i]
}

func (sorter TxInSorter) Less(i, j int) bool {
	if cmp := bytes.Compare(sorter[i].TxHash, sorter[j].TxHash); cmp != 0 {
		return cmp == -1
	}
	return sorter[i].OutIndex < sorter[j].OutIndex
}

// Sort Interface implementation for *TxOuput slice
func (sorter TxOutSorter) Len() int {
	return len(sorter)
}

func (sorter TxOutSorter) Swap(i, j int) {
	sorter[i], sorter[j] = sorter[j], sorter[i]
}

func (sorter TxOutSorter) Less(i, j int) bool {
	if sorter[i].Value != sorter[j].Value {
		return sorter[i].Value < sorter[j].Value
	}
	return bytes.Compare(sorter[i].LockScript, sorter[j].LockScript) == -1
}

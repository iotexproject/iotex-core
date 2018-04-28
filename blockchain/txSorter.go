// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
)

type txInSorter []*TxInput
type txOutSorter []*TxOutput

// Sort Interface implementation for *TxInput slice
func (sorter txInSorter) Len() int {
	return len(sorter)
}

func (sorter txInSorter) Swap(i, j int) {
	sorter[i], sorter[j] = sorter[j], sorter[i]
}

func (sorter txInSorter) Less(i, j int) bool {
	if cmp := bytes.Compare(sorter[i].TxHash, sorter[j].TxHash); cmp != 0 {
		return cmp == -1
	}
	return sorter[i].OutIndex < sorter[j].OutIndex
}

// Sort Interface implementation for *TxOuput slice
func (sorter txOutSorter) Len() int {
	return len(sorter)
}

func (sorter txOutSorter) Swap(i, j int) {
	sorter[i], sorter[j] = sorter[j], sorter[i]
}

func (sorter txOutSorter) Less(i, j int) bool {
	if sorter[i].Value != sorter[j].Value {
		return sorter[i].Value < sorter[j].Value
	}
	return bytes.Compare(sorter[i].LockScript, sorter[j].LockScript) == -1
}

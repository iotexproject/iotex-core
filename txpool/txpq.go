// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txpool

type txDescPriorityQueue []*TxDesc

func (pq txDescPriorityQueue) Len() int {
	return len(pq)
}

func (pq txDescPriorityQueue) Less(i, j int) bool {
	return pq[i].Priority > pq[j].Priority
}

func (pq txDescPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].idx = i
	pq[j].idx = j
}

func (pq *txDescPriorityQueue) Push(x interface{}) {
	n := len(*pq)
	txDesc := x.(*TxDesc)
	txDesc.idx = n
	*pq = append(*pq, txDesc)
}

func (pq *txDescPriorityQueue) Pop() interface{} {
	n := len(*pq)
	old := *pq
	txDesc := old[n-1]
	txDesc.idx = -1
	*pq = old[0 : n-1]
	return txDesc
}

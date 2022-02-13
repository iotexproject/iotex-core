package evm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
)

type (
	// test vector for Contract
	set struct {
		k     hash.Hash256
		v     []byte
		cause error
	}
	cntrTest struct {
		contract Contract
		codes    []code
		states   []set
	}
	// test vector for StateDBAdapter
	bal struct {
		addr common.Address
		v    *big.Int
	}
	code struct {
		addr common.Address
		v    []byte
	}
	evmSet struct {
		addr common.Address
		k    common.Hash
		v    common.Hash
	}
	sui struct {
		addr    common.Address
		suicide bool
		exist   bool
	}
	image struct {
		hash common.Hash
		v    []byte
	}
	stateDBTest struct {
		balance            []bal
		codes              []code
		states             []evmSet
		suicide            []sui
		preimage           []image
		logs               []*types.Log
		txLogs             []*action.TransactionLog
		logSize, txLogSize int
		logAddr, txLogAddr string
	}
)

var (
	bytecode = []byte("test contract creation")

	addr1 = common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	c1    = common.HexToAddress("01fc246633470cf62ae2a956d21e8d481c3a69e1")
	c2    = common.HexToAddress("3470cf62ae2a956d38d481c3a69e121e01fc2466")
	c3    = common.HexToAddress("956d21e8d481c3a6901fc246633470cf62ae2ae1")
	C4    = common.HexToAddress("121e01fc24663470cf62ae2a956d38d481c3a69e")

	k1b = hash.Hash256b([]byte("cat"))
	v1b = hash.Hash256b([]byte("cat"))
	k2b = hash.Hash256b([]byte("dog"))
	v2b = hash.Hash256b([]byte("dog"))
	k3b = hash.Hash256b([]byte("hen"))
	v3b = hash.Hash256b([]byte("hen"))
	k4b = hash.Hash256b([]byte("fox"))
	v4b = hash.Hash256b([]byte("fox"))
	k1  = common.BytesToHash(k1b[:])
	v1  = common.BytesToHash(v1b[:])
	k2  = common.BytesToHash(k2b[:])
	v2  = common.BytesToHash(v2b[:])
	k3  = common.BytesToHash(k3b[:])
	v3  = common.BytesToHash(v3b[:])
	k4  = common.BytesToHash(k4b[:])
	v4  = common.BytesToHash(v4b[:])
)

func newTestLog(addr common.Address) *types.Log {
	return &types.Log{
		Address: addr,
		Topics:  []common.Hash{k1},
	}
}

func newTestTxLog(addr common.Address) *action.TransactionLog {
	a, _ := address.FromBytes(addr.Bytes())
	return &action.TransactionLog{
		Sender: a.String(),
	}
}

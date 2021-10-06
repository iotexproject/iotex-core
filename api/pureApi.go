package api

import (
	"errors"
	"strconv"

	"github.com/iotexproject/iotex-core/config"
)

var (
	apiMap = map[string]func(*Server, interface{}) (interface{}, error){
		"eth_gasPrice":            gasPrice,
		"eth_chainId":             getChainId,
		"eth_blockNumber":         getBlockNumber,
		"eth_getBalance":          getBalance,
		"eth_getTransactionCount": getTransactionCount,
	}
)

func gasPrice(svr *Server, in interface{}) (interface{}, error) {
	val, err := svr.suggestGasPrice()
	if err != nil {
		return nil, err
	}
	return uint64ToHex(val), nil
}

func getChainId(svr *Server, in interface{}) (interface{}, error) {
	id := config.EVMNetworkID()
	return uint64ToHex(uint64(id)), nil
}

func getBlockNumber(svr *Server, in interface{}) (interface{}, error) {
	return uint64ToHex(svr.bc.TipHeight()), nil
}

func uint64ToHex(val uint64) string {
	return "0x" + strconv.FormatUint(val, 16)
}

// TODO:
func getBlockByNumber(svr *Server, in interface{}) (interface{}, error) {
	return nil, nil
}

func getBalance(svr *Server, in interface{}) (interface{}, error) {
	addr, ok := in.(string)
	if !ok {
		return nil, errors.New("wrong type of params")
	}
	accountMeta, _, err := svr.getAccount(addr)
	if err != nil {
		return nil, err
	}
	return accountMeta.Balance, nil
}

func getTransactionCount(svr *Server, in interface{}) (interface{}, error) {
	addr, ok := in.(string)
	if !ok {
		return nil, errors.New("wrong type of params")
	}
	accountMeta, _, err := svr.getAccount(addr)
	if err != nil {
		return nil, err
	}
	return uint64ToHex(accountMeta.PendingNonce), nil
}

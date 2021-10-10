package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

type (
	CallObject struct {
		From     string `json:"from,omitempty"`
		To       string `json:"to,omitempty"`
		Gas      string `json:"gas,omitempty"`
		GasPrice string `json:"gasPrice,omitempty"`
		Value    string `json:"value,omitempty"`
		Data     string `json:"data,omitempty"`
	}

	LogsRequest struct {
		Address   string   `json:"address,omitempty"`
		FromBlock string   `json:"fromBlock,omitempty"`
		ToBlock   string   `json:"toBlock,omitempty"`
		Topics    []string `json:"topics,omitempty"`
		Blockhash string   `json:"blockhash,omitempty"`
	}

	LogsObject struct {
		Removed          bool     `json:"removed,omitempty"`
		LogIndex         string   `json:"logIndex,omitempty"`
		TransactionIndex string   `json:"transactionIndex,omitempty"`
		TransactionHash  string   `json:"transactionHash,omitempty"`
		BlockHash        string   `json:"blockHash,omitempty"`
		BlockNumber      string   `json:"blockNumber,omitempty"`
		Address          string   `json:"address,omitempty"`
		Data             string   `json:"data,omitempty"`
		Topics           []string `json:"topics,omitempty"`
	}
)

var (
	apiMap = map[string]func(*Server, interface{}) (interface{}, error){
		"eth_gasPrice":                       gasPrice,
		"eth_getBlockByHash":                 getBlockByHash,
		"eth_chainId":                        getChainId,
		"eth_blockNumber":                    getBlockNumber,
		"eth_getBalance":                     getBalance,
		"eth_getTransactionCount":            getTransactionCount,
		"eth_call":                           call,
		"eth_getCode":                        getCode,
		"eth_protocolVersion":                getProtocolVersion,
		"web3_clientVersion":                 getNodeInfo,
		"net_version":                        getNetworkId,
		"net_peerCount":                      getPeerCount,
		"net_listening":                      isListening,
		"eth_syncing":                        isSyncing,
		"eth_mining":                         isMining,
		"eth_hashrate":                       getHashrate,
		"eth_getLogs":                        getLogs,
		"eth_getBlockTransactionCountByHash": getBlockTransactionCountByHash,
		"eth_getBlockByNumber":               getBlockByNumber,
		// func not implemented
		"eth_coinbase":                      unimplemented,
		"eth_accounts":                      unimplemented,
		"eth_getStorageAt":                  unimplemented,
		"eth_getUncleCountByBlockHash":      unimplemented,
		"eth_getUncleCountByBlockNumber":    unimplemented,
		"eth_sign":                          unimplemented,
		"eth_signTransaction":               unimplemented,
		"eth_sendTransaction":               unimplemented,
		"eth_getUncleByBlockHashAndIndex":   unimplemented,
		"eth_getUncleByBlockNumberAndIndex": unimplemented,
	}

	ErrUnkownType = errors.New("wrong type of params")
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

func getBlockByNumber(svr *Server, in interface{}) (interface{}, error) {
	blkNum, err := getStringFromArray(in)
	if err != nil {
		return nil, err
	}
	num, err := parseBlockNumber(svr, blkNum)
	if err != nil {
		return nil, err
	}
	isDetailed := in.([]interface{})[1].(bool)

	blkMeta, err := svr.getBlockMetas(num, 1)
	if err != nil {
		return nil, err
	}
	blk, err := getBlockWithTransactions(svr, blkMeta.BlkMetas[0], isDetailed)
	if err != nil {
		return nil, err
	}
	return *blk, nil
}

func getBalance(svr *Server, in interface{}) (interface{}, error) {
	addr, ok := in.(string)
	if !ok {
		return nil, ErrUnkownType
	}
	ioAddr, err := ethAddrToIoAddr(addr)
	if err != nil {
		return nil, err
	}
	accountMeta, _, err := svr.getAccount(ioAddr)
	if err != nil {
		return nil, err
	}

	return intStrToHex(accountMeta.Balance)
}

func getTransactionCount(svr *Server, in interface{}) (interface{}, error) {
	addr, ok := in.(string)
	if !ok {
		return nil, ErrUnkownType
	}
	ioAddr, err := ethAddrToIoAddr(addr)
	if err != nil {
		return nil, err
	}
	accountMeta, _, err := svr.getAccount(ioAddr)
	if err != nil {
		return nil, err
	}
	return uint64ToHex(accountMeta.PendingNonce), nil
}

func call(svr *Server, in interface{}) (interface{}, error) {
	jsonMarshaled, err := getJSONFromArray(in)
	if err != nil {
		return nil, err
	}
	var callObject CallObject
	err = json.Unmarshal(jsonMarshaled, &callObject)
	if err != nil {
		return nil, err
	}

	// token call
	if callObject.To == "0xb1f8e55c7f64d203c1400b9d8555d050f94adf39" {
		return nil, nil
	}

	from, err := ethAddrToIoAddr(callObject.From)
	to, err := ethAddrToIoAddr(callObject.To)
	value, err := hexStringToNumber(callObject.Value)
	gasLimit, err := hexStringToNumber(callObject.Gas)
	if err != nil {
		return nil, ErrUnkownType
	}

	ret, _, err := svr.readContract(from,
		to,
		big.NewInt(int64(value)),
		gasLimit,
		common.FromHex(callObject.Data))
	if err != nil {
		return nil, err
	}
	return "0x" + hex.EncodeToString(ret), nil
}

// TODO:
func estimateGas(svr *Server, in interface{}) (interface{}, error) {
	return nil, nil
}

func getCode(svr *Server, in interface{}) (interface{}, error) {
	addr, err := getStringFromArray(in)
	if err != nil {
		return nil, err
	}

	ioAddr, err := ethAddrToIoAddr(addr)
	if err != nil {
		return nil, err
	}
	accountMeta, _, err := svr.getAccount(ioAddr)
	if err != nil {
		return nil, err
	}
	return "0x" + hex.EncodeToString(accountMeta.ContractByteCode), nil
}

func getNodeInfo(svr *Server, in interface{}) (interface{}, error) {
	ret, _ := svr.GetServerMeta(context.Background(), nil)
	return ret.ServerMeta.PackageVersion + "/" + ret.ServerMeta.GoVersion, nil
}

func getNetworkId(svr *Server, in interface{}) (interface{}, error) {
	return config.EVMNetworkID(), nil
}

func getPeerCount(svr *Server, in interface{}) (interface{}, error) {
	return "0x64", nil
}

func isListening(svr *Server, in interface{}) (interface{}, error) {
	return true, nil
}

func getProtocolVersion(svr *Server, in interface{}) (interface{}, error) {
	return "64", nil
}

func isSyncing(svr *Server, in interface{}) (interface{}, error) {
	return false, nil
}

func isMining(svr *Server, in interface{}) (interface{}, error) {
	return false, nil
}

func getHashrate(svr *Server, in interface{}) (interface{}, error) {
	return "0x500000", nil
}

func getBlockTransactionCountByHash(svr *Server, in interface{}) (interface{}, error) {
	h, err := getStringFromArray(in)
	if err != nil {
		return nil, err
	}
	ret, err := svr.getBlockMetaByHash(removeHexPrefix(h))
	if err != nil {
		return nil, err
	}
	return uint64ToHex(uint64(ret[0].NumActions)), nil
}

func getBlockByHash(svr *Server, in interface{}) (interface{}, error) {
	h, err := getStringFromArray(in)
	if err != nil {
		return nil, err
	}
	isDetailed := in.([]interface{})[1].(bool)
	blkMeta, err := svr.getBlockMetaByHash(removeHexPrefix(h))
	if err != nil {
		return nil, err
	}

	blk, err := getBlockWithTransactions(svr, blkMeta[0], isDetailed)
	if err != nil {
		return nil, err
	}
	return *blk, nil
}

// func getTransactionByHash(svr *Server, in interface{}) (interface{}, error) {
// 	h, err := getStringFromArray(in)
// 	if err != nil {
// 		return nil, err
// 	}
// 	ret, err := svr.getSingleAction(h, true)
// 	// TODO:
// 	return nil, nil
// }

func getLogs(svr *Server, in interface{}) (interface{}, error) {
	req, err := getJSONFromArray(in)
	if err != nil {
		return nil, err
	}
	var logReq LogsRequest
	err = json.Unmarshal(req, &logReq)
	if err != nil {
		return nil, err
	}

	// construct block range(from, to)
	tipHeight := svr.bc.TipHeight()
	from, err := parseBlockNumber(svr, logReq.FromBlock)
	to, err := parseBlockNumber(svr, logReq.ToBlock)
	if err != nil {
		return nil, err
	}

	if from > tipHeight {
		return nil, status.Error(codes.InvalidArgument, "start block > tip height")
	}
	if to > tipHeight {
		to = tipHeight
	}

	// construct filter topics and addresses
	var filter iotexapi.LogsFilter
	if len(logReq.Address) > 0 {
		addr, err := ethAddrToIoAddr(logReq.Address)
		if err != nil {
			return nil, err
		}
		filter.Address = append(filter.Address, addr)
	}
	for _, val := range logReq.Topics {
		b, err := hex.DecodeString(val)
		if err != nil {
			return nil, err
		}
		filter.Topics = append(filter.Topics, &iotexapi.Topics{
			Topic: [][]byte{b},
		})
	}

	logs, err := svr.getLogsInRange(logfilter.NewLogFilter(&filter, nil, nil), from, to, 1000)
	if err != nil {
		return nil, err
	}
	// parse log results
	var ret []LogsObject
	for _, l := range logs {
		if len(l.Topics) > 0 {
			addr, _ := ioAddrToEthAddr(l.ContractAddress)
			var topics []string
			for _, val := range l.Topics {
				topics = append(topics, "0x"+hex.EncodeToString(val))
			}
			ret = append(ret, LogsObject{
				BlockHash:        "0x" + hex.EncodeToString(l.BlkHash),
				TransactionHash:  "0x" + hex.EncodeToString(l.ActHash),
				LogIndex:         uint64ToHex(uint64(l.Index)),
				BlockNumber:      uint64ToHex(l.BlkHeight),
				TransactionIndex: "0x1",
				Address:          addr,
				Data:             "0x" + hex.EncodeToString(l.Data),
				Topics:           topics,
			})
		}
	}
	return ret, nil
}

func unimplemented(svr *Server, in interface{}) (interface{}, error) {
	return nil, status.Error(codes.Unimplemented, "function not implemented")
}
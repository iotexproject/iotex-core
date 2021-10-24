package api

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/wunderlist/ttlcache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

type (
	logsRequest struct {
		Address   []string   `json:"address,omitempty"`
		FromBlock string     `json:"fromBlock,omitempty"`
		ToBlock   string     `json:"toBlock,omitempty"`
		Topics    [][]string `json:"topics,omitempty"`
	}

	logsObject struct {
		Removed          bool     `json:"removed"`
		LogIndex         string   `json:"logIndex"`
		TransactionIndex string   `json:"transactionIndex"`
		TransactionHash  string   `json:"transactionHash"`
		BlockHash        string   `json:"blockHash"`
		BlockNumber      string   `json:"blockNumber"`
		Address          string   `json:"address"`
		Data             string   `json:"data"`
		Topics           []string `json:"topics"`
	}

	receiptObject struct {
		TransactionIndex  string       `json:"transactionIndex"`
		TransactionHash   string       `json:"transactionHash"`
		BlockHash         string       `json:"blockHash"`
		BlockNumber       string       `json:"blockNumber"`
		From              string       `json:"from"`
		To                *string      `json:"to,omitempty"`
		CumulativeGasUsed string       `json:"cumulativeGasUsed"`
		GasUsed           string       `json:"gasUsed"`
		ContractAddress   *string      `json:"contractAddress,omitempty"`
		LogsBloom         string       `json:"logsBloom"`
		Logs              []logsObject `json:"logs"`
		Status            string       `json:"status"`
	}

	filterObject struct {
		LogHeight  uint64     `json:"logHeight"`
		FilterType string     `json:"filterType"`
		FromBlock  string     `json:"fromBlock,omitempty"`
		ToBlock    string     `json:"toBlock,omitempty"`
		Address    []string   `json:"address,omitempty"`
		Topics     [][]string `json:"topics,omitempty"`
	}
)

var (
	apiMap = map[string]func(*Server, interface{}) (interface{}, error){
		"eth_gasPrice":                            gasPrice,
		"eth_getBlockByHash":                      getBlockByHash,
		"eth_chainId":                             getChainID,
		"eth_blockNumber":                         getBlockNumber,
		"eth_getBalance":                          getBalance,
		"eth_getTransactionCount":                 getTransactionCount,
		"eth_call":                                call,
		"eth_getCode":                             getCode,
		"eth_protocolVersion":                     getProtocolVersion,
		"web3_clientVersion":                      getNodeInfo,
		"net_version":                             getNetworkId,
		"net_peerCount":                           getPeerCount,
		"net_listening":                           isListening,
		"eth_syncing":                             isSyncing,
		"eth_mining":                              isMining,
		"eth_hashrate":                            getHashrate,
		"eth_getLogs":                             getLogs,
		"eth_getBlockTransactionCountByHash":      getBlockTransactionCountByHash,
		"eth_getBlockByNumber":                    getBlockByNumber,
		"eth_estimateGas":                         estimateGas,
		"eth_sendRawTransaction":                  sendRawTransaction,
		"eth_getTransactionByHash":                getTransactionByHash,
		"eth_getTransactionByBlockNumberAndIndex": getTransactionByBlockNumberAndIndex,
		"eth_getTransactionByBlockHashAndIndex":   getTransactionByBlockHashAndIndex,
		"eth_getBlockTransactionCountByNumber":    getBlockTransactionCountByNumber,
		"eth_getTransactionReceipt":               getTransactionReceipt,
		"eth_getFilterLogs":                       getFilterLogs,
		"eth_getFilterChanges":                    getFilterChanges,
		"eth_uninstallFilter":                     uninstallFilter,
		"eth_newFilter":                           newFilter,
		"eth_newBlockFilter":                      newBlockFilter,
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
		"eth_pendingTransactions":           unimplemented,
	}

	errUnkownType = errors.New("wrong type of params")

	filterCache = ttlcache.NewCache(15 * time.Minute)
)

func gasPrice(svr *Server, in interface{}) (interface{}, error) {
	val, err := svr.suggestGasPrice()
	if err != nil {
		return nil, err
	}
	return uint64ToHex(val), nil
}

func getChainID(svr *Server, in interface{}) (interface{}, error) {
	id := config.EVMNetworkID()
	return uint64ToHex(uint64(id)), nil
}

func getBlockNumber(svr *Server, in interface{}) (interface{}, error) {
	return uint64ToHex(svr.bc.TipHeight()), nil
}

func getBlockByNumber(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	blkNum := web3Util.getStringFromArray(in, 0)
	num := web3Util.parseBlockNumber(svr, blkNum)
	isDetailed := web3Util.getBoolFromArray(in, 1)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}

	blkMeta, err := svr.getBlockMetas(num, 1)
	if err != nil || blkMeta == nil || len(blkMeta.BlkMetas) == 0 {
		if err == nil {
			err = errors.New("invalid block")
		}
		return nil, err
	}
	blk := web3Util.getBlockWithTransactions(svr, blkMeta.BlkMetas[0], isDetailed)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	return *blk, nil
}

func getBalance(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	addr := web3Util.getStringFromArray(in, 0)
	ioAddr := web3Util.ethAddrToIoAddr(addr)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	accountMeta, _, err := svr.getAccount(ioAddr)
	if err != nil {
		return nil, err
	}

	return web3Util.intStrToHex(accountMeta.Balance), web3Util.errMsg
}

func getTransactionCount(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	addr := web3Util.getStringFromArray(in, 0)
	ioAddr := web3Util.ethAddrToIoAddr(addr)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	accountMeta, _, err := svr.getAccount(ioAddr)
	if err != nil {
		return nil, err
	}
	return uint64ToHex(accountMeta.PendingNonce), nil
}

func call(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	jsonMarshaled := web3Util.getJSONFromArray(in)
	from, to, gasLimit, value, data := web3Util.parseCallObject(jsonMarshaled)
	if to == "io1k8uw2hrlvnfq8s2qpwwc24ws2ru54heenx8chr" {
		return nil, nil
	}
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	ret, _, err := svr.readContract(from, to, value, gasLimit, data)
	if err != nil {
		return nil, err
	}
	return "0x" + hex.EncodeToString(ret), nil
}

func estimateGas(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	jsonMarshaled := web3Util.getJSONFromArray(in)
	from, to, _, value, data := web3Util.parseCallObject(jsonMarshaled)

	var estimatedGas uint64
	isContract := web3Util.isContractAddr(svr, to)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	if !isContract {
		// transfer
		estimatedGas = uint64(len(data))*action.TransferPayloadGas + action.TransferBaseIntrinsicGas
	} else {
		// execution
		ret, err := svr.estimateActionGasConsumptionForExecution(&iotextypes.Execution{
			Amount:   value.String(),
			Contract: to,
			Data:     data,
		}, from)
		if err != nil {
			return nil, err
		}
		estimatedGas = ret.Gas
	}
	if estimatedGas < 21000 {
		estimatedGas = 21000
	}
	return uint64ToHex(estimatedGas), nil
}

func sendRawTransaction(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	// parse raw data string from json request
	dataInString := web3Util.getStringFromArray(in, 0)
	tx, sig, pubkey := web3Util.DecodeRawTx(dataInString, svr.bc.ChainID())
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}

	// TODO: new a proto
	req := &iotexapi.SendActionRequest{
		Action: &iotextypes.Action{
			Core: &iotextypes.ActionCore{
				Version:  0,
				Nonce:    tx.Nonce(),
				GasLimit: tx.Gas(),
				GasPrice: tx.GasPrice().String(),
				ChainID:  0,
			},
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     iotextypes.Encoding_ETHEREUM_RLP,
		},
	}

	ioAddr, _ := address.FromBytes(tx.To().Bytes())
	to := ioAddr.String()

	// TODO: process special staking action

	isContract := web3Util.isContractAddr(svr, to)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}

	if isContract {
		// transfer
		req.Action.Core.Action = &iotextypes.ActionCore_Transfer{
			Transfer: &iotextypes.Transfer{
				Recipient: to,
				Payload:   tx.Data(),
				Amount:    tx.Value().String(),
			},
		}
	} else {
		// execution
		req.Action.Core.Action = &iotextypes.ActionCore_Execution{
			Execution: &iotextypes.Execution{
				Contract: to,
				Data:     tx.Data(),
				Amount:   tx.Value().String(),
			},
		}
	}

	ret, err := svr.SendAction(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return "0x" + ret.ActionHash, nil
}

func getCode(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	addr := web3Util.getStringFromArray(in, 0)
	ioAddr := web3Util.ethAddrToIoAddr(addr)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
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
	var web3Util web3Utils
	h := web3Util.getStringFromArray(in, 0)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	ret, err := svr.getBlockMetaByHash(removeHexPrefix(h))
	if err != nil {
		return nil, err
	}
	return uint64ToHex(uint64(ret[0].NumActions)), nil
}

func getBlockByHash(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	h := web3Util.getStringFromArray(in, 0)
	isDetailed := web3Util.getBoolFromArray(in, 1)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	blkMeta, err := svr.getBlockMetaByHash(removeHexPrefix(h))
	if err != nil {
		return nil, err
	}

	blk := web3Util.getBlockWithTransactions(svr, blkMeta[0], isDetailed)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	return *blk, nil
}

func getTransactionByHash(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	h := web3Util.getStringFromArray(in, 0)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	ret, err := svr.getSingleAction(removeHexPrefix(h), true)
	if err != nil {
		return nil, err
	}

	tx := web3Util.getTransactionCreateFromActionInfo(svr, ret.ActionInfo[0], svr.bc.ChainID())
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	return tx, nil
}

func getLogs(svr *Server, in interface{}) (interface{}, error) {
	logReq, err := parseLogRequest(in)
	if err != nil {
		return nil, err
	}

	return getLogsWithFilter(svr, logReq.FromBlock, logReq.ToBlock, logReq.Address, logReq.Topics)
}

func getTransactionReceipt(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	h := web3Util.getStringFromArray(in, 0)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	ret, err := svr.GetReceiptByAction(context.Background(),
		&iotexapi.GetReceiptByActionRequest{
			ActionHash: removeHexPrefix(h),
		},
	)
	if err != nil {
		return nil, err
	}

	receipt := ret.ReceiptInfo.Receipt

	var contractAddr *string
	if len(receipt.ContractAddress) > 0 {
		res := web3Util.ioAddrToEthAddr(receipt.ContractAddress)
		contractAddr = &res
	}

	act, err := svr.getSingleAction(removeHexPrefix(h), true)
	if err != nil {
		return nil, err
	}
	tx := web3Util.getTransactionFromActionInfo(act.ActionInfo[0], svr.bc.ChainID())
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	var logs []logsObject
	for _, v := range receipt.Logs {
		var topics []string
		for _, tp := range v.Topics {
			topics = append(topics, "0x"+hex.EncodeToString(tp))
		}
		log := logsObject{
			BlockHash:        "0x" + ret.ReceiptInfo.BlkHash,
			TransactionHash:  h,
			TransactionIndex: tx.TransactionIndex,
			LogIndex:         uint64ToHex(uint64(v.Index)),
			BlockNumber:      uint64ToHex(v.BlkHeight),
			Address:          web3Util.ioAddrToEthAddr(v.ContractAddress),
			Data:             "0x" + hex.EncodeToString(v.Data),
			Topics:           topics,
		}
		logs = append(logs, log)
	}
	return receiptObject{
		BlockHash:         "0x" + ret.ReceiptInfo.BlkHash,
		BlockNumber:       uint64ToHex(receipt.BlkHeight),
		ContractAddress:   contractAddr,
		CumulativeGasUsed: uint64ToHex(receipt.GasConsumed),
		From:              tx.From,
		GasUsed:           uint64ToHex(receipt.GasConsumed),
		LogsBloom:         "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		Status:            uint64ToHex(receipt.Status),
		To:                tx.To,
		TransactionHash:   h,
		TransactionIndex:  tx.TransactionIndex,
		Logs:              logs,
	}, nil

}

func getBlockTransactionCountByNumber(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	blkNum := web3Util.getStringFromArray(in, 0)
	num := web3Util.parseBlockNumber(svr, blkNum)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	blkMeta, err := svr.getBlockMetas(num, 1)
	if err != nil {
		return nil, err
	}
	return blkMeta.BlkMetas[0].NumActions, nil
}

func getTransactionByBlockHashAndIndex(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	h := web3Util.getStringFromArray(in, 0)
	idxStr := web3Util.getStringFromArray(in, 1)
	idx := web3Util.hexStringToNumber(idxStr)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	ret, err := svr.getActionsByBlock(removeHexPrefix(h), idx, 1)
	if err != nil {
		return nil, err
	}
	tx := web3Util.getTransactionCreateFromActionInfo(svr, ret.ActionInfo[0], svr.bc.ChainID())
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	return tx, nil
}

func getTransactionByBlockNumberAndIndex(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	blkNum := web3Util.getStringFromArray(in, 0)
	idxStr := web3Util.getStringFromArray(in, 1)
	num := web3Util.parseBlockNumber(svr, blkNum)
	idx := web3Util.hexStringToNumber(idxStr)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	blkMeta, err := svr.getBlockMetas(num, 1)
	if err != nil {
		return nil, err
	}
	ret, err := svr.getActionsByBlock(blkMeta.BlkMetas[0].Hash, idx, 1)
	if err != nil {
		return nil, err
	}
	tx := web3Util.getTransactionCreateFromActionInfo(svr, ret.ActionInfo[0], svr.bc.ChainID())
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	return tx, nil
}

func newFilter(svr *Server, in interface{}) (interface{}, error) {
	logReq, err := parseLogRequest(in)
	if err != nil {
		return nil, err
	}
	filterObj := filterObject{
		FilterType: "log",
		FromBlock:  logReq.FromBlock,
		ToBlock:    logReq.ToBlock,
		Address:    logReq.Address,
		Topics:     logReq.Topics,
	}

	// sha1 hash of data
	objInByte, _ := json.Marshal(filterObj)
	h := sha1.New()
	h.Write(objInByte)
	filterID := "0x" + hex.EncodeToString(h.Sum(nil))

	filterCache.Set(filterID, string(objInByte))
	return filterID, nil
}

func newBlockFilter(svr *Server, in interface{}) (interface{}, error) {
	var filterObj filterObject
	filterObj.FilterType = "block"
	filterObj.LogHeight = svr.bc.TipHeight()

	// sha1 hash of data
	objInByte, _ := json.Marshal(filterObj)
	h := sha1.New()
	h.Write(objInByte)
	filterID := "0x" + hex.EncodeToString(h.Sum(nil))

	filterCache.Set(filterID, string(objInByte))
	return filterID, nil
}

func uninstallFilter(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	_ = web3Util.getStringFromArray(in, 0)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	// TODO: res := filterCache.Delete(id)
	res := true
	return res, nil
}

func getFilterChanges(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	filterID := web3Util.getStringFromArray(in, 0)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}

	data, isFound := filterCache.Get(filterID)
	if !isFound {
		return []interface{}{}, nil
	}
	var filterObj filterObject
	if err := json.Unmarshal([]byte(data), &filterObj); err != nil {
		return nil, err
	}
	if filterObj.LogHeight >= svr.bc.TipHeight() {
		return []interface{}{}, nil
	}

	var ret interface{}
	newLogHeight := svr.bc.TipHeight()
	switch filterObj.FilterType {
	case "log":
		logs, err := getLogsWithFilter(svr, filterObj.FromBlock, filterObj.ToBlock, filterObj.Address, filterObj.Topics)
		if err != nil {
			return nil, err
		}
		ret = logs
	case "block":
		blkMeta, err := svr.getBlockMetas(filterObj.LogHeight, svr.cfg.API.RangeQueryLimit)
		if err != nil {
			return nil, err
		}

		var hashArr []string
		for _, v := range blkMeta.BlkMetas {
			hashArr = append(hashArr, "0x"+v.Hash)
		}
		if filterObj.LogHeight+1000 > newLogHeight {
			newLogHeight = filterObj.LogHeight + 1000
		}
		ret = hashArr
	default:
		return nil, errUnkownType
	}

	filterObj.LogHeight = newLogHeight
	objInByte, _ := json.Marshal(filterObj)
	filterCache.Set(filterID, string(objInByte))
	return ret, nil
}

func getFilterLogs(svr *Server, in interface{}) (interface{}, error) {
	// get filter from cache by id
	var web3Util web3Utils
	filterID := web3Util.getStringFromArray(in, 0)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	data, isFound := filterCache.Get(filterID)
	if !isFound {
		return []interface{}{}, nil
	}
	var filterObj filterObject
	if err := json.Unmarshal([]byte(data), &filterObj); err != nil {
		return nil, err
	}
	if filterObj.FilterType != "log" {
		return nil, status.Error(codes.Internal, "filter not found")
	}
	return getLogsWithFilter(svr, filterObj.FromBlock, filterObj.ToBlock, filterObj.Address, filterObj.Topics)
}

func unimplemented(svr *Server, in interface{}) (interface{}, error) {
	return nil, status.Error(codes.Unimplemented, "method not implemented")
}

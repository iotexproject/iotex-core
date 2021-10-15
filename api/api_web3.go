package api

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
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
	callObject struct {
		From     string `json:"from,omitempty"`
		To       string `json:"to"`
		Gas      string `json:"gas,omitempty"`
		GasPrice string `json:"gasPrice,omitempty"`
		Value    string `json:"value,omitempty"`
		Data     string `json:"data,omitempty"`
	}

	logsRequest struct {
		Address   []string   `json:"address,omitempty"`
		FromBlock string     `json:"fromBlock,omitempty"`
		ToBlock   string     `json:"toBlock,omitempty"`
		Topics    [][]string `json:"topics,omitempty"`
	}

	logsObject struct {
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

	receiptObject struct {
		TransactionIndex  string       `json:"transactionIndex,omitempty"`
		TransactionHash   string       `json:"transactionHash,omitempty"`
		BlockHash         string       `json:"blockHash,omitempty"`
		BlockNumber       string       `json:"blockNumber,omitempty"`
		From              string       `json:"from,omitempty"`
		To                string       `json:"to,omitempty"`
		CumulativeGasUsed string       `json:"cumulativeGasUsed,omitempty"`
		GasUsed           string       `json:"gasUsed,omitempty"`
		ContractAddress   *string      `json:"contractAddress,omitempty"`
		LogsBloom         string       `json:"logsBloom,omitempty"`
		Logs              []logsObject `json:"logs,omitempty"`
		Status            string       `json:"status,omitempty"`
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
	blkNum, err := getStringFromArray(in, 0)
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
		return nil, errUnkownType
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
		return nil, errUnkownType
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
	var callObj callObject
	err = json.Unmarshal(jsonMarshaled, &callObj)
	if err != nil {
		return nil, err
	}

	// token call
	if callObj.To == "0xb1f8e55c7f64d203c1400b9d8555d050f94adf39" {
		return nil, nil
	}

	from, err := ethAddrToIoAddr(callObj.From)
	to, err := ethAddrToIoAddr(callObj.To)
	value, err := hexStringToNumber(callObj.Value)
	gasLimit, err := hexStringToNumber(callObj.Gas)
	if err != nil {
		return nil, errUnkownType
	}

	ret, _, err := svr.readContract(from,
		to,
		big.NewInt(int64(value)),
		gasLimit,
		common.FromHex(callObj.Data))
	if err != nil {
		return nil, err
	}
	return "0x" + hex.EncodeToString(ret), nil
}

func estimateGas(svr *Server, in interface{}) (interface{}, error) {
	jsonMarshaled, err := getJSONFromArray(in)
	if err != nil {
		return nil, err
	}
	var callObj callObject
	err = json.Unmarshal(jsonMarshaled, &callObj)
	if err != nil {
		return nil, err
	}

	to, err := ethAddrToIoAddr(callObj.To)
	from := "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7"
	if len(callObj.From) > 0 {
		from, err = ethAddrToIoAddr(callObj.From)
	}
	data := common.FromHex(callObj.Data)
	value, err := hexStringToNumber(callObj.Value)
	if err != nil {
		return nil, err
	}

	var estimatedGas uint64
	if isContractAddr(svr, to) {
		// transfer
		estimatedGas = uint64(len(data))*action.TransferPayloadGas + action.TransferBaseIntrinsicGas
	} else {
		// execution
		ret, err := svr.estimateActionGasConsumptionForExecution(&iotextypes.Execution{
			Amount:   strconv.FormatUint(value, 10),
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
	return estimatedGas, nil
}

func sendRawTransaction(svr *Server, in interface{}) (interface{}, error) {
	// parse raw data string from json request
	dataInString, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}

	tx, sig, pubkey, err := DecodeRawTx(dataInString, svr.bc.ChainID())
	if err != nil {
		return nil, err
	}

	req := &iotexapi.SendActionRequest{
		Action: &iotextypes.Action{
			Core: &iotextypes.ActionCore{
				Version:  0,
				Nonce:    tx.Nonce(),
				GasLimit: tx.Gas(),
				GasPrice: tx.GasPrice().String(),
				ChainID:  0,
			},
			SenderPubKey: pubkey,
			Signature:    sig,
			Encoding:     iotextypes.Encoding_ETHEREUM_RLP,
		},
	}

	ioAddr, _ := address.FromBytes(tx.To().Bytes())
	to := ioAddr.String()

	// TODO: process special staking action

	if isContractAddr(svr, to) {
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
	addr, err := getStringFromArray(in, 0)
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
	h, err := getStringFromArray(in, 0)
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
	h, err := getStringFromArray(in, 0)
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

func getTransactionByHash(svr *Server, in interface{}) (interface{}, error) {
	h, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	ret, err := svr.getSingleAction(removeHexPrefix(h), true)
	if err != nil {
		return nil, err
	}

	tx := getTransactionCreateFromActionInfo(svr, ret.ActionInfo[0], svr.bc.ChainID())
	if tx == nil {
		return nil, errUnkownType
	}
	return *tx, nil
}

func getLogs(svr *Server, in interface{}) (interface{}, error) {
	logReq, err := parseLogRequest(in)
	if err != nil {
		return nil, err
	}

	return getLogsWithFilter(svr, logReq.FromBlock, logReq.ToBlock, logReq.Address, logReq.Topics)
}

func getTransactionReceipt(svr *Server, in interface{}) (interface{}, error) {
	h, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
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
		res, _ := ioAddrToEthAddr(receipt.ContractAddress)
		contractAddr = &res
	}

	act, err := svr.getSingleAction(removeHexPrefix(h), true)
	if err != nil {
		return nil, err
	}
	tx := getTransactionFromActionInfo(act.ActionInfo[0], svr.bc.ChainID())
	if tx == nil {
		return nil, errUnkownType
	}

	var logs []logsObject
	for _, v := range receipt.Logs {
		addr, _ := ioAddrToEthAddr(v.ContractAddress)
		var topics []string
		for _, tp := range v.Topics {
			topics = append(topics, "0x"+hex.EncodeToString(tp))
		}
		log := logsObject{
			BlockHash:        "0x" + ret.ReceiptInfo.BlkHash,
			TransactionHash:  h,
			TransactionIndex: *tx.TransactionIndex,
			LogIndex:         uint64ToHex(uint64(v.Index)),
			BlockNumber:      uint64ToHex(v.BlkHeight),
			Address:          addr,
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
		From:              *tx.From,
		GasUsed:           uint64ToHex(receipt.GasConsumed),
		LogsBloom:         "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		Status:            uint64ToHex(receipt.Status),
		To:                *tx.To,
		TransactionHash:   h,
		TransactionIndex:  *tx.TransactionIndex,
		Logs:              logs,
	}, nil

}

func getBlockTransactionCountByNumber(svr *Server, in interface{}) (interface{}, error) {
	blkNum, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	num, err := parseBlockNumber(svr, blkNum)
	if err != nil {
		return nil, err
	}

	blkMeta, err := svr.getBlockMetas(num, 1)
	if err != nil {
		return nil, err
	}
	return blkMeta.BlkMetas[0].NumActions, nil
}

func getTransactionByBlockHashAndIndex(svr *Server, in interface{}) (interface{}, error) {
	h, err := getStringFromArray(in, 0)
	idxStr, err := getStringFromArray(in, 1)
	if err != nil {
		return nil, err
	}
	idx, err := hexStringToNumber(idxStr)
	if err != nil {
		return nil, err
	}

	ret, err := svr.getActionsByBlock(removeHexPrefix(h), idx, 1)
	if err != nil {
		return nil, err
	}
	tx := getTransactionCreateFromActionInfo(svr, ret.ActionInfo[0], svr.bc.ChainID())

	if tx == nil {
		return nil, errUnkownType
	}
	return *tx, nil
}

func getTransactionByBlockNumberAndIndex(svr *Server, in interface{}) (interface{}, error) {
	blkNum, err := getStringFromArray(in, 0)
	idxStr, err := getStringFromArray(in, 1)
	if err != nil {
		return nil, err
	}
	num, err := parseBlockNumber(svr, blkNum)
	idx, err := hexStringToNumber(idxStr)
	if err != nil {
		return nil, err
	}
	blkMeta, err := svr.getBlockMetas(num, 1)
	if err != nil {
		return nil, err
	}

	ret, err := svr.getActionsByBlock(blkMeta.BlkMetas[0].Hash, idx, 1)
	if err != nil {
		return nil, err
	}
	tx := getTransactionCreateFromActionInfo(svr, ret.ActionInfo[0], svr.bc.ChainID())

	if tx == nil {
		return nil, errUnkownType
	}
	return *tx, nil
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
	_, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	// TODO: res := filterCache.Delete(id)
	res := true
	return res, nil
}

func getFilterChanges(svr *Server, in interface{}) (interface{}, error) {
	filterID, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	data, isFound := filterCache.Get(filterID)
	if !isFound {
		return []interface{}{}, nil
	}
	var filterObj filterObject
	err = json.Unmarshal([]byte(data), &filterObj)
	if err != nil {
		return nil, err
	}

	if filterObj.LogHeight > svr.bc.TipHeight() {
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
		blkMeta, err := svr.getBlockMetas(filterObj.LogHeight, 1000)
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
	filterID, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	data, isFound := filterCache.Get(filterID)
	if !isFound {
		return []interface{}{}, nil
	}
	var filterObj filterObject
	err = json.Unmarshal([]byte(data), &filterObj)
	if err != nil {
		return nil, err
	}

	if filterObj.FilterType != "log" {
		return nil, status.Error(codes.Internal, "filter not found")
	}

	return getLogsWithFilter(svr, filterObj.FromBlock, filterObj.ToBlock, filterObj.Address, filterObj.Topics)
}

func unimplemented(svr *Server, in interface{}) (interface{}, error) {
	return nil, status.Error(codes.Unimplemented, "function not implemented")
}

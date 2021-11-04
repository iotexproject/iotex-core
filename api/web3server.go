package api

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/wunderlist/ttlcache"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	contentType = "application/json"
)

type (
	Web3Server struct {
		web3Server  *http.Server
		coreService *coreService
	}

	web3Req struct {
		Jsonrpc string      `json:"jsonrpc,omitempty"`
		ID      *int        `json:"id"`
		Method  *string     `json:"method"`
		Params  interface{} `json:"params"`
	}

	web3Resp struct {
		Jsonrpc string      `json:"jsonrpc"`
		ID      int         `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   *web3Err    `json:"error,omitempty"`
	}

	web3Err struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
)

type (
	// TODO: all required?
	logsRequest struct {
		Address   []string
		FromBlock string
		ToBlock   string
		Topics    [][]string
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

func NewWeb3Server(core *coreService, httpPort int) *Web3Server {
	svr := &Web3Server{
		web3Server: &http.Server{
			Addr: ":" + strconv.Itoa(httpPort),
		},
		coreService: core,
	}
	http.Handle("/", svr)
	return svr
}

// Start starts the API server
func (svr *Web3Server) Start() error {
	go func() {
		// TODO: should err be returned from goroutine?
		if err := svr.web3Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.L().Fatal("Node failed to serve.", zap.Error(err))
		}
	}()
	return nil
}

// Stop stops the API server
func (svr *Web3Server) Stop() error {
	return svr.web3Server.Shutdown(context.Background())
}

func (svr *Web3Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "POST":
		httpResp := svr.handlePOSTReq(req)

		// write results into http reponse
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(httpResp); err != nil {
			log.L().Warn("fail to respond request.")
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (svr *Web3Server) handlePOSTReq(req *http.Request) interface{} {
	web3Reqs, err := parseWeb3Reqs(req)
	if err != nil {
		err := errors.Wrap(err, "failed to parse web3 requests.")
		return packAPIResult(nil, err, 0)
	}

	var web3Resps []web3Resp
	for _, web3Req := range web3Reqs {
		var (
			res interface{}
			err error
		)
		switch *web3Req.Method {
		case "eth_gasPrice":
			res, err = svr.suggestGasPrice()
		case "eth_getBlockByHash":
			res, err = svr.getBlockByHash(web3Req.Params)
		case "eth_chainId":
			res, err = svr.getChainID()
		case "eth_blockNumber":
			res, err = svr.getBlockNumber()
		case "eth_getBalance":
			res, err = svr.getBalance(web3Req.Params)
		case "eth_getTransactionCount":
			res, err = svr.getTransactionCount(web3Req.Params)
		case "eth_call":
			res, err = svr.call(web3Req.Params)
		case "eth_getCode":
			res, err = svr.getCode(web3Req.Params)
		case "eth_protocolVersion":
			res, err = svr.getProtocolVersion()
		case "web3_clientVersion":
			res, err = svr.getNodeInfo()
		case "net_version":
			res, err = svr.getNetworkID()
		case "net_peerCount":
			res, err = svr.getPeerCount()
		case "net_listening":
			res, err = svr.isListening()
		case "eth_syncing":
			res, err = svr.isSyncing()
		case "eth_mining":
			res, err = svr.isMining()
		case "eth_hashrate":
			res, err = svr.getHashrate()
		case "eth_getLogs":
			res, err = svr.getLogs(web3Req.Params)
		case "eth_getBlockTransactionCountByHash":
			res, err = svr.getBlockTransactionCountByHash(web3Req.Params)
		case "eth_getBlockByNumber":
			res, err = svr.getBlockByNumber(web3Req.Params)
		default:
			err := errors.Wrapf(errors.New("web3 method not found"), "method: %s\n", *web3Req.Method)
			return packAPIResult(nil, err, 0)
		}
		web3Resps = append(web3Resps, packAPIResult(res, err, *web3Req.ID))
	}

	if len(web3Resps) == 1 {
		return web3Resps[0]
	}
	return web3Resps
}

func parseWeb3Reqs(req *http.Request) ([]web3Req, error) {
	if req.Header.Get("Content-type") != "application/json" {
		return nil, errors.New("content-type is not application/json")
	}

	var web3Reqs []web3Req
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if d := bytes.TrimLeft(data, " \t\r\n"); len(d) > 0 && (d[0] != '[' || d[len(d)-1] != ']') {
		data = []byte("[" + string(d) + "]")
	}
	err = json.Unmarshal(data, &web3Reqs)
	if err != nil {
		return nil, err
	}
	for _, req := range web3Reqs {
		if req.ID == nil || req.Method == nil || req.Params == nil {
			return nil, errors.New("request field is incomplete")
		}
	}
	return web3Reqs, nil
}

// error code: https://eth.wiki/json-rpc/json-rpc-error-codes-improvement-proposal
func packAPIResult(res interface{}, err error, id int) web3Resp {
	if err != nil {
		var (
			errCode int
			errMsg  string
		)
		if s, ok := status.FromError(err); ok {
			errCode, errMsg = int(s.Code()), s.Message()
		} else {
			errCode, errMsg = -32603, err.Error()
		}
		return web3Resp{
			Jsonrpc: "2.0",
			ID:      id,
			Error: &web3Err{
				Code:    errCode,
				Message: errMsg,
			},
		}
	}
	return web3Resp{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  res,
	}
}

func (svr *Web3Server) suggestGasPrice() (interface{}, error) {
	ret, err := svr.coreService.SuggestGasPrice()
	if err != nil {
		return nil, err
	}
	return uint64ToHex(ret), nil
}

var (
	apiMap = map[string]func(*Server, interface{}) (interface{}, error){
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
	ret, err := svr.SuggestGasPrice(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	return uint64ToHex(ret.GasPrice), nil
}

func (svr *Web3Server) getChainID() (interface{}, error) {
	return uint64ToHex(uint64(svr.coreService.EVMNetworkID())), nil
}

func (svr *Web3Server) getBlockNumber() (interface{}, error) {
	return uint64ToHex(svr.coreService.bc.TipHeight()), nil
}

func (svr *Web3Server) getBlockByNumber(in interface{}) (interface{}, error) {
	blkNum, isDetailed, err := getStringAndBoolFromArray(in)
	if err != nil {
		return nil, err
	}
	num, err := svr.parseBlockNumber(blkNum)
	if err != nil {
		return nil, err
	}

	blkMetas, err := svr.coreService.BlockMetas(num, 1)
	// TODO: correct err == nil && blkMEta= []{}
	if err != nil || blkMetas == nil || len(blkMetas) == 0 {
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

func (svr *Web3Server) getBalance(in interface{}) (interface{}, error) {
	addr, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	ioAddr, err := ethAddrToIoAddr(addr)
	if err != nil {
		return nil, err
	}
	accountMeta, _, err := svr.coreService.Account(ioAddr)
	if err != nil {
		return nil, err
	}
	ret, err := intStrToHex(accountMeta.Balance)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (svr *Web3Server) getTransactionCount(in interface{}) (interface{}, error) {
	addr, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	ioAddr, err := ethAddrToIoAddr(addr)
	if err != nil {
		return nil, err
	}
	accountMeta, _, err := svr.coreService.Account(ioAddr)
	if err != nil {
		return nil, err
	}
	return uint64ToHex(accountMeta.PendingNonce), nil
}

func (svr *Web3Server) call(in interface{}) (interface{}, error) {
	from, to, gasLimit, value, data, err := parseCallObject(in)
	if err != nil {
		return nil, err
	}
	if to == "io1k8uw2hrlvnfq8s2qpwwc24ws2ru54heenx8chr" {
		return nil, nil
	}
	ret, _, err := svr.coreService.ReadContract(context.Background(),
		&iotextypes.Execution{
			Amount:   value.String(),
			Contract: to,
			Data:     data,
		},
		from,
		gasLimit,
	)
	if err != nil {
		return nil, err
	}
	return "0x" + ret, nil
}

func (svr *Web3Server) estimateGas(in interface{}) (interface{}, error) {
	from, to, _, value, data, err := parseCallObject(in)
	if err != nil {
		return nil, err
	}

	var estimatedGas uint64
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	if isContract, _ := svr.isContractAddr(to); !isContract {
		// transfer
		estimatedGas = uint64(len(data))*action.TransferPayloadGas + action.TransferBaseIntrinsicGas
	} else {
		// execution
		ret, err := svr.estimateActionGasConsumptionForExecution(context.Background(),
			&iotextypes.Execution{
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
	tx, sig, pubkey := web3Util.DecodeRawTx(dataInString, config.EVMNetworkID())
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
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
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     iotextypes.Encoding_ETHEREUM_RLP,
		},
	}

	ioAddr, _ := address.FromBytes(tx.To().Bytes())
	to := ioAddr.String()

	// TODO: process special staking action

	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}

	if isContract, _ := svr.isContractAddr(to); !isContract {
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

func (svr *Web3Server) getCode(in interface{}) (interface{}, error) {
	addr, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	ioAddr, err := ethAddrToIoAddr(addr)
	if err != nil {
		return nil, err
	}
	accountMeta, _, err := svr.coreService.Account(ioAddr)
	if err != nil {
		return nil, err
	}
	return "0x" + hex.EncodeToString(accountMeta.ContractByteCode), nil
}

func (svr *Web3Server) getNodeInfo() (interface{}, error) {
	packageVersion, _, _, goVersion, _ := svr.coreService.ServerMeta()
	return packageVersion + "/" + goVersion, nil
}

func (svr *Web3Server) getNetworkID() (interface{}, error) {
	return svr.coreService.EVMNetworkID(), nil
}

func (svr *Web3Server) getPeerCount() (interface{}, error) {
	return "0x64", nil
}

func (svr *Web3Server) isListening() (interface{}, error) {
	return true, nil
}

func (svr *Web3Server) getProtocolVersion() (interface{}, error) {
	return "64", nil
}

func (svr *Web3Server) isSyncing() (interface{}, error) {
	return false, nil
}

func (svr *Web3Server) isMining() (interface{}, error) {
	return false, nil
}

func (svr *Web3Server) getHashrate() (interface{}, error) {
	return "0x500000", nil
}

func (svr *Web3Server) getBlockTransactionCountByHash(in interface{}) (interface{}, error) {
	h, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	blkMeta, err := svr.coreService.BlockMetaByHash(removeHexPrefix(h))
	if err != nil {
		return nil, err
	}
	return uint64ToHex(uint64(blkMeta[0].NumActions)), nil
}

func (svr *Web3Server) getBlockByHash(in interface{}) (interface{}, error) {
	h, isDetailed, err := getStringAndBoolFromArray(in)
	if err != nil {
		return nil, err
	}
	blkMeta, err := svr.coreService.BlockMetaByHash(removeHexPrefix(h))
	if err != nil {
		return nil, err
	}

	blk, err := svr.getBlockWithTransactions(blkMeta[0], isDetailed)
	if err != nil {
		return nil, err
	}
	return *blk, nil
}

func getTransactionByHash(svr *Server, in interface{}) (interface{}, error) {
	var web3Util web3Utils
	h := web3Util.getStringFromArray(in, 0)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	ret, err := svr.getSingleAction(context.Background(), removeHexPrefix(h), true)
	if err != nil {
		return nil, err
	}

	tx := web3Util.getTransactionCreateFromActionInfo(svr, ret.ActionInfo[0], config.EVMNetworkID())
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	return tx, nil
}

func (svr *Web3Server) getLogs(in interface{}) (interface{}, error) {
	logReq, err := parseLogRequest(in)
	if err != nil {
		return nil, err
	}

	return svr.getLogsWithFilter(logReq.FromBlock, logReq.ToBlock, logReq.Address, logReq.Topics)
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

	act, err := svr.getSingleAction(context.Background(), removeHexPrefix(h), true)
	if err != nil {
		return nil, err
	}
	tx := web3Util.getTransactionFromActionInfo(act.ActionInfo[0], config.EVMNetworkID())
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
	ret, err := svr.getActionsByBlock(context.Background(), removeHexPrefix(h), idx, 1)
	if err != nil {
		return nil, err
	}
	tx := web3Util.getTransactionCreateFromActionInfo(svr, ret.ActionInfo[0], config.EVMNetworkID())
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
	ret, err := svr.getActionsByBlock(context.Background(), blkMeta.BlkMetas[0].Hash, idx, 1)
	if err != nil {
		return nil, err
	}
	tx := web3Util.getTransactionCreateFromActionInfo(svr, ret.ActionInfo[0], config.EVMNetworkID())
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

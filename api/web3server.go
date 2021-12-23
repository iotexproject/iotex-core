package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/go-pkgs/util"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	contentType = "application/json"
)

type (
	// Web3Server contains web3 server and the pointer to api coreservice
	Web3Server struct {
		web3Server  *http.Server
		coreService *coreService
		cache       apiCache
	}

	web3Resp struct {
		Jsonrpc string      `json:"jsonrpc"`
		ID      int         `json:"id"`
		Result  interface{} `json:"result"`
	}
	web3Err struct {
		Jsonrpc string     `json:"jsonrpc"`
		ID      int        `json:"id"`
		Error   errMessage `json:"error"`
	}

	errMessage struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
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
		To                *string      `json:"to"`
		CumulativeGasUsed string       `json:"cumulativeGasUsed"`
		GasUsed           string       `json:"gasUsed"`
		ContractAddress   *string      `json:"contractAddress"`
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
	web3ServerMtc = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "iotex_web3_api_metrics",
		Help: "web3 api metrics.",
	}, []string{"method"})

	errUnkownType     = errors.New("wrong type of params")
	errNullPointer    = errors.New("null pointer")
	errInvalidFormat  = errors.New("invalid format of request")
	errNotImplemented = errors.New("method not implemented")
	errInvalidFiterID = errors.New("filter not found")
	errInvalidBlock   = errors.New("invalid block")

	pendingBlockNumber  = "pending"
	latestBlockNumber   = "latest"
	earliestBlockNumber = "earliest"
)

func init() {
	prometheus.MustRegister(web3ServerMtc)
}

// NewWeb3Server creates a new web3 server
func NewWeb3Server(core *coreService, httpPort int, cacheURL string) *Web3Server {
	svr := &Web3Server{
		web3Server: &http.Server{
			Addr: ":" + strconv.Itoa(httpPort),
		},
		coreService: core,
	}

	mux := http.NewServeMux()
	mux.Handle("/", svr)
	svr.web3Server.Handler = mux
	svr.cache = newAPICache(15*time.Minute, cacheURL)
	return svr
}

// Start starts the API server
func (svr *Web3Server) Start() error {
	go func() {
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

	web3Resps := make([]interface{}, 0)
	for _, web3Req := range web3Reqs {
		var (
			res    interface{}
			err    error
			params = web3Req.Get("params").Value()
			method = web3Req.Get("method").Value()
		)
		switch method {
		case "eth_gasPrice":
			res, err = svr.gasPrice()
		case "eth_getBlockByHash":
			res, err = svr.getBlockByHash(params)
		case "eth_chainId":
			res, err = svr.getChainID()
		case "eth_blockNumber":
			res, err = svr.getBlockNumber()
		case "eth_getBalance":
			res, err = svr.getBalance(params)
		case "eth_getTransactionCount":
			res, err = svr.getTransactionCount(params)
		case "eth_call":
			res, err = svr.call(params)
		case "eth_getCode":
			res, err = svr.getCode(params)
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
			var filter *filterObject
			if filter, err = parseLogRequest(web3Req.Get("params")); err == nil {
				res, err = svr.getLogs(filter)
			}
		case "eth_getBlockTransactionCountByHash":
			res, err = svr.getBlockTransactionCountByHash(params)
		case "eth_getBlockByNumber":
			res, err = svr.getBlockByNumber(params)
		case "eth_estimateGas":
			res, err = svr.estimateGas(params)
		case "eth_sendRawTransaction":
			res, err = svr.sendRawTransaction(params)
		case "eth_getTransactionByHash":
			res, err = svr.getTransactionByHash(params)
		case "eth_getTransactionByBlockNumberAndIndex":
			res, err = svr.getTransactionByBlockNumberAndIndex(params)
		case "eth_getTransactionByBlockHashAndIndex":
			res, err = svr.getTransactionByBlockHashAndIndex(params)
		case "eth_getBlockTransactionCountByNumber":
			res, err = svr.getBlockTransactionCountByNumber(params)
		case "eth_getTransactionReceipt":
			res, err = svr.getTransactionReceipt(params)
		case "eth_getStorageAt":
			res, err = svr.getStorageAt(params)
		case "eth_getFilterLogs":
			res, err = svr.getFilterLogs(params)
		case "eth_getFilterChanges":
			res, err = svr.getFilterChanges(params)
		case "eth_uninstallFilter":
			res, err = svr.uninstallFilter(params)
		case "eth_newFilter":
			var filter *filterObject
			if filter, err = parseLogRequest(web3Req.Get("params")); err == nil {
				res, err = svr.newFilter(filter)
			}
		case "eth_newBlockFilter":
			res, err = svr.newBlockFilter()
		case "eth_coinbase", "eth_accounts", "eth_getUncleCountByBlockHash", "eth_getUncleCountByBlockNumber", "eth_sign", "eth_signTransaction", "eth_sendTransaction", "eth_getUncleByBlockHashAndIndex", "eth_getUncleByBlockNumberAndIndex", "eth_pendingTransactions":
			res, err = svr.unimplemented()
		default:
			err := errors.Wrapf(errors.New("web3 method not found"), "method: %s\n", web3Req.Get("method"))
			return packAPIResult(nil, err, 0)
		}
		if err != nil {
			// temporally used for monitor and debug
			log.L().Error("web3server",
				zap.String("requestParams", fmt.Sprintf("%+v", web3Req)),
				zap.Error(err))
		}
		web3Resps = append(web3Resps, packAPIResult(res, err, int(web3Req.Get("id").Int())))
		web3ServerMtc.WithLabelValues(method.(string)).Inc()
		web3ServerMtc.WithLabelValues("requests_total").Inc()
	}

	if len(web3Resps) == 1 {
		return web3Resps[0]
	}
	return web3Resps
}

func parseWeb3Reqs(req *http.Request) ([]gjson.Result, error) {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if !gjson.Valid(string(data)) {
		return nil, errors.New("request json format is not valid")
	}
	parsedReqs := gjson.Parse(string(data)).Array()
	// check rquired field
	for _, req := range parsedReqs {
		id := req.Get("id")
		method := req.Get("method")
		if !id.Exists() || !method.Exists() {
			return nil, errors.New("request field is incomplete")
		}
	}
	return parsedReqs, nil
}

// error code: https://eth.wiki/json-rpc/json-rpc-error-codes-improvement-proposal
func packAPIResult(res interface{}, err error, id int) interface{} {
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
		return web3Err{
			Jsonrpc: "2.0",
			ID:      id,
			Error: errMessage{
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

func (svr *Web3Server) gasPrice() (interface{}, error) {
	ret, err := svr.coreService.SuggestGasPrice()
	if err != nil {
		return nil, err
	}
	return uint64ToHex(ret), nil
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
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.InvalidArgument {
			return nil, nil
		}
		return nil, err
	}
	if len(blkMetas) == 0 {
		return nil, nil
	}
	return svr.getBlockWithTransactions(blkMetas[0], isDetailed)
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
	return intStrToHex(accountMeta.Balance)
}

// getTransactionCount returns the nonce for the given address
func (svr *Web3Server) getTransactionCount(in interface{}) (interface{}, error) {
	addr, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	ioAddr, err := ethAddrToIoAddr(addr)
	if err != nil {
		return nil, err
	}
	blkNum, err := getStringFromArray(in, 1)
	if err != nil {
		return nil, err
	}
	if blkNum == pendingBlockNumber {
		pendingNonce, err := svr.coreService.ap.GetPendingNonce(ioAddr.String())
		if err != nil {
			return nil, err
		}
		return uint64ToHex(pendingNonce), nil
	}
	// TODO: returns the nonce in given block height after archive mode is supported
	accountMeta, _, err := svr.coreService.Account(ioAddr)
	if err != nil {
		return nil, err
	}
	return uint64ToHex(accountMeta.GetPendingNonce()), nil
}

func (svr *Web3Server) call(in interface{}) (interface{}, error) {
	from, to, gasLimit, value, data, err := parseCallObject(in)
	if err != nil {
		return nil, err
	}
	if to == "io1k8uw2hrlvnfq8s2qpwwc24ws2ru54heenx8chr" {
		return nil, nil
	}
	callerAddr, err := address.FromString(from)
	if err != nil {
		return nil, errors.Wrapf(errUnkownType, "from: %s", from)
	}
	ret, _, err := svr.coreService.ReadContract(context.Background(),
		&iotextypes.Execution{
			Amount:   value.String(),
			Contract: to,
			Data:     data,
		},
		callerAddr,
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
	isContract, _ := svr.isContractAddr(to)
	// TODO: support more types of actions
	switch isContract {
	case true:
		// execution
		estimatedGas, err = svr.coreService.estimateActionGasConsumptionForExecution(context.Background(),
			&iotextypes.Execution{
				Amount:   value.String(),
				Contract: to,
				Data:     data,
			}, from)
		if err != nil {
			return nil, err
		}
	case false:
		// transfer
		estimatedGas = uint64(len(data))*action.TransferPayloadGas + action.TransferBaseIntrinsicGas
	}
	if estimatedGas < 21000 {
		estimatedGas = 21000
	}
	return uint64ToHex(estimatedGas), nil
}

func (svr *Web3Server) sendRawTransaction(in interface{}) (interface{}, error) {
	// parse raw data string from json request
	dataStr, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	tx, sig, pubkey, err := action.DecodeRawTx(dataStr, svr.coreService.EVMNetworkID())
	if err != nil {
		return nil, err
	}

	// load the value of gasPrice, value, to
	gasPrice, value, to := "0", "0", ""
	if tx.GasPrice() != nil {
		gasPrice = tx.GasPrice().String()
	}
	if tx.Value() != nil {
		value = tx.Value().String()
	}
	if tx.To() != nil {
		ioAddr, _ := address.FromBytes(tx.To().Bytes())
		to = ioAddr.String()
	}

	req := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Version:  0,
			Nonce:    tx.Nonce(),
			GasLimit: tx.Gas(),
			GasPrice: gasPrice,
			ChainID:  svr.coreService.ChainID(),
		},
		SenderPubKey: pubkey.Bytes(),
		Signature:    sig,
		Encoding:     iotextypes.Encoding_ETHEREUM_RLP,
	}

	// TODO: process special staking action

	if isContract, _ := svr.isContractAddr(to); !isContract {
		// transfer
		req.Core.Action = &iotextypes.ActionCore_Transfer{
			Transfer: &iotextypes.Transfer{
				Recipient: to,
				Payload:   tx.Data(),
				Amount:    value,
			},
		}
	} else {
		// execution
		req.Core.Action = &iotextypes.ActionCore_Execution{
			Execution: &iotextypes.Execution{
				Contract: to,
				Data:     tx.Data(),
				Amount:   value,
			},
		}
	}

	actionHash, err := svr.coreService.SendAction(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return "0x" + actionHash, nil
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
	return strconv.Itoa(int(svr.coreService.EVMNetworkID())), nil
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
	blkMeta, err := svr.coreService.BlockMetaByHash(util.Remove0xPrefix(h))
	if err != nil {
		return nil, errors.Wrap(err, "the block is not found")
	}
	return uint64ToHex(uint64(blkMeta.NumActions)), nil
}

func (svr *Web3Server) getBlockByHash(in interface{}) (interface{}, error) {
	h, isDetailed, err := getStringAndBoolFromArray(in)
	if err != nil {
		return nil, err
	}
	blkMeta, err := svr.coreService.BlockMetaByHash(util.Remove0xPrefix(h))
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	return svr.getBlockWithTransactions(blkMeta, isDetailed)
}

func (svr *Web3Server) getTransactionByHash(in interface{}) (interface{}, error) {
	h, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	actionInfos, err := svr.coreService.Action(util.Remove0xPrefix(h), false)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Unavailable {
			return nil, nil
		}
		return nil, err
	}
	return svr.getTransactionCreateFromActionInfo(actionInfos)
}

func (svr *Web3Server) getLogs(filter *filterObject) (interface{}, error) {
	from, to, err := svr.parseBlockRange(filter.FromBlock, filter.ToBlock)
	if err != nil {
		return nil, err
	}
	return svr.getLogsWithFilter(from, to, filter.Address, filter.Topics)
}

func (svr *Web3Server) getTransactionReceipt(in interface{}) (interface{}, error) {
	// parse action hash from request
	actHashStr, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}

	// acquire action receipt by action hash
	actHash, err := hash.HexStringToHash256(util.Remove0xPrefix(actHashStr))
	if err != nil {
		return nil, errors.Wrapf(errUnkownType, "actHash: %s", actHashStr)
	}
	receipt, blkHash, err := svr.coreService.ReceiptByAction(actHash)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}

	// read contract address from receipt
	var contractAddr *string
	if len(receipt.ContractAddress) > 0 {
		res, err := ioAddrToEthAddr(receipt.ContractAddress)
		if err != nil {
			return nil, err
		}
		contractAddr = &res
	}

	// acquire transaction index by action hash
	actInfo, err := svr.coreService.Action(util.Remove0xPrefix(actHashStr), true)
	if err != nil {
		return nil, err
	}
	tx, err := svr.getTransactionFromActionInfo(actInfo)
	if err != nil {
		return nil, err
	}

	// parse logs from receipt
	logs := make([]logsObject, 0)
	for _, v := range receipt.Logs() {
		topics := make([]string, 0)
		for _, tpc := range v.Topics {
			topics = append(topics, "0x"+hex.EncodeToString(tpc[:]))
		}
		addr, err := ioAddrToEthAddr(v.Address)
		if err != nil {
			return nil, err
		}
		log := logsObject{
			BlockHash:        "0x" + blkHash,
			TransactionHash:  "0x" + hex.EncodeToString(actHash[:]),
			TransactionIndex: tx.TransactionIndex,
			LogIndex:         uint64ToHex(uint64(v.Index)),
			BlockNumber:      uint64ToHex(v.BlockHeight),
			Address:          addr,
			Data:             "0x" + hex.EncodeToString(v.Data),
			Topics:           topics,
		}
		logs = append(logs, log)
	}
	return receiptObject{
		BlockHash:         "0x" + blkHash,
		BlockNumber:       uint64ToHex(receipt.BlockHeight),
		ContractAddress:   contractAddr,
		CumulativeGasUsed: uint64ToHex(receipt.GasConsumed),
		From:              tx.From,
		GasUsed:           uint64ToHex(receipt.GasConsumed),
		LogsBloom:         "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		Status:            uint64ToHex(receipt.Status),
		To:                tx.To,
		TransactionHash:   "0x" + hex.EncodeToString(actHash[:]),
		TransactionIndex:  tx.TransactionIndex,
		Logs:              logs,
	}, nil

}

func (svr *Web3Server) getBlockTransactionCountByNumber(in interface{}) (interface{}, error) {
	blkNum, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	num, err := svr.parseBlockNumber(blkNum)
	if err != nil {
		return nil, err
	}
	blkMetas, err := svr.coreService.BlockMetas(num, 1)
	if err != nil {
		return nil, err
	}
	if len(blkMetas) == 0 {
		return nil, errInvalidBlock
	}
	return uint64ToHex(uint64(blkMetas[0].NumActions)), nil
}

func (svr *Web3Server) getTransactionByBlockHashAndIndex(in interface{}) (interface{}, error) {
	blkHash, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	idxStr, err := getStringFromArray(in, 1)
	if err != nil {
		return nil, err
	}
	idx, err := hexStringToNumber(idxStr)
	if err != nil {
		return nil, err
	}
	actionInfos, err := svr.coreService.ActionsByBlock(util.Remove0xPrefix(blkHash), idx, 1)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	if len(actionInfos) == 0 {
		return nil, nil
	}
	return svr.getTransactionCreateFromActionInfo(actionInfos[0])
}

func (svr *Web3Server) getTransactionByBlockNumberAndIndex(in interface{}) (interface{}, error) {
	blkNum, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	idxStr, err := getStringFromArray(in, 1)
	if err != nil {
		return nil, err
	}
	num, err := svr.parseBlockNumber(blkNum)
	if err != nil {
		return nil, err
	}
	idx, err := hexStringToNumber(idxStr)
	if err != nil {
		return nil, err
	}
	blkMetas, err := svr.coreService.BlockMetas(num, 1)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.InvalidArgument {
			return nil, nil
		}
		return nil, err
	}
	if len(blkMetas) == 0 {
		return nil, nil
	}
	actionInfos, err := svr.coreService.ActionsByBlock(blkMetas[0].Hash, idx, 1)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	if len(actionInfos) == 0 {
		return nil, nil
	}
	return svr.getTransactionCreateFromActionInfo(actionInfos[0])
}

func (svr *Web3Server) getStorageAt(in interface{}) (interface{}, error) {
	ethAddr, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	storagePos, err := getStringFromArray(in, 1)
	if err != nil {
		return nil, err
	}
	contractAddr, err := address.FromHex(ethAddr)
	if err != nil {
		return nil, err
	}
	pos, err := hexToBytes(storagePos)
	if err != nil {
		return nil, err
	}
	val, err := svr.coreService.ReadContractStorage(context.Background(), contractAddr, pos)
	if err != nil {
		return nil, err
	}
	return "0x" + hex.EncodeToString(val), nil
}

func (svr *Web3Server) newFilter(filter *filterObject) (interface{}, error) {
	//check the validity of filter before caching
	if filter == nil {
		return nil, errNullPointer
	}
	if _, err := svr.parseBlockNumber(filter.FromBlock); err != nil {
		return nil, errors.Wrapf(errUnkownType, "from: %s", filter.FromBlock)
	}
	if _, err := svr.parseBlockNumber(filter.ToBlock); err != nil {
		return nil, errors.Wrapf(errUnkownType, "to: %s", filter.ToBlock)
	}
	for _, ethAddr := range filter.Address {
		if _, err := ethAddrToIoAddr(ethAddr); err != nil {
			return nil, err
		}
	}
	for _, tp := range filter.Topics {
		for _, str := range tp {
			if _, err := hexToBytes(str); err != nil {
				return nil, err
			}
		}
	}

	// cache filter and return hash value of the filter as filter id
	filter.FilterType = "log"
	objInByte, _ := json.Marshal(*filter)
	keyHash := hash.Hash256b(objInByte)
	filterID := hex.EncodeToString(keyHash[:])
	err := svr.cache.Set(filterID, objInByte)
	if err != nil {
		return nil, err
	}
	return "0x" + filterID, nil
}

func (svr *Web3Server) newBlockFilter() (interface{}, error) {
	filterObj := filterObject{
		FilterType: "block",
		LogHeight:  svr.coreService.bc.TipHeight(),
	}
	objInByte, _ := json.Marshal(filterObj)
	keyHash := hash.Hash256b(objInByte)
	filterID := hex.EncodeToString(keyHash[:])
	err := svr.cache.Set(filterID, objInByte)
	if err != nil {
		return nil, err
	}
	return "0x" + filterID, nil
}

func (svr *Web3Server) uninstallFilter(in interface{}) (interface{}, error) {
	id, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	return svr.cache.Del(util.Remove0xPrefix(id)), nil
}

func (svr *Web3Server) getFilterChanges(in interface{}) (interface{}, error) {
	filterID, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	filterID = util.Remove0xPrefix(filterID)
	filterObj, err := loadFilterFromCache(svr.cache, filterID)
	if err != nil {
		return nil, err
	}
	var (
		ret          interface{}
		newLogHeight uint64
		tipHeight    = svr.coreService.bc.TipHeight()
	)
	switch filterObj.FilterType {
	case "log":
		if filterObj.LogHeight > tipHeight {
			return []logsObject{}, nil
		}
		from, to, hasNewLogs, err := svr.getLogQueryRange(filterObj.FromBlock, filterObj.ToBlock, filterObj.LogHeight)
		if err != nil {
			return nil, err
		}
		if !hasNewLogs {
			return []logsObject{}, nil
		}
		logs, err := svr.getLogsWithFilter(from, to, filterObj.Address, filterObj.Topics)
		if err != nil {
			return nil, err
		}
		ret, newLogHeight = logs, tipHeight+1
	case "block":
		if filterObj.LogHeight > tipHeight {
			return []string{}, nil
		}
		queryCount := tipHeight - filterObj.LogHeight + 1
		if queryCount > svr.coreService.cfg.API.RangeQueryLimit {
			queryCount = svr.coreService.cfg.API.RangeQueryLimit
		}
		blkMetas, err := svr.coreService.BlockMetas(filterObj.LogHeight, queryCount)
		if err != nil {
			return nil, err
		}
		hashArr := make([]string, 0)
		for _, v := range blkMetas {
			hashArr = append(hashArr, "0x"+v.Hash)
		}
		ret, newLogHeight = hashArr, filterObj.LogHeight+queryCount
	default:
		return nil, errors.Wrapf(errUnkownType, "filterType: %s", filterObj.FilterType)
	}

	// update the logHeight of filter cache
	filterObj.LogHeight = newLogHeight
	objInByte, _ := json.Marshal(filterObj)
	if err = svr.cache.Set(filterID, objInByte); err != nil {
		return nil, err
	}
	return ret, nil
}

func (svr *Web3Server) getFilterLogs(in interface{}) (interface{}, error) {
	filterID, err := getStringFromArray(in, 0)
	if err != nil {
		return nil, err
	}
	filterID = util.Remove0xPrefix(filterID)
	filterObj, err := loadFilterFromCache(svr.cache, filterID)
	if err != nil {
		return nil, err
	}
	if filterObj.FilterType != "log" {
		return nil, errInvalidFiterID
	}
	from, to, err := svr.parseBlockRange(filterObj.FromBlock, filterObj.ToBlock)
	if err != nil {
		return nil, err
	}
	return svr.getLogsWithFilter(from, to, filterObj.Address, filterObj.Topics)
}

func (svr *Web3Server) unimplemented() (interface{}, error) {
	return nil, errNotImplemented
}

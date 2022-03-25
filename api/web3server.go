package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
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
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	contentType                 = "application/json"
	metamaskBalanceContractAddr = "io1k8uw2hrlvnfq8s2qpwwc24ws2ru54heenx8chr"
)

// Web3Server contains web3 server and the pointer to api coreservice
type Web3Server struct {
	queryLimit  uint64
	web3Server  *http.Server
	coreService CoreService
	cache       apiCache
}

type (
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

	errUnkownType        = errors.New("wrong type of params")
	errNullPointer       = errors.New("null pointer")
	errInvalidFormat     = errors.New("invalid format of request")
	errNotImplemented    = errors.New("method not implemented")
	errInvalidFilterID   = errors.New("filter not found")
	errInvalidBlock      = errors.New("invalid block")
	errUnsupportedAction = errors.New("the type of action is not supported")

	_pendingBlockNumber  = "pending"
	_latestBlockNumber   = "latest"
	_earliestBlockNumber = "earliest"
)

func init() {
	prometheus.MustRegister(web3ServerMtc)
}

// NewWeb3Server creates a new web3 server
func NewWeb3Server(core CoreService, httpPort int, cacheURL string, queryLimit uint64) *Web3Server {
	svr := &Web3Server{
		web3Server: &http.Server{
			Addr: ":" + strconv.Itoa(httpPort),
		},
		coreService: core,
		queryLimit:  queryLimit,
	}

	mux := http.NewServeMux()
	mux.Handle("/", svr)
	svr.web3Server.Handler = mux
	svr.cache = newAPICache(15*time.Minute, cacheURL)
	return svr
}

// Start starts the API server
func (svr *Web3Server) Start(_ context.Context) error {
	go func() {
		if err := svr.web3Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.L().Fatal("Node failed to serve.", zap.Error(err))
		}
	}()
	return nil
}

// Stop stops the API server
func (svr *Web3Server) Stop(_ context.Context) error {
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
	if !web3Reqs.IsArray() {
		return svr.handleWeb3Req(&web3Reqs)
	}
	web3Resps := make([]interface{}, 0)
	for _, web3Req := range web3Reqs.Array() {
		web3Resps = append(web3Resps, svr.handleWeb3Req(&web3Req))
	}
	return web3Resps
}

func (svr *Web3Server) handleWeb3Req(web3Req *gjson.Result) interface{} {
	var (
		res    interface{}
		err    error
		method = web3Req.Get("method").Value()
	)
	log.Logger("api").Debug("web3Debug", zap.String("requestParams", fmt.Sprintf("%+v", web3Req)))
	switch method {
	case "eth_accounts":
		res, err = svr.ethAccounts()
	case "eth_gasPrice":
		res, err = svr.gasPrice()
	case "eth_getBlockByHash":
		res, err = svr.getBlockByHash(web3Req)
	case "eth_chainId":
		res, err = svr.getChainID()
	case "eth_blockNumber":
		res, err = svr.getBlockNumber()
	case "eth_getBalance":
		res, err = svr.getBalance(web3Req)
	case "eth_getTransactionCount":
		res, err = svr.getTransactionCount(web3Req)
	case "eth_call":
		res, err = svr.call(web3Req)
	case "eth_getCode":
		res, err = svr.getCode(web3Req)
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
		filter, err = parseLogRequest(web3Req.Get("params"))
		if err == nil {
			res, err = svr.getLogs(filter)
		}
	case "eth_getBlockTransactionCountByHash":
		res, err = svr.getBlockTransactionCountByHash(web3Req)
	case "eth_getBlockByNumber":
		res, err = svr.getBlockByNumber(web3Req)
	case "eth_estimateGas":
		res, err = svr.estimateGas(web3Req)
	case "eth_sendRawTransaction":
		res, err = svr.sendRawTransaction(web3Req)
	case "eth_getTransactionByHash":
		res, err = svr.getTransactionByHash(web3Req)
	case "eth_getTransactionByBlockNumberAndIndex":
		res, err = svr.getTransactionByBlockNumberAndIndex(web3Req)
	case "eth_getTransactionByBlockHashAndIndex":
		res, err = svr.getTransactionByBlockHashAndIndex(web3Req)
	case "eth_getBlockTransactionCountByNumber":
		res, err = svr.getBlockTransactionCountByNumber(web3Req)
	case "eth_getTransactionReceipt":
		res, err = svr.getTransactionReceipt(web3Req)
	case "eth_getStorageAt":
		res, err = svr.getStorageAt(web3Req)
	case "eth_getFilterLogs":
		res, err = svr.getFilterLogs(web3Req)
	case "eth_getFilterChanges":
		res, err = svr.getFilterChanges(web3Req)
	case "eth_uninstallFilter":
		res, err = svr.uninstallFilter(web3Req)
	case "eth_newFilter":
		var filter *filterObject
		filter, err = parseLogRequest(web3Req.Get("params"))
		if err == nil {
			res, err = svr.newFilter(filter)
		}
	case "eth_newBlockFilter":
		res, err = svr.newBlockFilter()
	case "eth_coinbase", "eth_getUncleCountByBlockHash", "eth_getUncleCountByBlockNumber",
		"eth_sign", "eth_signTransaction", "eth_sendTransaction", "eth_getUncleByBlockHashAndIndex",
		"eth_getUncleByBlockNumberAndIndex", "eth_pendingTransactions":
		res, err = svr.unimplemented()
	default:
		res, err = nil, errors.Wrapf(errors.New("web3 method not found"), "method: %s\n", web3Req.Get("method"))
	}
	if err != nil {
		log.Logger("api").Error("web3server",
			zap.String("requestParams", fmt.Sprintf("%+v", web3Req)),
			zap.Error(err))
	} else {
		log.Logger("api").Debug("web3Debug", zap.String("response", fmt.Sprintf("%+v", res)))
	}
	web3ServerMtc.WithLabelValues(method.(string)).Inc()
	web3ServerMtc.WithLabelValues("requests_total").Inc()
	return packAPIResult(res, err, int(web3Req.Get("id").Int()))
}

func parseWeb3Reqs(req *http.Request) (gjson.Result, error) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return gjson.Result{}, err
	}
	if !gjson.Valid(string(data)) {
		return gjson.Result{}, errors.New("request json format is not valid")
	}
	ret := gjson.Parse(string(data))
	// check rquired field
	for _, req := range ret.Array() {
		id := req.Get("id")
		method := req.Get("method")
		if !id.Exists() || !method.Exists() {
			return gjson.Result{}, errors.New("request field is incomplete")
		}
	}
	return ret, nil
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

func (svr *Web3Server) ethAccounts() (interface{}, error) {
	return []string{}, nil
}

func (svr *Web3Server) gasPrice() (interface{}, error) {
	ret, err := svr.coreService.SuggestGasPrice()
	if err != nil {
		return nil, err
	}
	return Uint64ToHex(ret), nil
}

func (svr *Web3Server) getChainID() (interface{}, error) {
	return Uint64ToHex(uint64(svr.coreService.EVMNetworkID())), nil
}

func (svr *Web3Server) getBlockNumber() (interface{}, error) {
	return Uint64ToHex(svr.coreService.TipHeight()), nil
}

func (svr *Web3Server) getBlockByNumber(in *gjson.Result) (interface{}, error) {
	blkNum, isDetailed := in.Get("params.0"), in.Get("params.1")
	if !blkNum.Exists() || !isDetailed.Exists() {
		return nil, errInvalidFormat
	}
	num, err := svr.parseBlockNumber(blkNum.String())
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
	return svr.getBlockWithTransactions(blkMetas[0], isDetailed.Bool())
}

func (svr *Web3Server) getBalance(in *gjson.Result) (interface{}, error) {
	addr := in.Get("params.0")
	if !addr.Exists() {
		return nil, errInvalidFormat
	}
	ioAddr, err := ethAddrToIoAddr(addr.String())
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
func (svr *Web3Server) getTransactionCount(in *gjson.Result) (interface{}, error) {
	addr := in.Get("params.0")
	if !addr.Exists() {
		return nil, errInvalidFormat
	}
	ioAddr, err := ethAddrToIoAddr(addr.String())
	if err != nil {
		return nil, err
	}
	// TODO (liuhaai): returns the nonce in given block height after archive mode is supported
	// blkNum, err := getStringFromArray(in, 1)
	pendingNonce, err := svr.coreService.PendingNonce(ioAddr)
	if err != nil {
		return nil, err
	}
	return Uint64ToHex(pendingNonce), nil
}

func (svr *Web3Server) call(in *gjson.Result) (interface{}, error) {
	callerAddr, to, gasLimit, value, data, err := parseCallObject(in)
	if err != nil {
		return nil, err
	}
	if to == metamaskBalanceContractAddr {
		return nil, nil
	}
	exec, _ := action.NewExecution(to, 0, value, gasLimit, big.NewInt(0), data)
	ret, _, err := svr.coreService.ReadContract(context.Background(), callerAddr, exec)
	if err != nil {
		return nil, err
	}
	return "0x" + ret, nil
}

func (svr *Web3Server) estimateGas(in *gjson.Result) (interface{}, error) {
	from, to, gasLimit, value, data, err := parseCallObject(in)
	if err != nil {
		return nil, err
	}

	var tx *types.Transaction
	if len(to) == 0 {
		tx = types.NewContractCreation(0, value, gasLimit, big.NewInt(0), data)
	} else {
		toAddr, err := addrutil.IoAddrToEvmAddr(to)
		if err != nil {
			return "", err
		}
		tx = types.NewTransaction(0, toAddr, value, gasLimit, big.NewInt(0), data)
	}
	act, err := svr.ethTxToAction(tx)
	if err != nil {
		return nil, err
	}

	var estimatedGas uint64
	if exec, ok := act.(*action.Execution); ok {
		estimatedGas, err = svr.coreService.EstimateExecutionGasConsumption(context.Background(), exec, from)
	} else {
		estimatedGas, err = svr.coreService.EstimateGasForNonExecution(act)
	}
	if err != nil {
		return nil, err
	}
	if estimatedGas < 21000 {
		estimatedGas = 21000
	}
	return Uint64ToHex(estimatedGas), nil
}

func (svr *Web3Server) sendRawTransaction(in *gjson.Result) (interface{}, error) {
	dataStr := in.Get("params.0")
	if !dataStr.Exists() {
		return nil, errInvalidFormat
	}
	// parse raw data string from json request
	tx, sig, pubkey, err := action.DecodeRawTx(dataStr.String(), svr.coreService.EVMNetworkID())
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

func (svr *Web3Server) getCode(in *gjson.Result) (interface{}, error) {
	addr := in.Get("params.0")
	if !addr.Exists() {
		return nil, errInvalidFormat
	}
	ioAddr, err := ethAddrToIoAddr(addr.String())
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
	return version.PackageVersion + "/" + version.GoVersion, nil
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

func (svr *Web3Server) getBlockTransactionCountByHash(in *gjson.Result) (interface{}, error) {
	txHash := in.Get("params.0")
	if !txHash.Exists() {
		return nil, errInvalidFormat
	}
	blkMeta, err := svr.coreService.BlockMetaByHash(util.Remove0xPrefix(txHash.String()))
	if err != nil {
		return nil, errors.Wrap(err, "the block is not found")
	}
	return Uint64ToHex(uint64(blkMeta.NumActions)), nil
}

func (svr *Web3Server) getBlockByHash(in *gjson.Result) (interface{}, error) {
	blkHash, isDetailed := in.Get("params.0"), in.Get("params.1")
	if !blkHash.Exists() || !isDetailed.Exists() {
		return nil, errInvalidFormat
	}
	blkMeta, err := svr.coreService.BlockMetaByHash(util.Remove0xPrefix(blkHash.String()))
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	return svr.getBlockWithTransactions(blkMeta, isDetailed.Bool())
}

func (svr *Web3Server) getTransactionByHash(in *gjson.Result) (interface{}, error) {
	txHash := in.Get("params.0")
	if !txHash.Exists() {
		return nil, errInvalidFormat
	}
	actionInfos, err := svr.coreService.Action(util.Remove0xPrefix(txHash.String()), false)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Unavailable {
			return nil, nil
		}
		return nil, err
	}
	return svr.getTransactionFromActionInfo(actionInfos)
}

func (svr *Web3Server) getLogs(filter *filterObject) (interface{}, error) {
	from, to, err := svr.parseBlockRange(filter.FromBlock, filter.ToBlock)
	if err != nil {
		return nil, err
	}
	return svr.getLogsWithFilter(from, to, filter.Address, filter.Topics)
}

func (svr *Web3Server) getTransactionReceipt(in *gjson.Result) (interface{}, error) {
	// parse action hash from request
	actHashStr := in.Get("params.0")
	if !actHashStr.Exists() {
		return nil, errInvalidFormat
	}
	actHash, err := hash.HexStringToHash256(util.Remove0xPrefix(actHashStr.String()))
	if err != nil {
		return nil, errors.Wrapf(errUnkownType, "actHash: %s", actHashStr)
	}
	// acquire action receipt by action hash
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
	res, err := getExecutionContractAddr(receipt.ContractAddress)
	if err != nil {
		return nil, err
	}
	if len(res) > 0 {
		contractAddr = &res
	}

	// acquire transaction index by action hash
	actInfo, err := svr.coreService.Action(util.Remove0xPrefix(actHashStr.String()), true)
	if err != nil {
		return nil, err
	}
	tx, err := svr.getTransactionFromActionInfo(actInfo)
	if err != nil {
		return nil, err
	}

	// acquire logsBloom from blockMeta
	blkMeta, err := svr.coreService.BlockMetaByHash(blkHash)
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
			TransactionIndex: Uint64ToHex(uint64(v.TxIndex)),
			LogIndex:         Uint64ToHex(uint64(v.Index)),
			BlockNumber:      Uint64ToHex(v.BlockHeight),
			Address:          addr,
			Data:             "0x" + hex.EncodeToString(v.Data),
			Topics:           topics,
		}
		logs = append(logs, log)
	}
	return receiptObject{
		BlockHash:         "0x" + blkHash,
		BlockNumber:       Uint64ToHex(receipt.BlockHeight),
		ContractAddress:   contractAddr,
		CumulativeGasUsed: Uint64ToHex(receipt.GasConsumed),
		From:              tx.From,
		GasUsed:           Uint64ToHex(receipt.GasConsumed),
		LogsBloom:         getLogsBloomFromBlkMeta(blkMeta),
		Status:            Uint64ToHex(receipt.Status),
		To:                tx.To,
		TransactionHash:   "0x" + hex.EncodeToString(actHash[:]),
		TransactionIndex:  tx.TransactionIndex,
		Logs:              logs,
	}, nil

}

func (svr *Web3Server) getBlockTransactionCountByNumber(in *gjson.Result) (interface{}, error) {
	blkNum := in.Get("params.0")
	if !blkNum.Exists() {
		return nil, errInvalidFormat
	}
	num, err := svr.parseBlockNumber(blkNum.String())
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
	return Uint64ToHex(uint64(blkMetas[0].NumActions)), nil
}

func (svr *Web3Server) getTransactionByBlockHashAndIndex(in *gjson.Result) (interface{}, error) {
	blkHash, idxStr := in.Get("params.0"), in.Get("params.1")
	if !blkHash.Exists() || !idxStr.Exists() {
		return nil, errInvalidFormat
	}
	idx, err := hexStringToNumber(idxStr.String())
	if err != nil {
		return nil, err
	}
	actionInfos, err := svr.coreService.ActionsByBlock(util.Remove0xPrefix(blkHash.String()), idx, 1)
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
	return svr.getTransactionFromActionInfo(actionInfos[0])
}

func (svr *Web3Server) getTransactionByBlockNumberAndIndex(in *gjson.Result) (interface{}, error) {
	blkNum, idxStr := in.Get("params.0"), in.Get("params.1")
	if !blkNum.Exists() || !idxStr.Exists() {
		return nil, errInvalidFormat
	}
	num, err := svr.parseBlockNumber(blkNum.String())
	if err != nil {
		return nil, err
	}
	idx, err := hexStringToNumber(idxStr.String())
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
	return svr.getTransactionFromActionInfo(actionInfos[0])
}

func (svr *Web3Server) getStorageAt(in *gjson.Result) (interface{}, error) {
	ethAddr, storagePos := in.Get("params.0"), in.Get("params.1")
	if !ethAddr.Exists() || !storagePos.Exists() {
		return nil, errInvalidFormat
	}
	contractAddr, err := address.FromHex(ethAddr.String())
	if err != nil {
		return nil, err
	}
	pos, err := hexToBytes(storagePos.String())
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
		LogHeight:  svr.coreService.TipHeight(),
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

func (svr *Web3Server) uninstallFilter(in *gjson.Result) (interface{}, error) {
	id := in.Get("params.0")
	if !id.Exists() {
		return nil, errInvalidFormat
	}
	return svr.cache.Del(util.Remove0xPrefix(id.String())), nil
}

func (svr *Web3Server) getFilterChanges(in *gjson.Result) (interface{}, error) {
	id := in.Get("params.0")
	if !id.Exists() {
		return nil, errInvalidFormat
	}
	filterID := util.Remove0xPrefix(id.String())
	filterObj, err := loadFilterFromCache(svr.cache, filterID)
	if err != nil {
		return nil, err
	}
	var (
		ret          interface{}
		newLogHeight uint64
		tipHeight    = svr.coreService.TipHeight()
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

func (svr *Web3Server) getFilterLogs(in *gjson.Result) (interface{}, error) {
	id := in.Get("params.0")
	if !id.Exists() {
		return nil, errInvalidFormat
	}
	filterID := util.Remove0xPrefix(id.String())
	filterObj, err := loadFilterFromCache(svr.cache, filterID)
	if err != nil {
		return nil, err
	}
	if filterObj.FilterType != "log" {
		return nil, errInvalidFilterID
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

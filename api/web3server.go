package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
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
	apitypes "github.com/iotexproject/iotex-core/api/types"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
)

const (
	_metamaskBalanceContractAddr = "io1k8uw2hrlvnfq8s2qpwwc24ws2ru54heenx8chr"
)

type (
	// Web3Handler handle JRPC request
	Web3Handler interface {
		HandlePOSTReq(io.Reader, apitypes.Web3ResponseWriter) error
	}

	web3Handler struct {
		coreService CoreService
		cache       apiCache
	}
)

type (
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
	_web3ServerMtc = prometheus.NewCounterVec(prometheus.CounterOpts{
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
	prometheus.MustRegister(_web3ServerMtc)
}

// NewWeb3Handler creates a handle to process web3 requests
func NewWeb3Handler(core CoreService, cacheURL string) Web3Handler {
	return &web3Handler{
		coreService: core,
		cache:       newAPICache(15*time.Minute, cacheURL),
	}
}

// HandlePOSTReq handles web3 request
func (svr *web3Handler) HandlePOSTReq(reader io.Reader, writer apitypes.Web3ResponseWriter) error {
	web3Reqs, err := parseWeb3Reqs(reader)
	if err != nil {
		err := errors.Wrap(err, "failed to parse web3 requests.")
		return writer.Write(&web3Response{err: err})
	}
	if !web3Reqs.IsArray() {
		return svr.handleWeb3Req(&web3Reqs, writer)
	}
	batchWriter := apitypes.NewBatchWriter(writer)
	for _, web3Req := range web3Reqs.Array() {
		if err := svr.handleWeb3Req(&web3Req, batchWriter); err != nil {
			return err
		}
	}
	return batchWriter.Flush()
}

func (svr *web3Handler) handleWeb3Req(web3Req *gjson.Result, writer apitypes.Web3ResponseWriter) error {
	var (
		res    interface{}
		err    error
		method = web3Req.Get("method").Value()
	)
	log.Logger("api").Debug("web3Debug", zap.String("requestParams", fmt.Sprintf("%+v", web3Req)))
	_web3ServerMtc.WithLabelValues(method.(string)).Inc()
	_web3ServerMtc.WithLabelValues("requests_total").Inc()
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
	case "eth_subscribe":
		res, err = svr.subscribe(web3Req, writer)
	case "eth_unsubscribe":
		res, err = svr.unsubscribe(web3Req)
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
	return writer.Write(&web3Response{
		id:     int(web3Req.Get("id").Int()),
		result: res,
		err:    err,
	})
}

func parseWeb3Reqs(reader io.Reader) (gjson.Result, error) {
	data, err := io.ReadAll(reader)
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

func (svr *web3Handler) ethAccounts() (interface{}, error) {
	return []string{}, nil
}

func (svr *web3Handler) gasPrice() (interface{}, error) {
	ret, err := svr.coreService.SuggestGasPrice()
	if err != nil {
		return nil, err
	}
	return uint64ToHex(ret), nil
}

func (svr *web3Handler) getChainID() (interface{}, error) {
	return uint64ToHex(uint64(svr.coreService.EVMNetworkID())), nil
}

func (svr *web3Handler) getBlockNumber() (interface{}, error) {
	return uint64ToHex(svr.coreService.TipHeight()), nil
}

func (svr *web3Handler) getBlockByNumber(in *gjson.Result) (interface{}, error) {
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

func (svr *web3Handler) getBalance(in *gjson.Result) (interface{}, error) {
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
func (svr *web3Handler) getTransactionCount(in *gjson.Result) (interface{}, error) {
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
	return uint64ToHex(pendingNonce), nil
}

func (svr *web3Handler) call(in *gjson.Result) (interface{}, error) {
	callerAddr, to, gasLimit, value, data, err := parseCallObject(in)
	if err != nil {
		return nil, err
	}
	if to == _metamaskBalanceContractAddr {
		return nil, nil
	}
	exec, _ := action.NewExecution(to, 0, value, gasLimit, big.NewInt(0), data)
	ret, _, err := svr.coreService.ReadContract(context.Background(), callerAddr, exec)
	if err != nil {
		return nil, err
	}
	return "0x" + ret, nil
}

func (svr *web3Handler) estimateGas(in *gjson.Result) (interface{}, error) {
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
	elp, err := svr.ethTxToEnvelope(tx)
	if err != nil {
		return nil, err
	}

	var estimatedGas uint64
	if exec, ok := elp.Action().(*action.Execution); ok {
		estimatedGas, err = svr.coreService.EstimateExecutionGasConsumption(context.Background(), exec, from)
	} else {
		estimatedGas, err = svr.coreService.EstimateGasForNonExecution(elp.Action())
	}
	if err != nil {
		return nil, err
	}
	if estimatedGas < 21000 {
		estimatedGas = 21000
	}
	return uint64ToHex(estimatedGas), nil
}

func (svr *web3Handler) sendRawTransaction(in *gjson.Result) (interface{}, error) {
	dataStr := in.Get("params.0")
	if !dataStr.Exists() {
		return nil, errInvalidFormat
	}
	// parse raw data string from json request
	tx, sig, pubkey, err := action.DecodeRawTx(dataStr.String(), svr.coreService.EVMNetworkID())
	if err != nil {
		return nil, err
	}
	elp, err := svr.ethTxToEnvelope(tx)
	if err != nil {
		return nil, err
	}
	req := &iotextypes.Action{
		Core:         elp.Proto(),
		SenderPubKey: pubkey.Bytes(),
		Signature:    sig,
		Encoding:     iotextypes.Encoding_ETHEREUM_RLP,
	}
	actionHash, err := svr.coreService.SendAction(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return "0x" + actionHash, nil
}

func (svr *web3Handler) getCode(in *gjson.Result) (interface{}, error) {
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

func (svr *web3Handler) getNodeInfo() (interface{}, error) {
	packageVersion, _, _, goVersion, _ := svr.coreService.ServerMeta()
	return packageVersion + "/" + goVersion, nil
}

func (svr *web3Handler) getNetworkID() (interface{}, error) {
	return strconv.Itoa(int(svr.coreService.EVMNetworkID())), nil
}

func (svr *web3Handler) getPeerCount() (interface{}, error) {
	return "0x64", nil
}

func (svr *web3Handler) isListening() (interface{}, error) {
	return true, nil
}

func (svr *web3Handler) getProtocolVersion() (interface{}, error) {
	return "64", nil
}

func (svr *web3Handler) isSyncing() (interface{}, error) {
	start, curr, highest := svr.coreService.SyncingProgress()
	if curr >= highest {
		return false, nil
	}
	return &getSyncingResult{
		StartingBlock: uint64ToHex(start),
		CurrentBlock:  uint64ToHex(curr),
		HighestBlock:  uint64ToHex(highest),
	}, nil
}

func (svr *web3Handler) isMining() (interface{}, error) {
	return false, nil
}

func (svr *web3Handler) getHashrate() (interface{}, error) {
	return "0x500000", nil
}

func (svr *web3Handler) getBlockTransactionCountByHash(in *gjson.Result) (interface{}, error) {
	txHash := in.Get("params.0")
	if !txHash.Exists() {
		return nil, errInvalidFormat
	}
	blkMeta, err := svr.coreService.BlockMetaByHash(util.Remove0xPrefix(txHash.String()))
	if err != nil {
		return nil, errors.Wrap(err, "the block is not found")
	}
	return uint64ToHex(uint64(blkMeta.NumActions)), nil
}

func (svr *web3Handler) getBlockByHash(in *gjson.Result) (interface{}, error) {
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

func (svr *web3Handler) getTransactionByHash(in *gjson.Result) (interface{}, error) {
	txHash := in.Get("params.0")
	if !txHash.Exists() {
		return nil, errInvalidFormat
	}
	actHash, err := hash.HexStringToHash256(util.Remove0xPrefix(txHash.String()))
	if err != nil {
		return nil, err
	}

	selp, blkHash, _, _, err := svr.coreService.ActionByActionHash(actHash)
	if err != nil {
		if errors.Cause(err) == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	receipt, err := svr.coreService.ReceiptByActionHash(actHash)
	if err != nil {
		if errors.Cause(err) == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return svr.getTransactionFromActionInfo(hex.EncodeToString(blkHash[:]), selp, receipt)
}

func (svr *web3Handler) getLogs(filter *filterObject) (interface{}, error) {
	from, to, err := svr.parseBlockRange(filter.FromBlock, filter.ToBlock)
	if err != nil {
		return nil, err
	}
	return svr.getLogsWithFilter(from, to, filter.Address, filter.Topics)
}

func (svr *web3Handler) getTransactionReceipt(in *gjson.Result) (interface{}, error) {
	// parse action hash from request
	actHashStr := in.Get("params.0")
	if !actHashStr.Exists() {
		return nil, errInvalidFormat
	}
	actHash, err := hash.HexStringToHash256(util.Remove0xPrefix(actHashStr.String()))
	if err != nil {
		return nil, errors.Wrapf(errUnkownType, "actHash: %s", actHashStr.String())
	}

	// acquire action receipt by action hash
	selp, blockHash, _, _, err := svr.coreService.ActionByActionHash(actHash)
	if err != nil {
		if errors.Cause(err) == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	receipt, err := svr.coreService.ReceiptByActionHash(actHash)
	if err != nil {
		if errors.Cause(err) == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	to, contractAddr, err := getRecipientAndContractAddrFromAction(selp, receipt)
	if err != nil {
		return nil, err
	}

	// acquire logsBloom from blockMeta
	blkHash := hex.EncodeToString(blockHash[:])
	blkMeta, err := svr.coreService.BlockMetaByHash(blkHash)
	if err != nil {
		return nil, err
	}

	return &getReceiptResult{
		blockHash:       blockHash,
		from:            selp.SrcPubkey().Address(),
		to:              to,
		contractAddress: contractAddr,
		logsBloom:       blkMeta.LogsBloom,
		receipt:         receipt,
	}, nil

}

func (svr *web3Handler) getBlockTransactionCountByNumber(in *gjson.Result) (interface{}, error) {
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
	return uint64ToHex(uint64(blkMetas[0].NumActions)), nil
}

func (svr *web3Handler) getTransactionByBlockHashAndIndex(in *gjson.Result) (interface{}, error) {
	blkHashStr, idxStr := in.Get("params.0"), in.Get("params.1")
	if !blkHashStr.Exists() || !idxStr.Exists() {
		return nil, errInvalidFormat
	}
	idx, err := hexStringToNumber(idxStr.String())
	if err != nil {
		return nil, err
	}
	blkHash := util.Remove0xPrefix(blkHashStr.String())
	selps, receipts, err := svr.coreService.ActionsInBlockByHash(blkHash)
	if errors.Cause(err) == ErrNotFound || idx >= uint64(len(receipts)) || len(receipts) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return svr.getTransactionFromActionInfo(blkHash, selps[idx], receipts[idx])
}

func (svr *web3Handler) getTransactionByBlockNumberAndIndex(in *gjson.Result) (interface{}, error) {
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
	selps, receipts, err := svr.coreService.ActionsInBlockByHash(blkMetas[0].Hash)
	if errors.Cause(err) == ErrNotFound || idx >= uint64(len(receipts)) || len(receipts) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return svr.getTransactionFromActionInfo(blkMetas[0].Hash, selps[idx], receipts[idx])
}

func (svr *web3Handler) getStorageAt(in *gjson.Result) (interface{}, error) {
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

func (svr *web3Handler) newFilter(filter *filterObject) (interface{}, error) {
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

func (svr *web3Handler) newBlockFilter() (interface{}, error) {
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

func (svr *web3Handler) uninstallFilter(in *gjson.Result) (interface{}, error) {
	id := in.Get("params.0")
	if !id.Exists() {
		return nil, errInvalidFormat
	}
	return svr.cache.Del(util.Remove0xPrefix(id.String())), nil
}

func (svr *web3Handler) getFilterChanges(in *gjson.Result) (interface{}, error) {
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
			return []*getLogsResult{}, nil
		}
		from, to, hasNewLogs, err := svr.getLogQueryRange(filterObj.FromBlock, filterObj.ToBlock, filterObj.LogHeight)
		if err != nil {
			return nil, err
		}
		if !hasNewLogs {
			return []*getLogsResult{}, nil
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

func (svr *web3Handler) getFilterLogs(in *gjson.Result) (interface{}, error) {
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

func (svr *web3Handler) subscribe(in *gjson.Result, writer apitypes.Web3ResponseWriter) (interface{}, error) {
	subscription := in.Get("params.0")
	if !subscription.Exists() {
		return nil, errInvalidFormat
	}
	switch subscription.String() {
	case "newHeads":
		return svr.streamBlocks(writer)
	case "logs":
		filter, err := parseLogRequest(in.Get("params.1"))
		if err != nil {
			return nil, err
		}
		return svr.streamLogs(filter, writer)
	default:
		return nil, errInvalidFormat
	}
}

func (svr *web3Handler) streamBlocks(writer apitypes.Web3ResponseWriter) (interface{}, error) {
	chainListener := svr.coreService.ChainListener()
	streamID, err := chainListener.AddResponder(NewWeb3BlockListener(writer.Write))
	if err != nil {
		return nil, err
	}
	return streamID, nil
}

func (svr *web3Handler) streamLogs(filterObj *filterObject, writer apitypes.Web3ResponseWriter) (interface{}, error) {
	filter, err := newLogFilterFrom(filterObj.Address, filterObj.Topics)
	if err != nil {
		return nil, err
	}
	chainListener := svr.coreService.ChainListener()
	streamID, err := chainListener.AddResponder(NewWeb3LogListener(filter, writer.Write))
	if err != nil {
		return nil, err
	}
	return streamID, nil
}

func (svr *web3Handler) unsubscribe(in *gjson.Result) (interface{}, error) {
	id := in.Get("params.0")
	if !id.Exists() {
		return nil, errInvalidFormat
	}
	chainListener := svr.coreService.ChainListener()
	return chainListener.RemoveResponder(id.String())
}

func (svr *web3Handler) unimplemented() (interface{}, error) {
	return nil, errNotImplemented
}

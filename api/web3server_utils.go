package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-redis/redis/v8"
	"github.com/iotexproject/go-pkgs/cache/ttl"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/go-pkgs/util"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	logfilter "github.com/iotexproject/iotex-core/v2/api/logfilter"
	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/addrutil"
)

func hexStringToNumber(hexStr string) (uint64, error) {
	return strconv.ParseUint(util.Remove0xPrefix(hexStr), 16, 64)
}

func ethAddrToIoAddr(ethAddr string) (address.Address, error) {
	if ok := common.IsHexAddress(ethAddr); !ok {
		return nil, errors.Wrapf(errUnkownType, "ethAddr: %s", ethAddr)
	}
	return address.FromHex(ethAddr)
}

func ioAddrToEthAddr(ioAddr string) (string, error) {
	if len(ioAddr) == 0 {
		return "0x0000000000000000000000000000000000000000", nil
	}
	addr, err := addrutil.IoAddrToEvmAddr(ioAddr)
	if err != nil {
		return "", err
	}
	return addr.String(), nil
}

func uint64ToHex(val uint64) string {
	return "0x" + strconv.FormatUint(val, 16)
}

func bigIntToHex(b *big.Int) string {
	if b == nil || b.Sign() == 0 {
		return "0x0"
	}
	return "0x" + b.Text(16)
}

// mapper maps a slice of S to a slice of T
func mapper[S, T any](arr []S, fn func(S) T) []T {
	ret := make([]T, len(arr))
	for i, v := range arr {
		ret[i] = fn(v)
	}
	return ret
}

func intStrToHex(str string) (string, error) {
	amount, ok := new(big.Int).SetString(str, 10)
	if !ok {
		return "", errors.Wrapf(errUnkownType, "int: %s", str)
	}
	return "0x" + fmt.Sprintf("%x", amount), nil
}

func (svr *web3Handler) getBlockWithTransactions(blk *block.Block, receipts []*action.Receipt, isDetailed bool) (*getBlockResult, error) {
	if blk == nil || receipts == nil {
		return nil, errInvalidBlock
	}
	transactions := make([]interface{}, 0)
	for i, selp := range blk.Actions {
		if isDetailed {
			tx, err := svr.assembleConfirmedTransaction(blk.HashBlock(), selp, receipts[i])
			if err != nil {
				if errors.Cause(err) != errUnsupportedAction {
					h, _ := selp.Hash()
					log.Logger("api").Error("failed to get info from action", zap.Error(err), zap.String("actHash", hex.EncodeToString(h[:])))
				}
				continue
			}
			transactions = append(transactions, tx)
		} else {
			actHash, err := selp.Hash()
			if err != nil {
				return nil, err
			}
			transactions = append(transactions, "0x"+hex.EncodeToString(actHash[:]))
		}
	}
	return &getBlockResult{
		blk:          blk,
		transactions: transactions,
	}, nil
}

func (svr *web3Handler) assembleConfirmedTransaction(blkHash hash.Hash256, selp *action.SealedEnvelope, receipt *action.Receipt) (*getTransactionResult, error) {
	// sanity check
	if receipt == nil {
		return nil, errors.New("receipt is empty")
	}
	actHash, err := selp.Hash()
	if err != nil || actHash != receipt.ActionHash {
		return nil, errors.Errorf("the action %s of receipt doesn't match", hex.EncodeToString(actHash[:]))
	}
	return newGetTransactionResult(&blkHash, selp, receipt, svr.coreService.EVMNetworkID())
}

func (svr *web3Handler) assemblePendingTransaction(selp *action.SealedEnvelope) (*getTransactionResult, error) {
	return newGetTransactionResult(nil, selp, nil, svr.coreService.EVMNetworkID())
}

func getRecipientAndContractAddrFromAction(selp *action.SealedEnvelope, receipt *action.Receipt) (*string, *string, error) {
	// recipient is empty when contract is created
	if exec, ok := selp.Action().(*action.Execution); ok && len(exec.Contract()) == 0 {
		addr, err := ioAddrToEthAddr(receipt.ContractAddress)
		if err != nil {
			return nil, nil, err
		}
		return nil, &addr, nil
	}
	ethTx, err := selp.ToEthTx()
	if err != nil {
		return nil, nil, err
	}
	toTmp := ethTx.To().String()
	return &toTmp, nil, nil
}

func (svr *web3Handler) parseBlockNumber(str string) (uint64, error) {
	switch str {
	case _earliestBlockNumber:
		return 1, nil
	case "", _pendingBlockNumber, _latestBlockNumber:
		return svr.coreService.TipHeight(), nil
	default:
		return hexStringToNumber(str)
	}
}

func (svr *web3Handler) parseBlockRange(fromStr string, toStr string) (from uint64, to uint64, err error) {
	from, err = svr.parseBlockNumber(fromStr)
	if err != nil {
		return
	}
	to, err = svr.parseBlockNumber(toStr)
	return
}

func (svr *web3Handler) ethTxToEnvelope(tx *types.Transaction) (action.Envelope, error) {
	to := ""
	if tx.To() != nil {
		ioAddr, _ := address.FromBytes(tx.To().Bytes())
		to = ioAddr.String()
	}
	elpBuilder := (&action.EnvelopeBuilder{}).SetChainID(svr.coreService.ChainID())
	if to == address.StakingProtocolAddr {
		return elpBuilder.BuildStakingAction(tx)
	}
	if to == address.RewardingProtocol {
		return elpBuilder.BuildRewardingAction(tx)
	}
	isContract, err := svr.checkContractAddr(to)
	if err != nil {
		return nil, err
	}
	if isContract {
		return elpBuilder.BuildExecution(tx)
	}
	return elpBuilder.BuildTransfer(tx)
}

func (svr *web3Handler) checkContractAddr(to string) (bool, error) {
	if to == "" {
		return true, nil
	}
	ioAddr, err := address.FromString(to)
	if err != nil {
		return false, err
	}
	accountMeta, _, err := svr.coreService.Account(ioAddr)
	if err != nil {
		return false, err
	}
	return accountMeta.IsContract, nil
}

func (svr *web3Handler) getLogsWithFilter(from uint64, to uint64, addrs []string, topics [][]string) ([]*getLogsResult, error) {
	filter, err := newLogFilterFrom(addrs, topics)
	if err != nil {
		return nil, err
	}
	logs, hashes, err := svr.coreService.LogsInRange(filter, from, to, 0)
	if err != nil {
		return nil, err
	}
	ret := make([]*getLogsResult, 0, len(logs))
	for i := range logs {
		ret = append(ret, &getLogsResult{hashes[i], logs[i]})
	}
	return ret, nil
}

// construct filter topics and addresses
func newLogFilterFrom(addrs []string, topics [][]string) (*logfilter.LogFilter, error) {
	filter := iotexapi.LogsFilter{}
	for _, ethAddr := range addrs {
		ioAddr, err := ethAddrToIoAddr(ethAddr)
		if err != nil {
			return nil, err
		}
		filter.Address = append(filter.Address, ioAddr.String())
	}
	for _, tp := range topics {
		var topic [][]byte
		for _, str := range tp {
			b, err := hexToBytes(str)
			if err != nil {
				return nil, err
			}
			topic = append(topic, b)
		}
		filter.Topics = append(filter.Topics, &iotexapi.Topics{Topic: topic})
	}
	return logfilter.NewLogFilter(&filter), nil
}

func byteToHex(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}

func hexToBytes(str string) ([]byte, error) {
	str = util.Remove0xPrefix(str)
	if len(str)%2 == 1 {
		str = "0" + str
	}
	return hex.DecodeString(str)
}

func parseLogRequest(in gjson.Result) (*filterObject, error) {
	if !in.Exists() {
		return nil, errInvalidFormat
	}
	var logReq filterObject
	if len(in.Array()) > 0 {
		req := in.Array()[0]
		logReq.FromBlock = req.Get("fromBlock").String()
		logReq.ToBlock = req.Get("toBlock").String()
		for _, addr := range req.Get("address").Array() {
			logReq.Address = append(logReq.Address, addr.String())
		}
		for _, topics := range req.Get("topics").Array() {
			var topicArr []string
			if topics.IsArray() {
				for _, topic := range topics.Array() {
					topicArr = append(topicArr, util.Remove0xPrefix(topic.String()))
				}
			} else if str := topics.String(); str != "" {
				topicArr = append(topicArr, util.Remove0xPrefix(str))
			}
			logReq.Topics = append(logReq.Topics, topicArr)
		}
	}
	return &logReq, nil
}

type callMsg struct {
	From        address.Address  // the sender of the 'transaction'
	To          string           // the destination contract (empty for contract creation)
	Gas         uint64           // if 0, the call executes with near-infinite gas
	GasPrice    *big.Int         // wei <-> gas exchange ratio
	GasFeeCap   *big.Int         // EIP-1559 fee cap per gas.
	GasTipCap   *big.Int         // EIP-1559 tip per gas.
	Value       *big.Int         // amount of wei sent along with the call
	Data        []byte           // input data, usually an ABI-encoded contract method invocation
	AccessList  types.AccessList // EIP-2930 access list.
	BlockNumber rpc.BlockNumber
}

func parseCallObject(in *gjson.Result) (*callMsg, error) {
	var (
		from      address.Address
		to        string
		gasLimit  uint64
		gasPrice  *big.Int = big.NewInt(0)
		gasTipCap *big.Int
		gasFeeCap *big.Int
		value     *big.Int = big.NewInt(0)
		data      []byte
		acl       types.AccessList
		bn        = rpc.LatestBlockNumber
		err       error
	)
	fromStr := in.Get("params.0.from").String()
	if fromStr == "" {
		fromStr = "0x0000000000000000000000000000000000000000"
	}
	if from, err = ethAddrToIoAddr(fromStr); err != nil {
		return nil, err
	}

	toStr := in.Get("params.0.to").String()
	if toStr != "" {
		ioAddr, err := ethAddrToIoAddr(toStr)
		if err != nil {
			return nil, err
		}
		to = ioAddr.String()
	}

	gasStr := in.Get("params.0.gas").String()
	if gasStr != "" {
		if gasLimit, err = hexStringToNumber(gasStr); err != nil {
			return nil, err
		}
	}

	gasPriceStr := in.Get("params.0.gasPrice").String()
	if gasPriceStr != "" {
		var ok bool
		if gasPrice, ok = new(big.Int).SetString(util.Remove0xPrefix(gasPriceStr), 16); !ok {
			return nil, errors.Wrapf(errUnkownType, "gasPrice: %s", gasPriceStr)
		}
	}

	if gasTipCapStr := in.Get("params.0.maxPriorityFeePerGas").String(); gasTipCapStr != "" {
		var ok bool
		if gasTipCap, ok = new(big.Int).SetString(util.Remove0xPrefix(gasTipCapStr), 16); !ok {
			return nil, errors.Wrapf(errUnkownType, "gasTipCap: %s", gasTipCapStr)
		}
	}

	if gasFeeCapStr := in.Get("params.0.maxFeePerGas").String(); gasFeeCapStr != "" {
		var ok bool
		if gasFeeCap, ok = new(big.Int).SetString(util.Remove0xPrefix(gasFeeCapStr), 16); !ok {
			return nil, errors.Wrapf(errUnkownType, "gasFeeCap: %s", gasFeeCapStr)
		}
	}

	valStr := in.Get("params.0.value").String()
	if valStr != "" {
		var ok bool
		if value, ok = new(big.Int).SetString(util.Remove0xPrefix(valStr), 16); !ok {
			return nil, errors.Wrapf(errUnkownType, "value: %s", valStr)
		}
	}

	if input := in.Get("params.0.input"); input.Exists() {
		data = common.FromHex(input.String())
	} else {
		data = common.FromHex(in.Get("params.0.data").String())
	}

	if accessList := in.Get("params.0.accessList"); accessList.Exists() {
		acl = types.AccessList{}
		log.L().Info("raw acl", zap.String("accessList", accessList.Raw))
		if err := json.Unmarshal([]byte(accessList.Raw), &acl); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal access list %s", accessList.Raw)
		}
	}
	if bnParam := in.Get("params.1"); bnParam.Exists() {
		if err = bn.UnmarshalJSON([]byte(bnParam.String())); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal height %s", bnParam.String())
		}
		if bn == rpc.PendingBlockNumber {
			return nil, errors.Wrap(errNotImplemented, "pending block number is not supported")
		}
	}
	return &callMsg{
		From:        from,
		To:          to,
		Gas:         gasLimit,
		GasPrice:    gasPrice,
		GasFeeCap:   gasFeeCap,
		GasTipCap:   gasTipCap,
		Value:       value,
		Data:        data,
		AccessList:  acl,
		BlockNumber: bn,
	}, nil
}

func parseBlockNumber(in *gjson.Result) (rpc.BlockNumber, error) {
	if !in.Exists() {
		return rpc.LatestBlockNumber, nil
	}
	var height rpc.BlockNumber
	if err := height.UnmarshalJSON([]byte(in.String())); err != nil {
		return 0, err
	}
	if height == rpc.PendingBlockNumber {
		return 0, errors.Wrap(errNotImplemented, "pending block number is not supported")
	}
	return height, nil
}

func blockNumberToHeight(bn rpc.BlockNumber) (uint64, bool) {
	switch bn {
	case rpc.SafeBlockNumber, rpc.FinalizedBlockNumber, rpc.LatestBlockNumber:
		return 0, false
	case rpc.EarliestBlockNumber:
		return 1, true
	default:
		return uint64(bn), true
	}
}

func (call *callMsg) toUnsignedTx(chainID uint32) (*types.Transaction, error) {
	var (
		tx     *types.Transaction
		toAddr *common.Address
	)
	if len(call.To) != 0 {
		addr, err := addrutil.IoAddrToEvmAddr(call.To)
		if err != nil {
			return nil, err
		}
		toAddr = &addr
	}
	switch {
	case call.GasFeeCap != nil || call.GasTipCap != nil:
		tx = types.NewTx(&types.DynamicFeeTx{
			ChainID:    big.NewInt(int64(chainID)),
			GasTipCap:  big.NewInt(0),
			GasFeeCap:  big.NewInt(0),
			Gas:        call.Gas,
			To:         toAddr,
			Value:      call.Value,
			Data:       call.Data,
			AccessList: call.AccessList,
		})
	case call.AccessList != nil:
		tx = types.NewTx(&types.AccessListTx{
			ChainID:    big.NewInt(int64(chainID)),
			GasPrice:   big.NewInt(0),
			Gas:        call.Gas,
			To:         toAddr,
			Value:      call.Value,
			Data:       call.Data,
			AccessList: call.AccessList,
		})
	default:
		tx = types.NewTx(&types.LegacyTx{
			GasPrice: big.NewInt(0),
			Gas:      call.Gas,
			To:       toAddr,
			Value:    call.Value,
			Data:     call.Data,
		})
	}
	return tx, nil
}

func (svr *web3Handler) getLogQueryRange(fromStr, toStr string, logHeight uint64) (from uint64, to uint64, hasNewLogs bool, err error) {
	if from, to, err = svr.parseBlockRange(fromStr, toStr); err != nil {
		return
	}
	switch {
	case logHeight < from:
		hasNewLogs = true
		return
	case logHeight > to:
		hasNewLogs = false
		return
	default:
		from = logHeight
		hasNewLogs = true
		return
	}
}

func loadFilterFromCache(c apiCache, filterID string) (filterObject, error) {
	dataStr, isFound := c.Get(filterID)
	if !isFound {
		return filterObject{}, errInvalidFilterID
	}
	var filterObj filterObject
	if err := json.Unmarshal([]byte(dataStr), &filterObj); err != nil {
		return filterObject{}, err
	}
	return filterObj, nil
}

func newAPICache(expireTime time.Duration, remoteURL string) apiCache {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     remoteURL,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	if redisClient.Ping(context.Background()).Err() != nil {
		log.L().Info("local cache is used as API cache")
		filterCache, _ := ttl.NewCache(ttl.AutoExpireOption(expireTime))
		return &localCache{
			ttlCache: filterCache,
		}
	}
	log.L().Info("remote cache is used as API cache")
	return &remoteCache{
		redisCache: redisClient,
		expireTime: expireTime,
	}
}

type apiCache interface {
	Set(key string, data []byte) error
	Del(key string) bool
	Get(key string) ([]byte, bool)
}

type localCache struct {
	ttlCache *ttl.Cache
}

func (c *localCache) Set(key string, data []byte) error {
	if c.ttlCache == nil {
		return errNullPointer
	}
	c.ttlCache.Set(key, data)
	return nil
}

func (c *localCache) Del(key string) bool {
	if c.ttlCache == nil {
		return false
	}
	return c.ttlCache.Delete(key)
}

func (c *localCache) Get(key string) ([]byte, bool) {
	if c.ttlCache == nil {
		return nil, false
	}
	val, exist := c.ttlCache.Get(key)
	if !exist {
		return nil, false
	}
	ret, ok := val.([]byte)
	return ret, ok
}

type remoteCache struct {
	redisCache *redis.Client
	expireTime time.Duration
}

func (c *remoteCache) Set(key string, data []byte) error {
	if c.redisCache == nil {
		return errNullPointer
	}
	return c.redisCache.Set(context.Background(), key, data, c.expireTime).Err()
}

func (c *remoteCache) Del(key string) bool {
	if c.redisCache == nil {
		return false
	}
	err := c.redisCache.Unlink(context.Background(), key).Err()
	return err == nil
}

func (c *remoteCache) Get(key string) ([]byte, bool) {
	if c.redisCache == nil {
		return nil, false
	}
	ret, err := c.redisCache.Get(context.Background(), key).Bytes()
	if err == redis.Nil {
		return nil, false
	} else if err != nil {
		return nil, false
	}
	c.redisCache.Expire(context.Background(), key, c.expireTime)
	return ret, true
}

// fromLoggerStructLogs converts logger.StructLog to apitypes.StructLog
func fromLoggerStructLogs(logs []logger.StructLog) []apitypes.StructLog {
	ret := make([]apitypes.StructLog, len(logs))
	for index, log := range logs {
		ret[index] = apitypes.StructLog{
			Pc:            log.Pc,
			Op:            log.Op,
			Gas:           math.HexOrDecimal64(log.Gas),
			GasCost:       math.HexOrDecimal64(log.GasCost),
			Memory:        log.Memory,
			MemorySize:    log.MemorySize,
			Stack:         log.Stack,
			ReturnData:    log.ReturnData,
			Storage:       log.Storage,
			Depth:         log.Depth,
			RefundCounter: log.RefundCounter,
			OpName:        log.OpName(),
			ErrorString:   log.ErrorString(),
		}
	}
	return ret
}

func newGetTransactionResult(
	blkHash *hash.Hash256,
	selp *action.SealedEnvelope,
	receipt *action.Receipt,
	evmChainID uint32,
) (*getTransactionResult, error) {
	ethTx, err := selp.ToEthTx()
	if err != nil {
		return nil, err
	}
	var to *string
	if ethTx.To() != nil {
		tmp := ethTx.To().String()
		to = &tmp
	}
	var (
		tx *types.Transaction
	)
	if _, ok := selp.Envelope.(action.TxContainer); ok {
		tx = ethTx
	} else {
		signer, err := action.NewEthSigner(iotextypes.Encoding(selp.Encoding()), evmChainID)
		if err != nil {
			return nil, err
		}
		tx, err = action.RawTxToSignedTx(ethTx, signer, selp.Signature())
		if err != nil {
			return nil, err
		}
	}
	return &getTransactionResult{
		blockHash: blkHash,
		to:        to,
		ethTx:     tx,
		receipt:   receipt,
		pubkey:    selp.SrcPubkey(),
	}, nil
}

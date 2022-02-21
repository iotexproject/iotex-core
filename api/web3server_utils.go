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

	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
)

type (
	blockObject struct {
		Author           string        `json:"author"`
		Number           string        `json:"number"`
		Hash             string        `json:"hash"`
		ParentHash       string        `json:"parentHash"`
		Sha3Uncles       string        `json:"sha3Uncles"`
		LogsBloom        string        `json:"logsBloom"`
		TransactionsRoot string        `json:"transactionsRoot"`
		StateRoot        string        `json:"stateRoot"`
		ReceiptsRoot     string        `json:"receiptsRoot"`
		Miner            string        `json:"miner"`
		Difficulty       string        `json:"difficulty"`
		TotalDifficulty  string        `json:"totalDifficulty"`
		ExtraData        string        `json:"extraData"`
		Size             string        `json:"size"`
		GasLimit         string        `json:"gasLimit"`
		GasUsed          string        `json:"gasUsed"`
		Timestamp        string        `json:"timestamp"`
		Transactions     []interface{} `json:"transactions"`
		Signature        string        `json:"signature"`
		Step             string        `json:"step"`
		Uncles           []string      `json:"uncles"`
	}

	transactionObject struct {
		Hash             string  `json:"hash"`
		Nonce            string  `json:"nonce"`
		BlockHash        string  `json:"blockHash"`
		BlockNumber      string  `json:"blockNumber"`
		TransactionIndex string  `json:"transactionIndex"`
		From             string  `json:"from"`
		To               *string `json:"to"`
		Value            string  `json:"value"`
		GasPrice         string  `json:"gasPrice"`
		Gas              string  `json:"gas"`
		Input            string  `json:"input"`
		R                string  `json:"r"`
		S                string  `json:"s"`
		V                string  `json:"v"`
		StandardV        string  `json:"standardV"`
		Condition        *string `json:"condition"`
		Creates          *string `json:"creates"`
		ChainID          string  `json:"chainId"`
		PublicKey        string  `json:"publicKey"`
	}
)

const (
	_zeroLogsBloom = "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
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

func intStrToHex(str string) (string, error) {
	amount, ok := big.NewInt(0).SetString(str, 10)
	if !ok {
		return "", errors.Wrapf(errUnkownType, "int: %s", str)
	}
	return "0x" + fmt.Sprintf("%x", amount), nil
}

func getStringFromArray(in interface{}, i int) (string, error) {
	params, ok := in.([]interface{})
	if !ok || i < 0 || i >= len(params) {
		return "", errInvalidFormat
	}
	ret, ok := params[i].(string)
	if !ok {
		return "", errUnkownType
	}
	return ret, nil
}

func getStringAndBoolFromArray(in interface{}) (str string, b bool, err error) {
	params, ok := in.([]interface{})
	if !ok || len(params) != 2 {
		err = errInvalidFormat
		return
	}
	str, ok = params[0].(string)
	if !ok {
		err = errUnkownType
		return
	}
	b, ok = params[1].(bool)
	if !ok {
		err = errUnkownType
		return
	}
	return
}

func (svr *Web3Server) getBlockWithTransactions(blkMeta *iotextypes.BlockMeta, isDetailed bool) (blockObject, error) {
	transactions := make([]interface{}, 0)
	if blkMeta.Height > 0 {
		actionInfos, err := svr.coreService.ActionsByBlock(blkMeta.Hash, 0, svr.coreService.cfg.API.RangeQueryLimit)
		if err != nil {
			return blockObject{}, err
		}
		for _, info := range actionInfos {
			if isDetailed {
				tx, err := svr.getTransactionFromActionInfo(info)
				if err != nil {
					if errors.Cause(err) != errUnsupportedAction {
						log.Logger("api").Error("failed to get info from action", zap.Error(err), zap.String("info", fmt.Sprintf("%+v", info)))
					}
					continue
				}
				transactions = append(transactions, tx)
			} else {
				transactions = append(transactions, "0x"+info.ActHash)
			}
		}
	}

	producerAddr, err := ioAddrToEthAddr(blkMeta.ProducerAddress)
	if err != nil {
		return blockObject{}, err
	}
	// TODO: the value is the same as Babel's. It will be corrected in next pr
	return blockObject{
		Author:           producerAddr,
		Number:           uint64ToHex(blkMeta.Height),
		Hash:             "0x" + blkMeta.Hash,
		ParentHash:       "0x" + blkMeta.PreviousBlockHash,
		Sha3Uncles:       "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		LogsBloom:        getLogsBloomFromBlkMeta(blkMeta),
		TransactionsRoot: "0x" + blkMeta.TxRoot,
		StateRoot:        "0x" + blkMeta.DeltaStateDigest,
		ReceiptsRoot:     "0x" + blkMeta.TxRoot,
		Miner:            producerAddr,
		Difficulty:       "0xfffffffffffffffffffffffffffffffe",
		TotalDifficulty:  "0xff14700000000000000000000000486001d72",
		ExtraData:        "0x",
		Size:             uint64ToHex(uint64(blkMeta.NumActions)),
		GasLimit:         uint64ToHex(blkMeta.GasLimit),
		GasUsed:          uint64ToHex(blkMeta.GasUsed),
		Timestamp:        uint64ToHex(uint64(blkMeta.Timestamp.Seconds)),
		Transactions:     transactions,
		Step:             "373422302",
		Signature:        "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		Uncles:           []string{},
	}, nil
}

func (svr *Web3Server) getTransactionFromActionInfo(actInfo *iotexapi.ActionInfo) (transactionObject, error) {
	if actInfo.GetAction() == nil || actInfo.GetAction().GetCore() == nil {
		return transactionObject{}, errNullPointer
	}
	var (
		to     *string
		create *string
		value  = "0x0"
		data   = "0x"
		err    error
	)
	switch act := actInfo.Action.Core.Action.(type) {
	case *iotextypes.ActionCore_Transfer:
		value, err = intStrToHex(act.Transfer.GetAmount())
		if err != nil {
			return transactionObject{}, err
		}
		toTmp, err := ioAddrToEthAddr(act.Transfer.GetRecipient())
		if err != nil {
			return transactionObject{}, err
		}
		to = &toTmp
	case *iotextypes.ActionCore_Execution:
		value, err = intStrToHex(act.Execution.GetAmount())
		if err != nil {
			return transactionObject{}, err
		}
		if len(act.Execution.GetContract()) > 0 {
			toTmp, err := ioAddrToEthAddr(act.Execution.GetContract())
			if err != nil {
				return transactionObject{}, err
			}
			to = &toTmp
		}
		data = byteToHex(act.Execution.GetData())
		// recipient is empty when contract is created
		if to == nil {
			actHash, err := hash.HexStringToHash256(actInfo.ActHash)
			if err != nil {
				return transactionObject{}, errors.Wrapf(errUnkownType, "txHash: %s", actInfo.ActHash)
			}
			receipt, _, err := svr.coreService.ReceiptByAction(actHash)
			if err != nil {
				return transactionObject{}, err
			}
			addr, err := ioAddrToEthAddr(receipt.ContractAddress)
			if err != nil {
				return transactionObject{}, err
			}
			create = &addr
		}
	// TODO: support other type actions
	default:
		return transactionObject{}, errors.Wrapf(errUnsupportedAction, "actHash: %s", actInfo.ActHash)
	}

	vVal := uint64(actInfo.Action.Signature[64])
	if vVal < 27 {
		vVal += 27
	}

	from, err := ioAddrToEthAddr(actInfo.Sender)
	if err != nil {
		return transactionObject{}, err
	}
	gasPrice, err := intStrToHex(actInfo.Action.Core.GasPrice)
	if err != nil {
		return transactionObject{}, err
	}
	return transactionObject{
		Hash:             "0x" + actInfo.ActHash,
		Nonce:            uint64ToHex(actInfo.Action.Core.Nonce),
		BlockHash:        "0x" + actInfo.BlkHash,
		BlockNumber:      uint64ToHex(actInfo.BlkHeight),
		TransactionIndex: uint64ToHex(uint64(actInfo.Index)),
		From:             from,
		To:               to,
		Value:            value,
		GasPrice:         gasPrice,
		Gas:              uint64ToHex(actInfo.Action.Core.GasLimit),
		Input:            data,
		R:                byteToHex(actInfo.Action.Signature[:32]),
		S:                byteToHex(actInfo.Action.Signature[32:64]),
		V:                uint64ToHex(vVal),
		// TODO: the value is the same as Babel's. It will be corrected in next pr
		StandardV: uint64ToHex(vVal),
		Creates:   create,
		ChainID:   uint64ToHex(uint64(svr.coreService.EVMNetworkID())),
		PublicKey: byteToHex(actInfo.Action.SenderPubKey),
	}, nil
}

func (svr *Web3Server) parseBlockNumber(str string) (uint64, error) {
	switch str {
	case _earliestBlockNumber:
		return 1, nil
	case "", _pendingBlockNumber, _latestBlockNumber:
		return svr.coreService.bc.TipHeight(), nil
	default:
		return hexStringToNumber(str)
	}
}

func (svr *Web3Server) parseBlockRange(fromStr string, toStr string) (from uint64, to uint64, err error) {
	from, err = svr.parseBlockNumber(fromStr)
	if err != nil {
		return
	}
	to, err = svr.parseBlockNumber(toStr)
	return
}

func (svr *Web3Server) isContractAddr(addr string) (bool, error) {
	if addr == "" {
		return true, nil
	}
	ioAddr, err := address.FromString(addr)
	if err != nil {
		return false, err
	}
	accountMeta, _, err := svr.coreService.Account(ioAddr)
	if err != nil {
		return false, err
	}
	return accountMeta.IsContract, nil
}

func (svr *Web3Server) getLogsWithFilter(from uint64, to uint64, addrs []string, topics [][]string) ([]logsObject, error) {
	// construct filter topics and addresses
	var filter iotexapi.LogsFilter
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
	logs, err := svr.coreService.LogsInRange(logfilter.NewLogFilter(&filter, nil, nil), from, to, 0)
	if err != nil {
		return nil, err
	}

	// parse log results
	ret := make([]logsObject, 0)
	for _, l := range logs {
		topics := make([]string, 0)
		for _, val := range l.Topics {
			topics = append(topics, byteToHex(val))
		}
		contractAddr, err := ioAddrToEthAddr(l.ContractAddress)
		if err != nil {
			return nil, err
		}
		ret = append(ret, logsObject{
			BlockHash:        byteToHex(l.BlkHash),
			TransactionHash:  byteToHex(l.ActHash),
			LogIndex:         uint64ToHex(uint64(l.Index)),
			BlockNumber:      uint64ToHex(l.BlkHeight),
			TransactionIndex: uint64ToHex(uint64(l.TxIndex)),
			Address:          contractAddr,
			Data:             byteToHex(l.Data),
			Topics:           topics,
		})
	}
	return ret, nil
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

func getLogsBloomFromBlkMeta(blkMeta *iotextypes.BlockMeta) string {
	if len(blkMeta.LogsBloom) == 0 {
		return _zeroLogsBloom
	}
	return "0x" + blkMeta.LogsBloom
}

func parseLogRequest(in gjson.Result) (*filterObject, error) {
	var logReq filterObject
	if len(in.Array()) > 0 {
		req := in.Array()[0]
		logReq.FromBlock = req.Get("fromBlock").String()
		logReq.ToBlock = req.Get("toBlock").String()
		for _, addr := range req.Get("address").Array() {
			logReq.Address = append(logReq.Address, addr.String())
		}
		for _, topics := range req.Get("topics").Array() {
			if topics.IsArray() {
				var topicArr []string
				for _, topic := range topics.Array() {
					topicArr = append(topicArr, util.Remove0xPrefix(topic.String()))
				}
				logReq.Topics = append(logReq.Topics, topicArr)
			} else {
				logReq.Topics = append(logReq.Topics, []string{util.Remove0xPrefix(topics.String())})
			}
		}
	}
	return &logReq, nil
}

func parseCallObject(in interface{}) (address.Address, string, uint64, *big.Int, []byte, error) {
	var (
		from     address.Address
		to       string
		gasLimit uint64
		value    *big.Int
		data     []byte
	)

	params, ok := in.([]interface{})
	if !ok {
		return nil, "", 0, nil, nil, errInvalidFormat
	}
	params0, ok := params[0].(map[string]interface{})
	if !ok {
		return nil, "", 0, nil, nil, errInvalidFormat
	}
	req, err := json.Marshal(params0)
	if err != nil {
		return nil, "", 0, nil, nil, err
	}
	callObj := struct {
		From     string `json:"from,omitempty"`
		To       string `json:"to,omitempty"`
		Gas      string `json:"gas,omitempty"`
		GasPrice string `json:"gasPrice,omitempty"`
		Value    string `json:"value,omitempty"`
		Data     string `json:"data,omitempty"`
	}{}
	if err = json.Unmarshal(req, &callObj); err != nil {
		return nil, "", 0, nil, nil, err
	}
	if callObj.To != "" {
		var ioAddr address.Address
		if ioAddr, err = ethAddrToIoAddr(callObj.To); err != nil {
			return nil, "", 0, nil, nil, err
		}
		to = ioAddr.String()
	}
	if callObj.From == "" {
		callObj.From = "0x0000000000000000000000000000000000000000"
	}
	if from, err = ethAddrToIoAddr(callObj.From); err != nil {
		return nil, "", 0, nil, nil, err
	}
	if callObj.Value != "" {
		value, ok = big.NewInt(0).SetString(util.Remove0xPrefix(callObj.Value), 16)
		if !ok {
			return nil, "", 0, nil, nil, errors.Wrapf(errUnkownType, "value: %s", callObj.Value)
		}
	} else {
		value = big.NewInt(0)
	}
	if callObj.Gas != "" {
		if gasLimit, err = hexStringToNumber(callObj.Gas); err != nil {
			return nil, "", 0, nil, nil, err
		}
	}
	data = common.FromHex(callObj.Data)
	return from, to, gasLimit, value, data, nil
}

func (svr *Web3Server) getLogQueryRange(fromStr, toStr string, logHeight uint64) (from uint64, to uint64, hasNewLogs bool, err error) {
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

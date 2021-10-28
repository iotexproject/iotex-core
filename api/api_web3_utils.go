package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

type (
	web3Utils struct {
		errMsg error
	}
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
		To               *string `json:"to,omitempty"`
		Value            string  `json:"value"`
		GasPrice         string  `json:"gasPrice"`
		Gas              string  `json:"gas"`
		Input            string  `json:"input"`
		R                string  `json:"r"`
		S                string  `json:"s"`
		V                string  `json:"v"`
		StandardV        string  `json:"standardV"`
		Condition        string  `json:"condition"`
		Creates          string  `json:"creates"`
		ChainID          string  `json:"chainId"`
		PublicKey        string  `json:"publicKey"`
	}
)

func (h *web3Utils) hexStringToNumber(hexStr string) uint64 {
	if h.errMsg != nil {
		return 0
	}
	if hexStr == "" {
		return 0
	}
	var num uint64
	num, h.errMsg = strconv.ParseUint(removeHexPrefix(hexStr), 16, 64)
	return num
}

func (h *web3Utils) ethAddrToIoAddr(ethAddr string) string {
	if h.errMsg != nil {
		return ""
	}
	if ok := common.IsHexAddress(ethAddr); !ok {
		h.errMsg = errUnkownType
		return ""
	}
	ethAddress := common.HexToAddress(ethAddr)
	var ioAddress address.Address
	ioAddress, h.errMsg = address.FromBytes(ethAddress.Bytes())
	return ioAddress.String()
}

func (h *web3Utils) ioAddrToEthAddr(ioAddr string) string {
	if h.errMsg != nil {
		return ""
	}
	if len(ioAddr) == 0 {
		return "0x0000000000000000000000000000000000000000"
	}
	var addr common.Address
	addr, h.errMsg = util.IoAddrToEvmAddr(ioAddr)
	if h.errMsg != nil {
		return ""
	}
	return addr.String()
}

func uint64ToHex(val uint64) string {
	return "0x" + strconv.FormatUint(val, 16)
}

func (h *web3Utils) intStrToHex(str string) string {
	if h.errMsg != nil {
		return ""
	}
	amount, ok := big.NewInt(0).SetString(str, 10)
	if !ok {
		h.errMsg = errUnkownType
		return ""
	}
	return "0x" + fmt.Sprintf("%x", amount)
}

func (h *web3Utils) getStringFromArray(in interface{}, i int) string {
	if h.errMsg != nil {
		return ""
	}
	params, ok := in.([]interface{})
	if !ok || i < 0 || i >= len(params) {
		h.errMsg = errUnkownType
		return ""
	}
	ret, ok := params[i].(string)
	if !ok {
		h.errMsg = errUnkownType
		return ""
	}
	return ret
}

func (h *web3Utils) getBoolFromArray(in interface{}, i int) bool {
	if h.errMsg != nil {
		return false
	}
	params, ok := in.([]interface{})
	if !ok || i < 0 || i >= len(params) {
		h.errMsg = errUnkownType
		return false
	}
	ret, ok := params[i].(bool)
	if !ok {
		h.errMsg = errUnkownType
		return false
	}
	return ret
}

func (h *web3Utils) getJSONFromArray(in interface{}) []byte {
	if h.errMsg != nil {
		return nil
	}
	params, ok := in.([]interface{})
	if !ok {
		h.errMsg = errUnkownType
		return nil
	}
	params0, ok := params[0].(map[string]interface{})
	if !ok {
		h.errMsg = errUnkownType
		return nil
	}
	jsonMarshaled, err := json.Marshal(params0)
	if err != nil {
		h.errMsg = err
		return nil
	}
	return jsonMarshaled
}

func removeHexPrefix(hexStr string) string {
	ret := strings.Replace(hexStr, "0x", "", -1)
	ret = strings.Replace(ret, "0X", "", -1)
	return ret
}

func (h *web3Utils) getBlockWithTransactions(svr *Server, blkMeta *iotextypes.BlockMeta, isDetailed bool) *blockObject {
	if h.errMsg != nil {
		return nil
	}
	transactionsRoot := "0x"
	var transactions []interface{}
	if blkMeta.Height > 0 {
		var ret *iotexapi.GetActionsResponse
		ret, h.errMsg = svr.getActionsByBlock(context.Background(), blkMeta.Hash, 0, svr.cfg.API.RangeQueryLimit)
		if h.errMsg != nil {
			return nil
		}

		for _, v := range ret.ActionInfo {
			if isDetailed {
				tx := h.getTransactionFromActionInfo(v, config.EVMNetworkID())
				transactions = append(transactions, tx)
			} else {
				transactions = append(transactions, "0x"+v.ActHash)
			}
		}
		transactionsRoot = "0x" + blkMeta.TxRoot
	}

	if len(transactions) == 0 {
		transactionsRoot = "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
	}
	bloom := "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	if len(blkMeta.LogsBloom) > 0 {
		bloom = blkMeta.LogsBloom
	}
	miner := h.ioAddrToEthAddr(blkMeta.ProducerAddress)
	author := h.ioAddrToEthAddr(blkMeta.ProducerAddress)
	if h.errMsg != nil {
		return nil
	}
	return &blockObject{
		Author:           author,
		Number:           uint64ToHex(blkMeta.Height),
		Hash:             "0x" + blkMeta.Hash,
		ParentHash:       "0x" + blkMeta.PreviousBlockHash,
		Sha3Uncles:       "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		LogsBloom:        "0x" + bloom,
		TransactionsRoot: transactionsRoot,
		StateRoot:        "0x" + blkMeta.DeltaStateDigest,
		ReceiptsRoot:     "0x" + blkMeta.TxRoot,
		Miner:            miner,
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
	}
}

func (h *web3Utils) getTransactionFromActionInfo(actInfo *iotexapi.ActionInfo, chainID uint32) transactionObject {
	if h.errMsg != nil {
		return transactionObject{}
	}
	if actInfo.GetAction() == nil || actInfo.GetAction().GetCore() == nil {
		return transactionObject{}
	}
	var to *string
	value := "0x0"
	data := ""
	switch act := actInfo.Action.Core.Action.(type) {
	case *iotextypes.ActionCore_Transfer:
		value = h.intStrToHex(act.Transfer.GetAmount())
		toTmp := h.ioAddrToEthAddr(act.Transfer.GetRecipient())
		to = &toTmp
	case *iotextypes.ActionCore_Execution:
		value = h.intStrToHex(act.Execution.GetAmount())
		if len(act.Execution.GetContract()) > 0 {
			toTmp := h.ioAddrToEthAddr(act.Execution.GetContract())
			to = &toTmp
		}
		data = byteToHex(act.Execution.GetData())
	default:
		return transactionObject{}
	}

	vVal := uint64(actInfo.Action.Signature[64])
	if vVal < 27 {
		vVal += 27
	}

	from := h.ioAddrToEthAddr(actInfo.Sender)
	gasPrice := h.intStrToHex(actInfo.Action.Core.GasPrice)
	if h.errMsg != nil {
		return transactionObject{}
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
		StandardV:        uint64ToHex(vVal),
		ChainID:          uint64ToHex(uint64(chainID)),
		PublicKey:        byteToHex(actInfo.Action.SenderPubKey),
	}
}

func (h *web3Utils) getTransactionCreateFromActionInfo(svr *Server, actInfo *iotexapi.ActionInfo, chainID uint32) transactionObject {
	if h.errMsg != nil {
		return transactionObject{}
	}
	tx := h.getTransactionFromActionInfo(actInfo, chainID)
	if h.errMsg != nil {
		return transactionObject{}
	}
	if tx.To == nil {
		var receipt *iotexapi.GetReceiptByActionResponse
		receipt, h.errMsg = svr.GetReceiptByAction(context.Background(),
			&iotexapi.GetReceiptByActionRequest{
				ActionHash: (tx.Hash)[2:],
			},
		)
		if h.errMsg != nil {
			return transactionObject{}
		}
		addr := h.ioAddrToEthAddr(receipt.ReceiptInfo.Receipt.ContractAddress)
		tx.Creates = addr
	}
	return tx
}

// DecodeRawTx() decode raw data string into eth tx
func (h *web3Utils) DecodeRawTx(rawData string, chainID uint32) (tx *types.Transaction, sig []byte, pubkey crypto.PublicKey) {
	if h.errMsg != nil {
		return
	}

	var dataInString []byte
	dataInString, h.errMsg = hex.DecodeString(removeHexPrefix(rawData))
	if h.errMsg != nil {
		return
	}

	// decode raw data into rlp tx
	tx = &types.Transaction{}
	h.errMsg = rlp.DecodeBytes(dataInString, tx)
	if h.errMsg != nil {
		return
	}

	// extract signature and recover pubkey
	v, r, s := tx.RawSignatureValues()
	recID := uint32(v.Int64()) - 2*chainID - 8
	sig = make([]byte, 64, 65)
	rSize := len(r.Bytes())
	copy(sig[32-rSize:32], r.Bytes())
	sSize := len(s.Bytes())
	copy(sig[64-sSize:], s.Bytes())
	sig = append(sig, byte(recID))

	// recover public key
	rawHash := types.NewEIP155Signer(big.NewInt(int64(chainID))).Hash(tx)
	pubkey, h.errMsg = crypto.RecoverPubkey(rawHash[:], sig)
	return
}

func (h *web3Utils) parseBlockNumber(svr *Server, str string) uint64 {
	if h.errMsg != nil {
		return 0
	}
	switch str {
	case "earliest":
		return 1
	case "", "pending", "latest":
		return svr.bc.TipHeight()
	default:
		return h.hexStringToNumber(str)
	}
}

func (h *web3Utils) isContractAddr(svr *Server, addr string) bool {
	if h.errMsg != nil {
		return false
	}
	var accountMeta *iotexapi.GetAccountResponse
	accountMeta, h.errMsg = svr.GetAccount(context.Background(), &iotexapi.GetAccountRequest{
		Address: addr,
	})
	if h.errMsg != nil {
		return false
	}
	return accountMeta.AccountMeta.IsContract
}

func getLogsWithFilter(svr *Server, fromBlock string, toBlock string, addrs []string, topics [][]string) ([]logsObject, error) {
	var web3Util web3Utils
	// construct block range(from, to)
	tipHeight := svr.bc.TipHeight()
	from := web3Util.parseBlockNumber(svr, fromBlock)
	to := web3Util.parseBlockNumber(svr, toBlock)
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	if from > tipHeight {
		return nil, status.Error(codes.InvalidArgument, "start block > tip height")
	}
	if to > tipHeight {
		to = tipHeight
	}

	// construct filter topics and addresses
	var filter iotexapi.LogsFilter
	for _, ethAddr := range addrs {
		ioAddr := web3Util.ethAddrToIoAddr(ethAddr)
		filter.Address = append(filter.Address, ioAddr)
	}
	for _, tp := range topics {
		var topic [][]byte
		for _, str := range tp {
			b, err := hex.DecodeString(str)
			if err != nil {
				return nil, err
			}
			topic = append(topic, b)
		}
		filter.Topics = append(filter.Topics, &iotexapi.Topics{
			Topic: topic,
		})
	}
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	logs, err := svr.getLogsInRange(logfilter.NewLogFilter(&filter, nil, nil), from, to, 1000)
	if err != nil {
		return nil, err
	}
	// parse log results
	var ret []logsObject
	for _, l := range logs {
		var topics []string
		for _, val := range l.Topics {
			topics = append(topics, byteToHex(val))
		}
		ret = append(ret, logsObject{
			BlockHash:        byteToHex(l.BlkHash),
			TransactionHash:  byteToHex(l.ActHash),
			LogIndex:         uint64ToHex(uint64(l.Index)),
			BlockNumber:      uint64ToHex(l.BlkHeight),
			TransactionIndex: "0x1",
			Address:          web3Util.ioAddrToEthAddr(l.ContractAddress),
			Data:             byteToHex(l.Data),
			Topics:           topics,
		})
	}
	if web3Util.errMsg != nil {
		return nil, web3Util.errMsg
	}
	return ret, nil
}

func byteToHex(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}

func parseLogRequest(in interface{}) (*logsRequest, error) {
	params, ok := in.([]interface{})
	if !ok {
		return nil, errUnkownType
	}
	params0, ok := params[0].(map[string]interface{})
	if !ok {
		return nil, errUnkownType
	}

	var logReq logsRequest
	for k, v := range params0 {
		switch k {
		case "fromBlock":
			if logReq.FromBlock, ok = v.(string); !ok {
				return nil, errUnkownType
			}
		case "toBlock":
			if logReq.ToBlock, ok = v.(string); !ok {
				return nil, errUnkownType
			}
		case "address":
			switch str := v.(type) {
			case string:
				logReq.Address = append(logReq.Address, str)
			case []string:
				logReq.Address = append(logReq.Address, str...)
			default:
				return nil, errUnkownType
			}
		case "topics":
			for _, val := range v.([]interface{}) {
				switch str := val.(type) {
				case string:
					logReq.Topics = append(logReq.Topics, []string{str})
				case []string:
					logReq.Topics = append(logReq.Topics, str)
				default:
					return nil, errUnkownType
				}
			}
		default:
			return nil, errUnkownType
		}
	}
	return &logReq, nil
}

func (h *web3Utils) parseCallObject(in []byte) (from string, to string, gasLimit uint64, value *big.Int, data []byte) {
	if h.errMsg != nil {
		return
	}
	callObj := struct {
		From     string `json:"from,omitempty"`
		To       string `json:"to"`
		Gas      string `json:"gas,omitempty"`
		GasPrice string `json:"gasPrice,omitempty"`
		Value    string `json:"value,omitempty"`
		Data     string `json:"data,omitempty"`
	}{}
	h.errMsg = json.Unmarshal(in, &callObj)
	if h.errMsg != nil {
		return
	}
	if callObj.To == "" {
		h.errMsg = errors.New("missing 'to' address field")
		return
	}

	if callObj.From == "" {
		callObj.From = "0x0000000000000000000000000000000000000000"
	}

	from = h.ethAddrToIoAddr(callObj.From)
	to = h.ethAddrToIoAddr(callObj.To)
	value, ok := big.NewInt(0).SetString(removeHexPrefix(callObj.Value), 16)
	if !ok {
		h.errMsg = errUnkownType
		return
	}
	gasLimit = h.hexStringToNumber(callObj.Gas)
	data = common.FromHex(callObj.Data)
	return
}

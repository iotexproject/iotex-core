package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
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

	blockObject struct {
		Number           string        `json:"number,omitempty"`
		Hash             string        `json:"hash,omitempty"`
		ParentHash       string        `json:"parentHash,omitempty"`
		Sha3Uncles       string        `json:"sha3Uncles,omitempty"`
		LogsBloom        string        `json:"logsBloom,omitempty"`
		TransactionsRoot string        `json:"transactionsRoot,omitempty"`
		StateRoot        string        `json:"stateRoot,omitempty"`
		ReceiptsRoot     string        `json:"receiptsRoot,omitempty"`
		Miner            string        `json:"miner,omitempty"`
		Difficulty       string        `json:"difficulty,omitempty"`
		TotalDifficulty  string        `json:"totalDifficulty,omitempty"`
		ExtraData        string        `json:"extraData,omitempty"`
		Size             string        `json:"size,omitempty"`
		GasLimit         string        `json:"gasLimit,omitempty"`
		GasUsed          string        `json:"gasUsed,omitempty"`
		Timestamp        string        `json:"timestamp,omitempty"`
		Transactions     []interface{} `json:"transactions,omitempty"`
		Uncles           []string      `json:"uncles,omitempty"`
	}

	transactionObject struct {
		Hash             *string `json:"hash,omitempty"`
		Nonce            *string `json:"nonce,omitempty"`
		BlockHash        *string `json:"blockHash,omitempty"`
		BlockNumber      *string `json:"blockNumber,omitempty"`
		TransactionIndex *string `json:"transactionIndex,omitempty"`
		From             *string `json:"from,omitempty"`
		To               *string `json:"to,omitempty"`
		Value            *string `json:"value,omitempty"`
		GasPrice         *string `json:"gasPrice,omitempty"`
		Gas              *string `json:"gas,omitempty"`
		Input            *string `json:"input,omitempty"`
		R                *string `json:"r,omitempty"`
		S                *string `json:"s,omitempty"`
		V                *string `json:"v,omitempty"`
	}
)

var (
	apiMap = map[string]func(*Server, interface{}) (interface{}, error){
		"eth_gasPrice":                       gasPrice,
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
		"eth_coinbase":                       unimplemented,
		"eth_mining":                         isMining,
		"eth_hashrate":                       getHashrate,
		"eth_accounts":                       unimplemented,
		"eth_getStorageAt":                   unimplemented,
		"eth_getBlockTransactionCountByHash": getBlockTransactionCountByHash,
		"eth_getUncleCountByBlockHash":       unimplemented,
		"eth_getUncleCountByBlockNumber":     unimplemented,
		"eth_sign":                           unimplemented,
		"eth_signTransaction":                unimplemented,
		"eth_sendTransaction":                unimplemented,
		"eth_getBlockByHash":                 getBlockByHash,
	}

	TYPE_ERROR          = errors.New("wrong type of params")
	UNIMPLEMENTED_ERROR = status.Error(codes.Unimplemented, "function not implemented")
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

// TODO:
func getBlockByNumber(svr *Server, in interface{}) (interface{}, error) {
	return nil, nil
}

func getBalance(svr *Server, in interface{}) (interface{}, error) {
	addr, ok := in.(string)
	if !ok {
		return nil, TYPE_ERROR
	}
	ioAddr, err := ethAddrToIoAddr(addr)
	if err != nil {
		return nil, err
	}
	accountMeta, _, err := svr.getAccount(ioAddr)
	if err != nil {
		return nil, err
	}
	return accountMeta.Balance, nil
}

func getTransactionCount(svr *Server, in interface{}) (interface{}, error) {
	addr, ok := in.(string)
	if !ok {
		return nil, TYPE_ERROR
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
	params, ok := in.([]interface{})
	if !ok {
		return nil, TYPE_ERROR
	}
	params0, ok := params[0].(map[string]interface{})
	if !ok {
		return nil, TYPE_ERROR
	}
	jsonString, _ := json.Marshal(params0)
	var callObject CallObject
	err := json.Unmarshal(jsonString, &callObject)
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
		return nil, TYPE_ERROR
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
		panic(err)
	}
	return uint64ToHex(uint64(ret[0].NumActions)), nil
}

func getBlockByHash(svr *Server, in interface{}) (interface{}, error) {
	h, err := getStringFromArray(in)
	if err != nil {
		return nil, err
	}
	isDetailed := in.([]interface{})[1].(bool)
	ret, err := svr.getBlockMetaByHash(removeHexPrefix(h))
	if err != nil {
		panic(err)
	}
	blk := getBlockWithTransactions(svr, ret[0], isDetailed)
	return blk, nil
}

func unimplemented(svr *Server, in interface{}) (interface{}, error) {
	return nil, UNIMPLEMENTED_ERROR
}

// func getLogs(svr *Server, in interface{}) (interface{}, error) {

// }

// util functions
func hexStringToNumber(hexStr string) (uint64, error) {
	output, err := strconv.ParseUint(removeHexPrefix(hexStr), 16, 64)
	if err != nil {
		return 0, err
	}
	return output, nil
}

func ethAddrToIoAddr(ethAddr string) (string, error) {
	if ok := common.IsHexAddress(ethAddr); !ok {
		return "", TYPE_ERROR
	}
	ethAddress := common.HexToAddress(ethAddr)
	ioAddress, err := address.FromBytes(ethAddress.Bytes())
	if err != nil {
		return "", TYPE_ERROR
	}
	return ioAddress.String(), nil
}

func ioAddrToEthAddr(ioAddr string) (string, error) {
	addr, err := util.IoAddrToEvmAddr(ioAddr)
	if err != nil {
		return "", err
	}
	return addr.String(), nil
}

func uint64ToHex(val uint64) string {
	return "0x" + strconv.FormatUint(val, 16)
}

func getStringFromArray(in interface{}) (string, error) {
	params, ok := in.([]interface{})
	if !ok {
		return "", TYPE_ERROR
	}
	ret, ok := params[0].(string)
	if !ok {
		return "", TYPE_ERROR
	}
	return ret, nil
}

func removeHexPrefix(hexStr string) string {
	ret := strings.Replace(hexStr, "0x", "", -1)
	ret = strings.Replace(ret, "0X", "", -1)
	return ret
}

func getBlockWithTransactions(svr *Server, blkMeta *iotextypes.BlockMeta, isDetailed bool) blockObject {
	transactionsRoot := "0x"
	transactionsRoot = "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
	transactions := []interface{}{}
	if blkMeta.Height > 0 {
		ret, err := svr.getActionsByBlock(blkMeta.Hash, 0, 1000)
		if err != nil {
			panic(err)
		}

		acts := ret.ActionInfo

		// transactions := []string{}
		for _, v := range acts {
			if isDetailed {
				tx := getTransactionFromActionInfo(v)
				if tx != nil {
					transactions = append(transactions, *tx)
				}
			} else {
				transactions = append(transactions, "0x"+v.ActHash)
			}
		}

		if len(transactions) == 0 {
			transactionsRoot = "0x" + blkMeta.TxRoot
		}
	}

	bloom := "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	if len(blkMeta.LogsBloom) == 0 {
		bloom = blkMeta.LogsBloom
	}
	miner, _ := ioAddrToEthAddr(blkMeta.ProducerAddress)
	return blockObject{
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
		Uncles:           []string{},
	}
}

func getTransactionFromActionInfo(actInfo *iotexapi.ActionInfo) *transactionObject {
	if actInfo.GetAction() == nil || actInfo.GetAction().GetCore() == nil {
		return nil
	}
	value := "0x0"
	to := ""
	data := ""
	switch act := actInfo.Action.Core.Action.(type) {
	case *iotextypes.ActionCore_Transfer:
		amount, err := strconv.ParseInt(act.Transfer.GetAmount(), 10, 64)
		value = uint64ToHex(uint64(amount))
		to, err = ioAddrToEthAddr(act.Transfer.GetRecipient())
		if err != nil {
			return nil
		}
	case *iotextypes.ActionCore_Execution:
		amount, err := strconv.ParseInt(act.Execution.GetAmount(), 10, 64)
		value = uint64ToHex(uint64(amount))
		if len(act.Execution.GetContract()) > 0 {
			to, err = ioAddrToEthAddr(act.Execution.GetContract())
		}
		data = "0x" + hex.EncodeToString(act.Execution.GetData())
		if err != nil {
			return nil
		}
	// support other types of action
	default:
		return nil
	}

	r := "0x" + hex.EncodeToString(actInfo.Action.Signature[:32])
	s := "0x" + hex.EncodeToString(actInfo.Action.Signature[32:64])
	v_val := uint64(actInfo.Action.Signature[64])
	if v_val < 27 {
		v_val += 27
	}

	v := uint64ToHex(v_val)
	var blockHash, blockNumber *string
	if actInfo.BlkHeight > 0 {
		h := "0x" + actInfo.BlkHash
		num := uint64ToHex(actInfo.BlkHeight)
		blockHash, blockNumber = &h, &num
	}

	hash := "0x" + actInfo.ActHash
	nonce := uint64ToHex(actInfo.Action.Core.Nonce)
	transactionIndex := uint64ToHex(uint64(actInfo.Index))
	from, err := ethAddrToIoAddr(actInfo.Sender)
	_gasPrice, err := strconv.ParseInt(actInfo.Action.Core.GasPrice, 10, 64)
	gasPrice := uint64ToHex(uint64(_gasPrice))
	gasLimit := uint64ToHex(actInfo.Action.Core.GasLimit)
	if err != nil {
		return nil
	}
	return &transactionObject{
		Hash:             &hash,
		Nonce:            &nonce,
		BlockHash:        blockHash,
		BlockNumber:      blockNumber,
		TransactionIndex: &transactionIndex,
		From:             &from,
		To:               &to,
		Value:            &value,
		GasPrice:         &gasPrice,
		Gas:              &gasLimit,
		Input:            &data,
		R:                &r,
		S:                &s,
		V:                &v,
	}

}

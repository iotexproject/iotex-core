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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/config"
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
		Number           string   `json:"number,omitempty"`
		Hash             string   `json:"hash,omitempty"`
		ParentHash       string   `json:"parentHash,omitempty"`
		Nonce            string   `json:"nonce,omitempty"`
		Sha3Uncles       string   `json:"sha3Uncles,omitempty"`
		LogsBloom        string   `json:"logsBloom,omitempty"`
		TransactionsRoot string   `json:"transactionsRoot,omitempty"`
		StateRoot        string   `json:"stateRoot,omitempty"`
		ReceiptsRoot     string   `json:"receiptsRoot,omitempty"`
		Miner            string   `json:"miner,omitempty"`
		Difficulty       string   `json:"difficulty,omitempty"`
		TotalDifficulty  string   `json:"totalDifficulty,omitempty"`
		ExtraData        string   `json:"extraData,omitempty"`
		Size             string   `json:"size,omitempty"`
		GasLimit         string   `json:"gasLimit,omitempty"`
		GasUsed          string   `json:"gasUsed,omitempty"`
		Timestamp        string   `json:"timestamp,omitempty"`
		Transactions     []string `json:"transactions,omitempty"`
		Uncles           []string `json:"uncles,omitempty"`
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

// func getBlockByHash(svr *Server, in interface{}) (interface{}, error) {

// }

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

// func getBlockWithTransactions(svr *Server, blkMeta *iotextypes.BlockMeta, isDetailed bool) {
// 	transactionsRoot := "0x"
// 	if blkMeta.Height > 0 {
// 		ret, err := svr.getActionsByBlock(blkMeta.Hash, 0, 1000)
// 		if err != nil {
// 			panic(err)
// 		}

// 		acts := ret.ActionInfo

// 		transactions := []string{}
// 		for _, v := range acts {
// 			if isDetailed {

// 			} else {
// 				transactions = append(transactions, "0x"+v.ActHash)
// 			}
// 		}

// 		transactionsRoot = "0x" + blkMeta.TxRoot
// 	}

// }

func getTransactionFromActionInfo(actInfo *iotexapi.ActionInfo) {

}

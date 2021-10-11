package api

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl/util"
)

type (
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

func hexStringToNumber(hexStr string) (uint64, error) {
	num, err := strconv.ParseUint(removeHexPrefix(hexStr), 16, 64)
	if err != nil {
		return 0, err
	}
	return num, nil
}

func ethAddrToIoAddr(ethAddr string) (string, error) {
	if ok := common.IsHexAddress(ethAddr); !ok {
		return "", ErrUnkownType
	}
	ethAddress := common.HexToAddress(ethAddr)
	ioAddress, err := address.FromBytes(ethAddress.Bytes())
	if err != nil {
		return "", ErrUnkownType
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

func intStrToHex(str string) (string, error) {
	amount, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return "", err
	}
	return uint64ToHex(uint64(amount)), nil
}

func getStringFromArray(in interface{}) (string, error) {
	params, ok := in.([]interface{})
	if !ok {
		return "", ErrUnkownType
	}
	ret, ok := params[0].(string)
	if !ok {
		return "", ErrUnkownType
	}
	return ret, nil
}

func getJSONFromArray(in interface{}) ([]byte, error) {
	params, ok := in.([]interface{})
	if !ok {
		return nil, ErrUnkownType
	}
	params0, ok := params[0].(map[string]interface{})
	if !ok {
		return nil, ErrUnkownType
	}
	jsonMarshaled, err := json.Marshal(params0)
	if err != nil {
		return nil, err
	}
	return jsonMarshaled, nil
}

func removeHexPrefix(hexStr string) string {
	ret := strings.Replace(hexStr, "0x", "", -1)
	ret = strings.Replace(ret, "0X", "", -1)
	return ret
}

func getBlockWithTransactions(svr *Server, blkMeta *iotextypes.BlockMeta, isDetailed bool) (*blockObject, error) {
	transactionsRoot := "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
	transactions := []interface{}{}
	if blkMeta.Height > 0 {
		// get Actions by blk number, more efficient
		ret, err := svr.getActionsByBlock(blkMeta.Hash, 0, 1000)
		if err != nil {
			return nil, err
		}

		acts := ret.ActionInfo

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
	if len(blkMeta.LogsBloom) > 0 {
		bloom = blkMeta.LogsBloom
	}
	miner, err := ioAddrToEthAddr(blkMeta.ProducerAddress)
	if err != nil {
		return nil, ErrUnkownType
	}
	return &blockObject{
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
	}, nil
}

func getTransactionFromActionInfo(actInfo *iotexapi.ActionInfo) *transactionObject {
	if actInfo.GetAction() == nil || actInfo.GetAction().GetCore() == nil {
		return nil
	}
	value := "0x0"
	to := ""
	data := ""
	var err error
	switch act := actInfo.Action.Core.Action.(type) {
	case *iotextypes.ActionCore_Transfer:
		value, err = intStrToHex(act.Transfer.GetAmount())
		if err != nil {
			return nil
		}
		to, err = ioAddrToEthAddr(act.Transfer.GetRecipient())
		if err != nil {
			return nil
		}
	case *iotextypes.ActionCore_Execution:
		value, err = intStrToHex(act.Execution.GetAmount())
		if err != nil {
			return nil
		}
		if len(act.Execution.GetContract()) > 0 {
			to, err = ioAddrToEthAddr(act.Execution.GetContract())
			if err != nil {
				return nil
			}
		}
		data = "0x" + hex.EncodeToString(act.Execution.GetData())
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

func parseBlockNumber(svr *Server, str string) (uint64, error) {
	if str == "earliest" {
		return 1, nil
	}

	if str == "" || str == "pending" || str == "latest" {
		return svr.bc.TipHeight(), nil
	}

	res, err := hexStringToNumber(str)
	if err != nil {
		return 0, err
	}

	return res, nil
}

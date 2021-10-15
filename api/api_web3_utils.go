package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
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
		StandardV        *string `json:"standardV,omitempty"`
		Condition        *string `json:"condition,omitempty"`
		Creates          *string `json:"creates,omitempty"`
		ChainId          *string `json:"chainId,omitempty"`
		PublicKey        *string `json:"publicKey,omitempty"`
	}
)

func hexStringToNumber(hexStr string) (uint64, error) {
	if hexStr == "" {
		return 0, nil
	}
	num, err := strconv.ParseUint(removeHexPrefix(hexStr), 16, 64)
	if err != nil {
		return 0, err
	}
	return num, nil
}

func ethAddrToIoAddr(ethAddr string) (string, error) {
	if ok := common.IsHexAddress(ethAddr); !ok {
		return "", errUnkownType
	}
	ethAddress := common.HexToAddress(ethAddr)
	ioAddress, err := address.FromBytes(ethAddress.Bytes())
	if err != nil {
		return "", errUnkownType
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

func getStringFromArray(in interface{}, i int) (string, error) {
	params, ok := in.([]interface{})
	if !ok || i < 0 {
		return "", errUnkownType
	}
	ret, ok := params[i].(string)
	if !ok {
		return "", errUnkownType
	}
	return ret, nil
}

func getJSONFromArray(in interface{}) ([]byte, error) {
	params, ok := in.([]interface{})
	if !ok {
		return nil, errUnkownType
	}
	params0, ok := params[0].(map[string]interface{})
	if !ok {
		return nil, errUnkownType
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
				tx := getTransactionFromActionInfo(v, svr.bc.ChainID())
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
		return nil, errUnkownType
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

func getTransactionFromActionInfo(actInfo *iotexapi.ActionInfo, chainId uint32) *transactionObject {
	if actInfo.GetAction() == nil || actInfo.GetAction().GetCore() == nil {
		return nil
	}
	var blockHash, blockNumber, to *string
	value := "0x0"
	data := ""
	var err error
	switch act := actInfo.Action.Core.Action.(type) {
	case *iotextypes.ActionCore_Transfer:
		value, err = intStrToHex(act.Transfer.GetAmount())
		if err != nil {
			return nil
		}
		_to, err := ioAddrToEthAddr(act.Transfer.GetRecipient())
		if err != nil {
			return nil
		}
		to = &_to
	case *iotextypes.ActionCore_Execution:
		value, err = intStrToHex(act.Execution.GetAmount())
		if err != nil {
			return nil
		}
		if len(act.Execution.GetContract()) > 0 {
			_to, err := ioAddrToEthAddr(act.Execution.GetContract())
			if err != nil {
				return nil
			}
			to = &_to
		}
		data = byteToHex(act.Execution.GetData())
	default:
		return nil
	}

	r := byteToHex(actInfo.Action.Signature[:32])
	s := byteToHex(actInfo.Action.Signature[32:64])
	vVal := uint64(actInfo.Action.Signature[64])
	if vVal < 27 {
		vVal += 27
	}
	v := uint64ToHex(vVal)

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
	chainIdHex := uint64ToHex(uint64(chainId))
	pubkey := byteToHex(actInfo.Action.SenderPubKey)
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
		To:               to,
		Value:            &value,
		GasPrice:         &gasPrice,
		Gas:              &gasLimit,
		Input:            &data,
		R:                &r,
		S:                &s,
		V:                &v,
		StandardV:        &v,
		ChainId:          &chainIdHex,
		PublicKey:        &pubkey,
	}
}

func getTransactionCreateFromActionInfo(svr *Server, actInfo *iotexapi.ActionInfo, chainId uint32) *transactionObject {
	tx := getTransactionFromActionInfo(actInfo, chainId)
	if tx == nil {
		return nil
	}

	if tx.To == nil && tx.BlockHash != nil {
		receipt, err := svr.GetReceiptByAction(context.Background(),
			&iotexapi.GetReceiptByActionRequest{
				ActionHash: (*tx.Hash)[2:],
			},
		)
		if err != nil {
			return nil
		}
		addr, err := ioAddrToEthAddr(receipt.ReceiptInfo.Receipt.ContractAddress)
		if err != nil {
			return nil
		}
		tx.Creates = &addr
	}
	return tx
}

// DecodeRawTx() decode raw data string into eth tx
func DecodeRawTx(rawData string, chainId uint32) (*types.Transaction, []byte, []byte, error) {
	dataInString, err := hex.DecodeString(removeHexPrefix(rawData))
	if err != nil {
		return nil, nil, nil, err
	}

	// decode raw data into rlp tx
	tx := types.Transaction{}
	err = rlp.DecodeBytes(dataInString, &tx)
	if err != nil {
		return nil, nil, nil, err
	}

	// extract signature and recover pubkey
	v, r, s := tx.RawSignatureValues()
	recID := uint32(v.Int64()) - 2*chainId - 8
	sig := make([]byte, 64, 65)
	rSize := len(r.Bytes())
	copy(sig[32-rSize:32], r.Bytes())
	sSize := len(s.Bytes())
	copy(sig[64-sSize:], s.Bytes())
	sig = append(sig, byte(recID))

	// recover public key
	rawHash := types.NewEIP155Signer(big.NewInt(int64(chainId))).Hash(&tx)
	pubkey, err := crypto.RecoverPubkey(rawHash[:], sig)
	if err != nil {
		return nil, nil, nil, err
	}

	return &tx, sig, pubkey.Bytes(), nil
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

func isContractAddr(svr *Server, addr string) bool {
	accountMeta, _, _ := svr.getAccount(addr)
	return accountMeta.IsContract
}

func getLogsWithFilter(svr *Server, fromBlock string, toBlock string, addrs []string, topics [][]string) ([]logsObject, error) {
	// construct block range(from, to)
	tipHeight := svr.bc.TipHeight()
	from, err := parseBlockNumber(svr, fromBlock)
	to, err := parseBlockNumber(svr, toBlock)
	if err != nil {
		return nil, err
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
		ioAddr, err := ethAddrToIoAddr(ethAddr)
		if err != nil {
			return nil, err
		}
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

	logs, err := svr.getLogsInRange(logfilter.NewLogFilter(&filter, nil, nil), from, to, 1000)
	if err != nil {
		return nil, err
	}
	// parse log results
	var ret []logsObject
	for _, l := range logs {
		if len(l.Topics) > 0 {
			addr, _ := ioAddrToEthAddr(l.ContractAddress)
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
				Address:          addr,
				Data:             byteToHex(l.Data),
				Topics:           topics,
			})
		}
	}
	return ret, nil
}

func byteToHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
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
			logReq.FromBlock = v.(string)
		case "toBlock":
			logReq.ToBlock = v.(string)
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

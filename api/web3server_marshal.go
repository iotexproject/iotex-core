package api

import (
	"encoding/hex"
	"encoding/json"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
)

type (
	receiptObjectV2 struct {
		blockHash       hash.Hash256
		from            string
		to              *string
		contractAddress *string
		logsBloom       string
		receipt         *action.Receipt
	}

	logsObjectV2 struct {
		blockHash hash.Hash256
		log       *action.Log
	}

	transactionObjectV2 struct {
		blockHash hash.Hash256
		to        *string
		ethTx     types.Transaction
		receipt   *action.Receipt
		pubkey    crypto.PublicKey
		signature []byte
	}

	blockObjectV2 struct {
		blkMeta      *iotextypes.BlockMeta
		logsBloom    string
		transactions []interface{}
	}
)

func (obj *receiptObjectV2) MarshalJSON() ([]byte, error) {
	from, err := ioAddrToEthAddr(obj.from)
	if err != nil {
		return nil, err
	}

	logs := make([]*logsObjectV2, 0, len(obj.receipt.Logs()))
	for i := range obj.receipt.Logs() {
		logs[i] = &logsObjectV2{obj.blockHash, obj.receipt.Logs()[i]}
	}

	return json.Marshal(&struct {
		TransactionIndex  string          `json:"transactionIndex"`
		TransactionHash   string          `json:"transactionHash"`
		BlockHash         string          `json:"blockHash"`
		BlockNumber       string          `json:"blockNumber"`
		From              string          `json:"from"`
		To                *string         `json:"to"`
		CumulativeGasUsed string          `json:"cumulativeGasUsed"`
		GasUsed           string          `json:"gasUsed"`
		ContractAddress   *string         `json:"contractAddress"`
		LogsBloom         string          `json:"logsBloom"`
		Logs              []*logsObjectV2 `json:"logs"`
		Status            string          `json:"status"`
	}{
		TransactionIndex:  uint64ToHex(uint64(obj.receipt.TxIndex)),
		TransactionHash:   "0x" + hex.EncodeToString(obj.receipt.ActionHash[:]),
		BlockHash:         "0x" + hex.EncodeToString(obj.blockHash[:]),
		BlockNumber:       uint64ToHex(obj.receipt.BlockHeight),
		From:              from,
		To:                obj.to,
		CumulativeGasUsed: uint64ToHex(obj.receipt.GasConsumed),
		GasUsed:           uint64ToHex(obj.receipt.GasConsumed),
		ContractAddress:   obj.contractAddress,
		LogsBloom:         obj.logsBloom,
		Logs:              logs,
		Status:            uint64ToHex(obj.receipt.Status),
	})
}

func (obj *logsObjectV2) MarshalJSON() ([]byte, error) {
	addr, err := ioAddrToEthAddr(obj.log.Address)
	if err != nil {
		return nil, err
	}
	topics := make([]string, 0, len(obj.log.Topics))
	for _, tpc := range obj.log.Topics {
		topics = append(topics, "0x"+hex.EncodeToString(tpc[:]))
	}
	return json.Marshal(&struct {
		Removed          bool     `json:"removed"`
		LogIndex         string   `json:"logIndex"`
		TransactionIndex string   `json:"transactionIndex"`
		TransactionHash  string   `json:"transactionHash"`
		BlockHash        string   `json:"blockHash"`
		BlockNumber      string   `json:"blockNumber"`
		Address          string   `json:"address"`
		Data             string   `json:"data"`
		Topics           []string `json:"topics"`
	}{
		Removed:          false,
		LogIndex:         uint64ToHex(uint64(obj.log.Index)),
		TransactionIndex: uint64ToHex(uint64(obj.log.TxIndex)),
		TransactionHash:  "0x" + hex.EncodeToString(obj.log.ActionHash[:]),
		BlockHash:        "0x" + hex.EncodeToString(obj.blockHash[:]),
		BlockNumber:      uint64ToHex(uint64(obj.log.BlockHeight)),
		Address:          addr,
		Data:             "0x" + hex.EncodeToString(obj.log.Data),
		Topics:           topics,
	})
}

func (obj *transactionObjectV2) MarshalJSON() ([]byte, error) {
	value, _ := intStrToHex(obj.ethTx.Value().String())
	gasPrice, _ := intStrToHex(obj.ethTx.GasPrice().String())

	vVal := uint64(obj.signature[64])
	if vVal < 27 {
		vVal += 27
	}

	return json.Marshal(&struct {
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
	}{
		Hash:             "0x" + hex.EncodeToString(obj.receipt.ActionHash[:]),
		Nonce:            uint64ToHex(obj.ethTx.Nonce()),
		BlockHash:        "0x" + hex.EncodeToString(obj.blockHash[:]),
		BlockNumber:      uint64ToHex(obj.receipt.BlockHeight),
		TransactionIndex: uint64ToHex(uint64(obj.receipt.TxIndex)),
		From:             obj.pubkey.Address().Hex(),
		To:               obj.to,
		Value:            value,
		GasPrice:         gasPrice,
		Gas:              uint64ToHex(obj.ethTx.Gas()),
		Input:            byteToHex(obj.ethTx.Data()),
		R:                byteToHex(obj.signature[:32]),
		S:                byteToHex(obj.signature[32:64]),
		V:                uint64ToHex(vVal),
	})
}

func (obj *blockObjectV2) MarshalJSON() ([]byte, error) {
	producerAddr, err := ioAddrToEthAddr(obj.blkMeta.ProducerAddress)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&struct {
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
		Step             string        `json:"step"`
		Uncles           []string      `json:"uncles"`
	}{
		Author:           producerAddr,
		Number:           uint64ToHex(obj.blkMeta.Height),
		Hash:             "0x" + obj.blkMeta.Hash,
		ParentHash:       "0x" + obj.blkMeta.PreviousBlockHash,
		Sha3Uncles:       "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		LogsBloom:        obj.logsBloom,
		TransactionsRoot: "0x" + obj.blkMeta.TxRoot,
		StateRoot:        "0x" + obj.blkMeta.DeltaStateDigest,
		ReceiptsRoot:     "0x" + obj.blkMeta.ReceiptRoot,
		Miner:            producerAddr,
		Difficulty:       "0xfffffffffffffffffffffffffffffffe",
		TotalDifficulty:  "0xff14700000000000000000000000486001d72",
		ExtraData:        "0x",
		Size:             uint64ToHex(uint64(obj.blkMeta.NumActions)),
		GasLimit:         uint64ToHex(obj.blkMeta.GasLimit),
		GasUsed:          uint64ToHex(obj.blkMeta.GasUsed),
		Timestamp:        uint64ToHex(uint64(obj.blkMeta.Timestamp.Seconds)),
		Transactions:     obj.transactions,
		Step:             "373422302",
		Uncles:           []string{},
	})
}

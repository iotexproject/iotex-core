package api

import (
	"encoding/hex"
	"encoding/json"

	"github.com/iotexproject/go-pkgs/hash"
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
)

func (obj *receiptObjectV2) MarshalJSON() ([]byte, error) {
	from, err := ioAddrToEthAddr(obj.from)
	if err != nil {
		return nil, err
	}

	logs := make([]logsObjectV2, 0, len(obj.receipt.Logs()))
	for _, v := range obj.receipt.Logs() {
		logs = append(logs, logsObjectV2{obj.blockHash, v})
	}

	return json.Marshal(&struct {
		TransactionIndex  string         `json:"transactionIndex"`
		TransactionHash   string         `json:"transactionHash"`
		BlockHash         string         `json:"blockHash"`
		BlockNumber       string         `json:"blockNumber"`
		From              string         `json:"from"`
		To                *string        `json:"to"`
		CumulativeGasUsed string         `json:"cumulativeGasUsed"`
		GasUsed           string         `json:"gasUsed"`
		ContractAddress   *string        `json:"contractAddress"`
		LogsBloom         string         `json:"logsBloom"`
		Logs              []logsObjectV2 `json:"logs"`
		Status            string         `json:"status"`
	}{
		TransactionIndex:  uint64ToHex(uint64(obj.receipt.TxIndex)),
		TransactionHash:   "0x" + hex.EncodeToString(obj.receipt.ActionHash[:]),
		BlockHash:         "0x" + hex.EncodeToString(obj.blockHash[:]),
		BlockNumber:       uint64ToHex(obj.receipt.BlockHeight),
		From:              from,
		To:                nil, // TODO
		CumulativeGasUsed: uint64ToHex(obj.receipt.GasConsumed),
		GasUsed:           uint64ToHex(obj.receipt.GasConsumed),
		ContractAddress:   nil, // TODO
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

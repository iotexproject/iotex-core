package api

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

const (
	_zeroLogsBloom = "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
)

type (
	web3Response struct {
		id     any
		result interface{}
		err    error
	}

	errMessage struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data,omitempty"`
	}

	streamResponse struct {
		id     string
		result interface{}
	}

	streamParams struct {
		Subscription string      `json:"subscription"`
		Result       interface{} `json:"result"`
	}

	getBlockResult struct {
		blk          *block.Block
		transactions []interface{}
	}

	getTransactionResult struct {
		blockHash *hash.Hash256
		to        *string
		ethTx     *types.Transaction
		receipt   *action.Receipt
		pubkey    crypto.PublicKey
	}

	getReceiptResult struct {
		blockHash       hash.Hash256
		from            address.Address
		to              *string
		contractAddress *string
		logsBloom       string
		receipt         *action.Receipt
		txType          uint
		transferEvents  []*action.Log
	}

	getLogsResult struct {
		blockHash hash.Hash256
		log       *action.Log
	}

	getSyncingResult struct {
		StartingBlock string `json:"startingBlock"`
		CurrentBlock  string `json:"currentBlock"`
		HighestBlock  string `json:"highestBlock"`
	}

	debugTraceTransactionResult struct {
		Failed                       bool                              `json:"failed"`
		Revert                       string                            `json:"revert"`
		ReturnValue                  string                            `json:"returnValue"`
		Gas                          uint64                            `json:"gas"`
		StructLogs                   []apitypes.StructLog              `json:"structLogs"`
		ContractStorageAccesses      []contractStorageAccessJSON       `json:"contractStorageAccesses,omitempty"`
		ContractStorageAccessSummary *contractStorageAccessSummaryJSON `json:"contractStorageAccessSummary,omitempty"`
		ContractStorageWitnesses     []contractStorageWitnessJSON      `json:"contractStorageWitnesses,omitempty"`
	}

	blockTraceResult struct {
		TxHash hash.Hash256 `json:"txHash"`
		Result any          `json:"result"`
	}

	debugTraceBlockStorageSummaryResult struct {
		Summary      *contractStorageAccessSummaryJSON `json:"summary,omitempty"`
		Transactions []blockStorageAccessSummaryJSON   `json:"transactions"`
	}

	debugTraceBlockWitnessResult struct {
		Summary      *contractStorageWitnessSummaryJSON `json:"summary,omitempty"`
		Transactions []blockStorageWitnessJSON          `json:"transactions"`
	}

	contractStorageAccessJSON struct {
		Address string   `json:"address"`
		Reads   []string `json:"reads,omitempty"`
		Writes  []string `json:"writes,omitempty"`
	}

	blockStorageAccessSummaryJSON struct {
		TxHash       string `json:"txHash"`
		Contracts    uint64 `json:"contracts"`
		ReadSlots    uint64 `json:"readSlots"`
		WriteSlots   uint64 `json:"writeSlots"`
		TouchedSlots uint64 `json:"touchedSlots"`
	}

	contractStorageAccessSummaryJSON struct {
		Contracts    uint64 `json:"contracts"`
		ReadSlots    uint64 `json:"readSlots"`
		WriteSlots   uint64 `json:"writeSlots"`
		TouchedSlots uint64 `json:"touchedSlots"`
	}

	contractStorageWitnessSummaryJSON struct {
		Contracts  uint64 `json:"contracts"`
		Entries    uint64 `json:"entries"`
		ProofNodes uint64 `json:"proofNodes"`
		ProofBytes uint64 `json:"proofBytes"`
	}

	contractStorageWitnessJSON struct {
		Address     string                            `json:"address"`
		StorageRoot string                            `json:"storageRoot"`
		Entries     []contractStorageWitnessEntryJSON `json:"entries,omitempty"`
		ProofNodes  []string                          `json:"proofNodes,omitempty"`
	}

	contractStorageWitnessEntryJSON struct {
		Key   string `json:"key"`
		Value string `json:"value,omitempty"`
	}

	blockStorageWitnessJSON struct {
		TxHash     string                       `json:"txHash"`
		Contracts  uint64                       `json:"contracts"`
		Entries    uint64                       `json:"entries"`
		ProofNodes uint64                       `json:"proofNodes"`
		ProofBytes uint64                       `json:"proofBytes"`
		Witnesses  []contractStorageWitnessJSON `json:"witnesses,omitempty"`
	}

	feeHistoryResult struct {
		OldestBlock       string     `json:"oldestBlock"`
		BaseFeePerGas     []string   `json:"baseFeePerGas"`
		GasUsedRatio      []float64  `json:"gasUsedRatio"`
		BaseFeePerBlobGas []string   `json:"baseFeePerBlobGas"`
		BlobGasUsedRatio  []float64  `json:"blobGasUsedRatio"`
		Reward            [][]string `json:"reward,omitempty"`
	}
)

var (
	errInvalidObject = errors.New("invalid object")
)

func fromContractStorageAccesses(accesses []evm.ContractStorageAccess) []contractStorageAccessJSON {
	if len(accesses) == 0 {
		return nil
	}
	out := make([]contractStorageAccessJSON, 0, len(accesses))
	for _, access := range accesses {
		item := contractStorageAccessJSON{
			Address: access.Address.Hex(),
		}
		if len(access.Reads) > 0 {
			item.Reads = make([]string, 0, len(access.Reads))
			for _, h := range access.Reads {
				item.Reads = append(item.Reads, h.Hex())
			}
		}
		if len(access.Writes) > 0 {
			item.Writes = make([]string, 0, len(access.Writes))
			for _, h := range access.Writes {
				item.Writes = append(item.Writes, h.Hex())
			}
		}
		out = append(out, item)
	}
	return out
}

func summarizeContractStorageAccesses(accesses []evm.ContractStorageAccess) *contractStorageAccessSummaryJSON {
	if len(accesses) == 0 {
		return nil
	}
	summary := &contractStorageAccessSummaryJSON{
		Contracts: uint64(len(accesses)),
	}
	for _, access := range accesses {
		summary.ReadSlots += uint64(len(access.Reads))
		summary.WriteSlots += uint64(len(access.Writes))
		touched := make(map[common.Hash]struct{}, len(access.Reads)+len(access.Writes))
		for _, slot := range access.Reads {
			touched[slot] = struct{}{}
		}
		for _, slot := range access.Writes {
			touched[slot] = struct{}{}
		}
		summary.TouchedSlots += uint64(len(touched))
	}
	return summary
}

func summarizeContractStorageAccessesJSON(accesses []contractStorageAccessJSON) *contractStorageAccessSummaryJSON {
	if len(accesses) == 0 {
		return nil
	}
	summary := &contractStorageAccessSummaryJSON{
		Contracts: uint64(len(accesses)),
	}
	for _, access := range accesses {
		summary.ReadSlots += uint64(len(access.Reads))
		summary.WriteSlots += uint64(len(access.Writes))
		touched := make(map[string]struct{}, len(access.Reads)+len(access.Writes))
		for _, slot := range access.Reads {
			touched[slot] = struct{}{}
		}
		for _, slot := range access.Writes {
			touched[slot] = struct{}{}
		}
		summary.TouchedSlots += uint64(len(touched))
	}
	return summary
}

func fromContractStorageWitnesses(witnesses map[common.Address]*evm.ContractStorageWitness) []contractStorageWitnessJSON {
	if len(witnesses) == 0 {
		return nil
	}
	addrs := make([]common.Address, 0, len(witnesses))
	for addr := range witnesses {
		addrs = append(addrs, addr)
	}
	sort.Slice(addrs, func(i, j int) bool {
		return bytes.Compare(addrs[i][:], addrs[j][:]) < 0
	})

	out := make([]contractStorageWitnessJSON, 0, len(addrs))
	for _, addr := range addrs {
		witness := witnesses[addr]
		if witness == nil {
			continue
		}
		item := contractStorageWitnessJSON{
			Address:     addr.Hex(),
			StorageRoot: byteToHex(witness.StorageRoot[:]),
		}
		if len(witness.Entries) > 0 {
			item.Entries = make([]contractStorageWitnessEntryJSON, 0, len(witness.Entries))
			for _, entry := range witness.Entries {
				entryJSON := contractStorageWitnessEntryJSON{
					Key: byteToHex(entry.Key[:]),
				}
				if len(entry.Value) > 0 {
					entryJSON.Value = byteToHex(entry.Value)
				}
				item.Entries = append(item.Entries, entryJSON)
			}
		}
		if len(witness.ProofNodes) > 0 {
			item.ProofNodes = make([]string, 0, len(witness.ProofNodes))
			for _, node := range witness.ProofNodes {
				item.ProofNodes = append(item.ProofNodes, byteToHex(node))
			}
		}
		out = append(out, item)
	}
	return out
}

func summarizeContractStorageWitnessesJSON(witnesses []contractStorageWitnessJSON) *contractStorageWitnessSummaryJSON {
	if len(witnesses) == 0 {
		return nil
	}
	summary := &contractStorageWitnessSummaryJSON{
		Contracts: uint64(len(witnesses)),
	}
	for _, witness := range witnesses {
		summary.Entries += uint64(len(witness.Entries))
		summary.ProofNodes += uint64(len(witness.ProofNodes))
		for _, node := range witness.ProofNodes {
			summary.ProofBytes += uint64(len(common.FromHex(node)))
		}
	}
	return summary
}

func summarizeBlockTraceWitnessResults(results []*blockTraceResult) (*debugTraceBlockWitnessResult, error) {
	summary := &debugTraceBlockWitnessResult{
		Transactions: make([]blockStorageWitnessJSON, 0, len(results)),
	}
	if len(results) == 0 {
		return summary, nil
	}
	for _, result := range results {
		traceResult, ok := result.Result.(*debugTraceTransactionResult)
		if !ok {
			return nil, errors.Errorf("unsupported block trace result type %T", result.Result)
		}
		txSummary := summarizeContractStorageWitnessesJSON(traceResult.ContractStorageWitnesses)
		txItem := blockStorageWitnessJSON{
			TxHash:    "0x" + hex.EncodeToString(result.TxHash[:]),
			Witnesses: traceResult.ContractStorageWitnesses,
		}
		if txSummary != nil {
			txItem.Contracts = txSummary.Contracts
			txItem.Entries = txSummary.Entries
			txItem.ProofNodes = txSummary.ProofNodes
			txItem.ProofBytes = txSummary.ProofBytes
			if summary.Summary == nil {
				summary.Summary = &contractStorageWitnessSummaryJSON{}
			}
			summary.Summary.Contracts += txSummary.Contracts
			summary.Summary.Entries += txSummary.Entries
			summary.Summary.ProofNodes += txSummary.ProofNodes
			summary.Summary.ProofBytes += txSummary.ProofBytes
		}
		summary.Transactions = append(summary.Transactions, txItem)
	}
	return summary, nil
}

func summarizeBlockTraceStorageResults(results []*blockTraceResult) (*debugTraceBlockStorageSummaryResult, error) {
	summary := &debugTraceBlockStorageSummaryResult{
		Transactions: make([]blockStorageAccessSummaryJSON, 0, len(results)),
	}
	if len(results) == 0 {
		return summary, nil
	}
	type slotSet struct {
		reads   map[string]struct{}
		writes  map[string]struct{}
		touched map[string]struct{}
	}
	contracts := make(map[string]*slotSet)
	for _, result := range results {
		traceResult, ok := result.Result.(*debugTraceTransactionResult)
		if !ok {
			return nil, errors.Errorf("unsupported block trace result type %T", result.Result)
		}
		txSummary := summarizeContractStorageAccessesJSON(traceResult.ContractStorageAccesses)
		txItem := blockStorageAccessSummaryJSON{
			TxHash: "0x" + hex.EncodeToString(result.TxHash[:]),
		}
		if txSummary != nil {
			txItem.Contracts = txSummary.Contracts
			txItem.ReadSlots = txSummary.ReadSlots
			txItem.WriteSlots = txSummary.WriteSlots
			txItem.TouchedSlots = txSummary.TouchedSlots
			for _, access := range traceResult.ContractStorageAccesses {
				entry, ok := contracts[access.Address]
				if !ok {
					entry = &slotSet{
						reads:   make(map[string]struct{}),
						writes:  make(map[string]struct{}),
						touched: make(map[string]struct{}),
					}
					contracts[access.Address] = entry
				}
				for _, slot := range access.Reads {
					entry.reads[slot] = struct{}{}
					entry.touched[slot] = struct{}{}
				}
				for _, slot := range access.Writes {
					entry.writes[slot] = struct{}{}
					entry.touched[slot] = struct{}{}
				}
			}
		}
		summary.Transactions = append(summary.Transactions, txItem)
	}
	if len(contracts) == 0 {
		return summary, nil
	}
	summary.Summary = &contractStorageAccessSummaryJSON{
		Contracts: uint64(len(contracts)),
	}
	for _, entry := range contracts {
		summary.Summary.ReadSlots += uint64(len(entry.reads))
		summary.Summary.WriteSlots += uint64(len(entry.writes))
		summary.Summary.TouchedSlots += uint64(len(entry.touched))
	}
	return summary, nil
}

func (obj *web3Response) MarshalJSON() ([]byte, error) {
	if obj.err == nil {
		return json.Marshal(&struct {
			Jsonrpc string      `json:"jsonrpc"`
			ID      any         `json:"id"`
			Result  interface{} `json:"result"`
		}{
			Jsonrpc: "2.0",
			ID:      obj.id,
			Result:  obj.result,
		})
	}

	var (
		errCode int
		errMsg  string
	)
	// error code: https://eth.wiki/json-rpc/json-rpc-error-codes-improvement-proposal
	if s, ok := status.FromError(obj.err); ok {
		errCode, errMsg = int(s.Code()), s.Message()
	} else {
		errCode, errMsg = -32603, obj.err.Error()
	}

	return json.Marshal(&struct {
		Jsonrpc string     `json:"jsonrpc"`
		ID      any        `json:"id"`
		Error   errMessage `json:"error"`
	}{
		Jsonrpc: "2.0",
		ID:      obj.id,
		Error: errMessage{
			Code:    errCode,
			Message: errMsg,
			Data:    obj.result,
		},
	})
}

func getLogsBloomHex(logsbloom string) string {
	if len(logsbloom) == 0 {
		return _zeroLogsBloom
	}
	return "0x" + logsbloom
}

func (obj *getBlockResult) MarshalJSON() ([]byte, error) {
	if obj.blk == nil {
		return nil, errInvalidObject
	}

	var (
		blkHash           hash.Hash256
		producerAddress   string
		logsBloomStr      string
		gasLimit, gasUsed uint64
		baseFee           *hexutil.Big

		txs              = make([]interface{}, 0)
		preHash          = obj.blk.Header.PrevHash()
		txRoot           = obj.blk.Header.TxRoot()
		deltaStateDigest = obj.blk.Header.DeltaStateDigest()
		receiptRoot      = obj.blk.Header.ReceiptRoot()
		blobGasUsed      = hexutil.Uint64(obj.blk.Header.BlobGasUsed())
		excessBlobGas    = hexutil.Uint64(obj.blk.Header.ExcessBlobGas())
	)
	if obj.blk.Height() > 0 {
		producerAddress = obj.blk.Header.ProducerAddress()
		blkHash = obj.blk.Header.HashBlock()
	} else {
		blkHash = block.GenesisHash()
	}
	producerAddr, err := ioAddrToEthAddr(producerAddress)
	if err != nil {
		return nil, err
	}
	for _, tx := range obj.blk.Actions {
		gasLimit += tx.Gas()
	}
	for _, r := range obj.blk.Receipts {
		gasUsed += r.GasConsumed
	}
	if logsBloom := obj.blk.Header.LogsBloomfilter(); logsBloom != nil {
		logsBloomStr = hex.EncodeToString(logsBloom.Bytes())
	}
	if len(obj.transactions) > 0 {
		txs = obj.transactions
	}
	if obj.blk.Header.BaseFee() != nil {
		baseFee = (*hexutil.Big)(obj.blk.Header.BaseFee())
	}
	return json.Marshal(&struct {
		Author           string         `json:"author"`
		Number           string         `json:"number"`
		Hash             string         `json:"hash"`
		ParentHash       string         `json:"parentHash"`
		Sha3Uncles       string         `json:"sha3Uncles"`
		LogsBloom        string         `json:"logsBloom"`
		TransactionsRoot string         `json:"transactionsRoot"`
		StateRoot        string         `json:"stateRoot"`
		ReceiptsRoot     string         `json:"receiptsRoot"`
		Miner            string         `json:"miner"`
		Difficulty       string         `json:"difficulty"`
		TotalDifficulty  string         `json:"totalDifficulty"`
		ExtraData        string         `json:"extraData"`
		Size             string         `json:"size"`
		GasLimit         string         `json:"gasLimit"`
		GasUsed          string         `json:"gasUsed"`
		Timestamp        string         `json:"timestamp"`
		Transactions     []interface{}  `json:"transactions"`
		Step             string         `json:"step"`
		Uncles           []string       `json:"uncles"`
		BaseFeePerGas    *hexutil.Big   `json:"baseFeePerGas,omitempty"`
		BlobGasUsed      hexutil.Uint64 `json:"blobGasUsed,omitempty"`
		ExcessBlobGas    hexutil.Uint64 `json:"excessBlobGas,omitempty"`
	}{
		Author:           producerAddr,
		Number:           uint64ToHex(obj.blk.Height()),
		Hash:             "0x" + hex.EncodeToString(blkHash[:]),
		ParentHash:       "0x" + hex.EncodeToString(preHash[:]),
		Sha3Uncles:       "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		LogsBloom:        getLogsBloomHex(logsBloomStr),
		TransactionsRoot: "0x" + hex.EncodeToString(txRoot[:]),
		StateRoot:        "0x" + hex.EncodeToString(deltaStateDigest[:]),
		ReceiptsRoot:     "0x" + hex.EncodeToString(receiptRoot[:]),
		Miner:            producerAddr,
		Difficulty:       "0xfffffffffffffffffffffffffffffffe",
		TotalDifficulty:  "0xff14700000000000000000000000486001d72",
		ExtraData:        "0x",
		Size:             uint64ToHex(uint64(len(obj.blk.Actions))),
		GasLimit:         uint64ToHex(gasLimit),
		GasUsed:          uint64ToHex(gasUsed),
		Timestamp:        uint64ToHex(uint64(timestamppb.New(obj.blk.Header.Timestamp()).Seconds)),
		Transactions:     txs,
		Step:             "373422302",
		Uncles:           []string{},
		BaseFeePerGas:    baseFee,
		BlobGasUsed:      blobGasUsed,
		ExcessBlobGas:    excessBlobGas,
	})
}

func (obj *getTransactionResult) MarshalJSON() ([]byte, error) {
	if obj.pubkey == nil || obj.ethTx == nil {
		return nil, errInvalidObject
	}
	var (
		value, _    = intStrToHex(obj.ethTx.Value().String())
		gasPrice, _ = intStrToHex(obj.ethTx.GasPrice().String())
		v, r, s     = obj.ethTx.RawSignatureValues()
		txHash      = obj.ethTx.Hash().Bytes()
		blkNum      *string
		txIndex     *string
		blkHash     *string
	)

	if obj.receipt != nil {
		txHash = obj.receipt.ActionHash[:]
	}
	if obj.receipt != nil {
		tmp := uint64ToHex(obj.receipt.BlockHeight)
		blkNum = &tmp
	}
	if obj.receipt != nil {
		tmp := uint64ToHex(uint64(obj.receipt.TxIndex))
		txIndex = &tmp
	}
	if obj.blockHash != nil {
		tmp := "0x" + hex.EncodeToString(obj.blockHash[:])
		blkHash = &tmp
	}
	type rpcTransaction struct {
		Hash                string            `json:"hash"`
		Nonce               string            `json:"nonce"`
		BlockHash           *string           `json:"blockHash"`
		BlockNumber         *string           `json:"blockNumber"`
		TransactionIndex    *string           `json:"transactionIndex"`
		From                string            `json:"from"`
		To                  *string           `json:"to"`
		Value               string            `json:"value"`
		GasPrice            string            `json:"gasPrice"`
		Gas                 string            `json:"gas"`
		Input               string            `json:"input"`
		R                   string            `json:"r"`
		S                   string            `json:"s"`
		V                   string            `json:"v"`
		Type                hexutil.Uint64    `json:"type"`
		GasFeeCap           *hexutil.Big      `json:"maxFeePerGas,omitempty"`
		GasTipCap           *hexutil.Big      `json:"maxPriorityFeePerGas,omitempty"`
		MaxFeePerBlobGas    *hexutil.Big      `json:"maxFeePerBlobGas,omitempty"`
		Accesses            *types.AccessList `json:"accessList,omitempty"`
		ChainID             *hexutil.Big      `json:"chainId,omitempty"`
		BlobVersionedHashes []common.Hash     `json:"blobVersionedHashes,omitempty"`
		YParity             *hexutil.Uint64   `json:"yParity,omitempty"`
	}
	result := &rpcTransaction{
		Hash:             "0x" + hex.EncodeToString(txHash),
		Nonce:            uint64ToHex(obj.ethTx.Nonce()),
		BlockHash:        blkHash,
		BlockNumber:      blkNum,
		TransactionIndex: txIndex,
		From:             obj.pubkey.Address().Hex(),
		To:               obj.to,
		Value:            value,
		GasPrice:         gasPrice,
		Gas:              uint64ToHex(obj.ethTx.Gas()),
		Input:            byteToHex(obj.ethTx.Data()),
		R:                hexutil.EncodeBig(r),
		S:                hexutil.EncodeBig(s),
		V:                hexutil.EncodeBig(v),
		Type:             hexutil.Uint64(obj.ethTx.Type()),
	}
	tx := obj.ethTx
	switch tx.Type() {
	case types.LegacyTxType:
		// if a legacy transaction has an EIP-155 chain id, include it explicitly
		if id := tx.ChainId(); id.Sign() != 0 {
			result.ChainID = (*hexutil.Big)(id)
		}

	case types.AccessListTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId())
		result.YParity = &yparity

	case types.DynamicFeeTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId())
		result.YParity = &yparity
		result.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap())
		result.GasTipCap = (*hexutil.Big)(tx.GasTipCap())
		// if the transaction has been mined, compute the effective gas price
		if obj.receipt != nil {
			result.GasPrice = hexutil.EncodeBig(obj.receipt.EffectiveGasPrice)
		}

	case types.BlobTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId())
		result.YParity = &yparity
		result.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap())
		result.GasTipCap = (*hexutil.Big)(tx.GasTipCap())
		// if the transaction has been mined, compute the effective gas price
		if obj.receipt != nil {
			result.GasPrice = hexutil.EncodeBig(obj.receipt.EffectiveGasPrice)
		}
		result.MaxFeePerBlobGas = (*hexutil.Big)(tx.BlobGasFeeCap())
		result.BlobVersionedHashes = tx.BlobHashes()
	}
	return json.Marshal(result)
}

func (obj *getReceiptResult) MarshalJSON() ([]byte, error) {
	if obj.receipt == nil {
		return nil, errInvalidObject
	}
	logs := make([]*getLogsResult, 0, len(obj.receipt.Logs()))
	for _, v := range append(obj.receipt.Logs(), obj.transferEvents...) {
		logs = append(logs, &getLogsResult{obj.blockHash, v})
	}

	return json.Marshal(&struct {
		TransactionIndex  string           `json:"transactionIndex"`
		TransactionHash   string           `json:"transactionHash"`
		BlockHash         string           `json:"blockHash"`
		BlockNumber       string           `json:"blockNumber"`
		From              string           `json:"from"`
		To                *string          `json:"to"`
		CumulativeGasUsed string           `json:"cumulativeGasUsed"`
		GasUsed           string           `json:"gasUsed"`
		ContractAddress   *string          `json:"contractAddress"`
		LogsBloom         string           `json:"logsBloom"`
		Logs              []*getLogsResult `json:"logs"`
		Status            string           `json:"status"`
		Type              hexutil.Uint     `json:"type"`
		EffectiveGasPrice *hexutil.Big     `json:"effectiveGasPrice"`
		BlobGasUsed       hexutil.Uint64   `json:"blobGasUsed,omitempty"`
		BlobGasPrice      *hexutil.Big     `json:"blobGasPrice,omitempty"`
	}{
		TransactionIndex:  uint64ToHex(uint64(obj.receipt.TxIndex)),
		TransactionHash:   "0x" + hex.EncodeToString(obj.receipt.ActionHash[:]),
		BlockHash:         "0x" + hex.EncodeToString(obj.blockHash[:]),
		BlockNumber:       uint64ToHex(obj.receipt.BlockHeight),
		From:              obj.from.Hex(),
		To:                obj.to,
		CumulativeGasUsed: uint64ToHex(obj.receipt.GasConsumed),
		GasUsed:           uint64ToHex(obj.receipt.GasConsumed),
		ContractAddress:   obj.contractAddress,
		LogsBloom:         getLogsBloomHex(obj.logsBloom),
		Logs:              logs,
		Status:            uint64ToHex(obj.receipt.Status),
		Type:              hexutil.Uint(obj.txType),
		EffectiveGasPrice: (*hexutil.Big)(obj.receipt.EffectiveGasPrice),
		BlobGasUsed:       hexutil.Uint64(obj.receipt.BlobGasUsed),
		BlobGasPrice:      (*hexutil.Big)(obj.receipt.BlobGasPrice),
	})
}

func (obj *getLogsResult) MarshalJSON() ([]byte, error) {
	if obj.log == nil {
		return nil, errInvalidObject
	}
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

func (obj *streamResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Jsonrpc string       `json:"jsonrpc"`
		Method  string       `json:"method"`
		Params  streamParams `json:"params"`
	}{
		Jsonrpc: "2.0",
		Method:  "eth_subscription",
		Params: streamParams{
			Subscription: obj.id,
			Result:       obj.result,
		},
	})
}

func (obj *blockTraceResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		TxHash string `json:"txHash"`
		Result any    `json:"result"`
	}{
		TxHash: "0x" + hex.EncodeToString(obj.TxHash[:]),
		Result: obj.Result,
	})
}

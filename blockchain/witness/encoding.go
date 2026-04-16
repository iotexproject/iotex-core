package witness

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/witness/witnesspb"
)

var _protoMagic = []byte{'W', 'I', 'T', 'P', 'B', 1}

func marshalStoredBlockResult(result *BlockResult) ([]byte, error) {
	pb, err := blockResultToProto(result)
	if err != nil {
		return nil, err
	}
	data, err := proto.Marshal(pb)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(_protoMagic)+len(data))
	out = append(out, _protoMagic...)
	out = append(out, data...)
	return out, nil
}

func marshalStoredBlockResultFromJSON(raw json.RawMessage) ([]byte, error) {
	result, err := parseBlockResultJSON(raw)
	if err != nil {
		return nil, err
	}
	return marshalStoredBlockResult(result)
}

func parseStoredBlockResult(raw []byte) (*BlockResult, error) {
	if !isProtoWitness(raw) {
		return parseBlockResultJSON(raw)
	}
	var pb witnesspb.BlockResult
	if err := proto.Unmarshal(raw[len(_protoMagic):], &pb); err != nil {
		return nil, err
	}
	return blockResultFromProto(&pb)
}

func storedBlockResultToJSON(raw []byte) (json.RawMessage, error) {
	if !isProtoWitness(raw) {
		return append(json.RawMessage(nil), raw...), nil
	}
	result, err := parseStoredBlockResult(raw)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func isProtoWitness(raw []byte) bool {
	return len(raw) >= len(_protoMagic) && bytes.Equal(raw[:len(_protoMagic)], _protoMagic)
}

func parseBlockResultJSON(raw []byte) (*BlockResult, error) {
	var result BlockResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func blockResultToProto(result *BlockResult) (*witnesspb.BlockResult, error) {
	if result == nil {
		return &witnesspb.BlockResult{}, nil
	}
	out := &witnesspb.BlockResult{
		Transactions:      make([]*witnesspb.TransactionResult, 0, len(result.Transactions)),
		DebugWriteEntries: append([]string(nil), result.DebugWriteEntries...),
	}
	if result.Summary != nil {
		out.Summary = &witnesspb.Summary{
			Contracts:          result.Summary.Contracts,
			Entries:            result.Summary.Entries,
			ProofNodes:         result.Summary.ProofNodes,
			ProofBytes:         result.Summary.ProofBytes,
			WitnessGenDuration: result.Summary.WitnessGenDuration,
		}
	}
	for _, tx := range result.Transactions {
		txHash, err := hexToBytes(tx.TxHash)
		if err != nil {
			return nil, errors.Wrap(err, "invalid transaction hash")
		}
		txPB := &witnesspb.TransactionResult{
			TxHash:          txHash,
			Contracts:       tx.Contracts,
			Entries:         tx.Entries,
			ProofNodes:      tx.ProofNodes,
			ProofBytes:      tx.ProofBytes,
			Witnesses:       make([]*witnesspb.ContractWitness, 0, len(tx.Witnesses)),
			DebugStorageOps: make([]*witnesspb.StorageOpTrace, 0, len(tx.DebugStorageOps)),
		}
		for _, witness := range tx.Witnesses {
			address, err := hexToBytes(witness.Address)
			if err != nil {
				return nil, errors.Wrap(err, "invalid witness address")
			}
			storageRoot, err := hexToBytes(witness.StorageRoot)
			if err != nil {
				return nil, errors.Wrap(err, "invalid witness storage root")
			}
			witnessPB := &witnesspb.ContractWitness{
				Address:     address,
				StorageRoot: storageRoot,
				Entries:     make([]*witnesspb.ContractWitnessKV, 0, len(witness.Entries)),
				ProofNodes:  make([][]byte, 0, len(witness.ProofNodes)),
			}
			for _, entry := range witness.Entries {
				key, err := hexToBytes(entry.Key)
				if err != nil {
					return nil, errors.Wrap(err, "invalid witness key")
				}
				value, err := hexToBytes(entry.Value)
				if err != nil {
					return nil, errors.Wrap(err, "invalid witness value")
				}
				witnessPB.Entries = append(witnessPB.Entries, &witnesspb.ContractWitnessKV{
					Key:   key,
					Value: value,
				})
			}
			for _, node := range witness.ProofNodes {
				proofNode, err := hexToBytes(node)
				if err != nil {
					return nil, errors.Wrap(err, "invalid witness proof node")
				}
				witnessPB.ProofNodes = append(witnessPB.ProofNodes, proofNode)
			}
			txPB.Witnesses = append(txPB.Witnesses, witnessPB)
		}
		for _, op := range tx.DebugStorageOps {
			txPB.DebugStorageOps = append(txPB.DebugStorageOps, &witnesspb.StorageOpTrace{
				Op:         op.Op,
				Addr:       op.Addr,
				Key:        op.Key,
				Value:      op.Value,
				SnapshotId: int32(op.SnapshotID),
				ErrMsg:     op.ErrMsg,
			})
		}
		out.Transactions = append(out.Transactions, txPB)
	}
	return out, nil
}

func blockResultFromProto(pb *witnesspb.BlockResult) (*BlockResult, error) {
	if pb == nil {
		return &BlockResult{}, nil
	}
	out := &BlockResult{
		Transactions:      make([]TransactionResult, 0, len(pb.GetTransactions())),
		DebugWriteEntries: append([]string(nil), pb.GetDebugWriteEntries()...),
	}
	if summary := pb.GetSummary(); summary != nil {
		out.Summary = &Summary{
			Contracts:          summary.GetContracts(),
			Entries:            summary.GetEntries(),
			ProofNodes:         summary.GetProofNodes(),
			ProofBytes:         summary.GetProofBytes(),
			WitnessGenDuration: summary.GetWitnessGenDuration(),
		}
	}
	for _, txPB := range pb.GetTransactions() {
		tx := TransactionResult{
			TxHash:          bytesToHex(txPB.GetTxHash()),
			Contracts:       txPB.GetContracts(),
			Entries:         txPB.GetEntries(),
			ProofNodes:      txPB.GetProofNodes(),
			ProofBytes:      txPB.GetProofBytes(),
			Witnesses:       make([]ContractWitness, 0, len(txPB.GetWitnesses())),
			DebugStorageOps: make([]evm.StorageOpTraceJSON, 0, len(txPB.GetDebugStorageOps())),
		}
		for _, witnessPB := range txPB.GetWitnesses() {
			witness := ContractWitness{
				Address:     bytesToHex(witnessPB.GetAddress()),
				StorageRoot: bytesToHex(witnessPB.GetStorageRoot()),
				Entries:     make([]ContractWitnessKV, 0, len(witnessPB.GetEntries())),
				ProofNodes:  make([]string, 0, len(witnessPB.GetProofNodes())),
			}
			for _, entryPB := range witnessPB.GetEntries() {
				witness.Entries = append(witness.Entries, ContractWitnessKV{
					Key:   bytesToHex(entryPB.GetKey()),
					Value: bytesToHex(entryPB.GetValue()),
				})
			}
			for _, node := range witnessPB.GetProofNodes() {
				witness.ProofNodes = append(witness.ProofNodes, bytesToHex(node))
			}
			tx.Witnesses = append(tx.Witnesses, witness)
		}
		for _, opPB := range txPB.GetDebugStorageOps() {
			if opPB == nil {
				continue
			}
			tx.DebugStorageOps = append(tx.DebugStorageOps, evm.StorageOpTraceJSON{
				Op:         opPB.GetOp(),
				Addr:       opPB.GetAddr(),
				Key:        opPB.GetKey(),
				Value:      opPB.GetValue(),
				SnapshotID: int(opPB.GetSnapshotId()),
				ErrMsg:     opPB.GetErrMsg(),
			})
		}
		if len(tx.Witnesses) == 0 {
			tx.Witnesses = nil
		}
		if len(tx.DebugStorageOps) == 0 {
			tx.DebugStorageOps = nil
		}
		out.Transactions = append(out.Transactions, tx)
	}
	if len(out.Transactions) == 0 {
		out.Transactions = nil
	}
	if len(out.DebugWriteEntries) == 0 {
		out.DebugWriteEntries = nil
	}
	return out, nil
}

func hexToBytes(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	return decodeHex(value)
}

func decodeHex(value string) ([]byte, error) {
	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == value {
		trimmed = strings.TrimPrefix(value, "0X")
	}
	if trimmed == "" {
		return nil, nil
	}
	if len(trimmed)%2 == 1 {
		trimmed = "0" + trimmed
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func bytesToHex(value []byte) string {
	if len(value) == 0 {
		return ""
	}
	return "0x" + hex.EncodeToString(value)
}

package witness

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type Collector struct {
	current         map[common.Address]*evm.ContractStorageWitness
	actionWitnesses map[hash.Hash256]map[common.Address]*evm.ContractStorageWitness
}

func NewCollector() *Collector {
	return &Collector{
		actionWitnesses: make(map[hash.Hash256]map[common.Address]*evm.ContractStorageWitness),
	}
}

func (c *Collector) CaptureContractStorageAccesses([]evm.ContractStorageAccess) {}

func (c *Collector) CaptureContractStorageWitnesses(witnesses map[common.Address]*evm.ContractStorageWitness) {
	c.current = cloneWitnessMap(witnesses)
}

func (c *Collector) CaptureTx(_ []byte, receipt *action.Receipt) {
	if receipt == nil {
		return
	}
	c.actionWitnesses[receipt.ActionHash] = cloneWitnessMap(c.current)
	c.current = nil
}

func (c *Collector) Build(blk *block.Block) (*BlockResult, error) {
	result := &BlockResult{
		Transactions: make([]TransactionResult, 0, len(blk.Actions)),
	}
	for _, selp := range blk.Actions {
		actionHash, err := selp.Hash()
		if err != nil {
			return nil, err
		}
		txWitnesses := fromEVMWitnesses(c.actionWitnesses[actionHash])
		txResult := TransactionResult{
			TxHash:    "0x" + hex.EncodeToString(actionHash[:]),
			Witnesses: txWitnesses,
		}
		if summary := summarizeWitnesses(txWitnesses); summary != nil {
			txResult.Contracts = summary.Contracts
			txResult.Entries = summary.Entries
			txResult.ProofNodes = summary.ProofNodes
			txResult.ProofBytes = summary.ProofBytes
			if result.Summary == nil {
				result.Summary = &Summary{}
			}
			result.Summary.Contracts += summary.Contracts
			result.Summary.Entries += summary.Entries
			result.Summary.ProofNodes += summary.ProofNodes
			result.Summary.ProofBytes += summary.ProofBytes
		}
		result.Transactions = append(result.Transactions, txResult)
	}
	return result, nil
}

func cloneWitnessMap(witnesses map[common.Address]*evm.ContractStorageWitness) map[common.Address]*evm.ContractStorageWitness {
	if len(witnesses) == 0 {
		return nil
	}
	cloned := make(map[common.Address]*evm.ContractStorageWitness, len(witnesses))
	for addr, witness := range witnesses {
		if witness == nil {
			continue
		}
		clone := &evm.ContractStorageWitness{
			StorageRoot: witness.StorageRoot,
			Entries:     append([]evm.ContractStorageWitnessEntry(nil), witness.Entries...),
			ProofNodes:  make([][]byte, 0, len(witness.ProofNodes)),
		}
		for i := range clone.Entries {
			clone.Entries[i].Value = append([]byte(nil), clone.Entries[i].Value...)
		}
		for _, node := range witness.ProofNodes {
			clone.ProofNodes = append(clone.ProofNodes, append([]byte(nil), node...))
		}
		cloned[addr] = clone
	}
	return cloned
}

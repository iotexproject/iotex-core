package witness

import (
	"bytes"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
)

type (
	Summary struct {
		Contracts  uint64 `json:"contracts"`
		Entries    uint64 `json:"entries"`
		ProofNodes uint64 `json:"proofNodes"`
		ProofBytes uint64 `json:"proofBytes"`
	}

	BlockResult struct {
		Summary      *Summary            `json:"summary,omitempty"`
		Transactions []TransactionResult `json:"transactions"`
	}

	TransactionResult struct {
		TxHash     string            `json:"txHash"`
		Contracts  uint64            `json:"contracts"`
		Entries    uint64            `json:"entries"`
		ProofNodes uint64            `json:"proofNodes"`
		ProofBytes uint64            `json:"proofBytes"`
		Witnesses  []ContractWitness `json:"witnesses,omitempty"`
	}

	ContractWitness struct {
		Address     string              `json:"address"`
		StorageRoot string              `json:"storageRoot"`
		Entries     []ContractWitnessKV `json:"entries,omitempty"`
		ProofNodes  []string            `json:"proofNodes,omitempty"`
	}

	ContractWitnessKV struct {
		Key   string `json:"key"`
		Value string `json:"value,omitempty"`
	}
)

func summarizeWitnesses(witnesses []ContractWitness) *Summary {
	if len(witnesses) == 0 {
		return nil
	}
	summary := &Summary{
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

func fromEVMWitnesses(witnesses map[common.Address]*evm.ContractStorageWitness) []ContractWitness {
	if len(witnesses) == 0 {
		return nil
	}
	addrs := make([]common.Address, 0, len(witnesses))
	for addr := range witnesses {
		addrs = append(addrs, addr)
	}
	sortAddresses(addrs)
	out := make([]ContractWitness, 0, len(addrs))
	for _, addr := range addrs {
		witness := witnesses[addr]
		if witness == nil {
			continue
		}
		item := ContractWitness{
			Address:     addr.Hex(),
			StorageRoot: "0x" + hex.EncodeToString(witness.StorageRoot[:]),
		}
		if len(witness.Entries) > 0 {
			item.Entries = make([]ContractWitnessKV, 0, len(witness.Entries))
			for _, entry := range witness.Entries {
				entryJSON := ContractWitnessKV{
					Key: "0x" + hex.EncodeToString(entry.Key[:]),
				}
				if len(entry.Value) > 0 {
					entryJSON.Value = "0x" + hex.EncodeToString(entry.Value)
				}
				item.Entries = append(item.Entries, entryJSON)
			}
		}
		if len(witness.ProofNodes) > 0 {
			item.ProofNodes = make([]string, 0, len(witness.ProofNodes))
			for _, node := range witness.ProofNodes {
				item.ProofNodes = append(item.ProofNodes, "0x"+hex.EncodeToString(node))
			}
		}
		out = append(out, item)
	}
	return out
}

func (w ContractWitness) toEVMWitness() (*evm.ContractStorageWitness, error) {
	rootBytes := common.FromHex(w.StorageRoot)
	if len(rootBytes) != len(common.Hash{}) {
		return nil, errors.Errorf("invalid storage root length for %s", w.Address)
	}
	out := &evm.ContractStorageWitness{
		Entries:    make([]evm.ContractStorageWitnessEntry, 0, len(w.Entries)),
		ProofNodes: make([][]byte, 0, len(w.ProofNodes)),
	}
	copy(out.StorageRoot[:], rootBytes)
	for _, entry := range w.Entries {
		keyBytes := common.FromHex(entry.Key)
		if len(keyBytes) != len(common.Hash{}) {
			return nil, errors.Errorf("invalid storage key length for %s", entry.Key)
		}
		witnessEntry := evm.ContractStorageWitnessEntry{}
		copy(witnessEntry.Key[:], keyBytes)
		if entry.Value != "" {
			witnessEntry.Value = common.FromHex(entry.Value)
		}
		out.Entries = append(out.Entries, witnessEntry)
	}
	for _, node := range w.ProofNodes {
		out.ProofNodes = append(out.ProofNodes, common.FromHex(node))
	}
	return out, nil
}

func sortAddresses(addrs []common.Address) {
	for i := 0; i < len(addrs); i++ {
		for j := i + 1; j < len(addrs); j++ {
			if bytes.Compare(addrs[j][:], addrs[i][:]) < 0 {
				addrs[i], addrs[j] = addrs[j], addrs[i]
			}
		}
	}
}

package witness

import (
	"context"
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	byHashNamespace       = "blockWitnessByHash"
	hashByHeightNamespace = "blockWitnessHashByHeight"
)

type Store struct {
	db db.KVStore
}

func NewStore(dbPath string) *Store {
	return &Store{
		db: db.NewBoltDB(db.Config{DbPath: dbPath, NumRetries: 3}),
	}
}

func (s *Store) Start(ctx context.Context) error { return s.db.Start(ctx) }

func (s *Store) Stop(ctx context.Context) error { return s.db.Stop(ctx) }

func (s *Store) PutRaw(blockHash hash.Hash256, height uint64, raw json.RawMessage) error {
	if err := s.db.Put(byHashNamespace, blockHash[:], raw); err != nil {
		return err
	}
	return s.db.Put(hashByHeightNamespace, byteutil.Uint64ToBytesBigEndian(height), blockHash[:])
}

func (s *Store) PutBlock(blockHash hash.Hash256, height uint64, result *BlockResult) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.PutRaw(blockHash, height, raw)
}

func (s *Store) GetRawByHash(blockHash hash.Hash256) (json.RawMessage, error) {
	raw, err := s.db.Get(byHashNamespace, blockHash[:])
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), raw...), nil
}

func (s *Store) GetRawByHeight(height uint64) (hash.Hash256, json.RawMessage, error) {
	blockHashBytes, err := s.db.Get(hashByHeightNamespace, byteutil.Uint64ToBytesBigEndian(height))
	if err != nil {
		return hash.ZeroHash256, nil, err
	}
	blockHash := hash.BytesToHash256(blockHashBytes)
	raw, err := s.GetRawByHash(blockHash)
	if err != nil {
		return hash.ZeroHash256, nil, err
	}
	return blockHash, raw, nil
}

func ParseValidationContext(raw json.RawMessage) (evm.StatelessValidationContext, error) {
	var result BlockResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return evm.StatelessValidationContext{}, err
	}
	actionWitnesses := make(map[hash.Hash256]map[common.Address]*evm.ContractStorageWitness, len(result.Transactions))
	for _, tx := range result.Transactions {
		txHash := common.HexToHash(tx.TxHash)
		actionHash := hash.BytesToHash256(txHash[:])
		contractWitnesses := make(map[common.Address]*evm.ContractStorageWitness, len(tx.Witnesses))
		for _, witness := range tx.Witnesses {
			evmWitness, err := witness.toEVMWitness()
			if err != nil {
				return evm.StatelessValidationContext{}, err
			}
			contractWitnesses[common.HexToAddress(witness.Address)] = evmWitness
		}
		actionWitnesses[actionHash] = contractWitnesses
	}
	return evm.StatelessValidationContext{
		Enabled:         true,
		ActionWitnesses: actionWitnesses,
	}, nil
}

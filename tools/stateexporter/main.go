// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// stateexporter exports specified contracts' state (account, code, storage slots)
// from a full iotex-core stateDB into a standalone PebbleDB file.
//
// Usage:
//
//	go run . --source /path/to/trie.db --output /path/to/hotstate.db --contracts io1abc,io1def,...
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/trie/triepb"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/state/factory"
)

var (
	_sourcePath    string
	_outputPath    string
	_contractAddrs string
	_contractsFile string
)

func init() {
	flag.StringVar(&_sourcePath, "source", "", "Source stateDB path (trie.db)")
	flag.StringVar(&_outputPath, "output", "", "Output PebbleDB path for hot state")
	flag.StringVar(&_contractAddrs, "contracts", "", "Comma-separated contract addresses (io1... format)")
	flag.StringVar(&_contractsFile, "contracts-file", "", "File containing contract addresses, one per line")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: stateexporter --source <path> --output <path> [--contracts <addr1,addr2,...>] [--contracts-file <path>]\n\n")
		fmt.Fprintf(os.Stderr, "Exports specified contracts' state from a full stateDB into a standalone PebbleDB.\n")
		fmt.Fprintf(os.Stderr, "Provide addresses via --contracts, --contracts-file, or both.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
}

// rawStateReader implements protocol.StateReader by reading directly from a raw db.KVStore.
// It provides read-only access to stateDB data for trie reconstruction.
type rawStateReader struct {
	dao    db.KVStore
	height uint64
}

func newRawStateReader(dao db.KVStore) (*rawStateReader, error) {
	h, err := dao.Get(factory.AccountKVNamespace, []byte(factory.CurrentHeightKey))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read current height")
	}
	return &rawStateReader{
		dao:    dao,
		height: byteutil.BytesToUint64(h),
	}, nil
}

func (r *rawStateReader) Height() (uint64, error) {
	return r.height, nil
}

func (r *rawStateReader) State(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return 0, err
	}
	ns := cfg.Namespace
	if ns == "" {
		ns = factory.AccountKVNamespace
	}
	data, err := r.dao.Get(ns, cfg.Key)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == db.ErrBucketNotExist {
			return 0, errors.Wrapf(state.ErrStateNotExist, "state of %x doesn't exist in ns=%s", cfg.Key, ns)
		}
		return 0, errors.Wrapf(err, "error when getting state of %x from ns=%s", cfg.Key, ns)
	}
	if err := state.Deserialize(s, data); err != nil {
		return 0, errors.Wrapf(err, "error when deserializing state data")
	}
	return r.height, nil
}

func (r *rawStateReader) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	return 0, nil, errors.New("States() not supported in rawStateReader")
}

func (r *rawStateReader) ReadView(name string) (protocol.View, error) {
	return nil, errors.New("ReadView() not supported in rawStateReader")
}

// copyTrieNodes copies all trie nodes reachable from rootHash in the "Contract" namespace
// from srcDAO to outDAO. It walks the trie structure via protobuf parsing (DFS)
// and copies each node's raw bytes directly, preserving the exact trie structure and root hash.
func copyTrieNodes(srcDAO, outDAO db.KVStore, rootHash []byte) (nodeCount, leafCount int, err error) {
	if len(rootHash) == 0 || bytes.Equal(rootHash, hash.ZeroHash256[:]) {
		return 0, 0, nil
	}

	ns := evm.ContractKVNameSpace
	stack := [][]byte{rootHash}
	visited := make(map[string]bool)

	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		key := string(h)
		if visited[key] {
			continue
		}
		visited[key] = true

		// Read raw node data from source
		data, err := srcDAO.Get(ns, h)
		if err != nil {
			return nodeCount, leafCount, errors.Wrapf(err, "failed to read trie node %x", h)
		}

		// Write to output (exact same key and value)
		if err := outDAO.Put(ns, h, data); err != nil {
			return nodeCount, leafCount, errors.Wrapf(err, "failed to write trie node %x", h)
		}
		nodeCount++

		// Parse protobuf to find child references
		pb := &triepb.NodePb{}
		if err := proto.Unmarshal(data, pb); err != nil {
			return nodeCount, leafCount, errors.Wrapf(err, "failed to unmarshal trie node %x", h)
		}

		switch {
		case pb.GetBranch() != nil:
			for _, child := range pb.GetBranch().Branches {
				stack = append(stack, child.Path)
			}
		case pb.GetExtend() != nil:
			stack = append(stack, pb.GetExtend().Value)
		case pb.GetLeaf() != nil:
			leafCount++
		}

		if nodeCount%10000 == 0 {
			log.L().Info("copying trie nodes...", zap.Int("nodes", nodeCount), zap.Int("leaves", leafCount))
		}
	}
	return nodeCount, leafCount, nil
}

// exportContract exports a single contract's data (account, code, all storage trie nodes)
// from source DB to output DB via direct raw copy.
func exportContract(
	srcReader *rawStateReader,
	outDAO db.KVStore,
	addr address.Address,
) (nodeCount, leafCount int, err error) {
	addrHash := hash.BytesToHash160(addr.Bytes())
	addrStr := addr.String()

	// 1. Load account from source
	account := &state.Account{}
	if _, err := srcReader.State(account, protocol.LegacyKeyOption(addrHash)); err != nil {
		return 0, 0, errors.Wrapf(err, "failed to load account for %s", addrStr)
	}
	if !account.IsContract() {
		return 0, 0, fmt.Errorf("address %s is not a contract (no CodeHash)", addrStr)
	}
	log.L().Info("loaded account",
		zap.String("address", addrStr),
		zap.String("root", fmt.Sprintf("%x", account.Root)),
		zap.String("codeHash", fmt.Sprintf("%x", account.CodeHash)),
		zap.Uint64("nonce", account.PendingNonce()),
		zap.String("balance", account.Balance.String()),
	)

	// 2. Write account to output
	if err := outDAO.Put(factory.AccountKVNamespace, addrHash[:], mustSerialize(account)); err != nil {
		return 0, 0, errors.Wrapf(err, "failed to write account for %s", addrStr)
	}

	// 3. Copy contract code
	if len(account.CodeHash) > 0 {
		code, err := srcReader.dao.Get(evm.CodeKVNameSpace, account.CodeHash)
		if err != nil {
			return 0, 0, errors.Wrapf(err, "failed to read code for %s", addrStr)
		}
		if err := outDAO.Put(evm.CodeKVNameSpace, account.CodeHash, code); err != nil {
			return 0, 0, errors.Wrapf(err, "failed to write code for %s", addrStr)
		}
		log.L().Info("copied code", zap.String("address", addrStr), zap.Int("codeSize", len(code)))
	}

	// 4. Copy all trie nodes (raw byte-for-byte copy preserves exact root hash)
	if account.Root == hash.ZeroHash256 {
		log.L().Info("contract has empty storage (zero root)", zap.String("address", addrStr))
		return 0, 0, nil
	}

	nodeCount, leafCount, err = copyTrieNodes(srcReader.dao, outDAO, account.Root[:])
	if err != nil {
		return nodeCount, leafCount, errors.Wrapf(err, "failed to copy trie nodes for %s", addrStr)
	}

	// 5. Verify root node exists in output DB
	if _, err := outDAO.Get(evm.ContractKVNameSpace, account.Root[:]); err != nil {
		return nodeCount, leafCount, errors.Wrapf(err, "verification failed: root node %x not found in output DB for %s", account.Root, addrStr)
	}

	log.L().Info("exported contract storage",
		zap.String("address", addrStr),
		zap.Int("trieNodes", nodeCount),
		zap.Int("storageSlots", leafCount),
		zap.String("rootHash", fmt.Sprintf("%x", account.Root)),
	)
	return nodeCount, leafCount, nil
}

func mustSerialize(s state.Serializer) []byte {
	data, err := s.Serialize()
	if err != nil {
		panic(fmt.Sprintf("failed to serialize: %v", err))
	}
	return data
}

// loadAddresses collects contract addresses from --contracts (comma-separated)
// and --contracts-file (one per line, supports # comments and blank lines).
// Duplicates are removed.
func loadAddresses(csv, filePath string) ([]address.Address, error) {
	seen := make(map[string]bool)
	var addrs []address.Address

	addOne := func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" || strings.HasPrefix(s, "#") {
			return nil
		}
		if seen[s] {
			return nil
		}
		addr, err := address.FromString(s)
		if err != nil {
			return errors.Wrapf(err, "invalid address %q", s)
		}
		seen[s] = true
		addrs = append(addrs, addr)
		return nil
	}

	// from --contracts flag
	if csv != "" {
		for _, s := range strings.Split(csv, ",") {
			if err := addOne(s); err != nil {
				return nil, err
			}
		}
	}

	// from --contracts-file
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open contracts file %s", filePath)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			if err := addOne(scanner.Text()); err != nil {
				return nil, errors.Wrapf(err, "line %d of %s", lineNo, filePath)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, errors.Wrapf(err, "error reading %s", filePath)
		}
	}

	if len(addrs) == 0 {
		return nil, errors.New("no valid contract addresses provided")
	}
	return addrs, nil
}

func main() {
	if _sourcePath == "" || _outputPath == "" {
		flag.Usage()
		os.Exit(1)
	}
	if _contractAddrs == "" && _contractsFile == "" {
		fmt.Fprintf(os.Stderr, "Error: at least one of --contracts or --contracts-file is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Collect contract addresses from both sources
	addrs, err := loadAddresses(_contractAddrs, _contractsFile)
	if err != nil {
		log.L().Fatal("failed to load contract addresses", zap.Error(err))
	}
	if len(addrs) == 0 {
		log.L().Fatal("no valid contract addresses provided")
	}

	ctx := context.Background()

	// Open source DB read-only
	srcCfg := db.DefaultConfig
	srcCfg.ReadOnly = true
	srcDAO, err := db.CreateKVStore(srcCfg, _sourcePath)
	if err != nil {
		log.L().Fatal("failed to create source KVStore", zap.Error(err))
	}
	if err := srcDAO.Start(ctx); err != nil {
		log.L().Fatal("failed to start source DB", zap.Error(err))
	}
	defer srcDAO.Stop(ctx)

	srcReader, err := newRawStateReader(srcDAO)
	if err != nil {
		log.L().Fatal("failed to create state reader", zap.Error(err))
	}
	log.L().Info("source DB opened",
		zap.String("path", _sourcePath),
		zap.Uint64("height", srcReader.height),
	)

	// Open output DB (new PebbleDB)
	outCfg := db.DefaultConfig
	outCfg.DBType = db.DBPebble
	outDAO, err := db.CreateKVStore(outCfg, _outputPath)
	if err != nil {
		log.L().Fatal("failed to create output KVStore", zap.Error(err))
	}
	if err := outDAO.Start(ctx); err != nil {
		log.L().Fatal("failed to start output DB", zap.Error(err))
	}
	defer outDAO.Stop(ctx)

	// Write height to output
	if err := outDAO.Put(factory.AccountKVNamespace, []byte(factory.CurrentHeightKey), byteutil.Uint64ToBytes(srcReader.height)); err != nil {
		log.L().Fatal("failed to write height to output DB", zap.Error(err))
	}

	// Export each contract
	totalNodes := 0
	totalLeaves := 0
	for _, addr := range addrs {
		nodes, leaves, err := exportContract(srcReader, outDAO, addr)
		if err != nil {
			log.L().Error("failed to export contract",
				zap.String("address", addr.String()),
				zap.Error(err),
			)
			continue
		}
		totalNodes += nodes
		totalLeaves += leaves
	}

	log.L().Info("export complete",
		zap.Int("contracts", len(addrs)),
		zap.Int("trieNodes", totalNodes),
		zap.Int("storageSlots", totalLeaves),
		zap.Uint64("height", srcReader.height),
		zap.String("output", _outputPath),
	)
}

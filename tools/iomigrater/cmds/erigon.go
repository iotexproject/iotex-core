package cmd

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/erigontech/erigon-lib/chain"
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/kv/mdbx"
	erigonlog "github.com/erigontech/erigon-lib/log/v3"
	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
	"github.com/iotexproject/iotex-core/v2/tools/iomigrater/common"
)

var (
	initErigonDbCmdUse = map[string]string{
		"english": "init-erigon-db",
		"chinese": "初始化 Erigon 数据库",
	}
	initErigonDbCmdShorts = map[string]string{
		"english": "Initialize Erigon database for IoTeX",
		"chinese": "初始化 IoTeX 的 Erigon 数据库",
	}

	// InitErigonDb Used to Sub command.
	InitErigonDb = &cobra.Command{
		Use:   common.TranslateInLang(initErigonDbCmdUse) + " <trieDBPath> <erigonDBPath>",
		Short: common.TranslateInLang(migrateDbCmdShorts),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			validate, _ := cmd.Flags().GetBool("validate")
			return initErigonDB(args[0], args[1], dryRun, validate)
		},
	}
)

func init() {
	InitErigonDb.Flags().Bool("dry-run", false, "Run in dry-run mode without committing changes to database")
	InitErigonDb.Flags().Bool("validate", false, "Run in validate mode without committing changes to database")
}

func initErigonDB(trieDBPath, erigonDBPath string, dryRun bool, validate bool) error {
	fmt.Println("🚀 Initializing Erigon database...", trieDBPath, erigonDBPath)
	if dryRun {
		fmt.Println("🧪 Running in dry-run mode - no changes will be committed")
	}

	// prepare databases
	g, err := genesis.New("")
	if err != nil {
		return errors.Wrap(err, "failed to create genesis")
	}
	cfg, err := config.New(nil, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create config")
	}
	ctx := context.Background()
	statedb, height, err := openStateDB(trieDBPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open stateDB at %s", trieDBPath)
	}
	defer statedb.Stop(ctx)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: height,
	})
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		EvmNetworkID: cfg.Chain.EVMNetworkID,
		GetBlockTime: func(uint64) (time.Time, error) {
			return time.Time{}, nil
		},
	})
	edb, err := openErigonDB(ctx, erigonDBPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open erigonDB at %s", erigonDBPath)
	}
	defer edb.Close()

	// prepare state reader and writer
	tx, err := edb.BeginRw(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r := erigonstate.NewPlainStateReader(tx)
	intraBlockState := erigonstate.New(r)
	sm := &stateWriter{
		ctx:             ctx,
		intraBlockState: intraBlockState,
		sr:              r}
	sr := &stateReader{store: statedb}

	// initializing includes:
	// 1. create system contracts on erigonDB if not exists
	var backend systemcontracts.ContractDeployer
	backend = factory.NewContractBackend(ctx, intraBlockState, r)
	if err := systemcontracts.DeploySystemContractsIfNotExist(backend); err != nil {
		return err
	}

	// 2. read staking state from trieDB and write to erigonDB
	stakingState, err := readStakingStates(sr)
	if err != nil {
		return err
	}

	//  - candidates
	candidates, err := readCandidates(&stakingState.candidates)
	if err != nil {
		return err
	}
	if !validate {
		if err = putCandidates(sm, candidates); err != nil {
			return errors.Wrap(err, "failed to put candidates")
		}
	}
	if err = checkCandidates(sm, candidates); err != nil {
		return err
	}
	//  - candidate list
	keys, candidateLists, err := readCandidateList(&stakingState.candidateList)
	if err != nil {
		return errors.Wrap(err, "failed to read candidate list")
	}
	if !validate {
		if err = putCandidateList(sm, keys, candidateLists); err != nil {
			return errors.Wrap(err, "failed to put candidate list")
		}
	}
	if err = checkCandidateList(sm, candidateLists); err != nil {
		return errors.Wrap(err, "failed to check candidate list")
	}
	//  - endorsement
	keys, endorsements, err := readEndorsements(&stakingState.endorsements)
	if err != nil {
		return errors.Wrap(err, "failed to read endorsements")
	}
	if !validate {
		if err = putEndorsements(sm, keys, endorsements); err != nil {
			return errors.Wrap(err, "failed to put endorsements")
		}
	}
	if err = checkEndorsements(sm, keys, endorsements); err != nil {
		return errors.Wrap(err, "failed to check endorsements")
	}

	//  - total bucket count
	totalBucket, err := readTotalBucketCount(&stakingState.totalBucket)
	if err != nil {
		return errors.Wrap(err, "failed to read total bucket count")
	}
	if !validate {
		if err = putTotalBucketCount(sm, totalBucket); err != nil {
			return errors.Wrap(err, "failed to put total bucket count")
		}
	}
	if err = checkTotalBucketCount(sm, totalBucket); err != nil {
		return errors.Wrap(err, "failed to check total bucket count")
	}
	//  - vote bucket
	buckets, err := readBuckets(&stakingState.voteBuckets)
	if err != nil {
		return errors.Wrap(err, "failed to read vote buckets")
	}
	if !validate {
		if err = putBuckets(sm, buckets); err != nil {
			return errors.Wrap(err, "failed to put vote buckets")
		}
	}
	if err = checkBuckets(sm, buckets); err != nil {
		return errors.Wrap(err, "failed to check vote buckets")
	}
	//  - totalAmount
	totalAmount, err := readTotalAmount(&stakingState.totalAmount)
	if err != nil {
		return errors.Wrap(err, "failed to read total amount")
	}
	if !validate {
		if err = putTotalAmount(sm, totalAmount); err != nil {
			return errors.Wrap(err, "failed to put total amount")
		}
	}
	if err = checkTotalAmount(sm, totalAmount); err != nil {
		return errors.Wrap(err, "failed to check total amount")
	}

	//  - bucket indices
	biKeys, indices, err := readBucketIndices(&stakingState.bucketIndices)
	if err != nil {
		return errors.Wrap(err, "failed to read bucket indices")
	}
	if !validate {
		if err = putBucketIndices(sm, biKeys, indices); err != nil {
			return errors.Wrap(err, "failed to put bucket indices")
		}
	}
	if err = checkBucketIndices(sm, indices); err != nil {
		return errors.Wrap(err, "failed to check bucket indices")
	}
	// commit the changes to erigonDB
	return commitErigonDB(0, tx, sm, dryRun)
}

func openErigonDB(ctx context.Context, path string) (kv.RwDB, error) {
	log.L().Info("📊 starting history state index")
	lg := erigonlog.New()
	lg.SetHandler(erigonlog.StdoutHandler)
	rw, err := mdbx.NewMDBX(lg).Path(path).WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
		defaultBuckets["erigonsystem"] = kv.TableCfgItem{}
		return defaultBuckets
	}).Open(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open history state index")
	}
	return rw, nil
}

func openStateDB(path string) (db.KVStore, uint64, error) {
	dbCfg := db.DefaultConfig
	store, err := db.CreateKVStore(dbCfg, path)
	if err != nil {
		return nil, 0, errors.Wrapf(err, "failed to open stateDB at %s", path)
	}
	if err := store.Start(context.Background()); err != nil {
		return nil, 0, errors.Wrapf(err, "failed to start stateDB at %s", path)
	}
	heightBytes, err := store.Get("Account", []byte("currentHeight"))
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	height := byteutil.BytesToUint64(heightBytes)
	return store, height, nil
}

func readCandidates(state *stakingState) ([]*staking.Candidate, error) {
	cands := make(staking.CandidateList, 0, len(state.Keys))
	for i := 0; i < len(state.Keys); i++ {
		c := &staking.Candidate{}
		if err := c.Deserialize(state.Values[i]); err != nil {
			return nil, errors.Wrapf(err, "failed to deserialize candidate")
		}
		cands = append(cands, c)
	}
	return cands, nil
}

func putCandidates(sm *stateWriter, cands []*staking.Candidate) error {
	for _, c := range cands {
		if err := sm.PutObject("Candidate", c.GetIdentifier().Bytes(), c); err != nil {
			return errors.Wrapf(err, "failed to put candidate %s", c.GetIdentifier().String())
		}
	}
	return nil
}

func checkCandidates(sm *stateWriter, expect []*staking.Candidate) error {
	_, actual, err := sm.List("Candidate", &staking.Candidate{})
	if err != nil {
		return err
	}
	if len(actual) != len(expect) {
		return fmt.Errorf("candidate list size mismatch: expect %d, actual %d", len(expect), len(actual))
	}
	sort.Slice(expect, func(i, j int) bool {
		return expect[i].GetIdentifier().String() < expect[j].GetIdentifier().String()
	})
	sort.Slice(actual, func(i, j int) bool {
		return actual[i].(*staking.Candidate).GetIdentifier().String() < actual[j].(*staking.Candidate).GetIdentifier().String()
	})
	for i := range expect {
		if !expect[i].Equal(actual[i].(*staking.Candidate)) {
			return fmt.Errorf("candidate identifier mismatch at index %d: expect %+v, actual %+v",
				i, expect[i], actual[i].(*staking.Candidate))
		}
	}
	fmt.Println("\033[32m✅ Candidate list check passed\033[0m")
	return nil
}

type stateReader struct {
	store db.KVStore
}

func (sr *stateReader) States(namespace string, keys [][]byte) (uint64, state.Iterator, error) {
	keys, values, err := readStates(sr.store, namespace, keys)
	if err != nil {
		return 0, nil, err
	}
	iter, err := state.NewIterator(keys, values)
	if err != nil {
		return 0, nil, err
	}
	return 0, iter, nil
}

func readStates(kvStore db.KVStore, namespace string, keys [][]byte) ([][]byte, [][]byte, error) {
	var (
		ks, values [][]byte
		err        error
	)
	if keys == nil {
		ks, values, err = kvStore.Filter(namespace, func(_, _ []byte) bool { return true }, nil, nil)
		if err != nil {
			if errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == db.ErrBucketNotExist {
				return nil, nil, errors.Wrapf(state.ErrStateNotExist, "failed to get states of ns = %x", namespace)
			}
			return nil, nil, err
		}
		return ks, values, nil
	}
	for _, key := range keys {
		value, err := kvStore.Get(namespace, key)
		switch errors.Cause(err) {
		case db.ErrNotExist, db.ErrBucketNotExist:
			values = append(values, nil)
			ks = append(ks, key)
		case nil:
			values = append(values, value)
			ks = append(ks, key)
		default:
			return nil, nil, err
		}
	}
	return ks, values, nil
}

type stateWriter struct {
	ctx             context.Context
	intraBlockState *erigonstate.IntraBlockState
	sr              erigonstate.StateReader
}

func (store *stateWriter) PutObject(ns string, key []byte, obj any) (err error) {
	if cs, ok := obj.(protocol.ContractStorage); ok {
		log.L().Debug("put object", zap.String("namespace", ns), log.Hex("key", key), zap.String("type", fmt.Sprintf("%T", obj)), zap.String("content", fmt.Sprintf("%+v", obj)))
		return cs.StoreToContract(ns, key, factory.NewContractBackend(store.ctx, store.intraBlockState, store.sr))
	}
	return errors.Wrapf(factory.ErrNotSupported, "PutObject is not supported for type %T in namespace %s with key %x", obj, ns, key)
}

func (store *stateWriter) GetObject(ns string, key []byte, obj any) error {
	if cs, ok := obj.(protocol.ContractStorage); ok {
		log.L().Debug("get object", zap.String("namespace", ns), log.Hex("key", key), zap.String("type", fmt.Sprintf("%T", obj)))
		return cs.LoadFromContract(ns, key, factory.NewContractBackend(store.ctx, store.intraBlockState, store.sr))
	}
	return errors.Wrapf(factory.ErrNotSupported, "GetObject is not supported for type %T in namespace %s with key %x", obj, ns, key)
}

func (store *stateWriter) List(ns string, obj any) ([][]byte, []any, error) {
	if cs, ok := obj.(protocol.ContractStorage); ok {
		log.L().Debug("list object", zap.String("namespace", ns), zap.String("type", fmt.Sprintf("%T", obj)))
		return cs.ListFromContract(ns, factory.NewContractBackend(store.ctx, store.intraBlockState, store.sr))
	}
	return nil, nil, errors.Wrapf(factory.ErrNotSupported, "List is not supported for type %T in namespace %s", obj, ns)
}

func commitErigonDB(height uint64, tx kv.RwTx, sm *stateWriter, dryRun bool) error {
	tsw := erigonstate.NewPlainStateWriter(tx, tx, height)
	rules := &chain.Rules{}

	err := sm.intraBlockState.FinalizeTx(rules, erigonstate.NewNoopWriter())
	if err != nil {
		return errors.Wrap(err, "failed to finalize transaction")
	}

	err = sm.intraBlockState.CommitBlock(rules, tsw)
	if err != nil {
		return errors.Wrap(err, "failed to commit block to erigonDB")
	}
	err = tsw.WriteChangeSets()
	if err != nil {
		return errors.Wrap(err, "failed to write change sets to erigonDB")
	}
	err = tsw.WriteHistory()
	if err != nil {
		return errors.Wrap(err, "failed to write history to erigonDB")
	}
	if dryRun {
		fmt.Println("\033[33m🧪 Dry-run mode: Erigon database changes will be rolled back\033[0m")
		tx.Rollback()
		return nil
	}
	fmt.Println("\033[33m💾 Erigon database about to be committed\033[0m")
	return tx.Commit()
}

func readCandidateList(sstate *stakingState) ([][]byte, []staking.CandidateList, error) {
	results := make([]staking.CandidateList, len(sstate.Keys))
	for i := range results {
		if err := state.Deserialize(&results[i], sstate.Values[i]); err != nil {
			return nil, nil, err
		}
	}
	return sstate.Keys, results, nil
}

func putCandidateList(sm *stateWriter, keys [][]byte, candidateLists []staking.CandidateList) error {
	for i, cl := range candidateLists {
		if err := sm.PutObject("CandsMap", keys[i], &cl); err != nil {
			return errors.Wrapf(err, "failed to put candidate list %s", string(keys[i]))
		}
	}
	return nil
}

func checkCandidateList(sm *stateWriter, expect []staking.CandidateList) error {
	keys, actual, err := sm.List("CandsMap", &staking.CandidateList{})
	if err != nil {
		return err
	}
	if len(actual) != len(expect) {
		return fmt.Errorf("candidate list size mismatch: expect %d, actual %d", len(expect), len(actual))
	}
	sort.Slice(expect, func(i, j int) bool {
		return string(keys[i]) < string(keys[j])
	})
	sort.Slice(actual, func(i, j int) bool {
		return string(keys[i]) < string(keys[j])
	})
	for i := range expect {
		for j := range expect[i] {
			if !expect[i][j].Equal((*actual[i].(*staking.CandidateList))[j]) {
				return fmt.Errorf("candidate mismatch at index %d in list %d: expect %+v, actual %+v",
					j, i, expect[i][j], (*actual[i].(*staking.CandidateList))[j])
			}
		}
	}
	fmt.Println("\033[32m✅ Candidate list check passed\033[0m")
	return nil
}

func readEndorsements(state *stakingState) ([][]byte, []*staking.Endorsement, error) {
	endorses := make([]*staking.Endorsement, 0, len(state.Keys))
	for i, v := range state.Values {
		e := &staking.Endorsement{}
		if err := e.Deserialize(v); err != nil {
			return nil, nil, errors.Wrapf(err, "failed to deserialize endorsement from key %x", state.Keys[i])
		}
		endorses = append(endorses, e)
	}
	return state.Keys, endorses, nil
}

func putEndorsements(sm *stateWriter, keys [][]byte, endorsements []*staking.Endorsement) error {
	for i, e := range endorsements {
		if err := sm.PutObject("Staking", keys[i], e); err != nil {
			return errors.Wrapf(err, "failed to put endorsement %x", keys[i])
		}
	}
	return nil
}

func checkEndorsements(sm *stateWriter, keys [][]byte, expect []*staking.Endorsement) error {
	_, actual, err := sm.List("Staking", &staking.Endorsement{})
	if err != nil {
		return err
	}
	if len(actual) != len(expect) {
		return fmt.Errorf("endorsement list size mismatch: expect %d, actual %d", len(expect), len(actual))
	}
	sort.Slice(expect, func(i, j int) bool {
		return expect[i].ExpireHeight < expect[j].ExpireHeight
	})
	sort.Slice(actual, func(i, j int) bool {
		return actual[i].(*staking.Endorsement).ExpireHeight < actual[j].(*staking.Endorsement).ExpireHeight
	})
	for i := range expect {
		if expect[i].ExpireHeight != (actual[i].(*staking.Endorsement).ExpireHeight) {
			return fmt.Errorf("endorsement identifier mismatch at index %d: expect %+v, actual %+v",
				i, expect[i], actual[i].(*staking.Endorsement))
		}
	}
	fmt.Println("\033[32m✅ Endorsement list check passed\033[0m")
	return nil
}

func readTotalBucketCount(state *stakingState) (uint64, error) {
	total := &staking.TotalBucketCount{}
	if err := total.Deserialize(state.Values[0]); err != nil {
		return 0, errors.Wrapf(err, "failed to deserialize total bucket count from value %x", state.Values[0])
	}
	return total.Count(), nil
}

func putTotalBucketCount(sm *stateWriter, count uint64) error {
	if err := sm.PutObject("Staking", staking.TotalBucketKey, staking.NewTotalBucketCount(count)); err != nil {
		return errors.Wrapf(err, "failed to put total bucket count %d", count)
	}
	log.S().Infof("Put total bucket count %d to contract", count)
	return nil
}

func checkTotalBucketCount(sm *stateWriter, expect uint64) error {
	total := &staking.TotalBucketCount{}
	if err := sm.GetObject("Staking", staking.TotalBucketKey, total); err != nil {
		return err
	}
	if *total != *staking.NewTotalBucketCount(expect) {
		return fmt.Errorf("total bucket count mismatch: expect %d, actual %d", expect, total)
	}
	fmt.Println("\033[32m✅ Total bucket count check passed\033[0m")
	return nil
}

func readBuckets(state *stakingState) ([]*staking.VoteBucket, error) {
	buckets := make([]*staking.VoteBucket, 0, len(state.Keys))
	for i := 0; i < len(state.Keys); i++ {
		b := &staking.VoteBucket{}
		if err := b.Deserialize(state.Values[i]); err != nil {
			return nil, errors.Wrapf(err, "failed to deserialize vote bucket from key %x", state.Keys[i])
		}
		buckets = append(buckets, b)
	}
	return buckets, nil
}

func putBuckets(sm *stateWriter, buckets []*staking.VoteBucket) error {
	for _, b := range buckets {
		if err := sm.PutObject("Staking", bucketKey(b.Index), b); err != nil {
			return errors.Wrapf(err, "failed to put vote bucket %d", b.Index)
		}
	}
	return nil
}

func checkBuckets(sm *stateWriter, expect []*staking.VoteBucket) error {
	_, actual, err := sm.List("Staking", &staking.VoteBucket{})
	if err != nil {
		return err
	}
	if len(actual) != len(expect) {
		return fmt.Errorf("vote bucket list size mismatch: expect %d, actual %d", len(expect), len(actual))
	}
	sort.Slice(expect, func(i, j int) bool {
		return expect[i].Index < expect[j].Index
	})
	sort.Slice(actual, func(i, j int) bool {
		return actual[i].(*staking.VoteBucket).Index < actual[j].(*staking.VoteBucket).Index
	})
	for i := range expect {
		if expect[i].Index != (actual[i].(*staking.VoteBucket).Index) {
			return fmt.Errorf("vote bucket index mismatch at index %d: expect %+v, actual %+v",
				i, expect[i], actual[i].(*staking.VoteBucket))
		}
	}
	fmt.Println("\033[32m✅ Vote bucket list check passed\033[0m")
	return nil
}

func bucketKey(index uint64) []byte {
	key := []byte{1}
	return append(key, byteutil.Uint64ToBytesBigEndian(index)...)
}

func readTotalAmount(state *stakingState) (*staking.TotalAmount, error) {
	total := &staking.TotalAmount{}
	if err := total.Deserialize(state.Values[0]); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize total amount from value %x", state.Values[0])
	}
	return total, nil
}

func putTotalAmount(sm *stateWriter, total *staking.TotalAmount) error {
	if err := sm.PutObject("Staking", _bucketPoolAddrKey, total); err != nil {
		return errors.Wrapf(err, "failed to put total amount %+v", total)
	}
	log.S().Infof("Put total amount %+v to contract", total)
	return nil
}

func checkTotalAmount(sm *stateWriter, expect *staking.TotalAmount) error {
	total := &staking.TotalAmount{}
	if err := sm.GetObject("Staking", _bucketPoolAddrKey, total); err != nil {
		return err
	}
	totalB, err := total.Serialize()
	if err != nil {
		return errors.Wrapf(err, "failed to serialize total amount %+v", total)
	}
	expectB, err := expect.Serialize()
	if err != nil {
		return errors.Wrapf(err, "failed to serialize expected total amount %+v", expect)
	}
	if !bytes.Equal(totalB, expectB) {
		return fmt.Errorf("total amount mismatch: expect %+v, actual %+v", expect, total)
	}
	fmt.Println("\033[32m✅ Total amount check passed\033[0m")
	return nil
}

var _bucketPoolAddr = hash.Hash160b([]byte("bucketPool"))
var _bucketPoolAddrKey = append([]byte{0}, _bucketPoolAddr[:]...)

func readBucketIndices(state *stakingState) ([][]byte, []staking.BucketIndices, error) {
	var bucketIndices []staking.BucketIndices
	for i := 0; i < len(state.Values); i++ {
		bi := &staking.BucketIndices{}
		if err := bi.Deserialize(state.Values[i]); err != nil {
			return nil, nil, errors.Wrapf(err, "failed to deserialize bucket indices from key %x", state.Keys[i])
		}
		bucketIndices = append(bucketIndices, *bi)
	}
	return state.Keys, bucketIndices, nil
}

func putBucketIndices(sm *stateWriter, keys [][]byte, bucketIndices []staking.BucketIndices) error {
	for i, bi := range bucketIndices {
		if err := sm.PutObject("Staking", keys[i], &bi); err != nil {
			return errors.Wrapf(err, "failed to put bucket indices %x", keys[i])
		}
	}
	return nil
}

func checkBucketIndices(sm *stateWriter, expect []staking.BucketIndices) error {
	_, actual, err := sm.List("Staking", &staking.BucketIndices{})
	if err != nil {
		return err
	}
	if len(actual) != len(expect) {
		return fmt.Errorf("bucket indices size mismatch: expect %d, actual %d", len(expect), len(actual))
	}
	for i := range expect {
		es, err := expect[i].Serialize()
		if err != nil {
			return errors.Wrapf(err, "failed to serialize expected bucket indices %d", i)
		}
		as, err := actual[i].(*staking.BucketIndices).Serialize()
		if err != nil {
			return errors.Wrapf(err, "failed to serialize actual bucket indices %d", i)
		}
		if !bytes.Equal(es, as) {
			return fmt.Errorf("bucket indices mismatch at index %d: expect %x, actual %x", i, es, as)
		}
	}
	fmt.Println("\033[32m✅ Bucket indices check passed\033[0m")
	return nil
}

type stakingState struct {
	Keys   [][]byte
	Values [][]byte
}

type stakingStates struct {
	bucketIndices stakingState
	totalAmount   stakingState
	candidates    stakingState
	candidateList stakingState
	endorsements  stakingState
	totalBucket   stakingState
	voteBuckets   stakingState
}

func readStakingStates(sr *stateReader) (*stakingStates, error) {
	// Read staking states from the "Staking" namespace
	stakingStates := stakingStates{
		bucketIndices: stakingState{Keys: [][]byte{}, Values: [][]byte{}},
		totalAmount:   stakingState{Keys: [][]byte{}, Values: [][]byte{}},
		candidates:    stakingState{Keys: [][]byte{}, Values: [][]byte{}},
		candidateList: stakingState{Keys: [][]byte{}, Values: [][]byte{}},
		endorsements:  stakingState{Keys: [][]byte{}, Values: [][]byte{}},
		totalBucket:   stakingState{Keys: [][]byte{}, Values: [][]byte{}},
		voteBuckets:   stakingState{Keys: [][]byte{}, Values: [][]byte{}},
	}
	ks, values, err := sr.store.Filter("Staking", func(k, v []byte) bool { return true }, nil, nil)
	if err != nil {
		if errors.Cause(err) != db.ErrNotExist && errors.Cause(err) != db.ErrBucketNotExist {
			return nil, errors.Wrap(err, "failed to filter staking states")
		}
	}
	for i, k := range ks {
		if len(k) == 21 && (k[0] == 2 || k[0] == 3) {
			stakingStates.bucketIndices.Keys = append(stakingStates.bucketIndices.Keys, k)
			stakingStates.bucketIndices.Values = append(stakingStates.bucketIndices.Values, values[i])
		} else if bytes.Equal(k, _bucketPoolAddrKey) {
			stakingStates.totalAmount.Keys = append(stakingStates.totalAmount.Keys, k)
			stakingStates.totalAmount.Values = append(stakingStates.totalAmount.Values, values[i])
		} else if len(k) > 0 && k[0] == 4 {
			stakingStates.endorsements.Keys = append(stakingStates.endorsements.Keys, k)
			stakingStates.endorsements.Values = append(stakingStates.endorsements.Values, values[i])
		} else if len(k) > 0 && k[0] == 1 {
			stakingStates.voteBuckets.Keys = append(stakingStates.voteBuckets.Keys, k)
			stakingStates.voteBuckets.Values = append(stakingStates.voteBuckets.Values, values[i])
		} else if bytes.Equal(k, staking.TotalBucketKey) {
			stakingStates.totalBucket.Keys = append(stakingStates.totalBucket.Keys, k)
			stakingStates.totalBucket.Values = append(stakingStates.totalBucket.Values, values[i])
		} else {
			return nil, fmt.Errorf("unexpected key %x in Staking namespace", k)
		}
	}

	ks, values, err = sr.store.Filter("Candidate", func(k, v []byte) bool { return true }, nil, nil)
	if err != nil {
		if errors.Cause(err) != db.ErrNotExist && errors.Cause(err) != db.ErrBucketNotExist {
			return nil, errors.Wrap(err, "failed to filter candidate states")
		}
	}
	for i, k := range ks {
		stakingStates.candidates.Keys = append(stakingStates.candidates.Keys, k)
		stakingStates.candidates.Values = append(stakingStates.candidates.Values, values[i])
	}

	ks, values, err = sr.store.Filter("CandsMap", func(k, v []byte) bool { return true }, nil, nil)
	if err != nil {
		if errors.Cause(err) != db.ErrNotExist && errors.Cause(err) != db.ErrBucketNotExist {
			return nil, errors.Wrap(err, "failed to filter candidate states")
		}
	}
	for i, k := range ks {
		if bytes.Equal(k, []byte("name")) || bytes.Equal(k, []byte("operator")) || bytes.Equal(k, []byte("owner")) {
			stakingStates.candidateList.Keys = append(stakingStates.candidateList.Keys, k)
			stakingStates.candidateList.Values = append(stakingStates.candidateList.Values, values[i])
		} else {
			return nil, fmt.Errorf("unexpected key %x in CandsMap namespace", k)
		}
	}
	return &stakingStates, nil
}

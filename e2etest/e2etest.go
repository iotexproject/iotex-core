package e2etest

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	errReceiptNotFound = fmt.Errorf("receipt not found")
)

type (
	actionWithTime struct {
		act *action.SealedEnvelope
		t   time.Time
	}
	testcase struct {
		name    string
		preFunc func(*e2etest)
		preActs []*actionWithTime
		act     *actionWithTime
		expect  []actionExpect
	}
	accountNonceManager map[string]uint64
	e2etest             struct {
		cfg      config.Config
		svr      *itx.Server
		cs       *chainservice.ChainService
		t        *testing.T
		nonceMgr accountNonceManager
		api      iotexapi.APIServiceClient
	}
)

func (m accountNonceManager) pop(addr string) uint64 {
	nonce := m[addr]
	m[addr] = nonce + 1
	return nonce
}

func newE2ETest(t *testing.T, cfg config.Config) *e2etest {
	require := require.New(t)
	// Create a new blockchain
	svr, err := itx.NewServer(cfg)
	require.NoError(err)
	ctx := context.Background()
	require.NoError(svr.Start(ctx))
	// Create a new API service client
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", cfg.API.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(err)
	return &e2etest{
		cfg:      cfg,
		svr:      svr,
		cs:       svr.ChainService(cfg.Chain.ID),
		t:        t,
		nonceMgr: make(accountNonceManager),
		api:      iotexapi.NewAPIServiceClient(conn),
	}
}

func (e *e2etest) run(cases []*testcase) {
	ctx := context.Background()
	// run subcases
	for _, sub := range cases {
		e.t.Run(sub.name, func(t *testing.T) {
			e.withTest(t).runCase(ctx, sub)
		})
	}
}

func (e *e2etest) runCase(ctx context.Context, c *testcase) {
	require := require.New(e.t)
	bc := e.cs.Blockchain()
	ap := e.cs.ActionPool()
	// run pre-function
	if c.preFunc != nil {
		c.preFunc(e)
	}
	// run pre-actions
	for i, act := range c.preActs {
		_, receipt, err := addOneTx(ctx, ap, bc, act)
		require.NoErrorf(err, "failed to add pre-action %d", i)
		require.EqualValuesf(iotextypes.ReceiptStatus_Success, receipt.Status, "pre-action %d failed", i)
	}
	// run action
	act, receipt, err := addOneTx(ctx, ap, bc, c.act)
	for _, exp := range c.expect {
		exp.expect(e, act, receipt, err)
	}
}

func (e *e2etest) teardown() {
	require := require.New(e.t)
	require.NoError(e.svr.Stop(context.Background()))
	// clean up
	clearDBPaths(&e.cfg)
}

func (e *e2etest) withTest(t *testing.T) *e2etest {
	return &e2etest{
		cfg: e.cfg,
		svr: e.svr,
		cs:  e.cs,
		t:   t,
		api: e.api,
	}
}

func (e *e2etest) getCandidateByName(name string) (*iotextypes.CandidateV2, error) {
	methodName, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME,
	})
	if err != nil {
		return nil, err
	}
	arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByName_{
			CandidateByName: &iotexapi.ReadStakingDataRequest_CandidateByName{
				CandName: name,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	resp, err := e.api.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("staking"),
		MethodName: methodName,
		Arguments:  [][]byte{arg},
	})
	if err != nil {
		return nil, err
	}
	candidate := &iotextypes.CandidateV2{}
	if err = proto.Unmarshal(resp.GetData(), candidate); err != nil {
		return nil, err
	}
	return candidate, nil
}

func (e *e2etest) getBucket(index uint64, contractAddr string) (*iotextypes.VoteBucket, error) {
	methodName, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_INDEXES,
	})
	if err != nil {
		return nil, err
	}
	arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByIndexes{
			BucketsByIndexes: &iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes{
				Index: []uint64{index},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	resp, err := e.api.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("staking"),
		MethodName: methodName,
		Arguments:  [][]byte{arg},
	})
	if err != nil {
		return nil, err
	}
	bucketList := &iotextypes.VoteBucketList{}
	if err = proto.Unmarshal(resp.GetData(), bucketList); err != nil {
		return nil, err
	}
	idx := slices.IndexFunc(bucketList.Buckets, func(e *iotextypes.VoteBucket) bool {
		return e.Index == index && e.ContractAddress == contractAddr
	})
	if idx < 0 {
		return nil, nil
	}
	return bucketList.Buckets[idx], nil

}

func addOneTx(ctx context.Context, ap actpool.ActPool, bc blockchain.Blockchain, tx *actionWithTime) (*action.SealedEnvelope, *action.Receipt, error) {
	if err := ap.Add(ctx, tx.act); err != nil {
		return tx.act, nil, err
	}
	blk, err := createAndCommitBlock(bc, ap, tx.t)
	if err != nil {
		return tx.act, nil, err
	}
	h, err := tx.act.Hash()
	if err != nil {
		return tx.act, nil, err
	}
	for _, r := range blk.Receipts {
		if r.ActionHash == h {
			return tx.act, r, nil
		}
	}

	return tx.act, nil, errors.Wrapf(errReceiptNotFound, "for action %x, %T, %+v", h, tx.act.Envelope.Action(), tx.act.Envelope.Action())
}

func createAndCommitBlock(bc blockchain.Blockchain, ap actpool.ActPool, blkTime time.Time) (*block.Block, error) {
	blk, err := bc.MintNewBlock(blkTime)
	if err != nil {
		return nil, err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return nil, err
	}
	ap.Reset()
	return blk, nil
}

func mustNoErr[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

func initDBPaths(r *require.Assertions, cfg *config.Config) {
	testTriePath, err := testutil.PathOfTempFile("trie")
	r.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	r.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	r.NoError(err)
	testContractIndexPath, err := testutil.PathOfTempFile("contractindex")
	r.NoError(err)
	testBloomfilterIndexPath, err := testutil.PathOfTempFile("bloomfilterindex")
	r.NoError(err)
	testCandidateIndexPath, err := testutil.PathOfTempFile("candidateindex")
	r.NoError(err)
	testSystemLogPath, err := testutil.PathOfTempFile("systemlog")
	r.NoError(err)
	testConsensusPath, err := testutil.PathOfTempFile("consensus")
	r.NoError(err)

	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.BlobStoreDBPath = ""
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.ContractStakingIndexDBPath = testContractIndexPath
	cfg.Chain.BloomfilterIndexDBPath = testBloomfilterIndexPath
	cfg.Chain.CandidateIndexDBPath = testCandidateIndexPath
	cfg.System.SystemLogDBPath = testSystemLogPath
	cfg.Consensus.RollDPoS.ConsensusDBPath = testConsensusPath
}

func clearDBPaths(cfg *config.Config) {
	testutil.CleanupPath(cfg.Chain.ChainDBPath)
	testutil.CleanupPath(cfg.Chain.TrieDBPath)
	testutil.CleanupPath(cfg.Chain.BloomfilterIndexDBPath)
	testutil.CleanupPath(cfg.Chain.CandidateIndexDBPath)
	testutil.CleanupPath(cfg.Chain.StakingIndexDBPath)
	testutil.CleanupPath(cfg.Chain.ContractStakingIndexDBPath)
	testutil.CleanupPath(cfg.DB.DbPath)
	testutil.CleanupPath(cfg.Chain.IndexDBPath)
	testutil.CleanupPath(cfg.System.SystemLogDBPath)
	testutil.CleanupPath(cfg.Consensus.RollDPoS.ConsensusDBPath)
}

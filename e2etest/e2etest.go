package e2etest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-resty/resty/v2"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/actpool"
	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/chainservice"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/server/itx"
	"github.com/iotexproject/iotex-core/v2/testutil"
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
		name        string
		preFunc     func(*e2etest)
		preActs     []*actionWithTime
		act         *actionWithTime
		expect      []actionExpect
		acts        []*actionWithTime
		blockExpect func(test *e2etest, blk *block.Block, err error)
	}
	accountNonceManager map[string]uint64
	e2etest             struct {
		cfg      config.Config
		svr      *itx.Server
		cs       *chainservice.ChainService
		t        *testing.T
		nonceMgr accountNonceManager
		api      iotexapi.APIServiceClient
		ethcli   *ethclient.Client
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
	cli, err := ethclient.Dial(fmt.Sprintf("http://localhost:%d", cfg.API.HTTPPort))
	require.NoError(err)
	return &e2etest{
		cfg:      cfg,
		svr:      svr,
		cs:       svr.ChainService(cfg.Chain.ID),
		t:        t,
		nonceMgr: make(accountNonceManager),
		api:      iotexapi.NewAPIServiceClient(conn),
		ethcli:   cli,
	}
}

func (e *e2etest) run(cases []*testcase) {
	if len(cases) == 0 {
		return
	}
	ctx := context.Background()
	e.t.Run(cases[0].name, func(t *testing.T) {
		se := e.withTest(t)
		se.runCase(ctx, cases[0])
		se.run(cases[1:])
	})
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
		_, receipt, _, err := addOneTx(ctx, ap, bc, act)
		require.NoErrorf(err, "failed to add pre-action %d", i)
		require.EqualValuesf(iotextypes.ReceiptStatus_Success, receipt.Status, "pre-action %d failed: %s", i, receipt.ExecutionRevertMsg())
	}
	// run action
	if c.act != nil {
		act, receipt, blk, err := addOneTx(ctx, ap, bc, c.act)
		for _, exp := range c.expect {
			exp.expect(e, act, receipt, err)
		}
		if c.blockExpect != nil {
			c.blockExpect(e, blk, err)
		}
	} else if len(c.acts) > 0 {
		_, _, blk, err := runTxs(ctx, ap, bc, c.acts)
		if c.blockExpect != nil {
			c.blockExpect(e, blk, err)
		}
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
		cfg:    e.cfg,
		svr:    e.svr,
		cs:     e.cs,
		t:      t,
		api:    e.api,
		ethcli: e.ethcli,
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

func (e *e2etest) getBlobs(height uint64) ([]*apitypes.BlobSidecarResult, error) {
	body := fmt.Sprintf(`{
		"jsonrpc":"2.0",
		"method":"eth_getBlobSidecars",
		"params":[
			"%s"
		],
		"id":1
		}`, hexutil.EncodeUint64(height))
	url := fmt.Sprintf("http://localhost:%d", e.cfg.API.HTTPPort)
	type web3Response struct {
		ID     int                           `json:"id"`
		Result []*apitypes.BlobSidecarResult `json:"result"`
		Err    string                        `json:"err"`
	}
	result := &web3Response{}
	resp, err := resty.New().R().SetBody(body).SetResult(result).Post(url)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, errors.Errorf("failed to get blobs: %s", resp.String())
	}
	err = json.Unmarshal([]byte(resp.String()), result)
	if err != nil {
		return nil, err
	}
	return result.Result, nil
}

func runTxs(ctx context.Context, ap actpool.ActPool, bc blockchain.Blockchain, txs []*actionWithTime) ([]*action.SealedEnvelope, []*action.Receipt, *block.Block, error) {
	for _, tx := range txs {
		if err := ap.Add(ctx, tx.act); err != nil {
			return nil, nil, nil, err
		}
	}
	blk, err := createAndCommitBlock(bc, ap, txs[len(txs)-1].t)
	if err != nil {
		return nil, nil, nil, err
	}
	receipts := make([]*action.Receipt, 0, len(txs))
	actions := make([]*action.SealedEnvelope, 0, len(txs))
firstLoop:
	for _, tx := range txs {
		actions = append(actions, tx.act)
		h, err := tx.act.Hash()
		if err != nil {
			return nil, nil, nil, err
		}
		for _, r := range blk.Receipts {
			if r.ActionHash == h {
				receipts = append(receipts, r)
				continue firstLoop
			}
		}
		receipts = append(receipts, nil)
	}
	return actions, receipts, blk, nil
}

func addOneTx(ctx context.Context, ap actpool.ActPool, bc blockchain.Blockchain, tx *actionWithTime) (*action.SealedEnvelope, *action.Receipt, *block.Block, error) {
	ctx, err := bc.Context(ctx)
	if err != nil {
		return tx.act, nil, nil, err
	}
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: bc.TipHeight() + 1,
	}))
	if err := ap.Add(ctx, tx.act); err != nil {
		return tx.act, nil, nil, err
	}
	blk, err := createAndCommitBlock(bc, ap, tx.t)
	if err != nil {
		return tx.act, nil, nil, err
	}
	h, err := tx.act.Hash()
	if err != nil {
		return tx.act, nil, nil, err
	}
	for _, r := range blk.Receipts {
		if r.ActionHash == h {
			return tx.act, r, blk, nil
		}
	}

	return tx.act, nil, nil, errors.Wrapf(errReceiptNotFound, "for action %x, %v, %T, %+v", h, tx.act.Envelope.TxType(), tx.act.Envelope.Action(), tx.act.Envelope.Action())
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
	testBlobPath, err := testutil.PathOfTempFile("blob")
	r.NoError(err)
	testStakingIndexPath, err := testutil.PathOfTempFile("stakingindex")
	r.NoError(err)

	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.BlobStoreDBPath = ""
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.StakingIndexDBPath = testStakingIndexPath
	cfg.Chain.ContractStakingIndexDBPath = testContractIndexPath
	cfg.Chain.BloomfilterIndexDBPath = testBloomfilterIndexPath
	cfg.Chain.CandidateIndexDBPath = testCandidateIndexPath
	cfg.System.SystemLogDBPath = testSystemLogPath
	cfg.Consensus.RollDPoS.ConsensusDBPath = testConsensusPath
	cfg.Chain.BlobStoreDBPath = testBlobPath

	if cfg.ActPool.Store != nil {
		testActionStorePath, err := os.MkdirTemp(os.TempDir(), "actionstore")
		r.NoError(err)
		cfg.ActPool.Store.Datadir = testActionStorePath
	}
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
	testutil.CleanupPath(cfg.Chain.BlobStoreDBPath)
	if cfg.ActPool.Store != nil {
		testutil.CleanupPath(cfg.ActPool.Store.Datadir)
	}
}

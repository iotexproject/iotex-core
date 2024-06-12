package e2etest

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

type (
	actionWithTime struct {
		act *action.SealedEnvelope
		t   time.Time
	}
	testcase struct {
		name    string
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
	return &e2etest{
		cfg:      cfg,
		svr:      svr,
		cs:       svr.ChainService(cfg.Chain.ID),
		t:        t,
		nonceMgr: make(accountNonceManager),
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
	// run pre-actions
	for _, act := range c.preActs {
		_, _, err := addOneTx(ctx, ap, bc, act)
		require.NoError(err)
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
	}
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
	return tx.act, nil, errors.Errorf("failed to find receipt for %x", h)
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
	testContractIndexPathV2, err := testutil.PathOfTempFile("contractindexv2")
	r.NoError(err)
	testSGDIndexPath, err := testutil.PathOfTempFile("sgdindex")
	r.NoError(err)

	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.ContractStakingIndexDBPath = testContractIndexPath
	cfg.Chain.ContractStakingIndexDBPathV2 = testContractIndexPathV2
	cfg.Chain.SGDIndexDBPath = testSGDIndexPath
}

func clearDBPaths(cfg *config.Config) {
	testutil.CleanupPath(cfg.Chain.ChainDBPath)
	testutil.CleanupPath(cfg.Chain.TrieDBPath)
	testutil.CleanupPath(cfg.Chain.BloomfilterIndexDBPath)
	testutil.CleanupPath(cfg.Chain.CandidateIndexDBPath)
	testutil.CleanupPath(cfg.Chain.StakingIndexDBPath)
	testutil.CleanupPath(cfg.Chain.ContractStakingIndexDBPath)
	testutil.CleanupPath(cfg.Chain.ContractStakingIndexDBPathV2)
	testutil.CleanupPath(cfg.DB.DbPath)
	testutil.CleanupPath(cfg.Chain.IndexDBPath)
	testutil.CleanupPath(cfg.System.SystemLogDBPath)
	testutil.CleanupPath(cfg.Chain.SGDIndexDBPath)
}

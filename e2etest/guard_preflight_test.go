// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/api"
	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/server/itx"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func (f *guardFixture) packDirect(t *testing.T, tx *actionWithTime) (*action.SealedEnvelope, *action.Receipt, *block.Block, error) {
	t.Helper()
	return f.test.packDirect(t, tx)
}

// Guard protocol tests exercise the real RPC handlers through an in-memory
// transport; they do not need to start P2P or bind a TCP listener.
type guardRPCTransport struct{ handler api.Web3Handler }

func (tr guardRPCTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	err := tr.handler.HandlePOSTReq(req.Context(), req.Body, apitypes.NewResponseWriter(func(v interface{}) (int, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return 0, err
		}
		return w.Write(b)
	}))
	if err != nil {
		return nil, err
	}
	return w.Result(), nil
}

type guardStateClient struct {
	iotexapi.APIServiceClient
	core api.CoreService
}

func (c guardStateClient) ReadState(_ context.Context, in *iotexapi.ReadStateRequest, _ ...grpc.CallOption) (*iotexapi.ReadStateResponse, error) {
	return c.core.ReadState(string(in.ProtocolID), in.Height, in.MethodName, in.Arguments)
}
func newGuardInProcessFixture(t *testing.T, opts ...func(*config.Config)) *guardFixture {
	cfg := initCfg(require.New(t))
	cfg.Genesis.ToBeEnabledBlockHeight = guardFork
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	for _, opt := range opts {
		opt(&cfg)
	}
	svr, err := itx.NewServer(cfg)
	require.NoError(t, err)
	ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), cfg.Genesis), protocol.BlockCtx{}))
	cs := svr.ChainService(cfg.Chain.ID)
	require.NoError(t, cs.Start(ctx))
	apiServer, err := cs.NewAPIServer(cfg.API, true)
	require.NoError(t, err)
	core := apiServer.CoreService()
	rpcClient, err := rpc.DialOptions(context.Background(), "http://guard.test", rpc.WithHTTPClient(&http.Client{Transport: guardRPCTransport{api.NewWeb3Handler(core, "", 100)}}))
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	e := &e2etest{cfg: cfg, svr: svr, cs: cs, t: t, nonceMgr: make(accountNonceManager), api: guardStateClient{core: core}, ethcli: ethclient.NewClient(rpcClient)}
	now := time.Now()
	return &guardFixture{test: e, chainID: cfg.Chain.ID, domain: bucketguard.NewDomain(cfg.Chain.ID, cfg.Genesis.Hash()), gasLimit: 4_000_000, gasPrice: big.NewInt(unit.Qev*2 + 1), start: now, blockTime: now, pending: map[*actionWithTime]pendingAct{}}
}

func (f *guardFixture) teardown() {
	require.NoError(f.test.t, f.test.cs.Stop(context.Background()))
	clearDBPaths(&f.test.cfg)
}

// Each create binds ACCEPT to its own outer nonce. Admission must allow a
// future account nonce while still verifying both signatures against tip state.
func TestVaultBucketQueuedCreates(t *testing.T) {
	f := newVaultChainFixture(t)
	const id = 3
	owner := identityset.Address(id)
	f.run([]*testcase{{name: "bootstrap", preActs: []*actionWithTime{f.register(1, "cand1"), f.createStake(id, "cand1", guardStakeAmount), f.createStake(id, "cand1", guardStakeAmount)}, act: f.createStake(id, "cand1", guardStakeAmount), expect: []actionExpect{successExpect}}})
	key := guardKey(0x64)
	ctx := mustNoErr(f.test.cs.Blockchain().Context(context.Background()))
	for i := 0; i < 2; i++ {
		nonce := f.test.nonceMgr[owner.String()]
		m := &bucketguard.VaultAction{Method: "vaultCreate", Amount: unit.ConvertIotxToRau(100), OwnerTxNonce: nonce, Guard: bucketguard.NewGuard{AlgID: guardAlg, PublicKey: key.PublicKey().Bytes()}}
		m.Guard.AcceptSig = mustNoErr(key.Sign([]byte(bucketguard.VaultAcceptContext), bucketguard.EncodeVaultAccept(f.domain, 0, nonce, 1, owner.Bytes(), bucketguard.UnbindDelayBlocks, guardAlg, m.Guard.PublicKey, [64]byte{})))
		tx := vaultWeb3Tx(t, f, id, mustNoErr(action.NewVaultAction(m)))
		require.NoError(t, f.test.cs.ActionPool().Add(ctx, tx.act))
	}
	require.EqualValues(t, 2, f.test.cs.ActionPool().GetSize())
	blk := mustNoErr(createAndCommitBlock(f.test.cs.Blockchain(), f.test.cs.ActionPool(), f.tick()))
	created := 0
	for _, receipt := range blk.Receipts {
		if receipt.Status == uint64(iotextypes.ReceiptStatus_Success) {
			created++
		}
	}
	require.GreaterOrEqual(t, created, 2)
	for i := uint64(0); i < 2; i++ {
		require.NotNil(t, mustNoErr(staking.ReadVaultBucket(f.test.cs.StateFactory(), i)))
	}
	require.Zero(t, f.test.cs.ActionPool().GetSize())
}

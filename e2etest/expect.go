package e2etest

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

var (
	successExpect = &basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_Success), ""}
)

type (
	actionExpect interface {
		expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error)
	}
	basicActionExpect struct {
		err                error
		status             uint64
		executionRevertMsg string
	}
	fullActionExpect struct {
		contractAddress string
		gasConsumed     uint64
		txLogs          []*action.TransactionLog
	}
	candidateExpect struct {
		candName string
		cand     *iotextypes.CandidateV2
	}
	bucketExpect struct {
		bucket *iotextypes.VoteBucket
	}
	noBucketExpect struct {
		bucketIndex     uint64
		contractAddress string
	}
	executionExpect struct {
		contractAddress string
	}
	accountExpect struct {
		addr    address.Address
		balance string
		nonce   uint64
	}
	functionExpect struct {
		fn func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error)
	}
)

func (be *basicActionExpect) expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
	require := require.New(test.t)
	require.ErrorIs(err, be.err)
	if receipt == nil {
		require.Nil(receipt)
		return
	}
	require.Equalf(be.status, receipt.Status, "revert msg: %s", receipt.ExecutionRevertMsg())
	require.Equal(be.executionRevertMsg, receipt.ExecutionRevertMsg())
}

func (fe *fullActionExpect) expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
	require := require.New(test.t)
	require.Equal(fe.contractAddress, receipt.ContractAddress)
	require.Equal(fe.gasConsumed, receipt.GasConsumed)
	require.ElementsMatch(fe.txLogs, receipt.TransactionLogs())
}

func (ce *candidateExpect) expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
	require := require.New(test.t)
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME,
	}
	methodBytes, err := proto.Marshal(method)
	require.NoError(err)
	r := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByName_{
			CandidateByName: &iotexapi.ReadStakingDataRequest_CandidateByName{
				CandName: ce.candName,
			},
		},
	}
	cs := test.svr.ChainService(test.cfg.Chain.ID)
	sr := cs.StateFactory()
	bc := cs.Blockchain()
	prtcl, ok := cs.Registry().Find("staking")
	require.True(ok)
	stkPrtcl := prtcl.(*staking.Protocol)
	reqBytes, err := proto.Marshal(r)
	require.NoError(err)
	ctx := protocol.WithRegistry(context.Background(), cs.Registry())
	ctx = genesis.WithGenesisContext(ctx, test.cfg.Genesis)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: bc.TipHeight(),
	})
	ctx = protocol.WithFeatureCtx(ctx)
	respData, _, err := stkPrtcl.ReadState(ctx, sr, methodBytes, reqBytes)
	require.NoError(err)
	candidate := &iotextypes.CandidateV2{}
	require.NoError(proto.Unmarshal(respData, candidate))
	require.EqualValues(ce.cand.String(), candidate.String())
}

func (be *bucketExpect) expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
	require := require.New(test.t)
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_INDEXES,
	}
	methodBytes, err := proto.Marshal(method)
	require.NoError(err)
	r := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByIndexes{
			BucketsByIndexes: &iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes{
				Index: []uint64{be.bucket.Index},
			},
		},
	}
	cs := test.svr.ChainService(test.cfg.Chain.ID)
	sr := cs.StateFactory()
	bc := cs.Blockchain()
	prtcl, ok := cs.Registry().Find("staking")
	require.True(ok)
	stkPrtcl := prtcl.(*staking.Protocol)
	reqBytes, err := proto.Marshal(r)
	require.NoError(err)
	ctx := protocol.WithRegistry(context.Background(), cs.Registry())
	ctx = genesis.WithGenesisContext(ctx, test.cfg.Genesis)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: bc.TipHeight(),
	})
	ctx = protocol.WithFeatureCtx(ctx)
	respData, _, err := stkPrtcl.ReadState(ctx, sr, methodBytes, reqBytes)
	require.NoError(err)
	vbs := &iotextypes.VoteBucketList{}
	require.NoError(proto.Unmarshal(respData, vbs))
	idx := slices.IndexFunc(vbs.Buckets, func(vb *iotextypes.VoteBucket) bool {
		return vb.ContractAddress == be.bucket.ContractAddress
	})
	require.Greaterf(idx, -1, "bucket not found, index: %d, contract: %s", be.bucket.Index, be.bucket.ContractAddress)
	require.EqualValues(be.bucket.String(), vbs.Buckets[idx].String())
}

func (be *noBucketExpect) expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
	require := require.New(test.t)
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_INDEXES,
	}
	methodBytes, err := proto.Marshal(method)
	require.NoError(err)
	r := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByIndexes{
			BucketsByIndexes: &iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes{
				Index: []uint64{be.bucketIndex},
			},
		},
	}
	cs := test.svr.ChainService(test.cfg.Chain.ID)
	sr := cs.StateFactory()
	bc := cs.Blockchain()
	prtcl, ok := cs.Registry().Find("staking")
	require.True(ok)
	stkPrtcl := prtcl.(*staking.Protocol)
	reqBytes, err := proto.Marshal(r)
	require.NoError(err)
	ctx := protocol.WithRegistry(context.Background(), cs.Registry())
	ctx = genesis.WithGenesisContext(ctx, test.cfg.Genesis)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: bc.TipHeight(),
	})
	ctx = protocol.WithFeatureCtx(ctx)
	respData, _, err := stkPrtcl.ReadState(ctx, sr, methodBytes, reqBytes)
	require.NoError(err)
	vbs := &iotextypes.VoteBucketList{}
	require.NoError(proto.Unmarshal(respData, vbs))
	idx := slices.IndexFunc(vbs.Buckets, func(vb *iotextypes.VoteBucket) bool {
		return vb.ContractAddress == be.contractAddress
	})
	require.EqualValues(idx, -1)
}

func (ee *executionExpect) expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
	require := require.New(test.t)
	require.NoError(err)
	require.Equal(ee.contractAddress, receipt.ContractAddress)
}

func (ce *accountExpect) expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
	require := require.New(test.t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cli, err := ethclient.DialContext(ctx, fmt.Sprintf("http://localhost:%d", test.cfg.API.HTTPPort))
	addr := common.BytesToAddress(ce.addr.Bytes())
	balance, err := cli.BalanceAt(context.Background(), addr, nil)
	require.NoError(err)
	require.Equal(ce.balance, balance.String())
	nonce, err := cli.NonceAt(context.Background(), addr, nil)
	require.NoError(err)
	require.Equal(ce.nonce, nonce)
}

func (fe *functionExpect) expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
	fe.fn(test, act, receipt, err)
}

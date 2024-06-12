package e2etest

import (
	"context"
	"slices"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
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
	candidateExpect struct {
		candName string
		cand     *iotextypes.CandidateV2
	}
	bucketExpect struct {
		bucket *iotextypes.VoteBucket
	}
	executionExpect struct {
		contractAddress string
	}
)

func (be *basicActionExpect) expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
	require := require.New(test.t)
	require.ErrorIs(err, be.err)
	require.Equal(be.status, receipt.Status)
	require.Equal(be.executionRevertMsg, receipt.ExecutionRevertMsg())
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
	require.Greater(idx, -1)
	require.EqualValues(be.bucket.String(), vbs.Buckets[idx].String())
}

func (ee *executionExpect) expect(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
	require := require.New(test.t)
	require.NoError(err)
	require.Equal(ee.contractAddress, receipt.ContractAddress)
}

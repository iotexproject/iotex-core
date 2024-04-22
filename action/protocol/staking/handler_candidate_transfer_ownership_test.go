package staking

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil/testdb"
)

func TestProtocol_HandleCandidateTransferOwnership(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	require.NoError(err)
	t.Log(csm.SM())
	// create protocol
	p, err := NewProtocol(depositGas, &BuilderConfig{
		Staking:                  genesis.Default.Staking,
		PersistStakingPatchBlock: math.MaxUint64,
	}, nil, nil, genesis.Default.GreenlandBlockHeight)
	require.NoError(err)
	cfg := deepcopy.Copy(genesis.Default).(genesis.Genesis)
	ctx := genesis.WithGenesisContext(context.Background(), cfg)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	cc, ok := v.(*ViewData)
	require.True(ok)
	require.NoError(sm.WriteView(_protocolID, cc))

	initCandidateCfgs := []struct {
		Owner    address.Address
		Operator address.Address
		Reward   address.Address
		Voter    address.Address
		Name     string
	}{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), identityset.Address(1), "test1"},
		{identityset.Address(2), identityset.Address(8), identityset.Address(1), identityset.Address(5), "test2"},
		{identityset.Address(3), identityset.Address(9), identityset.Address(11), identityset.Address(6), "test3"},
	}

	for _, candCfg := range initCandidateCfgs {
		selfStakeAmount := big.NewInt(0)
		selfStakeBucketID := uint64(candidateNoSelfStakeBucketIndex)

		cand := &Candidate{
			Owner:              candCfg.Owner,
			Operator:           candCfg.Operator,
			Reward:             candCfg.Reward,
			Name:               candCfg.Name,
			Votes:              big.NewInt(0),
			SelfStakeBucketIdx: selfStakeBucketID,
			SelfStake:          selfStakeAmount,
		}
		require.NoError(csm.putCandidate(cand))
	}
	tests := []struct {
		name string
		// params
		initCandidateCfgIds []uint64
		initBalance         int64
		caller              address.Address
		owner               address.Address
		payload             []byte

		nonce       uint64
		gasLimit    uint64
		blkGasLimit uint64
		gasPrice    *big.Int
		// expect
		err         error
		status      iotextypes.ReceiptStatus
		expectOwner address.Address
		expectVoter address.Address
	}{
		{
			"transfer ownership to self",
			[]uint64{1},
			1300000,
			identityset.Address(1),
			identityset.Address(1),
			nil,

			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			nil,
			iotextypes.ReceiptStatus_Success,
			identityset.Address(1),
			identityset.Address(1),
		},
	}
	csm, err = NewCandidateStateManager(sm, false)
	require.NoError(err)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// init candidates
			candidates := make([]*Candidate, 0)
			for _, cfg := range initCandidateCfgs {
				candidates = append(candidates, &Candidate{
					Owner:    cfg.Owner,
					Operator: cfg.Operator,
					Reward:   cfg.Reward,
					Voter:    cfg.Voter,
					Name:     cfg.Name,
				})
			}
			require.NoError(setupAccount(sm, test.caller, test.initBalance))
			act, err := action.NewCandidateTransferOwnership(test.nonce, test.gasLimit, test.gasPrice, test.owner.String(), test.payload)
			require.NoError(err)

			IntrinsicGas, _ := act.IntrinsicGas()
			ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
				Caller:       test.caller,
				GasPrice:     test.gasPrice,
				IntrinsicGas: IntrinsicGas,
				Nonce:        test.nonce,
			})
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
				BlockHeight:    1,
				BlockTimeStamp: timeBlock,
				GasLimit:       test.blkGasLimit,
			})
			cfg := deepcopy.Copy(genesis.Default).(genesis.Genesis)
			cfg.TsunamiBlockHeight = 1
			ctx = genesis.WithGenesisContext(ctx, cfg)
			ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
			require.Equal(test.err, errors.Cause(p.Validate(ctx, act, sm)))
			_, _, err = p.handleCandidateTransferOwnership(ctx, act, csm)
			if test.err != nil {
				require.Error(err)
				require.Equal(test.err.Error(), err.Error())
				return
			}
			require.NoError(err)
			// require.Equal( test.status, receipt.Status)
			// check owner and voter
			candidate := csm.GetByOwner(identityset.Address(2))
			require.NotNil(t, candidate)
			require.Equal(t, test.expectOwner, candidate.Owner)
			require.Equal(t, test.expectVoter, candidate.Voter)
		})
	}
}

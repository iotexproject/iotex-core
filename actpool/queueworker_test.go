package actpool

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account/accountpb"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

func TestQueueWorker(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	ap, sf, err := newTestActPool(ctrl)
	r.NoError(err)

	senderKey := identityset.PrivateKey(28)
	senderAddrStr := identityset.Address(28).String()

	// setup sf mock to return a valid account
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account *state.Account, opts ...protocol.StateOption) (uint64, error) {
			pb := &accountpb.Account{
				Nonce:   1, // Confirmed nonce
				Balance: "10000000000",
				Type:    accountpb.AccountType_ZERO_NONCE,
			}
			account.FromProto(pb)
			return 1, nil
		}).AnyTimes()
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()

	jobQueue := make(chan workerJob, 1)
	worker := newQueueWorker(ap, jobQueue)
	r.NotNil(worker)
	// This is a bit of a hack. The actpool allocates workers internally based on sender address.
	// For this test, we will replace all workers with our test worker.
	for i := 0; i < len(ap.worker); i++ {
		ap.worker[i] = worker
	}

	g := genesis.TestDefault()
	bctx := protocol.BlockCtx{}
	ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), g), bctx))

	r.NoError(worker.Start())
	defer worker.Stop()

	// Test Handle
	act, err := signedTransfer(senderKey, 1, big.NewInt(100), "io1mflp9m6hcgm2qcghchsdqj3z3adcdw2h72f73h", nil, 100000, big.NewInt(1))
	r.NoError(err)

	job := workerJob{
		ctx: ctx,
		act: act,
		err: make(chan error, 1),
	}

	jobQueue <- job
	err = <-job.err
	r.NoError(err)

	// Verify action was added
	pNonce, err := ap.GetPendingNonce(senderAddrStr)
	r.NoError(err)
	r.Equal(uint64(2), pNonce)
}

type testQueueWorker struct {
	ctrl    *gomock.Controller
	ap      *actPool
	sf      *mock_chainmanager.MockStateReader
	worker  *queueWorker
	ctx     context.Context
	require *require.Assertions
}

func newTestQueueWorker(t *testing.T, mockSetup func(*mock_chainmanager.MockStateReader, *actPool)) *testQueueWorker {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	ap, sf, err := newTestActPool(ctrl)
	r.NoError(err)

	if mockSetup != nil {
		mockSetup(sf, ap)
	}

	g := genesis.TestDefault()
	bctx := protocol.BlockCtx{}
	ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), g), bctx))

	return &testQueueWorker{
		ctrl:    ctrl,
		ap:      ap,
		sf:      sf,
		worker:  newQueueWorker(ap, make(chan workerJob)),
		ctx:     ctx,
		require: r,
	}
}

func TestQueueWorker_HandleErrors(t *testing.T) {
	senderKey := identityset.PrivateKey(28)
	newJob := func(r *require.Assertions, ctx context.Context, nonce uint64, amount *big.Int) workerJob {
		act, err := signedTransfer(senderKey, nonce, amount, "io1mflp9m6hcgm2qcghchsdqj3z3adcdw2h72f73h", nil, 100000, big.NewInt(1))
		r.NoError(err)
		return workerJob{
			ctx: ctx,
			act: act,
			err: make(chan error, 1),
		}
	}

	t.Run("nonce too low", func(t *testing.T) {
		test := newTestQueueWorker(t, func(sf *mock_chainmanager.MockStateReader, _ *actPool) {
			sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
				func(account *state.Account, opts ...protocol.StateOption) (uint64, error) {
					pb := &accountpb.Account{
						Nonce:   5, // Confirmed nonce
						Balance: "10000000000",
						Type:    accountpb.AccountType_ZERO_NONCE,
					}
					account.FromProto(pb)
					return 1, nil
				}).AnyTimes()
		})
		job := newJob(test.require, test.ctx, 4, big.NewInt(100)) // Action nonce is 4
		err := test.worker.Handle(job)
		test.require.ErrorIs(err, action.ErrNonceTooLow)
	})

	t.Run("nonce too high", func(t *testing.T) {
		test := newTestQueueWorker(t, func(sf *mock_chainmanager.MockStateReader, ap *actPool) {
			sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
				func(account *state.Account, opts ...protocol.StateOption) (uint64, error) {
					pb := &accountpb.Account{
						Nonce:   5, // Confirmed nonce
						Balance: "10000000000",
						Type:    accountpb.AccountType_ZERO_NONCE,
					}
					account.FromProto(pb)
					return 1, nil
				}).AnyTimes()
		})
		job := newJob(test.require, test.ctx, 5+test.ap.cfg.MaxNumActsPerAcct, big.NewInt(100))
		err := test.worker.Handle(job)
		test.require.ErrorIs(err, action.ErrNonceTooHigh)
	})

	t.Run("insufficient funds", func(t *testing.T) {
		test := newTestQueueWorker(t, func(sf *mock_chainmanager.MockStateReader, _ *actPool) {
			sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
				func(account *state.Account, opts ...protocol.StateOption) (uint64, error) {
					pb := &accountpb.Account{
						Nonce:   0,
						Balance: "100", // Low balance
						Type:    accountpb.AccountType_ZERO_NONCE,
					}
					account.FromProto(pb)
					return 1, nil
				}).AnyTimes()
		})
		// Action cost is 100000 (gas) * 1 (gasprice) + 1000 (amount) = 101000
		job := newJob(test.require, test.ctx, 1, big.NewInt(1000))
		err := test.worker.Handle(job)
		test.require.ErrorIs(err, action.ErrInsufficientFunds)
	})

	t.Run("context canceled", func(t *testing.T) {
		test := newTestQueueWorker(t, nil)
		ctx, cancel := context.WithCancel(test.ctx)
		cancel() // Cancel context immediately
		job := newJob(test.require, ctx, 1, big.NewInt(100))
		err := test.worker.Handle(job)
		test.require.ErrorIs(err, context.Canceled)
	})
}

func TestQueueWorker_StateError(t *testing.T) {
	stateErr := errors.New("state error")

	t.Run("handle state error", func(t *testing.T) {
		test := newTestQueueWorker(t, func(sf *mock_chainmanager.MockStateReader, _ *actPool) {
			sf.EXPECT().State(gomock.Any(), gomock.Any()).Return(uint64(0), stateErr).Times(1)
		})
		act, err := signedTransfer(identityset.PrivateKey(28), 1, big.NewInt(100), "io1mflp9m6hcgm2qcghchsdqj3z3adcdw2h72f73h", nil, 100000, big.NewInt(1))
		test.require.NoError(err)
		job := workerJob{ctx: test.ctx, act: act}
		err = test.worker.Handle(job)
		test.require.ErrorIs(err, stateErr)
	})

	t.Run("reset state error", func(t *testing.T) {
		senderKey := identityset.PrivateKey(28)
		senderAddrStr := identityset.Address(28).String()
		test := newTestQueueWorker(t, func(sf *mock_chainmanager.MockStateReader, _ *actPool) {
			// Mock a successful state read for the initial add
			sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
				func(account *state.Account, opts ...protocol.StateOption) (uint64, error) {
					pb := &accountpb.Account{
						Nonce:   1,
						Balance: "10000000000",
						Type:    accountpb.AccountType_ZERO_NONCE,
					}
					account.FromProto(pb)
					return 1, nil
				}).Times(1)
			sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
			// Mock a failed state read for the reset
			sf.EXPECT().State(gomock.Any(), gomock.Any()).Return(uint64(0), stateErr).Times(1)
		})

		act, err := signedTransfer(senderKey, 1, big.NewInt(100), "io1mflp9m6hcgm2qcghchsdqj3z3adcdw2h72f73h", nil, 100000, big.NewInt(1))
		test.require.NoError(err)
		err = test.worker.Handle(workerJob{ctx: test.ctx, act: act})
		test.require.NoError(err)
		test.require.False(test.worker.accountActs.Account(senderAddrStr).Empty())

		test.worker.Reset(test.ctx)
		test.require.True(test.worker.accountActs.Account(senderAddrStr).Empty())
	})
}

func TestQueueWorker_EdgeCases(t *testing.T) {
	t.Run("replace with overflow", func(t *testing.T) {
		test := newTestQueueWorker(t, func(sf *mock_chainmanager.MockStateReader, ap *actPool) {
			ap.cfg.MaxNumActsPerPool = 1 // Set pool size to 1
			sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
				func(account *state.Account, opts ...protocol.StateOption) (uint64, error) {
					pb := &accountpb.Account{
						Nonce:   1,
						Balance: "10000000000",
						Type:    accountpb.AccountType_ZERO_NONCE,
					}
					account.FromProto(pb)
					return 1, nil
				}).AnyTimes()
			sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
		})

		senderKey := identityset.PrivateKey(28)
		act1, err := signedTransfer(senderKey, 1, big.NewInt(100), "io1mflp9m6hcgm2qcghchsdqj3z3adcdw2h72f73h", nil, 100000, big.NewInt(1))
		test.require.NoError(err)
		act2, err := signedTransfer(senderKey, 1, big.NewInt(100), "io1mflp9m6hcgm2qcghchsdqj3z3adcdw2h72f73h", nil, 100000, big.NewInt(2))
		test.require.NoError(err)

		// Add first action
		err = test.worker.Handle(workerJob{ctx: test.ctx, act: act1})
		test.require.NoError(err)

		// Replace with second action, causing overflow
		job := workerJob{ctx: test.ctx, act: act2, rep: true}
		err = test.worker.Handle(job)
		test.require.ErrorIs(err, action.ErrTxPoolOverflow)
	})

	t.Run("pending actions with timeout", func(t *testing.T) {
		test := newTestQueueWorker(t, func(sf *mock_chainmanager.MockStateReader, ap *actPool) {
			ap.cfg.ActionExpiry = 1 * time.Millisecond // Set a short expiry
			sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
				func(account *state.Account, opts ...protocol.StateOption) (uint64, error) {
					pb := &accountpb.Account{
						Nonce:   1,
						Balance: "10000000000",
						Type:    accountpb.AccountType_ZERO_NONCE,
					}
					account.FromProto(pb)
					return 1, nil
				}).AnyTimes()
			sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
		})

		// Action with nonce 2, which is greater than the pending nonce 1
		act, err := signedTransfer(identityset.PrivateKey(28), 2, big.NewInt(100), "io1mflp9m6hcgm2qcghchsdqj3z3adcdw2h72f73h", nil, 100000, big.NewInt(1))
		test.require.NoError(err)

		err = test.worker.Handle(workerJob{ctx: test.ctx, act: act})
		test.require.NoError(err)

		time.Sleep(2 * time.Millisecond) // Wait for action to expire

		pending := test.worker.PendingActions(test.ctx)
		test.require.Empty(pending)
	})

	t.Run("all actions for non-existent account", func(t *testing.T) {
		test := newTestQueueWorker(t, nil)
		addr := identityset.Address(29)
		acts, ok := test.worker.AllActions(addr)
		test.require.False(ok)
		test.require.Nil(acts)
	})

	t.Run("pending nonce for non-existent account", func(t *testing.T) {
		test := newTestQueueWorker(t, nil)
		addr := identityset.Address(29)
		nonce, ok := test.worker.PendingNonce(addr)
		test.require.False(ok)
		test.require.Zero(nonce)
	})
}

func signedTransfer(
	senderKey crypto.PrivateKey,
	nonce uint64,
	amount *big.Int,
	recipient string,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*action.SealedEnvelope, error) {
	transfer := action.NewTransfer(amount, recipient, payload)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasLimit(gasLimit).
		SetGasPrice(gasPrice).
		SetAction(transfer).
		Build()
	return action.Sign(elp, senderKey)
}

func newTestActPool(ctrl *gomock.Controller) (*actPool, *mock_chainmanager.MockStateReader, error) {
	cfg := DefaultConfig
	cfg.ActionExpiry = 10 * time.Second
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	ap, err := NewActPool(genesis.TestDefault(), sf, cfg)
	if err != nil {
		return nil, nil, err
	}
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	actPool, ok := ap.(*actPool)
	if !ok {
		panic("wrong type")
	}
	return actPool, sf, nil
}

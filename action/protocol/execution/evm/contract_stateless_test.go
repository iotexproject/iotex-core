package evm

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func cloneContractAccount(src *state.Account) *state.Account {
	dst := &state.Account{
		Balance:  big.NewInt(0),
		Root:     src.Root,
		CodeHash: append([]byte(nil), src.CodeHash...),
	}
	return dst
}

func TestStatelessContractCommitMatchesFullContract(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	addr := hash.BytesToHash160(_c1[:])
	full, err := newContract(addr, &state.Account{}, sm, false)
	require.NoError(err)
	require.NoError(full.SetState(_k1b, _v1b[:]))
	require.NoError(full.SetState(_k2b, _v2b[:]))
	require.NoError(full.Commit())

	_, err = full.GetState(_k1b)
	require.NoError(err)
	witness, err := full.BuildStorageWitness(ContractStorageAccess{
		Writes: []common.Hash{common.BytesToHash(_k1b[:])},
	})
	require.NoError(err)

	account := cloneContractAccount(full.SelfState())
	stateless, err := newStatelessContract(addr, account, sm, false, witness)
	require.NoError(err)

	require.NoError(full.SetState(_k1b, _v3b[:]))
	require.NoError(stateless.SetState(_k1b, _v3b[:]))
	require.NoError(full.Commit())
	require.NoError(stateless.Commit())

	require.Equal(full.SelfState().Root, stateless.SelfState().Root)
}

func TestStatelessContractRejectsUnprovenAccess(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	addr := hash.BytesToHash160(_c1[:])
	full, err := newContract(addr, &state.Account{}, sm, false)
	require.NoError(err)
	require.NoError(full.SetState(_k1b, _v1b[:]))
	require.NoError(full.Commit())
	_, err = full.GetState(_k1b)
	require.NoError(err)

	witness, err := full.BuildStorageWitness(ContractStorageAccess{
		Reads: []common.Hash{common.BytesToHash(_k1b[:])},
	})
	require.NoError(err)

	stateless, err := newStatelessContract(addr, cloneContractAccount(full.SelfState()), sm, false, witness)
	require.NoError(err)

	_, err = stateless.GetState(_k2b)
	require.Error(err)
	require.ErrorIs(err, ErrMissingContractStorageWitness)

	err = stateless.SetState(_k2b, _v2b[:])
	require.Error(err)
	require.True(errors.Is(err, ErrMissingContractStorageWitness))
}

func TestErigonStateDBAdapterUsesStatelessContractWhenWitnessEnabled(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	addr := hash.BytesToHash160(_c1[:])
	full, err := newContract(addr, &state.Account{}, sm, false)
	require.NoError(err)
	require.NoError(full.SetState(_k1b, _v1b[:]))
	require.NoError(full.Commit())
	_, err = full.GetState(_k1b)
	require.NoError(err)

	witness, err := full.BuildStorageWitness(ContractStorageAccess{
		Reads: []common.Hash{common.BytesToHash(_k1b[:])},
	})
	require.NoError(err)

	actionHash := hash.Hash256b([]byte("stateless-erigon-test"))
	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{ActionHash: actionHash})
	ctx = WithStatelessValidationCtx(ctx, StatelessValidationContext{
		Enabled: true,
		ActionWitnesses: map[hash.Hash256]map[common.Address]*ContractStorageWitness{
			actionHash: {
				common.BytesToAddress(addr[:]): witness,
			},
		},
	})

	stateDB, err := NewStateDBAdapter(sm, 1, hash.ZeroHash256, WithContext(ctx))
	require.NoError(err)

	erigonStateDB := NewErigonStateDBAdapter(stateDB, nil)
	contract, err := erigonStateDB.newContract(addr, cloneContractAccount(full.SelfState()))
	require.NoError(err)

	adapter, ok := contract.(*contractAdapter)
	require.True(ok)
	_, ok = adapter.Contract.(*contractStateless)
	require.True(ok)
}

func TestStatelessContractAllowsFirstWriteOnEmptyRoot(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	addr := hash.BytesToHash160(_c1[:])
	full, err := newContract(addr, &state.Account{}, sm, false)
	require.NoError(err)
	require.NoError(full.SetState(_k1b, _v1b[:]))
	require.NoError(full.Commit())

	account := &state.Account{}
	witness := &ContractStorageWitness{
		StorageRoot: hash.ZeroHash256,
		Entries: []ContractStorageWitnessEntry{
			{Key: _k1b},
		},
		ProofNodes: [][]byte{{0x12, 0x00}},
	}
	stateless, err := newStatelessContract(addr, account, sm, false, witness)
	require.NoError(err)

	_, err = stateless.GetState(_k1b)
	require.ErrorIs(err, trie.ErrNotExist)

	require.NoError(stateless.SetState(_k1b, _v1b[:]))
	require.NoError(stateless.Commit())
	require.Equal(full.SelfState().Root, stateless.SelfState().Root)
}

func TestStatelessContractCommitPersistsTrieNodesToStateManager(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	addr := hash.BytesToHash160(_c1[:])
	full, err := newContract(addr, &state.Account{}, sm, false)
	require.NoError(err)
	require.NoError(full.SetState(_k1b, _v1b[:]))
	require.NoError(full.Commit())
	_, err = full.GetState(_k1b)
	require.NoError(err)

	witness, err := full.BuildStorageWitness(ContractStorageAccess{
		Writes: []common.Hash{common.BytesToHash(_k1b[:])},
	})
	require.NoError(err)

	stateless, err := newStatelessContract(addr, cloneContractAccount(full.SelfState()), sm, false, witness)
	require.NoError(err)
	require.NoError(stateless.SetState(_k1b, _v3b[:]))
	require.NoError(stateless.Commit())

	reloaded, err := newContract(addr, cloneContractAccount(stateless.SelfState()), sm, false)
	require.NoError(err)
	value, err := reloaded.GetState(_k1b)
	require.NoError(err)
	require.Equal(_v3b[:], value)
}

func TestDeployContractStatelessMatchesFullExecution(t *testing.T) {
	require := require.New(t)

	deployHex := "6001600055600a600c600039600a6000f360005460005260206000f3"
	deployData, err := hex.DecodeString(deployHex)
	require.NoError(err)

	buildCtx := func(actionHash hash.Hash256, witnessByAddr map[common.Address]*ContractStorageWitness) context.Context {
		g := genesis.TestDefault()
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:     identityset.Address(27),
			ActionHash: actionHash,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight: g.SumatraBlockHeight,
			Producer:    identityset.Address(27),
			GasLimit:    2_000_000,
		})
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithBlockchainCtx(protocol.WithFeatureCtx(ctx), protocol.BlockchainCtx{
			ChainID:      1,
			EvmNetworkID: 100,
		})
		ctx = WithHelperCtx(ctx, HelperContext{
			GetBlockHash: func(uint64) (hash.Hash256, error) {
				return hash.ZeroHash256, nil
			},
			GetBlockTime: func(uint64) (time.Time, error) {
				return time.Time{}, nil
			},
			DepositGasFunc: func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
				return nil, nil
			},
		})
		if witnessByAddr != nil {
			ctx = WithStatelessValidationCtx(ctx, StatelessValidationContext{
				Enabled: true,
				ActionWitnesses: map[hash.Hash256]map[common.Address]*ContractStorageWitness{
					actionHash: witnessByAddr,
				},
			})
		}
		return ctx
	}
	initSender := func(sm protocol.StateManager) {
		acct, err := accountutil.LoadOrCreateAccount(sm, identityset.Address(27))
		require.NoError(err)
		require.NoError(acct.AddBalance(big.NewInt(0).SetUint64(0).Mul(big.NewInt(1_000_000_000_000_000_000), big.NewInt(1_000_000))))
		require.NoError(acct.SetPendingNonce(1))
		require.NoError(accountutil.StoreAccount(sm, identityset.Address(27), acct))
	}
	buildExecution := func() (*action.SealedEnvelope, hash.Hash256) {
		exec := action.NewExecution("", big.NewInt(0), deployData)
		elp := (&action.EnvelopeBuilder{}).
			SetNonce(1).
			SetGasPrice(big.NewInt(0)).
			SetGasLimit(2_000_000).
			SetAction(exec).
			Build()
		selp := action.FakeSeal(elp, identityset.PrivateKey(27).PublicKey())
		h, err := selp.Hash()
		require.NoError(err)
		return selp, h
	}

	fullCtrl := gomock.NewController(t)
	fullSM, err := initMockStateManager(fullCtrl)
	require.NoError(err)
	initSender(fullSM)
	senderAccount, err := accountutil.LoadAccount(fullSM, identityset.Address(27))
	require.NoError(err)
	require.NotNil(senderAccount.Balance)
	require.True(senderAccount.Balance.Sign() > 0)
	fullSelp, fullHash := buildExecution()
	var captured map[common.Address]*ContractStorageWitness
	fullCtx := WithTracerCtx(buildCtx(fullHash, nil), TracerContext{
		CaptureContractStorageAccesses: func([]ContractStorageAccess) {},
		CaptureContractStorageWitnesses: func(w map[common.Address]*ContractStorageWitness) {
			captured = w
		},
	})
	_, fullReceipt, err := ExecuteContract(fullCtx, fullSM, fullSelp.Envelope)
	require.NoError(err)
	require.NotNil(fullReceipt)
	require.NotEmptyf(fullReceipt.ContractAddress, "deploy returned empty contract address: status=%d gas=%d output=%x", fullReceipt.Status, fullReceipt.GasConsumed, fullReceipt.Output)
	fullContractAddr, err := address.FromString(fullReceipt.ContractAddress)
	require.NoError(err)
	contractAddr := common.BytesToAddress(fullContractAddr.Bytes())
	fullAccount, err := accountutil.LoadAccount(fullSM, fullContractAddr)
	require.NoError(err)
	require.NotNil(captured[contractAddr])

	statelessCtrl := gomock.NewController(t)
	statelessSM, err := initMockStateManager(statelessCtrl)
	require.NoError(err)
	initSender(statelessSM)
	statelessSelp, statelessHash := buildExecution()
	statelessCtx := buildCtx(statelessHash, captured)
	_, statelessReceipt, err := ExecuteContract(statelessCtx, statelessSM, statelessSelp.Envelope)
	require.NoError(err)
	require.Equal(fullReceipt.ContractAddress, statelessReceipt.ContractAddress)
	statelessContractAddr, err := address.FromString(statelessReceipt.ContractAddress)
	require.NoError(err)
	statelessAccount, err := accountutil.LoadAccount(statelessSM, statelessContractAddr)
	require.NoError(err)

	require.Equal(fullAccount.Root, statelessAccount.Root)
	require.Equal(fullAccount.CodeHash, statelessAccount.CodeHash)
	require.Equal(fullAccount.PendingNonce(), statelessAccount.PendingNonce())
}

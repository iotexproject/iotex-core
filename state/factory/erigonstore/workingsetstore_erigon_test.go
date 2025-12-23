package erigonstore

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account/accountpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/enc"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestErigonStoreNativeState(t *testing.T) {
	r := require.New(t)
	dbpath := "./data/erigonstore_testdb"
	r.NoError(os.RemoveAll(dbpath))
	edb := NewErigonDB(dbpath)
	r.NoError(edb.Start(context.Background()))
	defer func() {
		edb.Stop(context.Background())
	}()
	g := genesis.TestDefault()

	fmt.Printf("block: %d -----------------------\n", 0)
	ctx := context.Background()
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 0})
	ctx = protocol.WithFeatureCtx(ctx)
	store, err := edb.NewErigonStore(ctx, 0)
	r.NoError(err)
	r.NoError(store.CreateGenesisStates(ctx))
	r.NoError(store.FinalizeTx(ctx))
	r.NoError(store.Commit(ctx, 0))

	height := uint64(1)
	t.Run("state.CandidateList", func(t *testing.T) {
		fmt.Printf("block: %d -----------------------\n", height)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		ctx = protocol.WithFeatureCtx(ctx)
		store, err = edb.NewErigonStore(ctx, 1)
		r.NoError(err)
		candlist := &state.CandidateList{
			&state.Candidate{Address: identityset.Address(1).String(), Votes: big.NewInt(1_000_000_000), RewardAddress: identityset.Address(2).String(), BLSPubKey: []byte("cpc")},
			&state.Candidate{Address: identityset.Address(5).String(), Votes: big.NewInt(2_000_000_000), RewardAddress: identityset.Address(6).String(), BLSPubKey: []byte("iotex")},
			&state.Candidate{Address: identityset.Address(9).String(), Votes: big.NewInt(2_000_000_000), RewardAddress: identityset.Address(10).String(), BLSPubKey: []byte("dev")},
			&state.Candidate{Address: identityset.Address(13).String(), Votes: big.NewInt(2_000_000_000), RewardAddress: identityset.Address(14).String(), BLSPubKey: []byte("test")},
			&state.Candidate{Address: identityset.Address(17).String(), Votes: big.NewInt(2_000_000_000), RewardAddress: identityset.Address(18).String(), BLSPubKey: []byte("prod")},
		}
		ns := state.SystemNamespace
		key := []byte("key1")
		r.NoError(store.PutObject(ns, key, candlist))
		r.NoError(store.FinalizeTx(ctx))
		r.NoError(store.Commit(ctx, 0))

		height = 2
		fmt.Printf("block: %d -----------------------\n", height)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		ctx = protocol.WithFeatureCtx(ctx)
		store, err = edb.NewErigonStore(ctx, height)
		r.NoError(err)
		defer store.Close()
		got := &state.CandidateList{}
		r.NoError(store.GetObject(ns, key, got))
		r.Equal(candlist, got)
	})

	t.Run("staking.Candidate", func(t *testing.T) {
		height++
		fmt.Printf("block: %d -----------------------\n", height)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		ctx = protocol.WithFeatureCtx(ctx)
		store, err = edb.NewErigonStore(ctx, 1)
		r.NoError(err)
		candlist := &staking.CandidateList{
			&staking.Candidate{Owner: identityset.Address(1), Operator: identityset.Address(2), Reward: identityset.Address(3), Identifier: nil, BLSPubKey: []byte("pubkey"), Name: "dev", Votes: big.NewInt(1_000_000_000), SelfStake: big.NewInt(100_000_000), SelfStakeBucketIdx: 1},
			&staking.Candidate{Owner: identityset.Address(5), Operator: identityset.Address(6), Reward: identityset.Address(7), Identifier: nil, BLSPubKey: []byte("pubkey2"), Name: "iotex", Votes: big.NewInt(2_000_000_000), SelfStake: big.NewInt(200_000_000), SelfStakeBucketIdx: 2},
			&staking.Candidate{Owner: identityset.Address(9), Operator: identityset.Address(10), Reward: identityset.Address(11), Identifier: nil, BLSPubKey: []byte("pubkey3"), Name: "test", Votes: big.NewInt(3_000_000_000), SelfStake: big.NewInt(300_000_000), SelfStakeBucketIdx: 3},
			&staking.Candidate{Owner: identityset.Address(13), Operator: identityset.Address(14), Reward: identityset.Address(15), Identifier: nil, BLSPubKey: []byte("pubkey4"), Name: "prod", Votes: big.NewInt(4_000_000_000), SelfStake: big.NewInt(400_000_000), SelfStakeBucketIdx: 4},
			&staking.Candidate{Owner: identityset.Address(17), Operator: identityset.Address(18), Reward: identityset.Address(19), Identifier: nil, BLSPubKey: []byte("pubkey5"), Name: "cpc", Votes: big.NewInt(5_000_000_000), SelfStake: big.NewInt(500_000_000), SelfStakeBucketIdx: 5},
		}
		ns := state.CandsMapNamespace
		key := []byte("key1")
		r.NoError(store.PutObject(ns, key, candlist))
		r.NoError(store.FinalizeTx(ctx))
		r.NoError(store.Commit(ctx, 0))

		height++
		fmt.Printf("block: %d -----------------------\n", height)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		ctx = protocol.WithFeatureCtx(ctx)
		store, err = edb.NewErigonStore(ctx, height)
		r.NoError(err)
		defer store.Close()
		got := &staking.CandidateList{}
		r.NoError(store.GetObject(ns, key, got))
		r.Equal(candlist, got)
	})

	t.Run("rewarding.RewardHistory", func(t *testing.T) {
		height++
		fmt.Printf("block: %d -----------------------\n", height)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		ctx = protocol.WithFeatureCtx(ctx)
		store, err = edb.NewErigonStore(ctx, height)
		r.NoError(err)
		defer store.Close()
		rh := &rewarding.RewardHistory{}
		ns := state.AccountKVNamespace
		var indexBytes [8]byte
		enc.MachineEndian.PutUint64(indexBytes[:], height)
		key := append(state.RewardingKeyPrefix[:], state.BlockRewardHistoryKeyPrefix...)
		key = append(key, indexBytes[:]...)
		nextKey := append(state.RewardingKeyPrefix[:], state.BlockRewardHistoryKeyPrefix...)
		enc.MachineEndian.PutUint64(indexBytes[:], height+1)
		nextKey = append(nextKey, indexBytes[:]...)
		// test GetObject/DeleteObject/PutObject
		r.NoError(store.GetObject(ns, key, rh))
		r.ErrorIs(store.GetObject(ns, nextKey, rh), state.ErrStateNotExist)
		r.NoError(store.DeleteObject(ns, key, rh))
		r.ErrorIs(store.GetObject(ns, key, rh), state.ErrStateNotExist)
		r.NoError(store.PutObject(ns, key, rh))
		r.NoError(store.GetObject(ns, key, rh))
		r.NoError(store.FinalizeTx(ctx))
		r.NoError(store.Commit(ctx, 0))
	})

	t.Run("rewarding.Fund", func(t *testing.T) {
		height++
		fmt.Printf("block: %d -----------------------\n", height)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		ctx = protocol.WithFeatureCtx(ctx)
		store, err = edb.NewErigonStore(ctx, height)
		r.NoError(err)
		defer store.Close()
		rh := &rewarding.Fund{}
		rhpb := &rewardingpb.Fund{
			TotalBalance:     "1000000000000000000",
			UnclaimedBalance: "500000000000000000",
		}
		data, err := proto.Marshal(rhpb)
		r.NoError(err)
		r.NoError(rh.Deserialize(data))
		ns := state.AccountKVNamespace
		_fundKey := []byte("fnd")
		key := append(state.RewardingKeyPrefix[:], _fundKey...)
		r.NoError(store.PutObject(ns, key, rh))
		got := &rewarding.Fund{}
		r.NoError(store.GetObject(ns, key, got))
		r.EqualValues(rh, got)
		r.NoError(store.FinalizeTx(ctx))
		r.NoError(store.Commit(ctx, 0))

		height++
		fmt.Printf("block: %d -----------------------\n", height)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		ctx = protocol.WithFeatureCtx(ctx)
		store, err = edb.NewErigonStore(ctx, height)
		r.NoError(err)
		defer store.Close()
		rhpb = &rewardingpb.Fund{
			TotalBalance:     "1000000000000000000",
			UnclaimedBalance: "400000000000000000",
		}
		data, err = proto.Marshal(rhpb)
		r.NoError(err)
		r.NoError(rh.Deserialize(data))
		r.NoError(store.PutObject(ns, key, rh))
		got = &rewarding.Fund{}
		r.NoError(store.GetObject(ns, key, got))
		r.EqualValues(rh, got)
		r.NoError(store.FinalizeTx(ctx))
		r.NoError(store.Commit(ctx, 0))
	})

	t.Run("state.Account", func(t *testing.T) {
		height++
		fmt.Printf("block: %d -----------------------\n", height)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		ctx = protocol.WithFeatureCtx(ctx)
		store, err = edb.NewErigonStore(ctx, height)
		r.NoError(err)
		defer store.Close()
		empty := crypto.Keccak256Hash(nil)
		acctPb := &accountpb.Account{
			Type:     accountpb.AccountType_DEFAULT,
			Balance:  "1000000000000000000",
			Nonce:    10,
			Root:     []byte("root"),
			CodeHash: empty[:],
		}
		acct := &state.Account{}
		acct.FromProto(acctPb)
		ns := state.AccountKVNamespace
		key := identityset.Address(0).Bytes()
		gotAcct := &state.Account{}
		r.ErrorIs(store.GetObject(ns, key, gotAcct), state.ErrStateNotExist)
		fmt.Printf("Storing account at address: %x\n", key)
		r.NoError(store.PutObject(ns, key, acct))
		fmt.Printf("Stored account at address: %x\n", key)
		r.NoError(store.GetObject(ns, key, gotAcct))
		r.Equal(acct, gotAcct)
		r.NoError(store.FinalizeTx(ctx))
		r.NoError(store.Commit(ctx, 0))
	})

}

package blockindex

import (
	"context"
	"encoding/hex"
	"math/big"
	"sync/atomic"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	_testSGDContractAddress = "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms"
)

func TestNewSGDRegistry(t *testing.T) {
	r := require.New(t)

	t.Run("kvStore is nil", func(t *testing.T) {
		r.Panics(func() {
			NewSGDRegistry(_testSGDContractAddress, 0, nil)
		})
	})

	t.Run("invalid contract address", func(t *testing.T) {
		kvStore := db.NewMemKVStore()
		r.Panics(func() {
			NewSGDRegistry("invalid contract", 0, kvStore)
		})
	})

	t.Run("valid", func(t *testing.T) {
		testDBPath, err := testutil.PathOfTempFile("sgd")
		r.NoError(err)
		ctx := context.Background()
		cfg := db.DefaultConfig
		cfg.DbPath = testDBPath
		kvStore := db.NewBoltDB(cfg)
		sgdRegistry := NewSGDRegistry(_testSGDContractAddress, 0, kvStore)
		r.NoError(sgdRegistry.Start(ctx))
		defer func() {
			r.NoError(sgdRegistry.Stop(ctx))
			testutil.CleanupPath(testDBPath)
		}()

		nonce := uint64(0)
		startHeight, err := sgdRegistry.StartHeight()
		r.NoError(err)
		r.Equal(nonce, startHeight)
		hh, err := sgdRegistry.Height()
		r.NoError(err)
		r.Equal(nonce, hh)
		registerAddress, err := address.FromHex("5b38da6a701c568545dcfcb03fcb875f56beddc4")
		r.NoError(err)
		receiverAddress, err := address.FromHex("78731d3ca6b7e34ac0f824c42a7cc18a495cabab")
		r.NoError(err)
		t.Run("registerContract", func(t *testing.T) {
			builder := block.NewTestingBuilder()
			event := _sgdABI.Events["ContractRegistered"]
			data, _ := hex.DecodeString("0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc400000000000000000000000078731d3ca6b7e34ac0f824c42a7cc18a495cabab")
			exec, err := action.SignedExecution(_testSGDContractAddress, identityset.PrivateKey(27), atomic.AddUint64(&nonce, 1), big.NewInt(0), 10000000, big.NewInt(9000000000000), data)
			r.NoError(err)
			h, _ := exec.Hash()
			logs := &action.Log{
				Address: _testSGDContractAddress,
				Topics:  []hash.Hash256{hash.Hash256(event.ID)},
				Data:    data,
			}
			blk := createTestingBlock(builder, 1, h, exec, logs)
			r.NoError(sgdRegistry.PutBlock(ctx, blk))
			receiver, percentage, isApproved, err := sgdRegistry.CheckContract(ctx, registerAddress.String())
			r.NoError(err)
			r.Equal(_sgdPercentage, percentage)
			r.Equal(receiverAddress, receiver)
			r.False(isApproved)

			lists, err := sgdRegistry.FetchContracts(ctx)
			r.NoError(err)
			r.Equal(1, len(lists))
			r.Equal(registerAddress.Bytes(), lists[0].Contract.Bytes())
			r.Equal(receiverAddress.Bytes(), lists[0].Receiver.Bytes())
			r.False(lists[0].Approved)
		})
		t.Run("approveContract", func(t *testing.T) {
			builder := block.NewTestingBuilder()
			event := _sgdABI.Events["ContractApproved"]
			data, _ := hex.DecodeString("0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4")
			exec, err := action.SignedExecution(_testSGDContractAddress, identityset.PrivateKey(27), atomic.AddUint64(&nonce, 1), big.NewInt(0), 10000000, big.NewInt(9000000000000), data)
			r.NoError(err)
			h, _ := exec.Hash()
			logs := &action.Log{
				Address: _testSGDContractAddress,
				Topics:  []hash.Hash256{hash.Hash256(event.ID)},
				Data:    data,
			}
			blk := createTestingBlock(builder, 1, h, exec, logs)
			r.NoError(sgdRegistry.PutBlock(ctx, blk))
			receiver, percentage, isApproved, err := sgdRegistry.CheckContract(ctx, registerAddress.String())
			r.NoError(err)
			r.Equal(receiverAddress, receiver)
			r.True(isApproved)
			r.Equal(_sgdPercentage, percentage)
		})
		t.Run("disapproveContract", func(t *testing.T) {
			builder := block.NewTestingBuilder()
			event := _sgdABI.Events["ContractDisapproved"]
			data, _ := hex.DecodeString("0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4")
			exec, err := action.SignedExecution(_testSGDContractAddress, identityset.PrivateKey(27), atomic.AddUint64(&nonce, 1), big.NewInt(0), 10000000, big.NewInt(9000000000000), data)
			r.NoError(err)
			h, _ := exec.Hash()
			logs := &action.Log{
				Address: _testSGDContractAddress,
				Topics:  []hash.Hash256{hash.Hash256(event.ID)},
				Data:    data,
			}
			blk := createTestingBlock(builder, 1, h, exec, logs)
			r.NoError(sgdRegistry.PutBlock(ctx, blk))
			receiver, percentage, isApproved, err := sgdRegistry.CheckContract(ctx, registerAddress.String())
			r.NoError(err)
			r.Equal(receiverAddress, receiver)
			r.False(isApproved)
			r.Equal(_sgdPercentage, percentage)
		})
		t.Run("removeContract", func(t *testing.T) {
			builder := block.NewTestingBuilder()
			event := _sgdABI.Events["ContractRemoved"]
			data, _ := hex.DecodeString("0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4")
			exec, err := action.SignedExecution(_testSGDContractAddress, identityset.PrivateKey(27), atomic.AddUint64(&nonce, 1), big.NewInt(0), 10000000, big.NewInt(9000000000000), data)
			r.NoError(err)
			h, _ := exec.Hash()
			logs := &action.Log{
				Address: _testSGDContractAddress,
				Topics:  []hash.Hash256{hash.Hash256(event.ID)},
				Data:    data,
			}
			blk := createTestingBlock(builder, 2, h, exec, logs)
			r.NoError(sgdRegistry.PutBlock(ctx, blk))
			receiver, percentage, isApproved, err := sgdRegistry.CheckContract(ctx, registerAddress.String())
			r.ErrorContains(err, "not exist in DB")
			r.Nil(receiver)
			r.False(isApproved)
			hh, err := sgdRegistry.Height()
			r.NoError(err)
			r.Equal(blk.Height(), hh)
			r.Equal(uint64(0), percentage)

			_, err = sgdRegistry.FetchContracts(ctx)
			r.ErrorIs(err, state.ErrStateNotExist)
		})
	})
}

func createTestingBlock(builder *block.TestingBuilder, height uint64, h hash.Hash256, act action.SealedEnvelope, logs *action.Log) *block.Block {
	block.LoadGenesisHash(&genesis.Default)
	r := &action.Receipt{
		Status:      1,
		BlockHeight: height,
		ActionHash:  h,
	}

	blk, _ := builder.
		SetHeight(height).
		SetPrevBlockHash(h).
		AddActions(act).
		SetReceipts([]*action.Receipt{
			r.AddLogs(logs),
		}).
		SetTimeStamp(testutil.TimestampNow().UTC()).
		SignAndBuild(identityset.PrivateKey(27))
	return &blk
}

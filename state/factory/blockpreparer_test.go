package factory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestBlockPreparer_PrepareOrWait(t *testing.T) {
	preparer := newBlockPreparer()
	prevHash := hash.Hash256b([]byte("previousHash"))
	timestamp := time.Now()
	mockBlk := &block.Block{}
	called := false

	// Mock mint function
	mintFn := func() (*block.Block, error) {
		if called {
			return nil, errors.New("block already minted")
		}
		called = true
		return mockBlk, nil
	}
	mintFn2 := func() (*block.Block, error) {
		return &block.Block{
			Body: block.Body{
				Actions: []*action.SealedEnvelope{},
			},
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		blk1, err := preparer.PrepareOrWait(ctx, prevHash[:], timestamp, mintFn)
		require.NoError(t, err)
		require.Equal(t, mockBlk, blk1)
		wg.Done()
	}()
	go func() {
		blk2, err := preparer.PrepareOrWait(ctx, prevHash[:], timestamp.Add(time.Second), mintFn2)
		require.NoError(t, err)
		require.NotEqual(t, mockBlk, blk2)
		wg.Done()
	}()
	blk, err := preparer.PrepareOrWait(ctx, prevHash[:], timestamp, mintFn)
	require.NoError(t, err)
	require.Equal(t, mockBlk, blk)
	wg.Wait()
}

func TestBlockPreparer_PrepareOrWait_Timeout(t *testing.T) {
	preparer := newBlockPreparer()
	prevHash := hash.Hash256b([]byte("previousHash"))
	timestamp := time.Now()

	// Mock mint function that takes too long
	mintFn := func() (*block.Block, error) {
		time.Sleep(2 * time.Second)
		return &block.Block{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	blk, err := preparer.PrepareOrWait(ctx, prevHash[:], timestamp, mintFn)
	require.Error(t, err)
	require.Nil(t, blk)
}

func TestBlockPreparer_ReceiveBlock(t *testing.T) {
	preparer := newBlockPreparer()
	prevHash := hash.Hash256b([]byte("previousHash"))
	timestamp := time.Now()

	// Mock mint function
	mintFn := func() (*block.Block, error) {
		builder := &block.TestingBuilder{}
		blk, err := builder.SetPrevBlockHash(prevHash).SignAndBuild(identityset.PrivateKey(0))
		return &blk, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	blk, err := preparer.PrepareOrWait(ctx, prevHash[:], timestamp, mintFn)
	require.NoError(t, err)

	emptyblk := &block.Block{}
	require.NoError(t, preparer.ReceiveBlock(emptyblk))
	_, ok := preparer.tasks[prevHash]
	require.True(t, ok)
	_, ok = preparer.results[prevHash]
	require.True(t, ok)

	require.NoError(t, preparer.ReceiveBlock(blk))
	_, ok = preparer.tasks[prevHash]
	require.False(t, ok)
	_, ok = preparer.results[prevHash]
	require.False(t, ok)
}

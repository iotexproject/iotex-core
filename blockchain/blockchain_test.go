package blockchain

import (
	"context"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/filedao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockcreationsubscriber"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockdao"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockvalidator"
	"github.com/iotexproject/iotex-core/test/mock/mock_crypto"
	"github.com/iotexproject/iotex-core/test/mock/mock_facebookgo_clock"
	"github.com/iotexproject/iotex-core/test/mock/mock_iotex_address"
	"github.com/iotexproject/iotex-core/test/mock/mock_pubsubmanager"
)

func TestNewBlockchain(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	blkVldt := mock_blockvalidator.NewMockValidator(c)
	clock := mock_facebookgo_clock.NewMockClock(c)
	blkDAO := mock_blockdao.NewMockBlockDAO(c)

	options := []Option{
		BlockValidatorOption(blkVldt),
		ClockOption(clock),
	}

	t.Run("CheckParameters", func(t *testing.T) {
		t.Run("BlockDAOMustValid", func(t *testing.T) {
			defer func() {
				err := recover()
				r.Contains(err, "blockdao is nil")
			}()

			bc := NewBlockchain(Config{}, genesis.Genesis{}, nil, nil)
			r.Nil(bc)
		})
	})

	t.Run("ApplyBlockchainOption", func(t *testing.T) {
		t.Run("CatchPanicWhenApplyOptionFailed", func(t *testing.T) {
			defer func() {
				err := recover()
				r.Contains(err, "must error option")
			}()

			_options := append(options, func(*blockchain) error { return errors.New("must error option") })

			bc := NewBlockchain(Config{}, genesis.Genesis{}, blkDAO, nil, _options...)
			r.Nil(bc)
		})
	})

	t.Run("CreatePrometheusTimer", func(t *testing.T) {
		t.Run("CatchPanicWhenCreateTimerFactory", func(t *testing.T) {
			p := gomonkey.NewPatches()
			defer p.Reset()

			defer func() {
				r.Contains(recover(), "Failed to generate prometheus timer factory")
			}()

			p = p.ApplyFuncReturn(prometheustimer.New, nil, errors.New(t.Name()))

			bc := NewBlockchain(Config{}, genesis.Genesis{}, blkDAO, nil, options...)
			r.Nil(bc)
		})
	})

	t.Run("Success", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = p.ApplyFuncReturn(prometheustimer.New, &prometheustimer.TimerFactory{}, nil)

		bc := NewBlockchain(Config{}, genesis.Genesis{}, blkDAO, nil, options...)
		r.NotNil(bc)
	})
}

func Test_blockchain_ChainID(t *testing.T) {
	r := require.New(t)

	r.Equal((&blockchain{}).ChainID(), uint32(0))
}

func Test_blockchain_EvmNetworkID(t *testing.T) {
	r := require.New(t)

	r.Equal((&blockchain{}).EvmNetworkID(), uint32(0))
}

func Test_blockchain_Address(t *testing.T) {
	r := require.New(t)

	r.Equal((&blockchain{}).ChainAddress(), "")
}

func Test_blockchain_Start(t *testing.T) {
	r := require.New(t)

	ctx := context.Background()
	t.Run("WithTipContext", func(t *testing.T) {
		t.Run("FailedToWithTipContext", func(t *testing.T) {
			p := gomonkey.NewPatches()
			defer p.Reset()

			p = patchBlockchain_context(p, nil, errors.New(t.Name()))

			err := (&blockchain{}).Start(ctx)
			r.ErrorContains(err, t.Name())
		})
	})

	t.Run("StartLifecycle", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = patchBlockchain_context(p, ctx, nil)
		p = p.ApplyMethodReturn(&lifecycle.Lifecycle{}, "OnStart", nil)

		err := (&blockchain{}).Start(ctx)
		r.NoError(err)
	})
}

func Test_blockchain_Stop(t *testing.T) {
	r := require.New(t)
	p := gomonkey.NewPatches()
	defer p.Reset()

	p = p.ApplyMethodReturn(&lifecycle.Lifecycle{}, "OnStop", nil)

	r.NoError((&blockchain{}).Stop(context.Background()))
}

func Test_blockchain_BlockHeaderByHeight(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	dao := mock_blockdao.NewMockBlockDAO(c)
	dao.EXPECT().HeaderByHeight(gomock.Any()).Return(nil, nil).Times(1)

	h, err := (&blockchain{dao: dao}).BlockHeaderByHeight(0)
	r.NoError(err)
	r.Nil(h)
}

func Test_blockchain_BlockFooterByHeight(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	dao := mock_blockdao.NewMockBlockDAO(c)
	dao.EXPECT().FooterByHeight(gomock.Any()).Return(nil, nil).Times(1)

	h, err := (&blockchain{dao: dao}).BlockFooterByHeight(0)
	r.NoError(err)
	r.Nil(h)
}

func Test_blockchain_TipHash(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	dao := mock_blockdao.NewMockBlockDAO(c)

	t.Run("FailedToGetDAOHeight", func(t *testing.T) {
		dao.EXPECT().Height().Return(uint64(0), errors.New(t.Name())).Times(1)

		h := (&blockchain{dao: dao}).TipHash()
		r.Equal(h, hash.ZeroHash256)
	})

	t.Run("FailedToGetDAOBlockHash", func(t *testing.T) {
		dao.EXPECT().Height().Return(uint64(100), nil).Times(1)
		dao.EXPECT().GetBlockHash(gomock.Any()).Return(hash.Hash256{}, errors.New(t.Name())).Times(1)

		h := (&blockchain{dao: dao}).TipHash()
		r.Equal(h, hash.ZeroHash256)
	})

	t.Run("Success", func(t *testing.T) {
		h1 := hash.Hash256{1}
		dao.EXPECT().Height().Return(uint64(100), nil).Times(1)
		dao.EXPECT().GetBlockHash(gomock.Any()).Return(h1, nil).Times(1)

		h2 := (&blockchain{dao: dao}).TipHash()
		r.Equal(h1, h2)
	})
}

func Test_blockchain_TipHeight(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	dao := mock_blockdao.NewMockBlockDAO(c)
	bc := &blockchain{dao: dao}

	t.Run("GetDAOHeight", func(t *testing.T) {
		t.Run("CachePanic", func(t *testing.T) {
			defer func() {
				r.Contains(recover(), "failed to get tip height")
			}()
			dao.EXPECT().Height().Return(uint64(0), errors.New(t.Name())).Times(1)
			_ = bc.TipHeight()
		})
	})

	t.Run("Success", func(t *testing.T) {
		height := uint64(100)
		dao.EXPECT().Height().Return(height, nil).Times(1)
		r.Equal(bc.TipHeight(), height)
	})
}

func Test_blockchain_ValidateBlock(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	t.Run("CheckParameters", func(t *testing.T) {
		t.Run("BlkInvalid", func(t *testing.T) {
			err := (&blockchain{}).ValidateBlock(nil)
			r.Equal(err, ErrInvalidBlock)
		})
	})

	t.Run("GetTipInfo", func(t *testing.T) {
		t.Run("FailedToGetTipInfo", func(t *testing.T) {
			p := gomonkey.NewPatches()
			defer p.Reset()

			p = patchBlockchain_tipInfo(p, nil, errors.New(t.Name()))

			err := (&blockchain{}).ValidateBlock(&block.Block{})
			r.ErrorContains(err, t.Name())
		})
	})

	tip := &protocol.TipInfo{Height: 1}

	t.Run("ValidateTipBlockHeight", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = patchBlockchain_tipInfo(p, tip, nil)
		p = p.ApplyMethodReturn(&block.Header{}, "Height", tip.Height)

		err := (&blockchain{}).ValidateBlock(&block.Block{})
		r.ErrorContains(err, ErrInvalidTipHeight.Error())
	})

	t.Run("ValidatePrevBlockHash", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = patchBlockchain_tipInfo(p, tip, nil)
		p = p.ApplyMethodReturn(&block.Header{}, "Height", tip.Height+1)
		p = p.ApplyMethodReturn(&block.Header{}, "PrevHash", hash.Hash256{1})

		err := (&blockchain{}).ValidateBlock(&block.Block{})
		r.ErrorContains(err, ErrInvalidBlock.Error())
	})

	t.Run("VerifyBlockHeaderSignature", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = patchBlockchain_tipInfo(p, tip, nil)
		p = p.ApplyMethodReturn(&block.Header{}, "Height", tip.Height+1)
		p = p.ApplyMethodReturn(&block.Header{}, "PrevHash", hash.Hash256{})
		p = p.ApplyMethodReturn(&block.Header{}, "VerifySignature", false)

		err := (&blockchain{}).ValidateBlock(&block.Block{})
		r.ErrorContains(err, "failed to verify block's signature")
	})

	t.Run("VerifyBlockTxRoot", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = patchBlockchain_tipInfo(p, tip, nil)
		p = p.ApplyMethodReturn(&block.Header{}, "Height", tip.Height+1)
		p = p.ApplyMethodReturn(&block.Header{}, "PrevHash", hash.Hash256{})
		p = p.ApplyMethodReturn(&block.Header{}, "VerifySignature", true)
		p = p.ApplyMethodReturn(&block.Block{}, "VerifyTxRoot", errors.New(t.Name()))

		err := (&blockchain{}).ValidateBlock(&block.Block{})
		r.ErrorContains(err, t.Name())
	})

	t.Run("ValidateProducerAddress", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		pubkey := mock_crypto.NewMockPublicKey(c)

		p = patchBlockchain_tipInfo(p, tip, nil)
		p = p.ApplyMethodReturn(&block.Header{}, "Height", tip.Height+1)
		p = p.ApplyMethodReturn(&block.Header{}, "PrevHash", hash.Hash256{})
		p = p.ApplyMethodReturn(&block.Header{}, "VerifySignature", true)
		p = p.ApplyMethodReturn(&block.Block{}, "VerifyTxRoot", nil)
		p = p.ApplyMethodReturn(&block.Header{}, "PublicKey", pubkey)
		pubkey.EXPECT().Address().Return(nil).Times(1)

		err := (&blockchain{}).ValidateBlock(&block.Block{})
		r.ErrorContains(err, "failed to get address")
	})

	t.Run("ComposeContextsAndValidateBlock", func(t *testing.T) {
		t.Run("FailedToWithBlockchainContext", func(t *testing.T) {
			p := gomonkey.NewPatches()
			defer p.Reset()

			pubkey := mock_crypto.NewMockPublicKey(c)
			addr := mock_iotex_address.NewMockAddress(c)

			p = patchBlockchain_tipInfo(p, tip, nil)
			p = p.ApplyMethodReturn(&block.Header{}, "Height", tip.Height+1)
			p = p.ApplyMethodReturn(&block.Header{}, "PrevHash", hash.Hash256{})
			p = p.ApplyMethodReturn(&block.Header{}, "VerifySignature", true)
			p = p.ApplyMethodReturn(&block.Block{}, "VerifyTxRoot", nil)
			p = p.ApplyMethodReturn(&block.Header{}, "PublicKey", pubkey)
			pubkey.EXPECT().Address().Return(addr).Times(1)
			p = patchBlockchain_context(p, nil, errors.New(t.Name()))

			err := (&blockchain{}).ValidateBlock(&block.Block{})
			r.ErrorContains(err, t.Name())
		})

		t.Run("ValidateBlockWithBlockchainValidator", func(t *testing.T) {
			p := gomonkey.NewPatches()
			defer p.Reset()

			pubkey := mock_crypto.NewMockPublicKey(c)
			addr := mock_iotex_address.NewMockAddress(c)
			ctx := context.Background()

			p = patchBlockchain_tipInfo(p, tip, nil)
			p = p.ApplyMethodReturn(&block.Header{}, "Height", tip.Height+1)
			p = p.ApplyMethodReturn(&block.Header{}, "PrevHash", hash.Hash256{})
			p = p.ApplyMethodReturn(&block.Header{}, "VerifySignature", true)
			p = p.ApplyMethodReturn(&block.Block{}, "VerifyTxRoot", nil)
			p = p.ApplyMethodReturn(&block.Header{}, "PublicKey", pubkey)
			pubkey.EXPECT().Address().Return(addr).Times(2)
			p = patchBlockchain_context(p, ctx, nil)
			p = p.ApplyFuncReturn(protocol.WithFeatureCtx, ctx)

			t.Run("BlockchainValidatorIsNil", func(t *testing.T) {
				err := (&blockchain{}).ValidateBlock(&block.Block{})
				r.NoError(err)
			})
			t.Run("Success", func(t *testing.T) {
				blkVldt := mock_blockvalidator.NewMockValidator(c)
				blkVldt.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				err := (&blockchain{blockValidator: blkVldt}).ValidateBlock(&block.Block{})
				r.NoError(err)
			})
		})
	})

}

func Test_blockchain_Context(t *testing.T) {
	r := require.New(t)
	p := gomonkey.NewPatches()
	defer p.Reset()

	ctx := context.Background()

	p = patchBlockchain_context(p, ctx, nil)

	ctx2, err := (&blockchain{}).Context(ctx)
	r.Equal(ctx2, ctx)
	r.NoError(err)
}

func Test_blockchain_contextWithBlock(t *testing.T) {
	r := require.New(t)
	p := gomonkey.NewPatches()
	defer p.Reset()

	ctx := context.Background()

	p = p.ApplyFuncReturn(protocol.WithBlockCtx, ctx)

	ctx2 := (&blockchain{}).contextWithBlock(ctx, address.Address(nil), 100, time.Now())
	r.Equal(ctx2, ctx)
}

func Test_blockchain_context(t *testing.T) {
	r := require.New(t)

	bc := &blockchain{}
	ctx := context.Background()

	t.Run("GetTipValue", func(t *testing.T) {
		t.Run("FailedToGetTipValue", func(t *testing.T) {
			p := gomonkey.NewPatches()
			defer p.Reset()

			p = patchBlockchain_tipInfo(p, nil, errors.New(t.Name()))

			ctx2, err := bc.context(ctx, true)
			r.Nil(ctx2)
			r.ErrorContains(err, t.Name())
		})
	})

	t.Run("Success", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = patchBlockchain_tipInfo(p, &protocol.TipInfo{}, nil)
		p = p.ApplyFuncReturn(genesis.WithGenesisContext, ctx)
		p = p.ApplyFuncReturn(protocol.WithFeatureWithHeightCtx, ctx)

		ctx2, err := bc.context(ctx, true)
		r.Equal(ctx2, ctx)
		r.NoError(err)
	})
}

type mockBlockBuilderFactory struct{}

func (b *mockBlockBuilderFactory) NewBlockBuilder(context.Context, func(action.Envelope) (*action.SealedEnvelope, error)) (*block.Builder, error) {
	return nil, nil
}

func Test_blockchain_MintBlock(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	dao := mock_blockdao.NewMockBlockDAO(c)
	bbf := &mockBlockBuilderFactory{}
	bc := &blockchain{
		timerFactory: &prometheustimer.TimerFactory{},
		bbf:          bbf,
		dao:          dao,
	}

	t.Run("GetDAOHeight", func(t *testing.T) {
		t.Run("FailedToGetDaoHeight", func(t *testing.T) {
			dao.EXPECT().Height().Return(uint64(0), errors.New(t.Name())).Times(1)

			blk, err := bc.MintNewBlock(time.Now())
			r.Nil(blk)
			r.ErrorContains(err, t.Name())
		})
	})

	t.Run("ComposeRequiredContext", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		dao.EXPECT().Height().Return(uint64(100), nil).Times(1)
		p = patchBlockchain_tipInfo(p, nil, errors.New(t.Name()))

		blk, err := bc.MintNewBlock(time.Now())
		r.Nil(blk)
		r.ErrorContains(err, t.Name())
	})

	t.Run("NewBlockBuilder", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		ctx := context.Background()

		dao.EXPECT().Height().Return(uint64(100), nil).Times(1)
		p = patchBlockchain_tipInfo(p, &protocol.TipInfo{}, nil)
		p = p.ApplyMethodReturn(&Config{}, "ProducerAddress", address.Address(nil))
		p = p.ApplyMethodReturn(&Config{}, "ProducerPrivateKey", crypto.PrivateKey(nil))
		p = patchBlockchain_contextWithBlock(p, context.WithValue(ctx, 1, "withBlockInfo"))
		p = p.ApplyFuncReturn(protocol.WithFeatureCtx, context.WithValue(ctx, 2, "withProtocolFeature"))
		p = p.ApplyMethodReturn(&mockBlockBuilderFactory{}, "NewBlockBuilder", nil, errors.New(t.Name()))

		blk, err := bc.MintNewBlock(time.Now())
		r.Nil(blk)
		r.ErrorContains(err, t.Name())
	})

	t.Run("SignAndBuild", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		ctx := context.Background()

		dao.EXPECT().Height().Return(uint64(200), nil).Times(2)
		p = patchBlockchain_tipInfo(p, &protocol.TipInfo{}, nil)
		p = p.ApplyMethodReturn(&Config{}, "ProducerAddress", address.Address(nil))
		p = p.ApplyMethodReturn(&Config{}, "ProducerPrivateKey", crypto.PrivateKey(nil))
		p = patchBlockchain_contextWithBlock(p, context.WithValue(ctx, 1, "withBlockInfo"))
		p = p.ApplyFuncReturn(protocol.WithFeatureCtx, context.WithValue(ctx, 2, "withProtocolFeature"))
		p = p.ApplyMethodReturn(&mockBlockBuilderFactory{}, "NewBlockBuilder", &block.Builder{}, nil)
		p = p.ApplyMethodReturn(&block.Builder{}, "SignAndBuild", nil, errors.New(t.Name()))

		blk, err := bc.MintNewBlock(time.Now())
		r.Nil(blk)
		r.ErrorContains(err, t.Name())

		p = p.ApplyMethodReturn(&block.Builder{}, "SignAndBuild", block.Block{}, nil)
		blk, err = bc.MintNewBlock(time.Now())
		r.NotNil(blk)
		r.NoError(err)
	})
}

func Test_blockchain_CommitBlock(t *testing.T) {
	r := require.New(t)
	p := gomonkey.NewPatches()
	defer p.Reset()

	p = patchBlockchain_commitBlock(p, nil)

	bc := &blockchain{
		timerFactory: &prometheustimer.TimerFactory{},
	}
	r.NoError(bc.CommitBlock(&block.Block{}))
}

func Test_blockchain_AddBlockListener(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	ps := mock_pubsubmanager.NewMockPubSubManager(c)
	ps.EXPECT().AddBlockListener(gomock.Any()).Return(nil).Times(1)

	sub := mock_blockcreationsubscriber.NewMockBlockCreationSubscriber(c)

	bc := &blockchain{pubSubManager: ps}

	r.ErrorContains(bc.AddSubscriber(nil), "subscriber could not be nil")
	r.NoError(bc.AddSubscriber(sub))
}

func Test_blockchain_RemoveBlockListener(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	ps := mock_pubsubmanager.NewMockPubSubManager(c)
	ps.EXPECT().RemoveBlockListener(gomock.Any()).Return(nil).Times(1)

	sub := mock_blockcreationsubscriber.NewMockBlockCreationSubscriber(c)

	bc := &blockchain{pubSubManager: ps}
	r.NoError(bc.RemoveSubscriber(sub))
}

func Test_blockchain_Genesis(t *testing.T) {
	bc := &blockchain{}
	require.Equal(t, bc.genesis, bc.Genesis())
}

func Test_blockchain_tipInfo(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	dao := mock_blockdao.NewMockBlockDAO(c)
	bc := &blockchain{
		dao: dao,
	}

	t.Run("GetDaoHeight", func(t *testing.T) {
		t.Run("FailedToGetHeight", func(t *testing.T) {
			dao.EXPECT().Height().Return(uint64(0), errors.New(t.Name())).Times(1)

			tip, err := bc.tipInfo()
			r.Nil(tip)
			r.ErrorContains(err, t.Name())
		})
	})

	t.Run("ReturnGenesisTipInfoWhenTipHeightIs0", func(t *testing.T) {
		dao.EXPECT().Height().Return(uint64(0), nil).Times(1)

		tip, err := bc.tipInfo()
		r.NotNil(tip)
		r.NoError(err)
	})

	t.Run("GetDaoHeader", func(t *testing.T) {
		t.Run("FailedToGetHeader", func(t *testing.T) {
			dao.EXPECT().Height().Return(uint64(1), nil).Times(1)
			dao.EXPECT().HeaderByHeight(uint64(1)).Return(nil, errors.New(t.Name())).Times(1)

			tip, err := bc.tipInfo()
			r.Nil(tip)
			r.ErrorContains(err, t.Name())
		})
	})

	t.Run("Success", func(t *testing.T) {
		dao.EXPECT().Height().Return(uint64(1), nil).Times(1)
		dao.EXPECT().HeaderByHeight(uint64(1)).Return(&block.Header{}, nil).Times(1)

		tip, err := bc.tipInfo()
		r.NotNil(tip)
		r.NoError(err)
	})
}

func Test_blockchain_commitBlock(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	dao := mock_blockdao.NewMockBlockDAO(c)
	bc := &blockchain{
		timerFactory: &prometheustimer.TimerFactory{},
		dao:          dao,
	}

	t.Run("ComposeTipInfoContext", func(t *testing.T) {
		t.Run("FailedToCompose", func(t *testing.T) {
			p := gomonkey.NewPatches()
			defer p.Reset()

			p = patchBlockchain_context(p, nil, errors.New(t.Name()))
			r.ErrorContains(bc.CommitBlock(&block.Block{}), t.Name())
		})
	})

	t.Run("PutBlock", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = patchBlockchain_context(p, context.Background(), nil)

		t.Run("CaseAlreadyExistError", func(t *testing.T) {
			dao.EXPECT().PutBlock(gomock.Any(), gomock.Any()).Return(filedao.ErrAlreadyExist).Times(1)
			r.NoError(bc.CommitBlock(&block.Block{}))
		})
		t.Run("OtherFailureError", func(t *testing.T) {
			dao.EXPECT().PutBlock(gomock.Any(), gomock.Any()).Return(errors.New(t.Name())).Times(1)
			r.ErrorContains(bc.CommitBlock(&block.Block{}), t.Name())
		})
	})

	t.Run("HashBlockAndEmitToSubscribers", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = patchBlockchain_context(p, context.Background(), nil)
		dao.EXPECT().PutBlock(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		p = p.ApplyMethodReturn(&block.Header{}, "Height", uint64(100))
		p = p.ApplyMethodReturn(&block.Header{}, "HashBlock", hash.Hash256{})
		p = patchBlockchain_emitToSubscribers(p)
		r.NoError(bc.commitBlock(&block.Block{}))
	})
}

func Test_blockchain_emitToSubscribers(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()

	bc := &blockchain{}
	bc.emitToSubscribers(&block.Block{})

	ps := mock_pubsubmanager.NewMockPubSubManager(c)
	ps.EXPECT().SendBlockToSubscribers(gomock.Any()).Return().Times(1)
	bc.pubSubManager = ps
	bc.emitToSubscribers(&block.Block{})
}

func patchBlockchain_tipInfo(p *gomonkey.Patches, info *protocol.TipInfo, err error) *gomonkey.Patches {
	return p.ApplyPrivateMethod(
		&blockchain{},
		"tipInfo",
		func(*blockchain) (*protocol.TipInfo, error) {
			return info, err
		},
	)
}

func patchBlockchain_context(p *gomonkey.Patches, ctx context.Context, err error) *gomonkey.Patches {
	return p.ApplyPrivateMethod(
		&blockchain{},
		"context",
		func(*blockchain, context.Context, bool) (context.Context, error) {
			return ctx, err
		},
	)
}

func patchBlockchain_contextWithBlock(p *gomonkey.Patches, ctx context.Context) *gomonkey.Patches {
	return p.ApplyPrivateMethod(
		&blockchain{},
		"contextWithBlock",
		func(*blockchain, context.Context) context.Context {
			return ctx
		},
	)
}

func patchBlockchain_commitBlock(p *gomonkey.Patches, err error) *gomonkey.Patches {
	return p.ApplyPrivateMethod(
		&blockchain{},
		"commitBlock",
		func(*blockchain, *block.Block) error {
			return err
		},
	)
}

func patchBlockchain_emitToSubscribers(p *gomonkey.Patches) *gomonkey.Patches {
	return p.ApplyPrivateMethod(
		&blockchain{},
		"emitToSubscribers",
		func(*blockchain, *block.Block) {},
	)
}

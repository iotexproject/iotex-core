package consensus

import (
	"github.com/facebookgo/clock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"

	rp "github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/db"
)

type (

	// BuilderConfig returns the configuration of the builder
	BuilderConfig struct {
		Scheme             string
		DB                 db.Config
		Chain              blockchain.Config
		ActPool            actpool.Config
		Consensus          Config
		DardanellesUpgrade consensusfsm.DardanellesUpgrade
		BlockSync          blocksync.Config
		Genesis            genesis.Genesis
		SystemActive       bool
	}

	// Builder is the builder for rollDPoS
	Builder struct {
		cfg BuilderConfig
		// TODO: we should use keystore in the future
		encodedAddr       string
		priKey            crypto.PrivateKey
		chain             rolldpos.ChainManager
		blockDeserializer *block.Deserializer
		broadcastHandler  scheme.Broadcast
		clock             clock.Clock
		// TODO: explorer dependency deleted at #1085, need to add api params
		rp                   *rp.Protocol
		delegatesByEpochFunc rolldpos.DelegatesByEpochFunc
	}
)

// NewRollDPoSBuilder instantiates a Builder instance
func NewRollDPoSBuilder() *Builder {
	return &Builder{}
}

// SetConfig sets config
func (b *Builder) SetConfig(cfg BuilderConfig) *Builder {
	b.cfg = cfg
	return b
}

// SetAddr sets the address and key pair for signature
func (b *Builder) SetAddr(encodedAddr string) *Builder {
	b.encodedAddr = encodedAddr
	return b
}

// SetPriKey sets the private key
func (b *Builder) SetPriKey(priKey crypto.PrivateKey) *Builder {
	b.priKey = priKey
	return b
}

// SetChainManager sets the blockchain APIs
func (b *Builder) SetChainManager(chain rolldpos.ChainManager) *Builder {
	b.chain = chain
	return b
}

// SetBlockDeserializer set block deserializer
func (b *Builder) SetBlockDeserializer(deserializer *block.Deserializer) *Builder {
	b.blockDeserializer = deserializer
	return b
}

// SetBroadcast sets the broadcast callback
func (b *Builder) SetBroadcast(broadcastHandler scheme.Broadcast) *Builder {
	b.broadcastHandler = broadcastHandler
	return b
}

// SetClock sets the clock
func (b *Builder) SetClock(clock clock.Clock) *Builder {
	b.clock = clock
	return b
}

// SetDelegatesByEpochFunc sets delegatesByEpochFunc
func (b *Builder) SetDelegatesByEpochFunc(
	delegatesByEpochFunc rolldpos.DelegatesByEpochFunc,
) *Builder {
	b.delegatesByEpochFunc = delegatesByEpochFunc
	return b
}

// RegisterProtocol sets the rolldpos protocol
func (b *Builder) RegisterProtocol(rp *rp.Protocol) *Builder {
	b.rp = rp
	return b
}

// Build builds a rollDPoS consensus module
func (b *Builder) Build() (scheme.Scheme, error) {
	if b.chain == nil {
		return nil, errors.Wrap(rolldpos.ErrNewRollDPoS, "blockchain APIs is nil")
	}
	if b.broadcastHandler == nil {
		return nil, errors.Wrap(rolldpos.ErrNewRollDPoS, "broadcast callback is nil")
	}
	if b.clock == nil {
		b.clock = clock.New()
	}
	b.cfg.DB.DbPath = b.cfg.Consensus.RollDPoS.ConsensusDBPath
	ctx, err := rolldpos.NewRollDPoSCtx(
		consensusfsm.NewConsensusConfig(b.cfg.Consensus.RollDPoS.FSM, b.cfg.DardanellesUpgrade, b.cfg.Genesis, b.cfg.Consensus.RollDPoS.Delay),
		b.cfg.DB,
		b.cfg.SystemActive,
		b.cfg.Consensus.RollDPoS.ToleratedOvertime,
		b.cfg.Genesis.TimeBasedRotation,
		b.chain,
		b.blockDeserializer,
		b.rp,
		b.broadcastHandler,
		b.delegatesByEpochFunc,
		b.encodedAddr,
		b.priKey,
		b.clock,
		b.cfg.Genesis.BeringBlockHeight,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error when constructing consensus context")
	}
	cfsm, err := consensusfsm.NewConsensusFSM(ctx, b.clock)
	if err != nil {
		return nil, errors.Wrap(err, "error when constructing the consensus FSM")
	}
	return rolldpos.NewRollDPoS(cfsm, ctx, b.cfg.Consensus.RollDPoS.Delay), nil
}

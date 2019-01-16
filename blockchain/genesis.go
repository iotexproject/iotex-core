// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"io/ioutil"
	"math/big"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/state/factory"
)

const testnetActionPath = "testnet_actions.yaml"

// Genesis defines the Genesis default settings
type Genesis struct {
	TotalSupply         *big.Int
	BlockReward         *big.Int
	Timestamp           int64
	ParentHash          hash.Hash32B
	GenesisCoinbaseData string
	CreatorPubKey       string
	CreatorPrivKey      string
}

// GenesisAction is the root action struct, each package's action should be put as its sub struct
type GenesisAction struct {
	Creation       Creator     `yaml:"creator"`
	SelfNominators []Nominator `yaml:"selfNominators"`
	Transfers      []Transfer  `yaml:"transfers"`
	SubChains      []SubChain  `yaml:"subChains"`
}

// Creator is the Creator of the genesis block
type Creator struct {
	PubKey string `yaml:"pubKey"`
	PriKey string `yaml:"priKey"`
}

// Nominator is the Nominator struct for vote struct
type Nominator struct {
	PubKey string `yaml:"pubKey"`
	PriKey string `yaml:"priKey"`
}

// Transfer is the Transfer struct
type Transfer struct {
	Amount      int64  `yaml:"amount"`
	RecipientPK string `yaml:"recipientPK"`
}

// SubChain is the SubChain struct
type SubChain struct {
	ChainID            uint32 `yaml:"chainID"`
	SecurityDeposit    int64  `yaml:"securityDeposit"`
	OperationDeposit   int64  `yaml:"operationDeposit"`
	StartHeight        uint64 `yaml:"startHeight"`
	ParentHeightOffset uint64 `yaml:"parentHeightOffset"`
}

// Gen hardcodes genesis default settings
var Gen = &Genesis{
	TotalSupply:         ConvertIotxToRau(10000000000),
	BlockReward:         ConvertIotxToRau(5),
	Timestamp:           1524676419,
	ParentHash:          hash.Hash32B{},
	GenesisCoinbaseData: "Connecting the physical world, block by block",
}

// CreatorAddr returns the creator address on a particular chain
func (g *Genesis) CreatorAddr(chainID uint32) string {
	pk, _ := decodeKey(g.CreatorPubKey, "")
	return generateAddr(chainID, pk)
}

// CreatorPKHash returns the creator public key hash
func (g *Genesis) CreatorPKHash() hash.PKHash {
	pk, _ := decodeKey(g.CreatorPubKey, "")
	return keypair.HashPubKey(pk)
}

// NewGenesisActions creates a new genesis block
func NewGenesisActions(chainCfg config.Chain, ws factory.WorkingSet) []action.SealedEnvelope {
	actions := loadGenesisData(chainCfg)
	// add initial allocation
	alloc := big.NewInt(0)
	for _, transfer := range actions.Transfers {
		rpk, _ := decodeKey(transfer.RecipientPK, "")
		recipientAddr := generateAddr(chainCfg.ID, rpk)
		amount := ConvertIotxToRau(transfer.Amount)
		_, err := account.LoadOrCreateAccount(ws, recipientAddr, amount)
		if err != nil {
			log.L().Panic("Failed to add initial allocation.", zap.Error(err))
		}
		alloc.Add(alloc, amount)
	}
	// add creator
	Gen.CreatorPubKey = actions.Creation.PubKey
	creatorAddr := Gen.CreatorAddr(chainCfg.ID)
	_, err := account.LoadOrCreateAccount(ws, creatorAddr, alloc.Sub(Gen.TotalSupply, alloc))
	if err != nil {
		log.L().Panic("Failed to add creator.", zap.Error(err))
	}

	// TODO: convert vote to state operation as well
	acts := make([]action.SealedEnvelope, 0)
	for _, nominator := range actions.SelfNominators {
		pk, _ := decodeKey(nominator.PubKey, "")
		address := generateAddr(chainCfg.ID, pk)
		vote, err := action.NewVote(
			0,
			address,
			address,
			0,
			big.NewInt(0),
		)
		if err != nil {
			log.L().Panic("Fail to create the new vote action.", zap.Error(err))
		}
		bd := action.EnvelopeBuilder{}
		elp := bd.SetDestinationAddress(address).
			SetAction(vote).Build()
		selp := action.FakeSeal(elp, address, pk)
		acts = append(acts, selp)
	}

	// TODO: decouple start sub-chain from genesis block
	if chainCfg.EnableSubChainStartInGenesis {
		for _, sc := range actions.SubChains {
			start := action.NewStartSubChain(
				0,
				sc.ChainID,
				creatorAddr,
				ConvertIotxToRau(sc.SecurityDeposit),
				ConvertIotxToRau(sc.OperationDeposit),
				sc.StartHeight,
				sc.ParentHeightOffset,
				0,
				big.NewInt(0),
			)
			bd := action.EnvelopeBuilder{}
			elp := bd.SetAction(start).Build()
			pk, _ := decodeKey(Gen.CreatorPubKey, "")
			selp := action.FakeSeal(elp, creatorAddr, pk)
			acts = append(acts, selp)
		}
	}

	return acts
}

// decodeKey decodes the string keypair
func decodeKey(pubK string, priK string) (pk keypair.PublicKey, sk keypair.PrivateKey) {
	if len(pubK) > 0 {
		publicKey, err := keypair.DecodePublicKey(pubK)
		if err != nil {
			log.L().Panic("Fail to decode public key.", zap.Error(err))
		}
		pk = publicKey
	}
	if len(priK) > 0 {
		privateKey, err := keypair.DecodePrivateKey(priK)
		if err != nil {
			log.L().Panic("Fail to decode private key.", zap.Error(err))
		}
		sk = privateKey
	}
	return
}

// generateAddr returns the string address according to public key
func generateAddr(chainID uint32, pk keypair.PublicKey) string {
	pkHash := keypair.HashPubKey(pk)
	return address.New(chainID, pkHash[:]).Bech32()
}

// loadGenesisData loads data of creator and actions contained in genesis block
func loadGenesisData(chainCfg config.Chain) GenesisAction {
	var filePath string
	if chainCfg.GenesisActionsPath != "" {
		filePath = chainCfg.GenesisActionsPath
	} else {
		filePath = fileutil.GetFileAbsPath(testnetActionPath)
	}

	actionsBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.L().Panic("Fail to load genesis data.", zap.Error(err))
	}
	actions := GenesisAction{}
	if err := yaml.Unmarshal(actionsBytes, &actions); err != nil {
		log.L().Panic("Fail to unmarshal genesis data.", zap.Error(err))
	}
	return actions
}

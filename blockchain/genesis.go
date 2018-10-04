// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"io/ioutil"
	"math/big"

	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/pkg/version"
)

const testnetActionPath = "testnet_actions.yaml"
const testActionPath = "test_actions.yaml"

// Genesis defines the Genesis default settings
type Genesis struct {
	TotalSupply         uint64
	BlockReward         uint64
	Timestamp           uint64
	ParentHash          hash.Hash32B
	GenesisCoinbaseData string
	CreatorAddr         string
	CreatorPubKey       string
}

// GenesisAction is the root action struct, each package's action should be put as its sub struct
type GenesisAction struct {
	SelfNominators []Nominator `yaml:"selfNominators"`
	Transfers      []Transfer  `yaml:"transfers"`
}

// Nominator is the Nominator struct for vote struct
type Nominator struct {
	PubKey string `yaml:"pubKey"`
	PriKey string `yaml:"priKey"`
}

// Transfer is the Transfer struct
type Transfer struct {
	Amount       int64  `yaml:"amount"`
	SenderPri    string `yaml:"senderPri"`
	SenderPub    string `yaml:"senderPub"`
	RecipientPub string `yaml:"recipientPub"`
}

// Gen hardcodes genesis default settings
var Gen = &Genesis{
	TotalSupply:         uint64(10000000000),
	BlockReward:         uint64(5),
	Timestamp:           uint64(1524676419),
	ParentHash:          hash.Hash32B{},
	GenesisCoinbaseData: "Connecting the physical world, block by block",
	CreatorAddr:         "io1qyqsyqcy222ggazmccgf7dsx9m9vfqtadw82ygwhjnxtmx",
	CreatorPubKey:       "d01164c3afe47406728d3e17861a3251dcff39e62bdc2b93ccb69a02785a175e195b5605517fd647eb7dd095b3d862dffb087f35eacf10c6859d04a100dbfb7358eeca9d5c37c904",
}

// NewGenesisBlock creates a new genesis block
func NewGenesisBlock(cfg *config.Config) *Block {
	var filePath string
	if cfg != nil && cfg.Chain.GenesisActionsPath != "" {
		filePath = cfg.Chain.GenesisActionsPath
	} else {
		filePath = fileutil.GetFileAbsPath(testActionPath)
	}

	actionsBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create genesis block")
	}
	actions := GenesisAction{}
	if err := yaml.Unmarshal(actionsBytes, &actions); err != nil {
		logger.Fatal().Err(err).Msg("Fail to create genesis block")
	}
	votes := []*action.Vote{}
	for _, nominator := range actions.SelfNominators {

		prik, err := keypair.DecodePrivateKey(nominator.PriKey)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to read private key of nominator")
		}

		pubk, err := keypair.DecodePublicKey(nominator.PubKey)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to read public key of nominator")
		}
		pkHash := keypair.HashPubKey(pubk)
		address := address.New(cfg.Chain.ID, pkHash[:])

		vote, err := action.NewVote(
			0,
			address.IotxAddress(),
			address.IotxAddress(),
			0,
			big.NewInt(0),
		)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to create the new vote action")
		}

		if err := action.Sign(vote, prik); err != nil {
			logger.Fatal().Err(err).Msg("Fail to sign the new vote action")
		}

		vote.SetVoterPublicKey(pubk)
		votes = append(votes, vote)
	}

	transfers := []*action.Transfer{}
	for _, transfer := range actions.Transfers {

		recipientPubk, err := keypair.DecodePublicKey(transfer.RecipientPub)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to read public key of the recipient")
		}
		rpkHash := keypair.HashPubKey(recipientPubk)
		recipientAddr := address.New(cfg.Chain.ID, rpkHash[:])

		senderPrik, err := keypair.DecodePrivateKey(transfer.SenderPri)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to read private key of the sender")
		}

		senderPubk, err := keypair.DecodePublicKey(transfer.SenderPub)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to read public key of the sender")
		}
		spkHash := keypair.HashPubKey(senderPubk)
		senderAddr := address.New(cfg.Chain.ID, spkHash[:])

		tsf, err := action.NewTransfer(
			0,
			big.NewInt(transfer.Amount),
			senderAddr.IotxAddress(),
			recipientAddr.IotxAddress(),
			[]byte{},
			0,
			big.NewInt(0),
		)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to create the new transfer action")
		}

		if err := action.Sign(tsf, senderPrik); err != nil {
			logger.Fatal().Err(err).Msg("Fail to sign the new transfer action")
		}

		tsf.SetSenderPublicKey(senderPubk)
		transfers = append(transfers, tsf)
	}

	block := &Block{
		Header: &BlockHeader{
			version:       version.ProtocolVersion,
			chainID:       cfg.Chain.ID,
			height:        uint64(0),
			timestamp:     Gen.Timestamp,
			prevBlockHash: Gen.ParentHash,
			txRoot:        hash.ZeroHash32B,
			stateRoot:     hash.ZeroHash32B,
			blockSig:      []byte{},
		},
		Transfers: transfers,
		Votes:     votes,
	}

	block.Header.txRoot = block.TxRoot()
	return block
}

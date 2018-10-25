// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a testing tool to inject fake actions to the blockchain
// To use, run "make build" and " ./bin/actioninjector"

package main

import (
	"flag"
	_ "go.uber.org/automaxprocs"
	"golang.org/x/net/context"

	"fmt"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/rds"
	"github.com/iotexproject/iotex-core/indexservice"
	"github.com/iotexproject/iotex-core/logger"
	"os"
)

const (
	adminNumber = 2
)

func main() {
	// node address for this sync
	var nodeAddress string
	// start block id of the index build
	var fromBlockId int
	// end block id of the index build
	var toBlockId int
	// end point of rds
	var awsRDSEndpoint string
	// port of rds
	var awsRDSPort uint64
	// user of rds
	var awsRDSUser string
	// pass of rds
	var awsPass string
	// db name of rds
	var awsDBName string
	// target address for jrpc connection. Default is "127.0.0.1:14004"
	var explorerAddr string

	flag.StringVar(&nodeAddress, "node-address", "", "node address used in RDS")
	flag.IntVar(&fromBlockId, "from-block-id", 0, "sync from which block id")
	flag.IntVar(&toBlockId, "to-block-id", 0, "sync to which block id")
	flag.StringVar(&awsRDSEndpoint, "aws-rds-endpoint", "", "aws rds endpoint")
	flag.Uint64Var(&awsRDSPort, "aws-rds-port", 0, "aws rds port")
	flag.StringVar(&awsRDSUser, "aws-rds-user", "", "aws rds user")
	flag.StringVar(&awsPass, "aws-pass", "", "aws pass")
	flag.StringVar(&awsDBName, "aws-db-name", "", "aws db name")
	flag.StringVar(&explorerAddr, "explorer-addr", "127.0.0.1:14004", "target ip:port for jrpc connection")
	flag.Parse()

	cfg := config.Default
	cfg.Indexer.Enabled = true

	cfg.DB.RDS.AwsDBName = awsDBName
	cfg.DB.RDS.AwsRDSEndpoint = awsRDSEndpoint
	cfg.DB.RDS.AwsRDSUser = awsRDSUser
	cfg.DB.RDS.AwsPass = awsPass
	cfg.DB.RDS.AwsRDSPort = awsRDSPort
	rds := rds.NewAwsRDS(&cfg.DB.RDS)
	if err := rds.Start(context.Background()); err != nil {
		logger.Fatal().Err(err).Msg("error when start rds store")
	}

	idx := indexservice.Indexer{cfg.Indexer, rds, nodeAddress}
	var chainOpts []blockchain.Option
	chainOpts = []blockchain.Option{blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption()}
	// create Blockchain
	chain := blockchain.NewBlockchain(&cfg, chainOpts...)
	if chain == nil && cfg.Chain.EnableFallBackToFreshDB {
		logger.Warn().Msg("Chain db and trie db are falling back to fresh ones")
		if err := os.Rename(cfg.Chain.ChainDBPath, cfg.Chain.ChainDBPath+".old"); err != nil {
			logger.Fatal().Err(err).Msg("failed to rename old chain db")
		}
		if err := os.Rename(cfg.Chain.TrieDBPath, cfg.Chain.TrieDBPath+".old"); err != nil {
			logger.Fatal().Err(err).Msg("failed to rename old trie db")
		}
		chain = blockchain.NewBlockchain(&cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())
	}

	for i := fromBlockId; i <= toBlockId; i++ {
		blk, err := chain.GetBlockByHeight(uint64(i))
		if err != nil {
			logger.Fatal().Err(err).Msg(fmt.Sprintf("failed to get block height <%s>", i))
		}
		err = idx.BuildIndex(blk)
		if err != nil {
			logger.Fatal().Err(err).Msg(fmt.Sprintf("failed to build index for block height <%s>", i))
		}
	}
}

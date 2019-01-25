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
	"sync"
	"time"

	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/tools/util"
)

const (
	adminNumber = 2
)

func main() {
	// path of config file containing all the public/private key paris of addresses getting transfers from Creator in genesis block
	var configPath string
	// chain ID. Default is 1
	var chainID int
	// target address for jrpc connection. Default is "127.0.0.1:14004"
	var addr string
	// number of transfer injections. Default is 50
	var transferNum int
	// transfer gas limit. Default is 1000000
	var transferGasLimit int
	// transfer gas price. Default is 10
	var transferGasPrice int
	// transfer payload. Default is ""
	var transferPayload string
	// number of vote injections. Default is 50
	var voteNum int
	// vote gas limit. Default is 1000000
	var voteGasLimit int
	// vote gas price. Default is 10
	var voteGasPrice int
	// number of execution injections. Default is 50
	var executionNum int
	// smart contract address. Default is "io1qxxmp4gy39mjrgkvfpje6aqlwc77x8f4vu5kl9k6"
	var contract string
	// execution amount. Default is 0
	var executionAmount int
	// execution gas limit. Default is 1200000
	var executionGasLimit int
	// execution gas price. Default is 10
	var executionGasPrice int
	// execution data. Default is "2885ad2c"
	var executionData string
	// sleeping period between every two consecutive action injections in seconds. Default is 5
	var interval int
	// maximum number of rpc retries. Default is 5
	var retryNum int
	// sleeping period between two consecutive rpc retries in seconds. Default is 1
	var retryInterval int
	// aps indicates how many actions to be injected in one second. Default is 0
	var aps float64
	// duration indicates how long the injection will run in seconds. Default is 60
	var duration int
	// reset interval indicates the interval to reset nonce counter in seconds. Default is 10
	var resetInterval int

	flag.StringVar(&configPath, "injector-config-path", "./tools/actioninjector/gentsfaddrs.yaml", "path of config file of genesis transfer addresses")
	flag.IntVar(&chainID, "chain", 1, "id of target chain")
	flag.StringVar(&addr, "addr", "127.0.0.1:14004", "target ip:port for jrpc connection")
	flag.IntVar(&transferNum, "transfer-num", 50, "number of transfer injections")
	flag.IntVar(&transferGasLimit, "transfer-gas-limit", 20000, "transfer gas limit")
	flag.IntVar(&transferGasPrice, "transfer-gas-price", 10, "transfer gas price")
	flag.StringVar(&transferPayload, "transfer-payload", "", "transfer payload")
	flag.IntVar(&voteNum, "vote-num", 50, "number of vote injections")
	flag.IntVar(&voteGasLimit, "vote-gas-limit", 20000, "vote gas limit")
	flag.IntVar(&voteGasPrice, "vote-gas-price", 10, "vote gas price")
	flag.IntVar(&executionNum, "execution-num", 50, "number of execution injections")
	flag.StringVar(&contract, "contract", "io1qy8w2uj6qmvfgcy6dgrv24qc5qp26dfp5vx427vk", "smart contract address")
	flag.IntVar(&executionAmount, "execution-amount", 50, "execution amount")
	flag.IntVar(&executionGasLimit, "execution-gas-limit", 20000, "execution gas limit")
	flag.IntVar(&executionGasPrice, "execution-gas-price", 10, "execution gas price")
	flag.StringVar(&executionData, "execution-data", "2885ad2c", "execution data")
	flag.IntVar(&interval, "interval", 5, "sleep interval between two consecutively injected actions in seconds")
	flag.IntVar(&retryNum, "retry-num", 5, "maximum number of rpc retries")
	flag.IntVar(&retryInterval, "retry-interval", 1, "sleep interval between two consecutive rpc retries in seconds")
	flag.Float64Var(&aps, "aps", 0, "actions to be injected per second")
	flag.IntVar(&duration, "duration", 60, "duration when the injection will run in seconds")
	flag.IntVar(&resetInterval, "reset-interval", 10, "time interval to reset nonce counter in seconds")
	flag.Parse()

	proxy := explorer.NewExplorerProxy("http://" + addr)

	addrKeys, err := util.LoadAddresses(configPath, uint32(chainID))
	if err != nil {
		log.L().Fatal("Failed to load addresses from config path", zap.Error(err))
	}
	admins := addrKeys[len(addrKeys)-adminNumber:]
	delegates := addrKeys[:len(addrKeys)-adminNumber]

	counter, err := util.InitCounter(proxy, addrKeys)
	if err != nil {
		log.L().Fatal("Failed to initialize nonce counter", zap.Error(err))
	}

	// APS Mode
	if aps > 0 {
		d := time.Duration(duration) * time.Second
		wg := &sync.WaitGroup{}
		util.InjectByAps(wg, aps, counter, transferGasLimit, transferGasPrice, transferPayload, voteGasLimit, voteGasPrice,
			contract, executionAmount, executionGasLimit, executionGasPrice, executionData, proxy, admins, delegates, d,
			retryNum, retryInterval, resetInterval)
		wg.Wait()
	} else {
		util.InjectByInterval(transferNum, transferGasLimit, transferGasPrice, transferPayload, voteNum, voteGasLimit,
			voteGasPrice, executionNum, contract, executionAmount, executionGasLimit, executionGasPrice, executionData,
			interval, counter, proxy, admins, delegates, retryNum, retryInterval)
	}
}

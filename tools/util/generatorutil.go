// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/iotexproject/go-pkgs/cache/ttl"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/tools/executiontester/blockchain"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"go.uber.org/zap"
)

// InjectByAps injects Actions in APS Mode
func TxGenerator(
	txTotal uint64,
	counter map[string]uint64,
	transferGasLimit int,
	transferGasPrice int64,
	transferPayload string,
	voteGasLimit int,
	voteGasPrice int64,
	contract string,
	executionAmount int,
	executionGasLimit int,
	executionGasPrice int64,
	executionData string,
	fpToken blockchain.FpToken,
	fpContract string,
	debtor *AddressKey,
	creditor *AddressKey,
	client iotexapi.APIServiceClient,
	admins []*AddressKey,
	delegates []*AddressKey,
	duration time.Duration,
	retryNum int,
	retryInterval int,
	resetInterval int,
	expectedBalances *map[string]*big.Int,
	cs *chainservice.ChainService,
	pendingActionMap *ttl.Cache,
) {

	// load the nonce and balance of addr
	for _, admin := range admins {
		addr := admin.EncodedAddr
		err := backoff.Retry(func() error {
			acctDetails, err := client.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: addr})
			if err != nil {
				return err
			}
			counter[addr] = acctDetails.GetAccountMeta().PendingNonce
			return nil
		}, backoff.NewExponentialBackOff())
		if err != nil {
			log.L().Fatal("Failed to inject actions by APS",
				zap.Error(err),
				zap.String("addr", admin.EncodedAddr))
		}
	}
	for _, delegate := range delegates {
		addr := delegate.EncodedAddr
		err := backoff.Retry(func() error {
			acctDetails, err := client.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: addr})
			if err != nil {
				return err
			}
			counter[addr] = acctDetails.GetAccountMeta().PendingNonce
			return nil
		}, backoff.NewExponentialBackOff())
		if err != nil {
			log.L().Fatal("Failed to inject actions by APS",
				zap.Error(err),
				zap.String("addr", delegate.EncodedAddr))
		}
	}

	// TODO: load balance

	var wg sync.WaitGroup
	for i := 0; i < int(txTotal); i++ {
		wg.Add(1)

	}
	wg.Wait()
	// generate txs
	sender, recipient, nonce, amount := createTransferInjection(counter, delegates)
	// atomic.AddUint64(&totalTsfCreated, 1)

	selp, _, err := createSignedTransfer(sender, recipient, unit.ConvertIotxToRau(amount), nonce, gasLimit,
		gasPrice, payload)
	if err != nil {
		log.L().Fatal("Failed to inject transfer", zap.Error(err))
	}

}

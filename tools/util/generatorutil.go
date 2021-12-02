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
	"sync/atomic"

	"github.com/cenkalti/backoff"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type accountManager struct {
	mu           sync.RWMutex
	pendingNonce map[string]uint64
}

func NewAccountManager() *accountManager {
	return &accountManager{
		pendingNonce: make(map[string]uint64),
	}
}

func (ac *accountManager) getAndInc(addr string) uint64 {
	var ret uint64
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ret = ac.pendingNonce[addr]
	ac.pendingNonce[addr]++
	return ret
}

func (ac *accountManager) set(addr string, val uint64) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.pendingNonce[addr] = val
}

// InjectByAps injects Actions in APS Mode
func TxGenerator(
	txTotal uint64,
	client iotexapi.APIServiceClient,
	delegates []*AddressKey,
	gasLimit uint64,
	gasPrice *big.Int,
	actionType int,
) ([]action.SealedEnvelope, error) {

	accountManager := NewAccountManager()

	// load the nonce and balance of addr
	for _, delegate := range delegates {
		addr := delegate.EncodedAddr
		err := backoff.Retry(func() error {
			acctDetails, err := client.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: addr})
			if err != nil {
				return err
			}
			accountManager.set(addr, acctDetails.GetAccountMeta().PendingNonce)
			return nil
		}, backoff.NewExponentialBackOff())
		if err != nil {
			log.L().Fatal("Failed to inject actions by APS",
				zap.Error(err),
				zap.String("addr", delegate.EncodedAddr))
			return nil, err
		}
	}

	// TODO: load balance
	ret := make([]action.SealedEnvelope, txTotal)
	var wg sync.WaitGroup
	var txGenerated uint64
	for i := 0; i < int(txTotal); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			randNum := i % len(delegates)
			sender, recipient := delegates[randNum], delegates[(randNum+1)%len(delegates)]
			// amount := int64(rand.Intn(5))
			amount := int64(0)
			nonce := accountManager.getAndInc(sender.EncodedAddr)

			actionType = 1
			var (
				selp action.SealedEnvelope
				err  error
			)
			switch actionType {
			case 1:
				selp, _, err = createSignedTransfer(sender, recipient, unit.ConvertIotxToRau(amount), nonce, gasLimit,
					gasPrice, "")
				// case 2:
				// 	selp, _, err = createSignedExecution(sender, recipient, nonce, unit.ConvertIotxToRau(amount), gasLimit,
				// 		gasPrice, "")
			}
			if err != nil {
				log.L().Fatal("Failed to inject transfer", zap.Error(err))
			}

			ret[i] = selp
			atomic.AddUint64(&txGenerated, 1)
			// TODO: collect balance info
		}(i)

	}
	wg.Wait()
	if txGenerated != txTotal {
		return nil, errors.New("gen failed")
	}
	return ret, nil
}

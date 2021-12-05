// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"context"
	"math/big"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/cenkalti/backoff"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type nounceManager struct {
	mu           sync.RWMutex
	pendingNonce map[string]uint64
}

type AccountManager struct {
	AccountList []*AddressKey
	nonceMng    nounceManager
}

func NewAccountManager(accounts []*AddressKey) *AccountManager {
	return &AccountManager{
		AccountList: accounts,
		nonceMng: nounceManager{
			pendingNonce: make(map[string]uint64),
		},
	}
}

func (ac *AccountManager) Get(addr string) uint64 {
	ac.nonceMng.mu.RLock()
	defer ac.nonceMng.mu.RUnlock()
	return ac.nonceMng.pendingNonce[addr]
}

func (ac *AccountManager) GetAndInc(addr string) uint64 {
	var ret uint64
	ac.nonceMng.mu.Lock()
	defer ac.nonceMng.mu.Unlock()
	ret = ac.nonceMng.pendingNonce[addr]
	ac.nonceMng.pendingNonce[addr]++
	return ret
}

func (ac *AccountManager) GetAllAddr() []string {
	var ret []string
	for _, v := range ac.AccountList {
		ret = append(ret, v.EncodedAddr)
	}
	return ret
}

func (ac *AccountManager) Set(addr string, val uint64) {
	ac.nonceMng.mu.Lock()
	defer ac.nonceMng.mu.Unlock()
	ac.nonceMng.pendingNonce[addr] = val
}

func (ac *AccountManager) UpdateNonce(client iotexapi.APIServiceClient) error {
	// load the nonce and balance of addr
	for _, account := range ac.AccountList {
		addr := account.EncodedAddr
		err := backoff.Retry(func() error {
			acctDetails, err := client.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: addr})
			if err != nil {
				return err
			}
			ac.Set(addr, acctDetails.GetAccountMeta().PendingNonce)
			return nil
		}, backoff.NewExponentialBackOff())
		if err != nil {
			log.L().Fatal("Failed to inject actions by APS",
				zap.Error(err),
				zap.String("addr", account.EncodedAddr))
			return err
		}
	}
	return nil
}

// InjectByAps injects Actions in APS Mode
func TxGenerator(
	txTotal uint64,
	client iotexapi.APIServiceClient,
	delegates []*AddressKey,
	gasLimit uint64,
	gasPrice *big.Int,
	actionType int,
	contractAddr string,
) ([]action.SealedEnvelope, error) {

	accountManager := NewAccountManager(delegates)
	err := accountManager.UpdateNonce(client)
	if err != nil {
		panic(err)
	}

	ret := make([]action.SealedEnvelope, txTotal)
	var wg sync.WaitGroup
	var txGenerated uint64
	for i := 0; i < int(txTotal); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			selp, err := ActionGenerator(actionType, accountManager, gasLimit, gasPrice, contractAddr, "")
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

func ActionGenerator(
	actionType int,
	accountManager *AccountManager,
	gasLimit uint64,
	gasPrice *big.Int,
	contractAddr string,
	payLoad string,
) (action.SealedEnvelope, error) {
	var (
		selp      action.SealedEnvelope
		err       error
		delegates = accountManager.AccountList
		randNum   = rand.Intn(len(delegates))
		sender    = delegates[randNum]
		recipient = delegates[(randNum+1)%len(delegates)]
		nonce     = accountManager.GetAndInc(sender.EncodedAddr)
	)
	switch actionType {
	case 1:
		selp, _, err = createSignedTransfer(sender, recipient, big.NewInt(0), nonce, gasLimit, gasPrice, payLoad)
	case 2:
		selp, _, err = createSignedExecution(sender, contractAddr, nonce, big.NewInt(0), gasLimit, gasPrice, payLoad)
	}
	return selp, err
}

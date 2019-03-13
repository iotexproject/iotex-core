// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
)

func (p *Protocol) handleStartSubChain(ctx context.Context, start *action.StartSubChain, sm protocol.StateManager) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	account, subChainsInOp, err := p.validateStartSubChain(raCtx.Caller, start, sm)
	if err != nil {
		return err
	}
	return p.mutateSubChainState(raCtx.Caller, start, account, subChainsInOp, sm)
}

func (p *Protocol) validateStartSubChain(
	caller address.Address,
	start *action.StartSubChain,
	sm protocol.StateManager,
) (*state.Account, SubChainsInOperation, error) {

	if start.ChainID() == p.rootChain.ChainID() {
		return nil, nil, errors.Errorf("%d is used by main chain", start.ChainID())
	}
	subChainsInOp, err := p.subChainsInOperation(sm)
	if err != nil {
		return nil, nil, err
	}
	if _, ok := subChainsInOp.Get(start.ChainID()); ok {
		return nil, nil, errors.Errorf("%d is used by another sub-chain", start.ChainID())
	}
	if start.SecurityDeposit().Cmp(MinSecurityDeposit) < 0 {
		return nil, nil, errors.Errorf(
			"security deposit is smaller than the minimal requirement %s",
			MinSecurityDeposit.String(),
		)
	}
	account, err := p.accountWithEnoughBalance(
		caller.String(),
		big.NewInt(0).Add(start.SecurityDeposit(), start.OperationDeposit()),
		sm,
	)
	if err != nil {
		return nil, nil, err
	}
	return account, subChainsInOp, nil
}

func (p *Protocol) mutateSubChainState(
	caller address.Address,
	start *action.StartSubChain,
	acct *state.Account,
	subChainsInOp SubChainsInOperation,
	sm protocol.StateManager,
) error {
	addr, err := createSubChainAddress(caller.String(), start.Nonce())
	if err != nil {
		return err
	}
	sc := SubChain{
		ChainID:            start.ChainID(),
		SecurityDeposit:    start.SecurityDeposit(),
		OperationDeposit:   start.OperationDeposit(),
		StartHeight:        start.StartHeight(),
		ParentHeightOffset: start.ParentHeightOffset(),
		OwnerPublicKey:     start.OwnerPublicKey(),
		CurrentHeight:      0,
		DepositCount:       0,
	}
	if err := sm.PutState(addr, &sc); err != nil {
		return errors.Wrap(err, "error when putting sub-chain state")
	}
	acct.Balance = big.NewInt(0).Sub(acct.Balance, start.SecurityDeposit())
	acct.Balance = big.NewInt(0).Sub(acct.Balance, start.OperationDeposit())
	// TODO: this is not right, but currently the actions in a block is not processed according to the nonce
	accountutil.SetNonce(start, acct)
	if err := accountutil.StoreAccount(sm, caller.String(), acct); err != nil {
		return err
	}
	subChainsInOp = subChainsInOp.Append(InOperation{
		ID:   start.ChainID(),
		Addr: addr[:],
	})
	return sm.PutState(SubChainsInOperationKey, &subChainsInOp)
}

func createSubChainAddress(ownerAddr string, nonce uint64) (hash.Hash160, error) {
	addr, err := address.FromString(ownerAddr)
	if err != nil {
		return hash.ZeroHash160, errors.Wrapf(err, "cannot get the public key hash of address %s", ownerAddr)
	}
	bytes := make([]byte, 8)
	enc.MachineEndian.PutUint64(bytes, nonce)
	return hash.Hash160b(append(addr.Bytes(), bytes...)), nil
}

// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// SubChain returns the confirmed sub-chain state
func (p *Protocol) SubChain(addr address.Address) (*SubChain, error) {
	var subChain SubChain
	state, err := p.sf.State(byteutil.BytesTo20B(addr.Payload()), &subChain)
	if err != nil {
		return nil, errors.Wrapf(err, "error when loading state of %x", addr.Payload())
	}
	sc, ok := state.(*SubChain)
	if !ok {
		return nil, errors.New("error when casting state into sub-chain")
	}
	return sc, nil
}

// SubChainsInOperation returns the used chain IDs
func (p *Protocol) SubChainsInOperation() (state.SortedSlice, error) {
	var subChainsInOp state.SortedSlice
	s, err := p.sf.State(subChainsInOperationKey, &subChainsInOp)
	return processState(s, err)
}

func (p *Protocol) handleStartSubChain(start *action.StartSubChain, ws state.WorkingSet) error {
	account, subChainsInOp, err := p.validateStartSubChain(start, ws)
	if err != nil {
		return err
	}
	if err := p.mutateSubChainState(start, account, subChainsInOp, ws); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) validateStartSubChain(
	start *action.StartSubChain,
	ws state.WorkingSet,
) (*state.Account, state.SortedSlice, error) {
	if start.ChainID() == p.rootChain.ChainID() {
		return nil, nil, fmt.Errorf("%d is used by main chain", start.ChainID())
	}
	var subChainsInOp state.SortedSlice
	var err error
	if ws == nil {
		subChainsInOp, err = p.SubChainsInOperation()
	} else {
		subChainsInOp, err = cachedSubChainsInOperation(ws)
	}
	if err != nil {
		return nil, nil, errors.Wrap(err, "error when getting the state of used chain IDs")
	}
	if subChainsInOp.Exist(InOperation{ID: start.ChainID()}, SortInOperation) {
		return nil, nil, fmt.Errorf("%d is used by another sub-chain", start.ChainID())
	}
	var account *state.Account
	if ws == nil {
		account, err = p.sf.AccountState(start.OwnerAddress())
	} else {
		account, err = ws.CachedAccountState(start.OwnerAddress())
	}
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error when getting the state of address %s", start.OwnerAddress())
	}
	if start.SecurityDeposit().Cmp(MinSecurityDeposit) < 0 {
		return nil, nil, fmt.Errorf(
			"security deposit is smaller than the minimal requirement %s",
			MinSecurityDeposit.String(),
		)
	}
	if account.Balance.Cmp(start.SecurityDeposit()) < 0 {
		return nil, nil, errors.New("sub-chain owner doesn't have enough balance for security deposit")
	}
	if account.Balance.Cmp(big.NewInt(0).Add(start.SecurityDeposit(), start.OperationDeposit())) < 0 {
		return nil, nil, errors.New("sub-chain owner doesn't have enough balance for operation deposit")
	}
	return account, subChainsInOp, nil
}

func (p *Protocol) mutateSubChainState(
	start *action.StartSubChain,
	account *state.Account,
	subChainsInOp state.SortedSlice,
	ws state.WorkingSet,
) error {
	addr, err := createSubChainAddress(start.OwnerAddress(), start.Nonce())
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
	}
	if err := ws.PutState(addr, &sc); err != nil {
		return errors.Wrap(err, "error when putting sub-chain state")
	}
	account.Balance = big.NewInt(0).Sub(account.Balance, start.SecurityDeposit())
	account.Balance = big.NewInt(0).Sub(account.Balance, start.OperationDeposit())
	// TODO: this is not right, but currently the actions in a block is not processed according to the nonce
	if start.Nonce() > account.Nonce {
		account.Nonce = start.Nonce()
	}
	ownerPKHash, err := srcAddressPKHash(start.OwnerAddress())
	if err != nil {
		return err
	}
	if err := ws.PutState(ownerPKHash, account); err != nil {
		return err
	}
	subChainsInOp = subChainsInOp.Append(
		InOperation{
			ID:   start.ChainID(),
			Addr: address.New(p.rootChain.ChainID(), addr[:]).Bytes(),
		},
		SortInOperation,
	)
	if err := ws.PutState(subChainsInOperationKey, &subChainsInOp); err != nil {
		return err
	}
	return nil
}

func createSubChainAddress(ownerAddr string, nonce uint64) (hash.PKHash, error) {
	addr, err := address.IotxAddressToAddress(ownerAddr)
	if err != nil {
		return hash.ZeroPKHash, errors.Wrapf(err, "cannot get the public key hash of address %s", ownerAddr)
	}
	bytes := make([]byte, 8)
	enc.MachineEndian.PutUint64(bytes, nonce)
	return byteutil.BytesTo20B(hash.Hash160b(append(addr.Payload(), bytes...))), nil
}

func cachedSubChainsInOperation(ws state.WorkingSet) (state.SortedSlice, error) {
	var subChainsInOp state.SortedSlice
	s, err := ws.CachedState(subChainsInOperationKey, &subChainsInOp)
	return processState(s, err)
}

func processState(s state.State, err error) (state.SortedSlice, error) {
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return state.SortedSlice{}, nil
		}
		return nil, errors.Wrapf(err, "error when loading state of %x", subChainsInOperationKey)
	}
	uci, ok := s.(*state.SortedSlice)
	if !ok {
		return nil, errors.New("error when casting state into used chain IDs")
	}
	return *uci, nil
}

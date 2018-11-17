// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

func (p *Protocol) subChainToStop(subChainAddr string) (*SubChain, error) {
	iotxAddr, err := address.IotxAddressToAddress(subChainAddr)
	if err != nil {
		return nil, err
	}
	return p.SubChain(iotxAddr)
}

func (p *Protocol) validateSubChainOwnership(
	ownerPKHash hash.PKHash,
	sender string,
	ws factory.WorkingSet,
) (*state.Account, error) {
	account, err := p.account(sender, ws)
	if err != nil {
		return nil, err
	}
	senderPKHash, err := srcAddressPKHash(sender)
	if err != nil {
		return account, err
	}
	if !bytes.Equal(ownerPKHash[:], senderPKHash[:]) {
		return account, fmt.Errorf("sender %s is not the owner of sub-chain %x", sender, ownerPKHash)
	}
	return account, nil
}

func (p *Protocol) handleStopSubChain(stop *action.StopSubChain, ws factory.WorkingSet) error {
	stopHeight := stop.StopHeight()
	if stopHeight <= ws.Height() {
		return fmt.Errorf("stop height %d should not be lower than chain height %d", stopHeight, ws.Height())
	}
	subChainAddr := stop.ChainAddress()
	subChain, err := p.subChainToStop(subChainAddr)
	if err != nil {
		return errors.Wrapf(err, "error when processing address %s", subChainAddr)
	}
	subChain.StopHeight = stopHeight
	subChainPKHash, err := srcAddressPKHash(subChainAddr)
	if err != nil {
		return errors.Wrapf(err, "error when generating public key hash for address %s", subChainAddr)
	}
	if err := ws.PutState(subChainPKHash, subChain); err != nil {
		return err
	}
	account, err := p.validateSubChainOwnership(
		keypair.HashPubKey(subChain.OwnerPublicKey),
		stop.SrcAddr(),
		ws,
	)
	if err != nil {
		return errors.Wrapf(err, "error when getting the account of sender %s", stop.SrcAddr())
	}
	// TODO: this is not right, but currently the actions in a block is not processed according to the nonce
	if stop.Nonce() > account.Nonce {
		account.Nonce = stop.Nonce()
	}
	senderPKHash, err := srcAddressPKHash(stop.SrcAddr())
	if err != nil {
		return err
	}
	if err := ws.PutState(senderPKHash, account); err != nil {
		return err
	}
	// check that subchain is in register
	subChainsInOp, err := p.subChainsInOperation(ws)
	if err != nil {
		return errors.Wrap(err, "error when getting sub-chains in operation")
	}
	subChainsInOp, deleted := subChainsInOp.Delete(InOperation{ID: subChain.ChainID}, SortInOperation)
	if deleted <= 0 {
		return fmt.Errorf("address %s is not on a sub-chain in operation", subChainAddr)
	}
	if err := ws.PutState(SubChainsInOperationKey, &subChainsInOp); err != nil {
		return err
	}

	return nil
}

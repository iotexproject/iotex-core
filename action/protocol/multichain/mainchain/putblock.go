// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"fmt"
	"sort"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

func (p *Protocol) handlePutBlock(pb *action.PutBlock, sm protocol.StateManager) error {
	if err := p.validatePutBlock(pb, sm); err != nil {
		return err
	}
	proof := putBlockToBlockProof(pb)
	if err := sm.PutState(blockProofKey(proof.SubChainAddress, proof.Height), &proof); err != nil {
		return err
	}
	// Update the block producer's nonce
	addrHash, err := iotxaddress.AddressToPKHash(pb.ProducerAddress())
	if err != nil {
		return err
	}
	acct, err := account.LoadAccount(sm, addrHash)
	if err != nil {
		return err
	}

	protocol.SetNonce(pb, acct)
	return account.StoreAccount(sm, pb.ProducerAddress(), acct)
}

func (p *Protocol) validatePutBlock(pb *action.PutBlock, sm protocol.StateManager) error {
	// use owner address TODO

	// can only emit on one height
	if _, exist := p.getBlockProof(pb.SubChainAddress(), pb.Height()); exist {
		return fmt.Errorf("block %d already exists", pb.Height())
	}
	return nil
}

func (p *Protocol) getBlockProof(addr string, height uint64) (BlockProof, bool) {
	var bp BlockProof
	if err := p.sf.State(blockProofKey(addr, height), &bp); err != nil {
		return BlockProof{}, false
	}
	return bp, true
}

func blockProofKey(addr string, height uint64) hash.PKHash {
	stream := []byte{}
	stream = append(stream, addr...)
	temp := make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, height)
	stream = append(stream, temp...)
	return byteutil.BytesTo20B(hash.Hash160b(stream))
}

func putBlockToBlockProof(pb *action.PutBlock) BlockProof {
	roots := pb.Roots()
	keys := make([]string, 0, len(roots))
	for k := range roots {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	bpRoots := make([]MerkleRoot, 0, len(keys))
	for _, k := range keys {
		v := roots[k]
		mr := MerkleRoot{
			Name: k,
		}
		copy(mr.Value[:], v[:])
		bpRoots = append(bpRoots, mr)
	}

	return BlockProof{
		SubChainAddress:   pb.SubChainAddress(),
		Roots:             bpRoots,
		Height:            pb.Height(),
		ProducerPublicKey: pb.ProducerPublicKey(),
		ProducerAddress:   pb.ProducerAddress(),
	}
}

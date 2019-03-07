// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"math/big"
	"sort"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol/multichain/mainchain/mainchainpb"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// SubChain represents the state of a sub-chain in the state factory
type SubChain struct {
	ChainID            uint32
	SecurityDeposit    *big.Int
	OperationDeposit   *big.Int
	StartHeight        uint64
	StopHeight         uint64
	ParentHeightOffset uint64
	OwnerPublicKey     keypair.PublicKey
	CurrentHeight      uint64
	DepositCount       uint64
}

// Serialize serializes sub-chain state into bytes
func (bs SubChain) Serialize() ([]byte, error) {
	gen := &mainchainpb.SubChain{
		ChainID:            bs.ChainID,
		StartHeight:        bs.StartHeight,
		StopHeight:         bs.StopHeight,
		ParentHeightOffset: bs.ParentHeightOffset,
		OwnerPublicKey:     bs.OwnerPublicKey.Bytes(),
		CurrentHeight:      bs.CurrentHeight,
		DepositCount:       bs.DepositCount,
	}
	if bs.SecurityDeposit != nil {
		gen.SecurityDeposit = bs.SecurityDeposit.String()
	}
	if bs.OperationDeposit != nil {
		gen.OperationDeposit = bs.OperationDeposit.String()
	}
	return proto.Marshal(gen)
}

// Deserialize deserializes bytes into sub-chain state
func (bs *SubChain) Deserialize(data []byte) error {
	gen := &mainchainpb.SubChain{}
	if err := proto.Unmarshal(data, gen); err != nil {
		return err
	}
	pub, err := keypair.BytesToPublicKey(gen.OwnerPublicKey)
	if err != nil {
		return err
	}

	*bs = SubChain{
		ChainID:            gen.ChainID,
		SecurityDeposit:    &big.Int{},
		OperationDeposit:   &big.Int{},
		StartHeight:        gen.StartHeight,
		StopHeight:         gen.StopHeight,
		ParentHeightOffset: gen.ParentHeightOffset,
		OwnerPublicKey:     pub,
		CurrentHeight:      gen.CurrentHeight,
		DepositCount:       gen.DepositCount,
	}
	bs.SecurityDeposit.SetString(gen.SecurityDeposit, 10)
	bs.OperationDeposit.SetString(gen.OperationDeposit, 10)
	return nil
}

// MerkleRoot defines a merkle root in block proof.
type MerkleRoot struct {
	Name  string
	Value hash.Hash256
}

// BlockProof represents the block proof of a sub-chain in the state factory
type BlockProof struct {
	SubChainAddress   string
	Height            uint64
	Roots             []MerkleRoot
	ProducerPublicKey keypair.PublicKey
	ProducerAddress   string
}

// Serialize serialize block proof state into bytes
func (bp BlockProof) Serialize() ([]byte, error) {
	r := make([]*mainchainpb.MerkleRoot, len(bp.Roots))
	for i := range bp.Roots {
		r[i] = &mainchainpb.MerkleRoot{
			Name:  bp.Roots[i].Name,
			Value: bp.Roots[i].Value[:],
		}
	}

	gen := &mainchainpb.BlockProof{
		SubChainAddress:   bp.SubChainAddress,
		Height:            bp.Height,
		Roots:             r,
		ProducerPublicKey: bp.ProducerPublicKey.Bytes(),
		ProducerAddress:   bp.ProducerAddress,
	}
	return proto.Marshal(gen)
}

// Deserialize deserialize bytes into block proof state
func (bp *BlockProof) Deserialize(data []byte) error {
	gen := &mainchainpb.BlockProof{}
	if err := proto.Unmarshal(data, gen); err != nil {
		return err
	}
	pub, err := keypair.BytesToPublicKey(gen.ProducerPublicKey)
	if err != nil {
		return err
	}
	r := make([]MerkleRoot, len(gen.Roots))
	for i, v := range gen.Roots {
		r[i] = MerkleRoot{
			Name:  v.Name,
			Value: hash.BytesToHash256(v.Value),
		}
	}
	*bp = BlockProof{
		SubChainAddress:   gen.SubChainAddress,
		Height:            gen.Height,
		Roots:             r,
		ProducerPublicKey: pub,
		ProducerAddress:   gen.ProducerAddress,
	}
	return nil
}

// InOperation represents a record of a sub-chain in operation
type InOperation struct {
	ID   uint32
	Addr []byte
}

// SubChainsInOperation is a list of InOperation.
type SubChainsInOperation []InOperation

// Len returns length.
func (s SubChainsInOperation) Len() int { return len(s) }

// Less compares InOperation in list.
func (s SubChainsInOperation) Less(i, j int) bool { return s[i].ID < s[j].ID }

// Swap swaps elements in list.
func (s SubChainsInOperation) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Sort sorts SubChainsInOperation.
func (s SubChainsInOperation) Sort() { sort.Sort(s) }

// Get gets an element with given ID.
func (s SubChainsInOperation) Get(id uint32) (InOperation, bool) {
	for _, io := range s {
		if io.ID == id {
			return io, true
		}
	}
	return InOperation{}, false
}

// Append appends an element and return the new list.
func (s SubChainsInOperation) Append(in InOperation) SubChainsInOperation {
	n := append(s, in)
	n.Sort()
	return n
}

// Delete deletes an element and return the new list.
func (s SubChainsInOperation) Delete(id uint32) (SubChainsInOperation, bool) {
	for idx, in := range s {
		if in.ID == id {
			return append(s[:idx], s[idx+1:]...), true
		}
	}
	return append(s[:0:0], s...), false
}

// Serialize serializes list to binary.
func (s SubChainsInOperation) Serialize() ([]byte, error) {
	s.Sort()
	l := make([]*mainchainpb.InOperation, len(s))
	for i := range s {
		l[i] = &mainchainpb.InOperation{
			Id:      s[i].ID,
			Address: s[i].Addr,
		}
	}
	return proto.Marshal(&mainchainpb.SubChainsInOperation{InOp: l})
}

// Deserialize deserializes list from binary.
func (s *SubChainsInOperation) Deserialize(data []byte) error {
	gen := &mainchainpb.SubChainsInOperation{}
	if err := proto.Unmarshal(data, gen); err != nil {
		return err
	}
	l := make([]InOperation, len(gen.InOp))
	for i := range gen.InOp {
		l[i] = InOperation{
			ID:   gen.InOp[i].Id,
			Addr: gen.InOp[i].Address,
		}
	}
	*s = l
	return nil
}

// StartSubChainReceipt is the receipt to user after executed start sub chain operation.
type StartSubChainReceipt struct {
	SubChainAddress string
}

// Deposit represents the state of a deposit
type Deposit struct {
	Amount    *big.Int
	Addr      []byte
	Confirmed bool
}

// Serialize serializes deposit state into bytes
func (bs Deposit) Serialize() ([]byte, error) {
	gen := &mainchainpb.Deposit{
		Address:   bs.Addr,
		Confirmed: bs.Confirmed,
	}
	if bs.Amount != nil {
		gen.Amount = bs.Amount.String()
	}
	return proto.Marshal(gen)
}

// Deserialize deserializes bytes into deposit state
func (bs *Deposit) Deserialize(data []byte) error {
	gen := &mainchainpb.Deposit{}

	if err := proto.Unmarshal(data, gen); err != nil {
		return err
	}
	*bs = Deposit{
		Amount:    &big.Int{},
		Addr:      gen.Address,
		Confirmed: gen.Confirmed,
	}
	bs.Amount.SetString(gen.Amount, 10)
	return nil
}

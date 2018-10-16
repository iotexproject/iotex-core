// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"errors"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
)

// Payee defines the struct of payee
type Payee struct {
	Address string
	Amount  uint64
}

// BlockHeader defines the struct of block header
// make sure the variable type and order of this struct is same as "BlockHeaderPb" in blockchain.pb.go
type BlockHeader struct {
	version       uint32            // version
	chainID       uint32            // this chain's ID
	height        uint64            // block height
	timestamp     uint64            // unix timestamp
	prevBlockHash hash.Hash32B      // hash of previous block
	txRoot        hash.Hash32B      // merkle root of all transactions
	stateRoot     hash.Hash32B      // root of state trie
	receiptRoot   hash.Hash32B      // root of receipt trie
	blockSig      []byte            // block signature
	Pubkey        keypair.PublicKey // block producer's public key
	DKGID         []byte            // dkg ID of producer
	DKGPubkey     []byte            // dkg public key of producer
	DKGBlockSig   []byte            // dkg signature of producer
}

// Timestamp returns the timestamp in the block header
func (bh *BlockHeader) Timestamp() time.Time {
	return time.Unix(int64(bh.timestamp), 0)
}

// Block defines the struct of block
type Block struct {
	Header          *BlockHeader
	Transfers       []*action.Transfer
	Votes           []*action.Vote
	Actions         []action.Action
	Executions      []*action.Execution
	SecretProposals []*action.SecretProposal
	SecretWitness   *action.SecretWitness
	receipts        map[hash.Hash32B]*Receipt
	workingSet      state.WorkingSet
}

// NewBlock returns a new block
func NewBlock(
	chainID uint32,
	height uint64,
	prevBlockHash hash.Hash32B,
	timestamp uint64,
	tsf []*action.Transfer,
	vote []*action.Vote,
	executions []*action.Execution,
	actions []action.Action,
) *Block {
	block := &Block{
		Header: &BlockHeader{
			version:       version.ProtocolVersion,
			chainID:       chainID,
			height:        height,
			timestamp:     timestamp,
			prevBlockHash: prevBlockHash,
			txRoot:        hash.ZeroHash32B,
			stateRoot:     hash.ZeroHash32B,
			receiptRoot:   hash.ZeroHash32B,
		},
		Transfers:  tsf,
		Votes:      vote,
		Executions: executions,
		Actions:    actions,
	}

	block.Header.txRoot = block.TxRoot()
	return block
}

// NewSecretBlock returns a new DKG secret block
func NewSecretBlock(
	chainID uint32,
	height uint64,
	prevBlockHash hash.Hash32B,
	timestamp uint64,
	secretProposals []*action.SecretProposal,
	secretWitness *action.SecretWitness,
) *Block {
	block := &Block{
		Header: &BlockHeader{
			version:       version.ProtocolVersion,
			chainID:       chainID,
			height:        height,
			timestamp:     timestamp,
			prevBlockHash: prevBlockHash,
			txRoot:        hash.ZeroHash32B,
			stateRoot:     hash.ZeroHash32B,
			receiptRoot:   hash.ZeroHash32B,
		},
		SecretProposals: secretProposals,
		SecretWitness:   secretWitness,
	}

	block.Header.txRoot = block.TxRoot()
	return block
}

// IsDummyBlock checks whether block is a dummy block
func (b *Block) IsDummyBlock() bool {
	return b.Header.height > 0 &&
		len(b.Header.blockSig) == 0 &&
		b.Header.Pubkey == keypair.ZeroPublicKey &&
		len(b.Transfers)+len(b.Votes)+len(b.Executions)+len(b.Actions) == 0
}

// Height returns the height of this block
func (b *Block) Height() uint64 {
	return b.Header.height
}

// PrevHash returns the hash of prev block
func (b *Block) PrevHash() hash.Hash32B {
	return b.Header.prevBlockHash
}

// ByteStreamHeader returns a byte stream of the block header
func (b *Block) ByteStreamHeader() []byte {
	stream := make([]byte, 4)
	enc.MachineEndian.PutUint32(stream, b.Header.version)
	tmp4B := make([]byte, 4)
	enc.MachineEndian.PutUint32(tmp4B, b.Header.chainID)
	stream = append(stream, tmp4B...)
	tmp8B := make([]byte, 8)
	enc.MachineEndian.PutUint64(tmp8B, b.Header.height)
	stream = append(stream, tmp8B...)
	// TODO: exclude timestamp from block hash because dummy block needs to have a consistent hash no matter which
	// node produces it at a given height. Once we get rid of the dummy block concept, we need to include it into
	// the hash block hash again
	//enc.MachineEndian.PutUint64(tmp8B, b.Header.timestamp)
	stream = append(stream, tmp8B...)
	stream = append(stream, b.Header.prevBlockHash[:]...)
	stream = append(stream, b.Header.txRoot[:]...)
	stream = append(stream, b.Header.stateRoot[:]...)
	stream = append(stream, b.Header.receiptRoot[:]...)
	stream = append(stream, b.Header.Pubkey[:]...)
	return stream
}

// ByteStream returns a byte stream of the block
func (b *Block) ByteStream() []byte {
	stream := b.ByteStreamHeader()

	// Add the stream of blockSig
	stream = append(stream, b.Header.blockSig[:]...)
	stream = append(stream, b.Header.DKGID[:]...)
	stream = append(stream, b.Header.DKGPubkey[:]...)
	stream = append(stream, b.Header.DKGBlockSig[:]...)

	for _, t := range b.Transfers {
		stream = append(stream, t.ByteStream()...)
	}
	for _, v := range b.Votes {
		stream = append(stream, v.ByteStream()...)
	}
	for _, e := range b.Executions {
		stream = append(stream, e.ByteStream()...)
	}
	for _, sp := range b.SecretProposals {
		stream = append(stream, sp.ByteStream()...)
	}
	if b.SecretWitness != nil {
		stream = append(stream, b.SecretWitness.ByteStream()...)
	}
	for _, act := range b.Actions {
		stream = append(stream, act.ByteStream()...)
	}
	return stream
}

// ConvertToBlockHeaderPb converts BlockHeader to BlockHeaderPb
func (b *Block) ConvertToBlockHeaderPb() *iproto.BlockHeaderPb {
	pbHeader := iproto.BlockHeaderPb{}

	pbHeader.Version = b.Header.version
	pbHeader.ChainID = b.Header.chainID
	pbHeader.Height = b.Header.height
	pbHeader.Timestamp = b.Header.timestamp
	pbHeader.PrevBlockHash = b.Header.prevBlockHash[:]
	pbHeader.TxRoot = b.Header.txRoot[:]
	pbHeader.StateRoot = b.Header.stateRoot[:]
	pbHeader.ReceiptRoot = b.Header.receiptRoot[:]
	pbHeader.Signature = b.Header.blockSig[:]
	pbHeader.Pubkey = b.Header.Pubkey[:]
	pbHeader.DkgID = b.Header.DKGID[:]
	pbHeader.DkgPubkey = b.Header.DKGPubkey[:]
	pbHeader.DkgSignature = b.Header.DKGBlockSig[:]
	return &pbHeader
}

// ConvertToBlockPb converts Block to BlockPb
func (b *Block) ConvertToBlockPb() *iproto.BlockPb {
	actions := []*iproto.ActionPb{}
	for _, tsf := range b.Transfers {
		actions = append(actions, tsf.Proto())
	}
	for _, vote := range b.Votes {
		actions = append(actions, vote.Proto())
	}
	for _, execution := range b.Executions {
		actions = append(actions, execution.Proto())
	}
	for _, secretProposal := range b.SecretProposals {
		actions = append(actions, secretProposal.Proto())
	}
	if b.SecretWitness != nil {
		actions = append(actions, b.SecretWitness.Proto())
	}
	for _, act := range b.Actions {
		actions = append(actions, act.Proto())
	}
	return &iproto.BlockPb{Header: b.ConvertToBlockHeaderPb(), Actions: actions}
}

// Serialize returns the serialized byte stream of the block
func (b *Block) Serialize() ([]byte, error) {
	return proto.Marshal(b.ConvertToBlockPb())
}

// ConvertFromBlockHeaderPb converts BlockHeaderPb to BlockHeader
func (b *Block) ConvertFromBlockHeaderPb(pbBlock *iproto.BlockPb) {
	b.Header = new(BlockHeader)

	b.Header.version = pbBlock.GetHeader().GetVersion()
	b.Header.chainID = pbBlock.GetHeader().GetChainID()
	b.Header.height = pbBlock.GetHeader().GetHeight()
	b.Header.timestamp = pbBlock.GetHeader().GetTimestamp()
	copy(b.Header.prevBlockHash[:], pbBlock.GetHeader().GetPrevBlockHash())
	copy(b.Header.txRoot[:], pbBlock.GetHeader().GetTxRoot())
	copy(b.Header.stateRoot[:], pbBlock.GetHeader().GetStateRoot())
	copy(b.Header.receiptRoot[:], pbBlock.GetHeader().GetReceiptRoot())
	b.Header.blockSig = pbBlock.GetHeader().GetSignature()
	copy(b.Header.Pubkey[:], pbBlock.GetHeader().GetPubkey())
	b.Header.DKGID = pbBlock.GetHeader().GetDkgID()
	b.Header.DKGPubkey = pbBlock.GetHeader().GetDkgPubkey()
	b.Header.DKGBlockSig = pbBlock.GetHeader().GetDkgSignature()
}

// ConvertFromBlockPb converts BlockPb to Block
func (b *Block) ConvertFromBlockPb(pbBlock *iproto.BlockPb) {
	b.ConvertFromBlockHeaderPb(pbBlock)

	b.Transfers = []*action.Transfer{}
	b.Votes = []*action.Vote{}
	b.Executions = []*action.Execution{}
	b.SecretProposals = []*action.SecretProposal{}
	b.SecretWitness = nil

	for _, actPb := range pbBlock.Actions {
		if tfPb := actPb.GetTransfer(); tfPb != nil {
			tf := &action.Transfer{}
			tf.ConvertFromActionPb(actPb)
			b.Transfers = append(b.Transfers, tf)
		} else if votePb := actPb.GetVote(); votePb != nil {
			vote := &action.Vote{}
			vote.ConvertFromActionPb(actPb)
			b.Votes = append(b.Votes, vote)
		} else if executionPb := actPb.GetExecution(); executionPb != nil {
			execution := &action.Execution{}
			execution.ConvertFromActionPb(actPb)
			b.Executions = append(b.Executions, execution)
		} else if secretProposalPb := actPb.GetSecretProposal(); secretProposalPb != nil {
			secretProposal := &action.SecretProposal{}
			secretProposal.ConvertFromActionPb(actPb)
			b.SecretProposals = append(b.SecretProposals, secretProposal)
		} else if secretWitnessPb := actPb.GetSecretWitness(); secretWitnessPb != nil {
			secretWitness := &action.SecretWitness{}
			secretWitness.ConvertFromActionPb(actPb)
			b.SecretWitness = secretWitness
		} else {
			act := action.NewActionFromProto(actPb)
			b.Actions = append(b.Actions, act)
		}
	}
}

// Deserialize parses the byte stream into a Block
func (b *Block) Deserialize(buf []byte) error {
	pbBlock := iproto.BlockPb{}
	if err := proto.Unmarshal(buf, &pbBlock); err != nil {
		return err
	}

	b.ConvertFromBlockPb(&pbBlock)
	b.workingSet = nil

	// verify merkle root can match after deserialize
	txroot := b.TxRoot()
	if !bytes.Equal(b.Header.txRoot[:], txroot[:]) {
		return errors.New("Failed to match merkle root after deserialize")
	}
	return nil
}

// TxRoot returns the Merkle root of all txs and actions in this block.
func (b *Block) TxRoot() hash.Hash32B {
	var h []hash.Hash32B
	for _, t := range b.Transfers {
		h = append(h, t.Hash())
	}
	for _, v := range b.Votes {
		h = append(h, v.Hash())
	}
	for _, e := range b.Executions {
		h = append(h, e.Hash())
	}
	for _, sp := range b.SecretProposals {
		h = append(h, sp.Hash())
	}
	if b.SecretWitness != nil {
		h = append(h, b.SecretWitness.Hash())
	}
	for _, act := range b.Actions {
		h = append(h, act.Hash())
	}
	if len(h) == 0 {
		return hash.ZeroHash32B
	}
	return crypto.NewMerkleTree(h).HashTree()
}

// HashBlock return the hash of this block (actually hash of block header)
func (b *Block) HashBlock() hash.Hash32B {
	return blake2b.Sum256(b.ByteStreamHeader())
}

// VerifyStateRoot verifies the state root in header
func (b *Block) VerifyStateRoot(root hash.Hash32B) error {
	if b.Header.stateRoot != root {
		return errors.New("State root hash does not match")
	}
	return nil
}

// SignBlock allows signer to sign the block b
func (b *Block) SignBlock(signer *iotxaddress.Address) error {
	if signer.PrivateKey == keypair.ZeroPrivateKey {
		return errors.New("The private key is empty")
	}
	b.Header.Pubkey = signer.PublicKey
	blkHash := b.HashBlock()
	b.Header.blockSig = crypto.EC283.Sign(signer.PrivateKey, blkHash[:])
	return nil
}

// VerifySignature verifies the signature saved in block header
func (b *Block) VerifySignature() bool {
	blkHash := b.HashBlock()

	return crypto.EC283.Verify(b.Header.Pubkey, blkHash[:], b.Header.blockSig)
}

// ProducerAddress returns the address of producer
func (b *Block) ProducerAddress() string {
	chainID := make([]byte, 4)
	enc.MachineEndian.PutUint32(chainID, b.Header.chainID)
	addr, err := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, chainID, b.Header.Pubkey)
	if err != nil {
		return ""
	}
	return addr.RawAddress
}

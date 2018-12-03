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
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state/factory"
)

// GasLimit is the total gas limit could be consumed in a block
const GasLimit = uint64(1000000000)

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

// BlockFooter defines a set of proof of this block
type BlockFooter struct {
	// endorsements contain COMMIT endorsements from more than 2/3 delegates
	endorsements    *endorsement.Set
	commitTimestamp uint64
}

// Block defines the struct of block
type Block struct {
	Header          *BlockHeader
	Actions         []action.Action
	SecretProposals []*action.SecretProposal
	SecretWitness   *action.SecretWitness
	Receipts        map[hash.Hash32B]*action.Receipt
	workingSet      factory.WorkingSet
	Footer          *BlockFooter
}

// NewBlock returns a new block
func NewBlock(
	chainID uint32,
	height uint64,
	prevBlockHash hash.Hash32B,
	timestamp uint64,
	producer keypair.PublicKey,
	actions []action.Action,
) *Block {
	block := &Block{
		Header: &BlockHeader{
			version:       version.ProtocolVersion,
			chainID:       chainID,
			height:        height,
			timestamp:     timestamp,
			prevBlockHash: prevBlockHash,
			Pubkey:        producer,
			txRoot:        hash.ZeroHash32B,
			stateRoot:     hash.ZeroHash32B,
			receiptRoot:   hash.ZeroHash32B,
		},
		Actions: actions,
	}

	block.Header.txRoot = block.CalculateTxRoot()
	return block
}

// NewSecretBlock returns a new DKG secret block
func NewSecretBlock(
	chainID uint32,
	height uint64,
	prevBlockHash hash.Hash32B,
	timestamp uint64,
	producer keypair.PublicKey,
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
			Pubkey:        producer,
			txRoot:        hash.ZeroHash32B,
			stateRoot:     hash.ZeroHash32B,
			receiptRoot:   hash.ZeroHash32B,
		},
		SecretProposals: secretProposals,
		SecretWitness:   secretWitness,
	}

	block.Header.txRoot = block.CalculateTxRoot()
	return block
}

// Height returns the height of this block
func (b *Block) Height() uint64 {
	return b.Header.height
}

// PrevHash returns the hash of prev block
func (b *Block) PrevHash() hash.Hash32B {
	return b.Header.prevBlockHash
}

// TxRoot returns the hash of all actions in this block.
func (b *Block) TxRoot() hash.Hash32B {
	return b.Header.txRoot
}

// StateRoot returns the state root after apply this block.
func (b *Block) StateRoot() hash.Hash32B {
	return b.Header.stateRoot
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
	enc.MachineEndian.PutUint64(tmp8B, b.Header.timestamp)
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
func (b *Block) ConvertFromBlockPb(pbBlock *iproto.BlockPb) error {
	b.ConvertFromBlockHeaderPb(pbBlock)

	b.SecretProposals = []*action.SecretProposal{}
	b.SecretWitness = nil
	b.Actions = []action.Action{}

	for _, actPb := range pbBlock.Actions {
		if secretProposalPb := actPb.GetSecretProposal(); secretProposalPb != nil {
			secretProposal := &action.SecretProposal{}
			if err := secretProposal.LoadProto(actPb); err != nil {
				return err
			}
			b.SecretProposals = append(b.SecretProposals, secretProposal)
		} else if secretWitnessPb := actPb.GetSecretWitness(); secretWitnessPb != nil {
			secretWitness := &action.SecretWitness{}
			if err := secretWitness.LoadProto(actPb); err != nil {
				return err
			}
			b.SecretWitness = secretWitness
		} else if transferPb := actPb.GetTransfer(); transferPb != nil {
			transfer := &action.Transfer{}
			if err := transfer.LoadProto(actPb); err != nil {
				return err
			}
			b.Actions = append(b.Actions, transfer)
		} else if votePb := actPb.GetVote(); votePb != nil {
			vote := &action.Vote{}
			if err := vote.LoadProto(actPb); err != nil {
				return err
			}
			b.Actions = append(b.Actions, vote)
		} else if executionPb := actPb.GetExecution(); executionPb != nil {
			execution := &action.Execution{}
			if err := execution.LoadProto(actPb); err != nil {
				return err
			}
			b.Actions = append(b.Actions, execution)
		} else if startPb := actPb.GetStartSubChain(); startPb != nil {
			start := &action.StartSubChain{}
			if err := start.LoadProto(actPb); err != nil {
				return err
			}
			b.Actions = append(b.Actions, start)
		} else if stopPb := actPb.GetStopSubChain(); stopPb != nil {
			stop := &action.StopSubChain{}
			if err := stop.LoadProto(actPb); err != nil {
				return err
			}
			b.Actions = append(b.Actions, stop)
		} else if putPb := actPb.GetPutBlock(); putPb != nil {
			put := &action.PutBlock{}
			if err := put.LoadProto(actPb); err != nil {
				return err
			}
			b.Actions = append(b.Actions, put)
		} else if createDepositPb := actPb.GetCreateDeposit(); createDepositPb != nil {
			createDeposit := &action.CreateDeposit{}
			if err := createDeposit.LoadProto(actPb); err != nil {
				return err
			}
			b.Actions = append(b.Actions, createDeposit)
		} else if settleDepositPb := actPb.GetSettleDeposit(); settleDepositPb != nil {
			settleDeposit := &action.SettleDeposit{}
			if err := settleDeposit.LoadProto(actPb); err != nil {
				return err
			}
			b.Actions = append(b.Actions, settleDeposit)
		}
	}
	return nil
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
	txroot := b.CalculateTxRoot()
	if !bytes.Equal(b.Header.txRoot[:], txroot[:]) {
		return errors.New("Failed to match merkle root after deserialize")
	}
	return nil
}

// CalculateTxRoot returns the Merkle root of all txs and actions in this block.
func (b *Block) CalculateTxRoot() hash.Hash32B {
	var h []hash.Hash32B
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
		return errors.New("state root hash does not match")
	}
	return nil
}

// SignBlock allows signer to sign the block b
func (b *Block) SignBlock(signer *iotxaddress.Address) error {
	if signer.PrivateKey == keypair.ZeroPrivateKey {
		return errors.New("The private key is empty")
	}
	if signer.PublicKey != b.Header.Pubkey {
		return errors.New("The public key doesn't match")
	}
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
	pkHash := keypair.HashPubKey(b.Header.Pubkey)
	addr := address.New(b.Header.chainID, pkHash[:])

	return addr.IotxAddress()
}

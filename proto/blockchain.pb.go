// Code generated by protoc-gen-go. DO NOT EDIT.
// source: blockchain.proto

/*
Package iproto is a generated protocol buffer package.

It is generated from these files:
	blockchain.proto

It has these top-level messages:
	TransferPb
	VotePb
	ExecutionPb
	ActionPb
	BlockHeaderPb
	BlockPb
	BlockIndex
	BlockSync
	BlockContainer
	ViewChangeMsg
	Candidate
	CandidateList
	TestPayload
*/
package iproto

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type ViewChangeMsg_ViewChangeType int32

const (
	ViewChangeMsg_INVALID_VIEW_CHANGE_TYPE ViewChangeMsg_ViewChangeType = 0
	ViewChangeMsg_PROPOSE                  ViewChangeMsg_ViewChangeType = 1
	ViewChangeMsg_PREVOTE                  ViewChangeMsg_ViewChangeType = 2
	ViewChangeMsg_VOTE                     ViewChangeMsg_ViewChangeType = 3
)

var ViewChangeMsg_ViewChangeType_name = map[int32]string{
	0: "INVALID_VIEW_CHANGE_TYPE",
	1: "PROPOSE",
	2: "PREVOTE",
	3: "VOTE",
}
var ViewChangeMsg_ViewChangeType_value = map[string]int32{
	"INVALID_VIEW_CHANGE_TYPE": 0,
	"PROPOSE":                  1,
	"PREVOTE":                  2,
	"VOTE":                     3,
}

func (x ViewChangeMsg_ViewChangeType) String() string {
	return proto.EnumName(ViewChangeMsg_ViewChangeType_name, int32(x))
}
func (ViewChangeMsg_ViewChangeType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor0, []int{9, 0}
}

type TransferPb struct {
	// used by state-based model
	Amount       []byte `protobuf:"bytes,1,opt,name=amount,proto3" json:"amount,omitempty"`
	Sender       string `protobuf:"bytes,2,opt,name=sender" json:"sender,omitempty"`
	Recipient    string `protobuf:"bytes,3,opt,name=recipient" json:"recipient,omitempty"`
	Payload      []byte `protobuf:"bytes,4,opt,name=payload,proto3" json:"payload,omitempty"`
	SenderPubKey []byte `protobuf:"bytes,5,opt,name=senderPubKey,proto3" json:"senderPubKey,omitempty"`
	IsCoinbase   bool   `protobuf:"varint,6,opt,name=isCoinbase" json:"isCoinbase,omitempty"`
}

func (m *TransferPb) Reset()                    { *m = TransferPb{} }
func (m *TransferPb) String() string            { return proto.CompactTextString(m) }
func (*TransferPb) ProtoMessage()               {}
func (*TransferPb) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *TransferPb) GetAmount() []byte {
	if m != nil {
		return m.Amount
	}
	return nil
}

func (m *TransferPb) GetSender() string {
	if m != nil {
		return m.Sender
	}
	return ""
}

func (m *TransferPb) GetRecipient() string {
	if m != nil {
		return m.Recipient
	}
	return ""
}

func (m *TransferPb) GetPayload() []byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (m *TransferPb) GetSenderPubKey() []byte {
	if m != nil {
		return m.SenderPubKey
	}
	return nil
}

func (m *TransferPb) GetIsCoinbase() bool {
	if m != nil {
		return m.IsCoinbase
	}
	return false
}

type VotePb struct {
	Timestamp    uint64 `protobuf:"varint,1,opt,name=timestamp" json:"timestamp,omitempty"`
	SelfPubkey   []byte `protobuf:"bytes,2,opt,name=selfPubkey,proto3" json:"selfPubkey,omitempty"`
	VoterAddress string `protobuf:"bytes,3,opt,name=voterAddress" json:"voterAddress,omitempty"`
	VoteeAddress string `protobuf:"bytes,4,opt,name=voteeAddress" json:"voteeAddress,omitempty"`
}

func (m *VotePb) Reset()                    { *m = VotePb{} }
func (m *VotePb) String() string            { return proto.CompactTextString(m) }
func (*VotePb) ProtoMessage()               {}
func (*VotePb) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *VotePb) GetTimestamp() uint64 {
	if m != nil {
		return m.Timestamp
	}
	return 0
}

func (m *VotePb) GetSelfPubkey() []byte {
	if m != nil {
		return m.SelfPubkey
	}
	return nil
}

func (m *VotePb) GetVoterAddress() string {
	if m != nil {
		return m.VoterAddress
	}
	return ""
}

func (m *VotePb) GetVoteeAddress() string {
	if m != nil {
		return m.VoteeAddress
	}
	return ""
}

type ExecutionPb struct {
	// ExecutionPb should share these three fields with other Actions
	// TODO: extract these three fields to ActionPb
	Version        uint32 `protobuf:"varint,1,opt,name=version" json:"version,omitempty"`
	Nonce          uint64 `protobuf:"varint,2,opt,name=nonce" json:"nonce,omitempty"`
	Signature      []byte `protobuf:"bytes,3,opt,name=signature,proto3" json:"signature,omitempty"`
	Amount         []byte `protobuf:"bytes,4,opt,name=amount,proto3" json:"amount,omitempty"`
	Executor       string `protobuf:"bytes,5,opt,name=executor" json:"executor,omitempty"`
	Contract       string `protobuf:"bytes,6,opt,name=contract" json:"contract,omitempty"`
	ExecutorPubKey []byte `protobuf:"bytes,7,opt,name=executorPubKey,proto3" json:"executorPubKey,omitempty"`
	Gas            uint32 `protobuf:"varint,8,opt,name=gas" json:"gas,omitempty"`
	GasPrice       uint32 `protobuf:"varint,9,opt,name=gasPrice" json:"gasPrice,omitempty"`
	Data           []byte `protobuf:"bytes,10,opt,name=data,proto3" json:"data,omitempty"`
}

func (m *ExecutionPb) Reset()                    { *m = ExecutionPb{} }
func (m *ExecutionPb) String() string            { return proto.CompactTextString(m) }
func (*ExecutionPb) ProtoMessage()               {}
func (*ExecutionPb) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *ExecutionPb) GetVersion() uint32 {
	if m != nil {
		return m.Version
	}
	return 0
}

func (m *ExecutionPb) GetNonce() uint64 {
	if m != nil {
		return m.Nonce
	}
	return 0
}

func (m *ExecutionPb) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

func (m *ExecutionPb) GetAmount() []byte {
	if m != nil {
		return m.Amount
	}
	return nil
}

func (m *ExecutionPb) GetExecutor() string {
	if m != nil {
		return m.Executor
	}
	return ""
}

func (m *ExecutionPb) GetContract() string {
	if m != nil {
		return m.Contract
	}
	return ""
}

func (m *ExecutionPb) GetExecutorPubKey() []byte {
	if m != nil {
		return m.ExecutorPubKey
	}
	return nil
}

func (m *ExecutionPb) GetGas() uint32 {
	if m != nil {
		return m.Gas
	}
	return 0
}

func (m *ExecutionPb) GetGasPrice() uint32 {
	if m != nil {
		return m.GasPrice
	}
	return 0
}

func (m *ExecutionPb) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

type ActionPb struct {
	Version   uint32 `protobuf:"varint,1,opt,name=version" json:"version,omitempty"`
	Nonce     uint64 `protobuf:"varint,2,opt,name=nonce" json:"nonce,omitempty"`
	Signature []byte `protobuf:"bytes,3,opt,name=signature,proto3" json:"signature,omitempty"`
	// Types that are valid to be assigned to Action:
	//	*ActionPb_Transfer
	//	*ActionPb_Vote
	//	*ActionPb_Execution
	Action isActionPb_Action `protobuf_oneof:"action"`
}

func (m *ActionPb) Reset()                    { *m = ActionPb{} }
func (m *ActionPb) String() string            { return proto.CompactTextString(m) }
func (*ActionPb) ProtoMessage()               {}
func (*ActionPb) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

type isActionPb_Action interface {
	isActionPb_Action()
}

type ActionPb_Transfer struct {
	Transfer *TransferPb `protobuf:"bytes,10,opt,name=transfer,oneof"`
}
type ActionPb_Vote struct {
	Vote *VotePb `protobuf:"bytes,11,opt,name=vote,oneof"`
}
type ActionPb_Execution struct {
	Execution *ExecutionPb `protobuf:"bytes,12,opt,name=execution,oneof"`
}

func (*ActionPb_Transfer) isActionPb_Action()  {}
func (*ActionPb_Vote) isActionPb_Action()      {}
func (*ActionPb_Execution) isActionPb_Action() {}

func (m *ActionPb) GetAction() isActionPb_Action {
	if m != nil {
		return m.Action
	}
	return nil
}

func (m *ActionPb) GetVersion() uint32 {
	if m != nil {
		return m.Version
	}
	return 0
}

func (m *ActionPb) GetNonce() uint64 {
	if m != nil {
		return m.Nonce
	}
	return 0
}

func (m *ActionPb) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

func (m *ActionPb) GetTransfer() *TransferPb {
	if x, ok := m.GetAction().(*ActionPb_Transfer); ok {
		return x.Transfer
	}
	return nil
}

func (m *ActionPb) GetVote() *VotePb {
	if x, ok := m.GetAction().(*ActionPb_Vote); ok {
		return x.Vote
	}
	return nil
}

func (m *ActionPb) GetExecution() *ExecutionPb {
	if x, ok := m.GetAction().(*ActionPb_Execution); ok {
		return x.Execution
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*ActionPb) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _ActionPb_OneofMarshaler, _ActionPb_OneofUnmarshaler, _ActionPb_OneofSizer, []interface{}{
		(*ActionPb_Transfer)(nil),
		(*ActionPb_Vote)(nil),
		(*ActionPb_Execution)(nil),
	}
}

func _ActionPb_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*ActionPb)
	// action
	switch x := m.Action.(type) {
	case *ActionPb_Transfer:
		b.EncodeVarint(10<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Transfer); err != nil {
			return err
		}
	case *ActionPb_Vote:
		b.EncodeVarint(11<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Vote); err != nil {
			return err
		}
	case *ActionPb_Execution:
		b.EncodeVarint(12<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Execution); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("ActionPb.Action has unexpected type %T", x)
	}
	return nil
}

func _ActionPb_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*ActionPb)
	switch tag {
	case 10: // action.transfer
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(TransferPb)
		err := b.DecodeMessage(msg)
		m.Action = &ActionPb_Transfer{msg}
		return true, err
	case 11: // action.vote
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(VotePb)
		err := b.DecodeMessage(msg)
		m.Action = &ActionPb_Vote{msg}
		return true, err
	case 12: // action.execution
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(ExecutionPb)
		err := b.DecodeMessage(msg)
		m.Action = &ActionPb_Execution{msg}
		return true, err
	default:
		return false, nil
	}
}

func _ActionPb_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*ActionPb)
	// action
	switch x := m.Action.(type) {
	case *ActionPb_Transfer:
		s := proto.Size(x.Transfer)
		n += proto.SizeVarint(10<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case *ActionPb_Vote:
		s := proto.Size(x.Vote)
		n += proto.SizeVarint(11<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case *ActionPb_Execution:
		s := proto.Size(x.Execution)
		n += proto.SizeVarint(12<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

// header of a block
type BlockHeaderPb struct {
	Version       uint32 `protobuf:"varint,1,opt,name=version" json:"version,omitempty"`
	ChainID       uint32 `protobuf:"varint,2,opt,name=chainID" json:"chainID,omitempty"`
	Height        uint64 `protobuf:"varint,3,opt,name=height" json:"height,omitempty"`
	Timestamp     uint64 `protobuf:"varint,4,opt,name=timestamp" json:"timestamp,omitempty"`
	PrevBlockHash []byte `protobuf:"bytes,5,opt,name=prevBlockHash,proto3" json:"prevBlockHash,omitempty"`
	TxRoot        []byte `protobuf:"bytes,6,opt,name=txRoot,proto3" json:"txRoot,omitempty"`
	StateRoot     []byte `protobuf:"bytes,7,opt,name=stateRoot,proto3" json:"stateRoot,omitempty"`
	TrnxNumber    uint32 `protobuf:"varint,8,opt,name=trnxNumber" json:"trnxNumber,omitempty"`
	TrnxDataSize  uint32 `protobuf:"varint,9,opt,name=trnxDataSize" json:"trnxDataSize,omitempty"`
	Signature     []byte `protobuf:"bytes,10,opt,name=signature,proto3" json:"signature,omitempty"`
	Pubkey        []byte `protobuf:"bytes,11,opt,name=pubkey,proto3" json:"pubkey,omitempty"`
}

func (m *BlockHeaderPb) Reset()                    { *m = BlockHeaderPb{} }
func (m *BlockHeaderPb) String() string            { return proto.CompactTextString(m) }
func (*BlockHeaderPb) ProtoMessage()               {}
func (*BlockHeaderPb) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *BlockHeaderPb) GetVersion() uint32 {
	if m != nil {
		return m.Version
	}
	return 0
}

func (m *BlockHeaderPb) GetChainID() uint32 {
	if m != nil {
		return m.ChainID
	}
	return 0
}

func (m *BlockHeaderPb) GetHeight() uint64 {
	if m != nil {
		return m.Height
	}
	return 0
}

func (m *BlockHeaderPb) GetTimestamp() uint64 {
	if m != nil {
		return m.Timestamp
	}
	return 0
}

func (m *BlockHeaderPb) GetPrevBlockHash() []byte {
	if m != nil {
		return m.PrevBlockHash
	}
	return nil
}

func (m *BlockHeaderPb) GetTxRoot() []byte {
	if m != nil {
		return m.TxRoot
	}
	return nil
}

func (m *BlockHeaderPb) GetStateRoot() []byte {
	if m != nil {
		return m.StateRoot
	}
	return nil
}

func (m *BlockHeaderPb) GetTrnxNumber() uint32 {
	if m != nil {
		return m.TrnxNumber
	}
	return 0
}

func (m *BlockHeaderPb) GetTrnxDataSize() uint32 {
	if m != nil {
		return m.TrnxDataSize
	}
	return 0
}

func (m *BlockHeaderPb) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

func (m *BlockHeaderPb) GetPubkey() []byte {
	if m != nil {
		return m.Pubkey
	}
	return nil
}

// block consists of header followed by transactions
// hash of current block can be computed from header hence not stored
type BlockPb struct {
	Header  *BlockHeaderPb `protobuf:"bytes,1,opt,name=header" json:"header,omitempty"`
	Actions []*ActionPb    `protobuf:"bytes,2,rep,name=actions" json:"actions,omitempty"`
}

func (m *BlockPb) Reset()                    { *m = BlockPb{} }
func (m *BlockPb) String() string            { return proto.CompactTextString(m) }
func (*BlockPb) ProtoMessage()               {}
func (*BlockPb) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *BlockPb) GetHeader() *BlockHeaderPb {
	if m != nil {
		return m.Header
	}
	return nil
}

func (m *BlockPb) GetActions() []*ActionPb {
	if m != nil {
		return m.Actions
	}
	return nil
}

// index of block raw data file
type BlockIndex struct {
	Start  uint64   `protobuf:"varint,1,opt,name=start" json:"start,omitempty"`
	End    uint64   `protobuf:"varint,2,opt,name=end" json:"end,omitempty"`
	Offset []uint32 `protobuf:"varint,3,rep,packed,name=offset" json:"offset,omitempty"`
}

func (m *BlockIndex) Reset()                    { *m = BlockIndex{} }
func (m *BlockIndex) String() string            { return proto.CompactTextString(m) }
func (*BlockIndex) ProtoMessage()               {}
func (*BlockIndex) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{6} }

func (m *BlockIndex) GetStart() uint64 {
	if m != nil {
		return m.Start
	}
	return 0
}

func (m *BlockIndex) GetEnd() uint64 {
	if m != nil {
		return m.End
	}
	return 0
}

func (m *BlockIndex) GetOffset() []uint32 {
	if m != nil {
		return m.Offset
	}
	return nil
}

type BlockSync struct {
	Start uint64 `protobuf:"varint,2,opt,name=start" json:"start,omitempty"`
	End   uint64 `protobuf:"varint,3,opt,name=end" json:"end,omitempty"`
}

func (m *BlockSync) Reset()                    { *m = BlockSync{} }
func (m *BlockSync) String() string            { return proto.CompactTextString(m) }
func (*BlockSync) ProtoMessage()               {}
func (*BlockSync) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{7} }

func (m *BlockSync) GetStart() uint64 {
	if m != nil {
		return m.Start
	}
	return 0
}

func (m *BlockSync) GetEnd() uint64 {
	if m != nil {
		return m.End
	}
	return 0
}

// block container
// used to send old/existing blocks in block sync
type BlockContainer struct {
	Block *BlockPb `protobuf:"bytes,1,opt,name=block" json:"block,omitempty"`
}

func (m *BlockContainer) Reset()                    { *m = BlockContainer{} }
func (m *BlockContainer) String() string            { return proto.CompactTextString(m) }
func (*BlockContainer) ProtoMessage()               {}
func (*BlockContainer) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{8} }

func (m *BlockContainer) GetBlock() *BlockPb {
	if m != nil {
		return m.Block
	}
	return nil
}

type ViewChangeMsg struct {
	Vctype     ViewChangeMsg_ViewChangeType `protobuf:"varint,1,opt,name=vctype,enum=iproto.ViewChangeMsg_ViewChangeType" json:"vctype,omitempty"`
	Block      *BlockPb                     `protobuf:"bytes,2,opt,name=block" json:"block,omitempty"`
	BlockHash  []byte                       `protobuf:"bytes,3,opt,name=blockHash,proto3" json:"blockHash,omitempty"`
	SenderAddr string                       `protobuf:"bytes,4,opt,name=senderAddr" json:"senderAddr,omitempty"`
	Decision   bool                         `protobuf:"varint,5,opt,name=decision" json:"decision,omitempty"`
}

func (m *ViewChangeMsg) Reset()                    { *m = ViewChangeMsg{} }
func (m *ViewChangeMsg) String() string            { return proto.CompactTextString(m) }
func (*ViewChangeMsg) ProtoMessage()               {}
func (*ViewChangeMsg) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{9} }

func (m *ViewChangeMsg) GetVctype() ViewChangeMsg_ViewChangeType {
	if m != nil {
		return m.Vctype
	}
	return ViewChangeMsg_INVALID_VIEW_CHANGE_TYPE
}

func (m *ViewChangeMsg) GetBlock() *BlockPb {
	if m != nil {
		return m.Block
	}
	return nil
}

func (m *ViewChangeMsg) GetBlockHash() []byte {
	if m != nil {
		return m.BlockHash
	}
	return nil
}

func (m *ViewChangeMsg) GetSenderAddr() string {
	if m != nil {
		return m.SenderAddr
	}
	return ""
}

func (m *ViewChangeMsg) GetDecision() bool {
	if m != nil {
		return m.Decision
	}
	return false
}

// Candidates and list of candidates
type Candidate struct {
	Address          string `protobuf:"bytes,1,opt,name=address" json:"address,omitempty"`
	Votes            []byte `protobuf:"bytes,2,opt,name=votes,proto3" json:"votes,omitempty"`
	PubKey           []byte `protobuf:"bytes,3,opt,name=pubKey,proto3" json:"pubKey,omitempty"`
	CreationHeight   uint64 `protobuf:"varint,4,opt,name=creationHeight" json:"creationHeight,omitempty"`
	LastUpdateHeight uint64 `protobuf:"varint,5,opt,name=lastUpdateHeight" json:"lastUpdateHeight,omitempty"`
}

func (m *Candidate) Reset()                    { *m = Candidate{} }
func (m *Candidate) String() string            { return proto.CompactTextString(m) }
func (*Candidate) ProtoMessage()               {}
func (*Candidate) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{10} }

func (m *Candidate) GetAddress() string {
	if m != nil {
		return m.Address
	}
	return ""
}

func (m *Candidate) GetVotes() []byte {
	if m != nil {
		return m.Votes
	}
	return nil
}

func (m *Candidate) GetPubKey() []byte {
	if m != nil {
		return m.PubKey
	}
	return nil
}

func (m *Candidate) GetCreationHeight() uint64 {
	if m != nil {
		return m.CreationHeight
	}
	return 0
}

func (m *Candidate) GetLastUpdateHeight() uint64 {
	if m != nil {
		return m.LastUpdateHeight
	}
	return 0
}

type CandidateList struct {
	Candidates []*Candidate `protobuf:"bytes,1,rep,name=candidates" json:"candidates,omitempty"`
}

func (m *CandidateList) Reset()                    { *m = CandidateList{} }
func (m *CandidateList) String() string            { return proto.CompactTextString(m) }
func (*CandidateList) ProtoMessage()               {}
func (*CandidateList) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{11} }

func (m *CandidateList) GetCandidates() []*Candidate {
	if m != nil {
		return m.Candidates
	}
	return nil
}

// //////////////////////////////////////////////////////////////////////////////////////////////////
// BELOW ARE DEFINITIONS FOR TEST-ONLY MESSAGES!
// //////////////////////////////////////////////////////////////////////////////////////////////////
type TestPayload struct {
	MsgBody []byte `protobuf:"bytes,1,opt,name=msg_body,json=msgBody,proto3" json:"msg_body,omitempty"`
}

func (m *TestPayload) Reset()                    { *m = TestPayload{} }
func (m *TestPayload) String() string            { return proto.CompactTextString(m) }
func (*TestPayload) ProtoMessage()               {}
func (*TestPayload) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{12} }

func (m *TestPayload) GetMsgBody() []byte {
	if m != nil {
		return m.MsgBody
	}
	return nil
}

func init() {
	proto.RegisterType((*TransferPb)(nil), "iproto.TransferPb")
	proto.RegisterType((*VotePb)(nil), "iproto.VotePb")
	proto.RegisterType((*ExecutionPb)(nil), "iproto.ExecutionPb")
	proto.RegisterType((*ActionPb)(nil), "iproto.ActionPb")
	proto.RegisterType((*BlockHeaderPb)(nil), "iproto.BlockHeaderPb")
	proto.RegisterType((*BlockPb)(nil), "iproto.BlockPb")
	proto.RegisterType((*BlockIndex)(nil), "iproto.BlockIndex")
	proto.RegisterType((*BlockSync)(nil), "iproto.BlockSync")
	proto.RegisterType((*BlockContainer)(nil), "iproto.BlockContainer")
	proto.RegisterType((*ViewChangeMsg)(nil), "iproto.ViewChangeMsg")
	proto.RegisterType((*Candidate)(nil), "iproto.Candidate")
	proto.RegisterType((*CandidateList)(nil), "iproto.CandidateList")
	proto.RegisterType((*TestPayload)(nil), "iproto.TestPayload")
	proto.RegisterEnum("iproto.ViewChangeMsg_ViewChangeType", ViewChangeMsg_ViewChangeType_name, ViewChangeMsg_ViewChangeType_value)
}

func init() { proto.RegisterFile("blockchain.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 958 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xb4, 0x55, 0xdb, 0x6e, 0xeb, 0x44,
	0x17, 0xae, 0x13, 0xe7, 0xb4, 0x72, 0xf8, 0xf3, 0x0f, 0x07, 0x19, 0xb4, 0x85, 0x2a, 0xab, 0x6c,
	0x45, 0x5b, 0xa2, 0x82, 0xf6, 0x82, 0x1b, 0x6e, 0xda, 0x34, 0x22, 0x11, 0xa5, 0xb5, 0xa6, 0x21,
	0x88, 0xab, 0x6a, 0x6c, 0xaf, 0x26, 0xd6, 0x6e, 0xc6, 0x91, 0x67, 0x52, 0x1a, 0x1e, 0x82, 0x3b,
	0x5e, 0x80, 0x1b, 0x5e, 0x81, 0xb7, 0xe0, 0x51, 0x78, 0x05, 0x34, 0xcb, 0x63, 0x27, 0x2e, 0x1b,
	0xee, 0xb8, 0xca, 0x7c, 0xdf, 0xac, 0xac, 0x59, 0x87, 0x6f, 0x2d, 0xc3, 0x30, 0x7c, 0x4c, 0xa3,
	0xb7, 0xd1, 0x4a, 0x24, 0xf2, 0x74, 0x93, 0xa5, 0x3a, 0x65, 0xcd, 0x84, 0x7e, 0xfd, 0xdf, 0x1d,
	0x80, 0x79, 0x26, 0xa4, 0x7a, 0xc0, 0x2c, 0x08, 0xd9, 0x87, 0xd0, 0x14, 0xeb, 0x74, 0x2b, 0xb5,
	0xe7, 0x1c, 0x3b, 0xa3, 0x1e, 0xb7, 0xc8, 0xf0, 0x0a, 0x65, 0x8c, 0x99, 0x57, 0x3b, 0x76, 0x46,
	0x1d, 0x6e, 0x11, 0x7b, 0x05, 0x9d, 0x0c, 0xa3, 0x64, 0x93, 0xa0, 0xd4, 0x5e, 0x9d, 0xae, 0xf6,
	0x04, 0xf3, 0xa0, 0xb5, 0x11, 0xbb, 0xc7, 0x54, 0xc4, 0x9e, 0x4b, 0xee, 0x0a, 0xc8, 0x7c, 0xe8,
	0xe5, 0x1e, 0x82, 0x6d, 0xf8, 0x0d, 0xee, 0xbc, 0x06, 0x5d, 0x57, 0x38, 0xf6, 0x09, 0x40, 0xa2,
	0xc6, 0x69, 0x22, 0x43, 0xa1, 0xd0, 0x6b, 0x1e, 0x3b, 0xa3, 0x36, 0x3f, 0x60, 0xfc, 0x9f, 0x1d,
	0x68, 0x2e, 0x52, 0x8d, 0x41, 0x68, 0xc2, 0xd0, 0xc9, 0x1a, 0x95, 0x16, 0xeb, 0x0d, 0x45, 0xee,
	0xf2, 0x3d, 0x61, 0x1c, 0x29, 0x7c, 0x7c, 0x08, 0xb6, 0xe1, 0x5b, 0xdc, 0x51, 0x02, 0x3d, 0x7e,
	0xc0, 0x98, 0x60, 0x9e, 0x52, 0x8d, 0xd9, 0x45, 0x1c, 0x67, 0xa8, 0x94, 0xcd, 0xa3, 0xc2, 0x15,
	0x36, 0x58, 0xd8, 0xb8, 0x7b, 0x9b, 0x82, 0xf3, 0x7f, 0xa9, 0x41, 0x77, 0xf2, 0x8c, 0xd1, 0x56,
	0x27, 0xa9, 0x0c, 0x42, 0x93, 0xfe, 0x13, 0x66, 0x2a, 0x49, 0x25, 0xc5, 0xd4, 0xe7, 0x05, 0x64,
	0xef, 0x43, 0x43, 0xa6, 0x32, 0x42, 0x0a, 0xc6, 0xe5, 0x39, 0x30, 0x59, 0xa8, 0x64, 0x29, 0x85,
	0xde, 0x66, 0x48, 0x41, 0xf4, 0xf8, 0x9e, 0x38, 0x68, 0x8d, 0x5b, 0x69, 0xcd, 0xc7, 0xd0, 0x46,
	0x7a, 0x34, 0xcd, 0xa8, 0x8c, 0x1d, 0x5e, 0x62, 0x73, 0x17, 0xa5, 0x52, 0x67, 0x22, 0xd2, 0x54,
	0xc0, 0x0e, 0x2f, 0x31, 0x7b, 0x0d, 0x83, 0xc2, 0xce, 0x36, 0xa1, 0x45, 0x7e, 0x5f, 0xb0, 0x6c,
	0x08, 0xf5, 0xa5, 0x50, 0x5e, 0x9b, 0x32, 0x30, 0x47, 0xe3, 0x75, 0x29, 0x54, 0x90, 0x25, 0x11,
	0x7a, 0x1d, 0xa2, 0x4b, 0xcc, 0x18, 0xb8, 0xb1, 0xd0, 0xc2, 0x03, 0xf2, 0x45, 0x67, 0xff, 0x4f,
	0x07, 0xda, 0x17, 0xd1, 0x7f, 0x52, 0x94, 0xcf, 0xa1, 0xad, 0xad, 0x7a, 0xe9, 0xc9, 0xee, 0x19,
	0x3b, 0xcd, 0x95, 0x7d, 0xba, 0x57, 0xf5, 0xf4, 0x88, 0x97, 0x56, 0xec, 0x04, 0x5c, 0xd3, 0x34,
	0xaf, 0x4b, 0xd6, 0x83, 0xc2, 0x3a, 0x17, 0xd2, 0xf4, 0x88, 0xd3, 0x2d, 0x3b, 0x87, 0x0e, 0x16,
	0x9d, 0xf4, 0x7a, 0x64, 0xfa, 0x5e, 0x61, 0x7a, 0xd0, 0xe2, 0xe9, 0x11, 0xdf, 0xdb, 0x5d, 0xb6,
	0xa1, 0x29, 0x28, 0x4d, 0xff, 0x8f, 0x1a, 0xf4, 0x2f, 0xcd, 0xc8, 0x4d, 0x51, 0xc4, 0x34, 0x58,
	0xff, 0x9c, 0xb6, 0x07, 0x2d, 0x1a, 0xcc, 0xd9, 0x15, 0x25, 0xde, 0xe7, 0x05, 0x34, 0x1d, 0x5f,
	0x61, 0xb2, 0x5c, 0xe5, 0x93, 0xe5, 0x72, 0x8b, 0xaa, 0x6a, 0x77, 0x5f, 0xaa, 0xfd, 0x04, 0xfa,
	0x9b, 0x0c, 0x9f, 0xf2, 0xe7, 0x85, 0x5a, 0xd9, 0xd9, 0xaa, 0x92, 0xc6, 0xb7, 0x7e, 0xe6, 0x69,
	0x9a, 0xeb, 0xa2, 0xc7, 0x2d, 0xa2, 0x72, 0x6b, 0xa1, 0x91, 0xae, 0x5a, 0xb6, 0xdc, 0x05, 0x61,
	0x26, 0x49, 0x67, 0xf2, 0xf9, 0x66, 0xbb, 0x0e, 0x31, 0xb3, 0x92, 0x38, 0x60, 0xcc, 0x94, 0x18,
	0x74, 0x25, 0xb4, 0xb8, 0x4b, 0x7e, 0x2a, 0xd4, 0x51, 0xe1, 0xaa, 0x0d, 0x85, 0x77, 0xa8, 0x7c,
	0x93, 0xcf, 0x69, 0x37, 0x8f, 0x2b, 0x47, 0x7e, 0x0c, 0x2d, 0x0a, 0x3e, 0x08, 0xd9, 0x67, 0xa6,
	0x2c, 0xa6, 0xac, 0x54, 0xc9, 0xee, 0xd9, 0x07, 0x45, 0x63, 0x2a, 0x15, 0xe7, 0xd6, 0x88, 0xbd,
	0x81, 0x56, 0xde, 0x15, 0xe5, 0xd5, 0x8e, 0xeb, 0xa3, 0xee, 0xd9, 0xb0, 0xb0, 0x2f, 0x34, 0xc9,
	0x0b, 0x03, 0xff, 0x1a, 0x80, 0x9c, 0xcc, 0x64, 0x8c, 0xcf, 0x46, 0x90, 0x4a, 0x8b, 0x4c, 0xdb,
	0x8d, 0x92, 0x03, 0x33, 0x0f, 0x28, 0x63, 0x2b, 0x52, 0x73, 0x34, 0x31, 0xa7, 0x0f, 0x0f, 0x0a,
	0x4d, 0x9f, 0xea, 0xa3, 0x3e, 0xb7, 0xc8, 0x3f, 0x87, 0x0e, 0x79, 0xbb, 0xdb, 0xc9, 0x68, 0xef,
	0xac, 0xf6, 0x0e, 0x67, 0xf5, 0xd2, 0x99, 0xff, 0x25, 0x0c, 0xe8, 0x4f, 0xe3, 0x54, 0x6a, 0x91,
	0x48, 0xcc, 0xd8, 0xa7, 0xd0, 0xa0, 0xf5, 0x6d, 0xd3, 0xfd, 0x5f, 0x25, 0xdd, 0x20, 0xe4, 0xf9,
	0xad, 0xff, 0x6b, 0x0d, 0xfa, 0x8b, 0x04, 0x7f, 0x1c, 0xaf, 0x84, 0x5c, 0xe2, 0xb7, 0x6a, 0xc9,
	0xbe, 0x82, 0xe6, 0x53, 0xa4, 0x77, 0x1b, 0xa4, 0x7f, 0x0e, 0xce, 0x4e, 0x4a, 0xb1, 0x1f, 0x9a,
	0x1d, 0xa0, 0xf9, 0x6e, 0x83, 0xdc, 0xfe, 0x67, 0xff, 0x6c, 0xed, 0xdf, 0x9e, 0x35, 0xed, 0x0c,
	0x4b, 0xa9, 0xd9, 0xf9, 0x2c, 0x89, 0x7c, 0xf5, 0x9a, 0x9d, 0x6e, 0x76, 0xa4, 0x5d, 0x9a, 0x07,
	0x8c, 0x59, 0x25, 0x31, 0x46, 0x09, 0xcd, 0x45, 0x83, 0x36, 0x7c, 0x89, 0x7d, 0x0e, 0x83, 0x6a,
	0x68, 0xec, 0x15, 0x78, 0xb3, 0x9b, 0xc5, 0xc5, 0xf5, 0xec, 0xea, 0x7e, 0x31, 0x9b, 0x7c, 0x7f,
	0x3f, 0x9e, 0x5e, 0xdc, 0x7c, 0x3d, 0xb9, 0x9f, 0xff, 0x10, 0x4c, 0x86, 0x47, 0xac, 0x0b, 0xad,
	0x80, 0xdf, 0x06, 0xb7, 0x77, 0x93, 0xa1, 0x93, 0x83, 0xc9, 0xe2, 0x76, 0x3e, 0x19, 0xd6, 0x58,
	0x1b, 0x5c, 0x3a, 0xd5, 0xfd, 0xdf, 0x1c, 0xe8, 0x8c, 0x85, 0x8c, 0x93, 0x58, 0x68, 0x34, 0xa3,
	0x27, 0xec, 0x3e, 0x77, 0x28, 0xb4, 0x02, 0x9a, 0x6e, 0x99, 0x3d, 0xa0, 0xec, 0xd7, 0x22, 0x07,
	0x56, 0x9c, 0x66, 0x55, 0xd6, 0x4b, 0x71, 0x9a, 0x15, 0xf9, 0x1a, 0x06, 0x51, 0x86, 0xc2, 0x68,
	0x68, 0x9a, 0x0f, 0x6c, 0x3e, 0x95, 0x2f, 0x58, 0xf6, 0x06, 0x86, 0x8f, 0x42, 0xe9, 0xef, 0x36,
	0xe6, 0x75, 0x6b, 0xd9, 0x20, 0xcb, 0xbf, 0xf1, 0xfe, 0x25, 0xf4, 0xcb, 0x40, 0xaf, 0x13, 0xa5,
	0xd9, 0x17, 0x00, 0x51, 0x41, 0x98, 0x78, 0x8d, 0x94, 0xff, 0x5f, 0x34, 0xa5, 0x34, 0xe5, 0x07,
	0x46, 0xfe, 0x08, 0xba, 0x73, 0x54, 0x3a, 0xb0, 0x1f, 0xdd, 0x8f, 0xa0, 0xbd, 0x56, 0xcb, 0xfb,
	0x30, 0x8d, 0x77, 0xf6, 0xf3, 0xde, 0x5a, 0xab, 0xe5, 0x65, 0x1a, 0xef, 0xc2, 0x26, 0xb9, 0x39,
	0xff, 0x2b, 0x00, 0x00, 0xff, 0xff, 0xb1, 0xb1, 0xa1, 0x7b, 0x29, 0x08, 0x00, 0x00,
}

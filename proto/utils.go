// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package iproto

import (
	"errors"
	"github.com/golang/protobuf/proto"
)

// Magic header to identify IoTex traffic
const (
	MagicBroadcastMsgHeader uint32 = 4689
)

const (
	// UnknownProtoMsgType is an unknown message type that is not expected
	UnknownProtoMsgType uint32 = 0
	// MsgTxProtoMsgType is for transactions broadcasted within the network
	MsgTxProtoMsgType uint32 = 1
	// MsgBlockProtoMsgType is for blocks broadcasted within the network
	MsgBlockProtoMsgType uint32 = 2
	// ViewChangeMsgType is for consensus flows within the network
	ViewChangeMsgType uint32 = 3
	// MsgBlockSyncReqType is for requests among peers to sync blocks
	MsgBlockSyncReqType uint32 = 4
	// MsgBlockSyncDataType is the response to messages of type MsgBlockSyncReqType
	MsgBlockSyncDataType uint32 = 5
	// MsgActionType is the action message
	MsgActionType uint32 = 6
	// TestPayloadType is a test payload message type
	TestPayloadType uint32 = 10001
)

// GetTypeFromProtoMsg retrieves the proto message type
func GetTypeFromProtoMsg(msg proto.Message) (uint32, error) {
	switch msg.(type) {
	case *TxPb:
		return MsgTxProtoMsgType, nil
	case *BlockPb:
		return MsgBlockProtoMsgType, nil
	case *ViewChangeMsg:
		return ViewChangeMsgType, nil
	case *BlockSync:
		return MsgBlockSyncReqType, nil
	case *BlockContainer:
		return MsgBlockSyncDataType, nil
	case *ActionPb:
		return MsgActionType, nil
	case *TestPayload:
		return TestPayloadType, nil
	default:
		return UnknownProtoMsgType, errors.New("UnknownProtoMsgType proto message type")
	}
}

// TypifyProtoMsg unmarshal a proto message based on the given MessageType
func TypifyProtoMsg(tp uint32, msg []byte) (proto.Message, error) {
	var m proto.Message
	switch tp {
	case MsgTxProtoMsgType:
		m = &TxPb{}
	case MsgBlockProtoMsgType:
		m = &BlockPb{}
	case ViewChangeMsgType:
		m = &ViewChangeMsg{}
	case MsgBlockSyncReqType:
		m = &BlockSync{}
	case MsgBlockSyncDataType:
		m = &BlockContainer{}
	case MsgActionType:
		m = &ActionPb{}
	case TestPayloadType:
		m = &TestPayload{}
	default:
		return nil, errors.New("UnknownProtoMsgType proto message type")
	}

	err := proto.Unmarshal(msg, m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

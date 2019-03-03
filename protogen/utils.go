// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protogen

import (
	"errors"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/protogen/iotexrpc"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"github.com/iotexproject/iotex-core/protogen/testingpb"
)

// Magic header to identify IoTeX traffic
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
	// MsgBlockSyncReqType is for requests among peers to sync blocks
	MsgBlockSyncReqType uint32 = 3
	// MsgBlockSyncDataType is the response to messages of type MsgBlockSyncReqType
	MsgBlockSyncDataType uint32 = 4
	// MsgActionType is the action message
	MsgActionType uint32 = 5
	// MsgConsensusType is for consensus message
	MsgConsensusType uint32 = 6
	// TestPayloadType is a test payload message type
	TestPayloadType uint32 = 10001
)

// GetTypeFromProtoMsg retrieves the proto message type
func GetTypeFromProtoMsg(msg proto.Message) (uint32, error) {
	switch msg.(type) {
	case *iotextypes.Block:
		return MsgBlockProtoMsgType, nil
	case *iotexrpc.BlockSync:
		return MsgBlockSyncReqType, nil
	case *iotexrpc.BlockContainer:
		return MsgBlockSyncDataType, nil
	case *iotextypes.Action:
		return MsgActionType, nil
	case *iotexrpc.Consensus:
		return MsgConsensusType, nil
	case *testingpb.TestPayload:
		return TestPayloadType, nil
	default:
		return UnknownProtoMsgType, errors.New("UnknownProtoMsgType proto message type")
	}
}

// TypifyProtoMsg unmarshal a proto message based on the given MessageType
func TypifyProtoMsg(tp uint32, msg []byte) (proto.Message, error) {
	var m proto.Message
	switch tp {
	case MsgBlockProtoMsgType:
		m = &iotextypes.Block{}
	case MsgConsensusType:
		m = &iotexrpc.Consensus{}
	case MsgBlockSyncReqType:
		m = &iotexrpc.BlockSync{}
	case MsgBlockSyncDataType:
		m = &iotexrpc.BlockContainer{}
	case MsgActionType:
		m = &iotextypes.Action{}
	case TestPayloadType:
		m = &testingpb.TestPayload{}
	default:
		return nil, errors.New("UnknownProtoMsgType proto message type")
	}

	err := proto.Unmarshal(msg, m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

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

// GetTypeFromRPCMsg retrieves the proto message type
func GetTypeFromRPCMsg(msg proto.Message) (iotexrpc.MessageType, error) {
	switch msg.(type) {
	case *iotextypes.Block:
		return iotexrpc.MessageType_BLOCK, nil
	case *iotexrpc.BlockSync:
		return iotexrpc.MessageType_BLOCK_REQUEST, nil
	case *iotextypes.Action:
		return iotexrpc.MessageType_ACTION, nil
	case *iotextypes.ConsensusMessage:
		return iotexrpc.MessageType_CONSENSUS, nil
	case *testingpb.TestPayload:
		return iotexrpc.MessageType_TEST, nil
	default:
		return iotexrpc.MessageType_UNKNOWN, errors.New("unknown RPC message type")
	}
}

// TypifyRPCMsg unmarshal a proto message based on the given MessageType
func TypifyRPCMsg(t iotexrpc.MessageType, msg []byte) (proto.Message, error) {
	var m proto.Message
	switch t {
	case iotexrpc.MessageType_BLOCK:
		m = &iotextypes.Block{}
	case iotexrpc.MessageType_CONSENSUS:
		m = &iotextypes.ConsensusMessage{}
	case iotexrpc.MessageType_BLOCK_REQUEST:
		m = &iotexrpc.BlockSync{}
	case iotexrpc.MessageType_ACTION:
		m = &iotextypes.Action{}
	case iotexrpc.MessageType_TEST:
		m = &testingpb.TestPayload{}
	default:
		return nil, errors.New("unknown RPC message type")
	}

	err := proto.Unmarshal(msg, m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

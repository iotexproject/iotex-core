// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package systemlog

import (
	"math/big"

	"github.com/iotexproject/iotex-core/pkg/log"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockindex/systemlog/systemlogpb"

	"github.com/gogo/protobuf/proto"
	"github.com/iotexproject/iotex-core/action/protocol/execution/executionpb"
)

const (
	evmTransfer = iota
)

// SystemLog is the log only stored locally
type SystemLog struct {
	LogType int
	LogData interface{}
}

// EvmTransfer records a transfer executed in contract
type EvmTransfer struct {
	Amount *big.Int
	From   string
	To     string
}

// ConvertToSystemLogPb converts a SystemLog to protobuf's SystemLog
func (sl *SystemLog) ConvertToSystemLogPb() *systemlogpb.SystemLog {
	pb := &systemlogpb.SystemLog{}
	pb.LogType = int32(sl.LogType)
	switch sl.LogType {
	case evmTransfer:
		et, ok := sl.LogData.(*EvmTransfer)
		if !ok {
			log.L().Panic("Failed to convert evm transfer.")
		}
		data, err := et.Serialize()
		if err != nil {
			log.L().Error("Failed to serialize evm transfer.", zap.Error(err))
		}
		pb.LogData = data
	}
	return pb
}

// ConvertFromSystemLogPb converts a protobuf's SystemLog to SystemLog
func (sl *SystemLog) ConvertFromSystemLogPb(pb *systemlogpb.SystemLog) {
	sl.LogType = int(pb.LogType)
	switch sl.LogType {
	case evmTransfer:
		et := &EvmTransfer{}
		if err := et.Deserialize(pb.LogData); err != nil {
			log.L().Error("Failed to deserialize evm transfer.", zap.Error(err))
		}
		sl.LogData = et
	}
}

// Serialize returns a serialized byte stream for the SystemLog
func (sl *SystemLog) Serialize() ([]byte, error) {
	return proto.Marshal(sl.ConvertToSystemLogPb())
}

// Deserialize parse the byte stream into SystemLog
func (sl *SystemLog) Deserialize(buf []byte) error {
	pb := &systemlogpb.SystemLog{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return err
	}
	sl.ConvertFromSystemLogPb(pb)
	return nil
}

// ConvertToEvmTransferPb converts a EvmTransfer to protobuf's EvmTransfer
func (et *EvmTransfer) ConvertToEvmTransferPb() *executionpb.EvmTransfer {
	pb := &executionpb.EvmTransfer{}
	pb.Amount = et.Amount.String()
	pb.From = et.From
	pb.To = et.To

	return pb
}

// ConvertFromEvmTransferPb converts a protobuf's EvmTransfer to EvmTransfer
func (et *EvmTransfer) ConvertFromEvmTransferPb(pb *executionpb.EvmTransfer) {
	et.Amount.SetString(pb.Amount, 10)
	et.From = pb.From
	et.To = pb.To
}

// Serialize returns a serialized byte stream for the EvmTransfer
func (et *EvmTransfer) Serialize() ([]byte, error) {
	return proto.Marshal(et.ConvertToEvmTransferPb())
}

// Deserialize parse the byte stream into EvmTransfer
func (et *EvmTransfer) Deserialize(buf []byte) error {
	pb := &executionpb.EvmTransfer{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return err
	}
	et.ConvertFromEvmTransferPb(pb)
	return nil
}

// LogEvmTransfer pack EvmTransfer into SystemLog
func LogEvmTransfer(et *EvmTransfer) *SystemLog {
	sl := &SystemLog{}
	sl.LogType = evmTransfer
	sl.LogData = et

	return sl
}

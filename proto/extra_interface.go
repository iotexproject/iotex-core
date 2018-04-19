/*
 * Add extra interface functions for proto messages
 */

package iproto

import (
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/txvm"
)

// TxInputFixedSize defines teh fixed size of transaction input
const TxInputFixedSize = 44

// TotalSize returns the total size (in bytes) of transaction input
func (in *TxInputPb) TotalSize() uint32 {
	return TxInputFixedSize + uint32(in.UnlockScriptSize)
}

// ByteStream returns a raw byte stream of transaction input
func (in *TxInputPb) ByteStream() []byte {
	stream := in.TxHash[:]

	temp := make([]byte, 4)
	cm.MachineEndian.PutUint32(temp, uint32(in.OutIndex))
	stream = append(stream, temp...)
	cm.MachineEndian.PutUint32(temp, in.UnlockScriptSize)
	stream = append(stream, temp...)
	stream = append(stream, in.UnlockScript...)
	cm.MachineEndian.PutUint32(temp, in.Sequence)
	stream = append(stream, temp...)

	return stream
}

// UnlockSuccess checks whether the TxInput can unlock the provided script
func (in *TxInputPb) UnlockSuccess(lockScript []byte) bool {
	v, err := txvm.NewIVM(in.ByteStream(), append(in.UnlockScript, lockScript...))
	if err != nil {
		return false
	}
	if err := v.Execute(); err != nil {
		return false
	}
	return true
}

// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"errors"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol/multichain/subchain/subchainpb"
)

// DepositIndex represents the deposit index
type DepositIndex byte

// Serialize serializes deposit index into bytes
func (di DepositIndex) Serialize() ([]byte, error) {
	return proto.Marshal(&subchainpb.Deposit{Index: []byte{byte(di)}})
}

// Deserialize deserializes bytes into deposit index
func (di *DepositIndex) Deserialize(data []byte) error {
	gen := &subchainpb.Deposit{}
	if err := proto.Unmarshal(data, gen); err != nil {
		return err
	}
	if len(gen.Index) == 0 {
		return errors.New("nil data")
	}
	*di = DepositIndex(gen.Index[0])
	return nil
}

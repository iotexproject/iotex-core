// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import "github.com/iotexproject/iotex-proto/golang/iotextypes"

// Deserializer de-serializes an action
//
// It's a wrapper to set certain parameters in order to correctly de-serialize an action
// Currently the parameter is EVM network ID for tx in web3 format, it is called like
//
// act, err := (&Deserializer{}).SetEvmNetworkID(id).ActionToSealedEnvelope(pbAction)
type Deserializer struct {
	evmNetworkID uint32
}

// SetEvmNetworkID sets the evm network ID for web3 actions
func (ad *Deserializer) SetEvmNetworkID(id uint32) *Deserializer {
	ad.evmNetworkID = id
	return ad
}

// ActionToSealedEnvelope converts protobuf to SealedEnvelope
func (ad *Deserializer) ActionToSealedEnvelope(pbAct *iotextypes.Action) (SealedEnvelope, error) {
	var selp SealedEnvelope
	err := selp.loadProto(pbAct, ad.evmNetworkID)
	return selp, err
}

// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// Deserializer de-serializes an action
type Deserializer struct {
	withChainID bool
}

// ActionToSealedEnvelope converts protobuf to SealedEnvelope
func (ad *Deserializer) ActionToSealedEnvelope(pbAct *iotextypes.Action) (SealedEnvelope, error) {
	var (
		selp SealedEnvelope
		err  error
	)
	if ad.withChainID {
		err = selp.LoadProtoWithChainID(pbAct)
	} else {
		err = selp.LoadProto(pbAct)
	}
	return selp, err
}

// WithChainID sets whether or not to use chainID
func (ad *Deserializer) WithChainID(with bool) *Deserializer {
	ad.withChainID = with
	return ad
}

// ActionToEnvelope converts protobuf to Envelope
func (ad *Deserializer) ActionToEnvelope(pbActCore *iotextypes.ActionCore) (Envelope, error) {
	var (
		elp = &envelope{}
		err error
	)
	if ad.withChainID {
		err = elp.LoadProtoWithChainID(pbActCore)
	} else {
		err = elp.LoadProto(pbActCore)
	}
	return elp, err
}

// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	// BucketGuardEventSignature is the derivation string for the QSB event
	// topic. The event data is not ABI-encoded.
	BucketGuardEventSignature = "BucketGuard(uint64,uint64,uint64,uint64,address,address)"

	// Event op values. BIND is 0; guarded ops use the IIP op tag.
	BucketGuardOpBind            = uint64(0)
	BucketGuardOpUnstake         = uint64(1)
	BucketGuardOpWithdraw        = uint64(2)
	BucketGuardOpTransfer        = uint64(3)
	BucketGuardOpChangeCandidate = uint64(4)
	BucketGuardOpRestake         = uint64(5)
	BucketGuardOpRotate          = uint64(6)
)

var _bucketGuardEventTopic = hash.Hash256(crypto.Keccak256Hash([]byte(BucketGuardEventSignature)))

// PackBucketGuardEvent packs one QSB guard event. sponsor and counterparty
// must be 20-byte native addresses; counterparty is zero for ops that have no
// counterparty.
func PackBucketGuardEvent(
	bucketIndex, op, generation, nonce uint64,
	sponsor, counterparty []byte,
) (Topics, []byte, error) {
	if len(sponsor) != 20 {
		return nil, nil, errors.Errorf("sponsor must be 20 bytes, got %d", len(sponsor))
	}
	if len(counterparty) != 20 {
		return nil, nil, errors.Errorf("counterparty must be 20 bytes, got %d", len(counterparty))
	}

	topics := make(Topics, 3)
	topics[0] = _bucketGuardEventTopic
	topics[1] = uint64Topic(bucketIndex)
	topics[2] = uint64Topic(op)

	data := make([]byte, 0, 56)
	data = append(data, byteutil.Uint64ToBytesBigEndian(generation)...)
	data = append(data, byteutil.Uint64ToBytesBigEndian(nonce)...)
	data = append(data, sponsor...)
	data = append(data, counterparty...)
	return topics, data, nil
}

func uint64Topic(v uint64) hash.Hash256 {
	var topic hash.Hash256
	copy(topic[24:], byteutil.Uint64ToBytesBigEndian(v))
	return topic
}

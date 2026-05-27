// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BLSEndorsement is an endorsement signed with a BLS12-381 private key.
//
// Unlike Endorsement, which embeds the endorser's secp256k1 public key (and
// derives the iotex address from it), a BLSEndorsement carries the endorser's
// iotex address directly. Receivers look up the registered BLS public key for
// that address from candidate state to verify the signature, mirroring how
// the BlockFooter aggregate is verified against the epoch delegate bitmap.
//
// Used for COMMIT-stage consensus votes once BLS signature aggregation is
// activated (IIP-52).
type BLSEndorsement struct {
	ts        time.Time
	endorser  address.Address
	signature []byte
}

// NewBLSEndorsement creates a BLSEndorsement.
func NewBLSEndorsement(
	ts time.Time,
	endorser address.Address,
	sig []byte,
) *BLSEndorsement {
	cs := make([]byte, len(sig))
	copy(cs, sig)
	return &BLSEndorsement{
		ts:        ts.UTC(),
		endorser:  endorser,
		signature: cs,
	}
}

// EndorseBLS signs the document with a BLS12-381 private key. The endorser
// address argument is the delegate's iotex address (the BLS key derivation
// and the registered candidate identity are wired up by the caller, not by
// this package).
func EndorseBLS(
	doc Document,
	ts time.Time,
	endorser address.Address,
	signer *crypto.BLS12381PrivateKey,
) (*BLSEndorsement, error) {
	hash, err := hashDocWithTime(doc, ts)
	if err != nil {
		return nil, err
	}
	sig, err := signer.Sign(hash)
	if err != nil {
		return nil, err
	}
	return NewBLSEndorsement(ts, endorser, sig), nil
}

// VerifyBLSEndorsement checks the BLS signature in the endorsement against
// the document using the supplied public key. Callers are responsible for
// resolving pubKey from the endorser's iotex address via candidate state.
func VerifyBLSEndorsement(doc Document, en *BLSEndorsement, pubKey *crypto.BLS12381PublicKey) bool {
	if en == nil || pubKey == nil {
		return false
	}
	hash, err := hashDocWithTime(doc, en.ts)
	if err != nil {
		return false
	}
	return pubKey.Verify(hash, en.signature)
}

// Timestamp returns the signature time.
func (en *BLSEndorsement) Timestamp() time.Time { return en.ts }

// Endorser returns the endorser's iotex address.
func (en *BLSEndorsement) Endorser() address.Address { return en.endorser }

// Signature returns a copy of the BLS signature bytes.
func (en *BLSEndorsement) Signature() []byte {
	signature := make([]byte, len(en.signature))
	copy(signature, en.signature)
	return signature
}

// Proto converts to the iotextypes.BLSEndorsement protobuf message.
func (en *BLSEndorsement) Proto() *iotextypes.BLSEndorsement {
	return &iotextypes.BLSEndorsement{
		Timestamp: timestamppb.New(en.ts),
		Endorser:  en.endorser.Bytes(),
		Signature: en.Signature(),
	}
}

// LoadProto populates en from a protobuf message.
func (en *BLSEndorsement) LoadProto(pb *iotextypes.BLSEndorsement) error {
	if pb == nil {
		return errors.New("nil BLSEndorsement proto")
	}
	if err := pb.GetTimestamp().CheckValid(); err != nil {
		return err
	}
	endorser, err := address.FromBytes(pb.GetEndorser())
	if err != nil {
		return errors.Wrap(err, "invalid endorser address in BLSEndorsement")
	}
	if len(pb.GetSignature()) != crypto.BLSAggregateSignatureLength {
		return errors.Errorf(
			"invalid BLS signature length: got %d, want %d",
			len(pb.GetSignature()), crypto.BLSAggregateSignatureLength,
		)
	}
	en.ts = pb.GetTimestamp().AsTime()
	en.endorser = endorser
	en.signature = make([]byte, len(pb.GetSignature()))
	copy(en.signature, pb.GetSignature())
	return nil
}

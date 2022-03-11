// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type (
	// Document defines a signable docuement
	Document interface {
		Hash() ([]byte, error)
	}

	// Endorsement defines an endorsement with timestamp
	Endorsement struct {
		ts        time.Time
		endorser  crypto.PublicKey
		signature []byte
	}

	// EndorsedDocument is an signed document
	EndorsedDocument interface {
		Document() Document
		Endorsement() *Endorsement
	}
)

func hashDocWithTime(doc Document, ts time.Time) ([]byte, error) {
	h, err := doc.Hash()
	if err != nil {
		return nil, err
	}
	h = append(h, byteutil.Uint64ToBytes(uint64(ts.Unix()))...)
	h256 := hash.Hash256b(append(h, byteutil.Uint32ToBytes(uint32(ts.Nanosecond()))...))

	return h256[:], nil
}

// NewEndorsement creates a new Endorsement
func NewEndorsement(
	ts time.Time,
	endorserPubKey crypto.PublicKey,
	sig []byte,
) *Endorsement {
	cs := make([]byte, len(sig))
	copy(cs, sig)
	return &Endorsement{
		ts:        ts.UTC(),
		endorser:  endorserPubKey,
		signature: cs,
	}
}

// Endorse endorses a document
func Endorse(
	signer crypto.PrivateKey,
	doc Document,
	ts time.Time,
) (*Endorsement, error) {
	hash, err := hashDocWithTime(doc, ts)
	if err != nil {
		return nil, err
	}
	sig, err := signer.Sign(hash)
	if err != nil {
		return nil, err
	}
	return NewEndorsement(ts, signer.PublicKey(), sig), nil
}

// VerifyEndorsedDocument checks an endorsed document
func VerifyEndorsedDocument(endorsedDoc EndorsedDocument) bool {
	return VerifyEndorsement(endorsedDoc.Document(), endorsedDoc.Endorsement())
}

// VerifyEndorsement checks the signature in an endorsement against a document
func VerifyEndorsement(doc Document, en *Endorsement) bool {
	hash, err := hashDocWithTime(doc, en.Timestamp())
	if err != nil {
		return false
	}

	return en.Endorser().Verify(hash, en.Signature())
}

// Timestamp returns the signature time
func (en *Endorsement) Timestamp() time.Time {
	return en.ts
}

// Endorser returns the endorser's public key
func (en *Endorsement) Endorser() crypto.PublicKey {
	return en.endorser
}

// Signature returns the signature of this endorsement
func (en *Endorsement) Signature() []byte {
	signature := make([]byte, len(en.signature))
	copy(signature, en.signature)

	return signature
}

// Proto converts an endorsement to protobuf message
func (en *Endorsement) Proto() (*iotextypes.Endorsement, error) {
	ts := timestamppb.New(en.ts)
	return &iotextypes.Endorsement{
		Timestamp: ts,
		Endorser:  en.endorser.Bytes(),
		Signature: en.Signature(),
	}, nil
}

// LoadProto converts a protobuf message to endorsement
func (en *Endorsement) LoadProto(ePb *iotextypes.Endorsement) (err error) {
	if err = ePb.Timestamp.CheckValid(); err != nil {
		return err
	}
	eb := make([]byte, len(ePb.Endorser))
	copy(eb, ePb.Endorser)
	if en.endorser, err = crypto.BytesToPublicKey(eb); err != nil {
		return err
	}
	en.signature = make([]byte, len(ePb.Signature))
	copy(en.signature, ePb.Signature)

	return nil
}

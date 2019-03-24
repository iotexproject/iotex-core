// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"time"

	"github.com/golang/protobuf/ptypes"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

type (
	// Document defines a signable docuement
	Document interface {
		Hash() ([]byte, error)
	}

	// Endorsement defines an endorsement with timestamp
	Endorsement struct {
		ts        time.Time
		endorser  keypair.PublicKey
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
	endorserPubKey keypair.PublicKey,
	sig []byte,
) *Endorsement {
	cs := make([]byte, len(sig))
	copy(cs, sig)
	return &Endorsement{
		ts:        ts,
		endorser:  endorserPubKey,
		signature: cs,
	}
}

// Endorse endorses a document
func Endorse(
	signer keypair.PrivateKey,
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
func (en *Endorsement) Endorser() keypair.PublicKey {
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
	ts, err := ptypes.TimestampProto(en.ts)
	if err != nil {
		return nil, err
	}
	return &iotextypes.Endorsement{
		Timestamp: ts,
		Endorser:  en.endorser.Bytes(),
		Signature: en.Signature(),
	}, nil
}

// LoadProto converts a protobuf message to endorsement
func (en *Endorsement) LoadProto(ePb *iotextypes.Endorsement) (err error) {
	if en.ts, err = ptypes.Timestamp(ePb.Timestamp); err != nil {
		return
	}
	eb := make([]byte, len(ePb.Endorser))
	copy(eb, ePb.Endorser)
	if en.endorser, err = keypair.BytesToPublicKey(eb); err != nil {
		return
	}
	en.signature = make([]byte, len(ePb.Signature))
	copy(en.signature, ePb.Signature)

	return
}

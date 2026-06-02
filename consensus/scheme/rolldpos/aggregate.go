// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/endorsement"
)

// aggregateCommitEndorsements aggregates the per-delegate BLS COMMIT
// signatures in endorsements into a single BLS12-381 aggregate signature
// and builds a bitmap identifying which entries of delegates contributed.
//
// Bit i (LSB-first within each byte) of the returned bitmap is set when
// delegates[i] is the endorser of one of the input endorsements. The
// bitmap length is ceil(len(delegates) / 8).
//
// Each endorsement must carry a BLS signature (len(en.Signature()) ==
// crypto.BLSAggregateSignatureLength) and its endorser address must appear
// in delegates; otherwise the endorsement is rejected. Callers are
// responsible for ensuring that all signers signed the same message — that
// is the case for the COMMIT-vote path since the timestamp is a
// deterministic function of the round's start time and TTL configuration.
func aggregateCommitEndorsements(
	endorsements []*endorsement.Endorsement,
	delegates []*Delegate,
) (aggSig []byte, bitmap []byte, err error) {
	if len(endorsements) == 0 {
		return nil, nil, errors.New("no COMMIT endorsements to aggregate")
	}
	addrIdx := make(map[string]int, len(delegates))
	for i, d := range delegates {
		addrIdx[d.Address] = i
	}
	bitmap = make([]byte, (len(delegates)+7)/8)
	sigs := make([][]byte, 0, len(endorsements))
	for _, en := range endorsements {
		if len(en.Signature()) != crypto.BLSAggregateSignatureLength {
			return nil, nil, errors.Errorf(
				"non-BLS signature in COMMIT endorsement (len=%d, want %d)",
				len(en.Signature()), crypto.BLSAggregateSignatureLength,
			)
		}
		addr := en.Endorser().Address()
		if addr == nil {
			return nil, nil, errors.New("endorser address is nil")
		}
		idx, ok := addrIdx[addr.String()]
		if !ok {
			return nil, nil, errors.Errorf("endorser %s is not in the round's delegate set", addr.String())
		}
		bitmap[idx/8] |= 1 << uint(idx%8)
		sigs = append(sigs, en.Signature())
	}
	agg, err := crypto.NewBLSAggregateSignature(sigs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to aggregate BLS COMMIT signatures")
	}
	return agg.Bytes(), bitmap, nil
}

// bitmapSigners returns the addresses of the delegates whose bit is set in
// bitmap, in delegate-index order. Used by the verifier to reconstruct the
// signer set from a footer's signer_bitmap.
func bitmapSigners(bitmap []byte, delegates []*Delegate) ([]*Delegate, error) {
	out := make([]*Delegate, 0, len(delegates))
	for i, d := range delegates {
		if i/8 >= len(bitmap) {
			break
		}
		if bitmap[i/8]&(1<<uint(i%8)) != 0 {
			out = append(out, d)
		}
	}
	// Bits set beyond the delegate-list range are invalid — make sure the
	// bitmap doesn't claim signers that don't exist in this epoch.
	for i := len(delegates); i < len(bitmap)*8; i++ {
		if bitmap[i/8]&(1<<uint(i%8)) != 0 {
			return nil, errors.Errorf("signer bitmap has bit %d set beyond the delegate count (%d)", i, len(delegates))
		}
	}
	return out, nil
}

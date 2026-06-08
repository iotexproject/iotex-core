// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

// Verifier is the minimal contract a block-header producer public key must
// satisfy. The two production schemes — secp256k1 (crypto.PublicKey) and
// BLS12-381 (*crypto.BLS12381PublicKey) — both implement this interface
// without modification: Bytes and Verify exist on both today.
//
// Identity-derivation methods (Address, Hash, EcdsaPublicKey) are
// deliberately absent. BLS public keys have no iotex address — see the
// BLS Producer Identity follow-up to IIP-52 — and forcing them through
// the ECDSA-shaped address pipeline would invite silent truncation in
// hash.BytesToHash160 / common.BytesToAddress consumers. Code that needs
// the producer's string identity should call Header.ProducerAddress or
// Header.ProducerPubKey; code that wants the typed secp256k1 key should
// call Header.PublicKey and handle nil for BLS-signed headers.
type Verifier interface {
	Bytes() []byte
	Verify(msg, sig []byte) bool
}

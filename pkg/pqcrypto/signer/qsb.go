// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package signer

import (
	"crypto/hkdf"
	"crypto/sha512"
	"encoding/binary"
	"math"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// DefaultQSBScanWindow is the number of key indices, from 0, a restoring
// wallet scans by default for each vault index. It is a default, not a limit:
// key indices have no upper bound.
const DefaultQSBScanWindow = 1024

// DeriveQSBKey derives a bucket guard key per wallet profile QSB-KDF-v1
// (IIP-65 §12.1):
//
//	prk  = HKDF-Extract(salt = "IoTeX-PQ-Bucket-KDF-v1", IKM = bip39Seed)   // HMAC-SHA-512
//	okm  = HKDF-Expand(prk, "slh-dsa-sha2-192s" || u32be(vaultIndex) || u32be(keyIndex), 72)
//	key  = slh_keygen_internal(okm[0:24], okm[24:48], okm[48:72])
//
// bip39Seed is the 64-byte BIP-39 seed (PBKDF2 output, passphrase included),
// never a BIP-32 node or a secp256k1 key. vaultIndex is independent of BIP-44
// account numbering; it is 0 by default. This is a wallet interoperability
// rule, not consensus.
func DeriveQSBKey(bip39Seed []byte, vaultIndex, keyIndex uint32) (*PrivateKey, error) {
	prk, err := hkdf.Extract(sha512.New, bip39Seed, []byte("IoTeX-PQ-Bucket-KDF-v1"))
	if err != nil {
		return nil, err
	}
	info := []byte("slh-dsa-sha2-192s")
	info = binary.BigEndian.AppendUint32(info, vaultIndex)
	info = binary.BigEndian.AppendUint32(info, keyIndex)
	alg := pqcrypto.AlgSLHDSA192s
	okm, err := hkdf.Expand(sha512.New, prk, string(info), SeedSize(alg))
	if err != nil {
		return nil, err
	}
	return NewKeyFromSeeds(alg, okm)
}

// RecoverQSBKeyIndex scans key indices first..last of vaultIndex for the one
// whose public key equals target. It returns false if none matches.
func RecoverQSBKeyIndex(bip39Seed []byte, vaultIndex, first, last uint32, target *pqcrypto.PublicKey) (uint32, bool, error) {
	for i := first; i <= last; i++ {
		k, err := DeriveQSBKey(bip39Seed, vaultIndex, i)
		if err != nil {
			return 0, false, err
		}
		if k.PublicKey().Equal(target) {
			return i, true, nil
		}
		if i == math.MaxUint32 {
			break
		}
	}
	return 0, false, nil
}

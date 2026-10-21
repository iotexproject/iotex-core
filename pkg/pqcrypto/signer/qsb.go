// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package signer

import (
	"crypto/hkdf"
	"crypto/sha512"
	"encoding/binary"
	"fmt"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// MaxQSBKeyIndex is the highest key index of wallet profile QSB-KDF-v1.
// Recovery scans 0..MaxQSBKeyIndex for every account.
const MaxQSBKeyIndex = 1023

// DeriveQSBKey derives a bucket guard key per wallet profile QSB-KDF-v1
// (IIP "Quantum-Safe Staking Buckets" §12):
//
//	prk  = HKDF-Extract(salt = "IOTX-QSB-KDF-v1", IKM = bip39Seed)   // HMAC-SHA-512
//	okm  = HKDF-Expand(prk, "slh-dsa-sha2-192s" || u32be(account) || u32be(keyIndex), 72)
//	key  = slh_keygen_internal(okm[0:24], okm[24:48], okm[48:72])
//
// This is a wallet interoperability rule, not consensus.
func DeriveQSBKey(bip39Seed []byte, account, keyIndex uint32) (*PrivateKey, error) {
	if keyIndex > MaxQSBKeyIndex {
		return nil, fmt.Errorf("key index %d exceeds %d", keyIndex, MaxQSBKeyIndex)
	}
	prk, err := hkdf.Extract(sha512.New, bip39Seed, []byte("IOTX-QSB-KDF-v1"))
	if err != nil {
		return nil, err
	}
	info := []byte("slh-dsa-sha2-192s")
	info = binary.BigEndian.AppendUint32(info, account)
	info = binary.BigEndian.AppendUint32(info, keyIndex)
	alg := pqcrypto.AlgSLHDSA192s
	okm, err := hkdf.Expand(sha512.New, prk, string(info), SeedSize(alg))
	if err != nil {
		return nil, err
	}
	return NewKeyFromSeeds(alg, okm)
}

// RecoverQSBKeyIndex scans key indices 0..maxIndex of account for the one whose
// public key equals target. It returns false if none matches.
func RecoverQSBKeyIndex(bip39Seed []byte, account, maxIndex uint32, target *pqcrypto.PublicKey) (uint32, bool, error) {
	if maxIndex > MaxQSBKeyIndex {
		maxIndex = MaxQSBKeyIndex
	}
	for i := uint32(0); i <= maxIndex; i++ {
		k, err := DeriveQSBKey(bip39Seed, account, i)
		if err != nil {
			return 0, false, err
		}
		if k.PublicKey().Equal(target) {
			return i, true, nil
		}
	}
	return 0, false, nil
}

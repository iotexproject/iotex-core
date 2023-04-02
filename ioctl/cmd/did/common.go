// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package did

import (
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"
)

type (
	// Permit permit content for DID
	Permit struct {
		Separator  string `json:"DOMAIN_SEPARATOR"`
		PermitHash string `json:"permitHash"`
	}

	// Signature signature for typed message
	Signature struct {
		R string `json:"r"`
		S string `json:"s"`
		V uint64 `json:"v"`
	}

	// CreateRequest create DID request
	CreateRequest struct {
		Signature
		PublicKey string `json:"publicKey"`
	}
)

// GetPermit fetch DID permit from resolver
func GetPermit(endpoint, address string) (*Permit, error) {
	resp, err := http.Get(endpoint + "/did/" + address + "/permit")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data Permit
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// SignType sign typed message
func SignType(key *ecdsa.PrivateKey, separator, hash string) (*Signature, error) {
	separatorBytes, err := hexutil.Decode(separator)
	if err != nil {
		return nil, err
	}
	hashBytes, err := hexutil.Decode(hash)
	if err != nil {
		return nil, err
	}

	data := append([]byte{0x19, 0x01}, append(separatorBytes, hashBytes...)...)
	sha := sha3.NewLegacyKeccak256()
	sha.Write(data)
	signHash := sha.Sum(nil)

	sig, err := crypto.Sign(signHash, key)
	if err != nil {
		return nil, err
	}
	v := new(big.Int).SetBytes([]byte{sig[64] + 27})

	return &Signature{
		R: hexutil.Encode(sig[:32]),
		S: hexutil.Encode(sig[32:64]),
		V: v.Uint64(),
	}, nil
}

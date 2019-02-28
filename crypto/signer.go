// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"bytes"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/iotex-core/address"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// Signer defines the interface of signing a hash
type Signer interface {
	Sign(hash.Hash256) ([]byte, keypair.PublicKey, error)
}

type privateKeySigner struct {
	sk keypair.PrivateKey
}

// NewPrivateKeySigner constructs a signer from a private key
func NewPrivateKeySigner(sk keypair.PrivateKey) Signer {
	return &privateKeySigner{sk: sk}
}

func (s *privateKeySigner) Sign(h hash.Hash256) ([]byte, keypair.PublicKey, error) {
	sig, err := ethcrypto.Sign(h[:], s.sk)
	if err != nil {
		return nil, nil, err
	}
	pk, err := ethcrypto.SigToPub(h[:], sig)
	if err != nil {
		return nil, nil, err
	}
	return sig, pk, nil
}

type keystoreSigner struct {
	ks         *keystore.KeyStore
	addr       address.Address
	passphrase string
}

// NewKeystoreSigner contructs a signer from a key store
func NewKeystoreSigner(ks *keystore.KeyStore, addr address.Address, passphrase string) Signer {
	return &keystoreSigner{ks: ks, addr: addr, passphrase: passphrase}
}

func (s *keystoreSigner) Sign(h hash.Hash256) ([]byte, keypair.PublicKey, error) {
	for _, a := range s.ks.Accounts() {
		if bytes.Equal(s.addr.Bytes(), a.Address.Bytes()) {
			sig, err := s.ks.SignHashWithPassphrase(a, s.passphrase, h[:])
			if err != nil {
				return nil, nil, err
			}
			pk, err := ethcrypto.SigToPub(h[:], sig)
			if err != nil {
				return nil, nil, err
			}
			return sig, pk, nil
		}
	}
	return nil, nil, errors.Errorf("address %s isn't found int keystore", s.addr.String())
}

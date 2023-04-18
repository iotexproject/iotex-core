// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package did

import (
	"crypto/sha256"
	"encoding/json"

	"github.com/btcsuite/btcutil/base58"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

const (
	// DIDPrefix is the prefix string
	DIDPrefix = "did:io:"
	// DIDAuthType is the authentication type
	DIDAuthType = "EcdsaSecp256k1VerificationKey2019"
	// DIDOwner is the suffix string
	DIDOwner = "#owner"

	// KnownDIDContext known context for did
	KnownDIDContext = "https://www.w3.org/ns/did/v1"
	// Secp256k1DIDContext secp256k1 context for did
	Secp256k1DIDContext = "https://w3id.org/security/suites/secp256k1-2019/v1"
)

type (
	verificationMethod struct {
		ID              string `json:"id"`
		Type            string `json:"type"`
		Controller      string `json:"controller"`
		PublicKeyBase58 string `json:"publicKeyBase58,omitempty"`
	}

	verificationMethodSet interface{}

	serviceStruct struct {
		ID              string `json:"id,omitempty"`
		Type            string `json:"type,omitempty"`
		ServiceEndpoint string `json:"serviceEndpoint,omitempty"`
	}

	// Doc is the DID document struct
	Doc struct {
		Context            interface{}             `json:"@context,omitempty"`
		ID                 string                  `json:"id,omitempty"`
		Controller         string                  `json:"controller,omitempty"`
		VerificationMethod []verificationMethod    `json:"verificationMethod,omitempty"`
		Authentication     []verificationMethodSet `json:"authentication,omitempty"`
		AssertionMethod    []verificationMethodSet `json:"assertionMethod,omitempty"`
		Service            []serviceStruct         `json:"service,omitempty"`
	}
)

// Owner did document owner
func (doc *Doc) Owner() common.Address {
	return common.HexToAddress(doc.ID[7:])
}

// Bytes did document bytes
func (doc *Doc) Bytes() ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

// JSON did document json
func (doc *Doc) JSON() (string, error) {
	data, err := doc.Bytes()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Hash did document hash
func (doc *Doc) Hash() ([32]byte, error) {
	data, err := doc.Bytes()
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(data), nil
}

// AddService add service to did document
func (doc *Doc) AddService(tag, serviceType, endpoint string) {
	id := doc.ID + "#" + tag
	if doc.Service == nil {
		doc.Service = []serviceStruct{{
			ID:              id,
			Type:            serviceType,
			ServiceEndpoint: endpoint,
		}}
		return
	}
	for _, service := range doc.Service {
		if service.ID == id {
			service.Type = serviceType
			service.ServiceEndpoint = endpoint
			return
		}
	}
	doc.Service = append(doc.Service, serviceStruct{
		ID:              id,
		Type:            serviceType,
		ServiceEndpoint: endpoint,
	})
}

// RemoveService remove service from did document
func (doc *Doc) RemoveService(tag string) error {
	id := doc.ID + "#" + tag
	if doc.Service == nil || len(doc.Service) == 0 {
		return errors.New("service not exists")
	}
	services := make([]serviceStruct, len(doc.Service)-1)
	count := 0
	for i, service := range doc.Service {
		if service.ID != id {
			if count == len(services) {
				return errors.New("service not exists")
			}
			services[count] = doc.Service[i]
			count++
		}
	}
	doc.Service = services
	return nil
}

// NewDIDDoc new did document by public key
func NewDIDDoc(publicKey []byte) (*Doc, error) {
	pubKey, err := crypto.UnmarshalPubkey(publicKey)
	if err != nil {
		return nil, err
	}
	compressedPubKey := crypto.CompressPubkey(pubKey)
	address := crypto.PubkeyToAddress(*pubKey)
	if err != nil {
		return nil, err
	}

	doc := &Doc{
		Context: []string{
			KnownDIDContext,
			Secp256k1DIDContext,
		},
	}
	doc.ID = DIDPrefix + address.Hex()
	key0 := doc.ID + "#key-0"
	doc.VerificationMethod = []verificationMethod{{
		ID:              key0,
		Type:            DIDAuthType,
		Controller:      doc.ID,
		PublicKeyBase58: base58.Encode(compressedPubKey),
	}}
	doc.Authentication = []verificationMethodSet{key0}
	doc.AssertionMethod = []verificationMethodSet{key0}
	return doc, nil
}

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

func (doc *Doc) Owner() common.Address {
	return common.HexToAddress(doc.ID[7:])
}

func (doc *Doc) Bytes() ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

func (doc *Doc) Json() (string, error) {
	data, err := doc.Bytes()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (doc *Doc) Hash() ([32]byte, error) {
	data, err := doc.Bytes()
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(data), nil
}

func (doc *Doc) AddService(tag, serviceType, endpoint string) {
	id := doc.ID + "#" + tag
	if doc.Service == nil {
		doc.Service = []serviceStruct{{
			ID:              id,
			Type:            serviceType,
			ServiceEndpoint: endpoint,
		}}
	} else {
		found := false
		for _, service := range doc.Service {
			if service.ID == id {
				service.Type = serviceType
				service.ServiceEndpoint = endpoint
				found = true
			}
		}
		if !found {
			doc.Service = append(doc.Service, serviceStruct{
				ID:              id,
				Type:            serviceType,
				ServiceEndpoint: endpoint,
			})
		}
	}
}

func (doc *Doc) RemoveService(tag string) error {
	id := doc.ID + "#" + tag
	if doc.Service == nil {
		return errors.New("service not exists")
	}
	serices := make([]serviceStruct, len(doc.Service)-1)
	for i, service := range doc.Service {
		if service.ID != id {
			if i == len(serices) {
				return errors.New("service not exists")
			}
			serices[i] = doc.Service[i]
		}
	}
	doc.Service = serices
	return nil
}

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
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/suites/secp256k1-2019/v1",
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

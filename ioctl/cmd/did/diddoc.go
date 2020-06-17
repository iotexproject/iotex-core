// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

const (
	DIDPrefix   = "did:io:"
	DIDAuthType = "Secp256k1VerificationKey2018"
	DIDOwner    = "#owner"
)

type (
	authenticationStruct struct {
		Id           string `json:"id,omitempty"`
		Type         string `json:"type,omitempty"`
		Controller   string `json:"controller,omitempty"`
		PublicKeyHex string `json:"publicKeyHex,omitempty"`
	}
	DIDDoc struct {
		Context        string                 `json:"@context,omitempty"`
		Id             string                 `json:"id,omitempty"`
		Authentication []authenticationStruct `json:"authentication,omitempty"`
	}
)

func newDIDDoc() *DIDDoc {
	return &DIDDoc{
		Context: "https://www.w3.org/ns/did/v1",
	}
}

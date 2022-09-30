// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/spf13/cobra"
)

const (
	// DIDPrefix is the prefix string
	DIDPrefix = "did:io:"
	// DIDAuthType is the authentication type
	DIDAuthType = "EcdsaSecp256k1VerificationKey2019"
	// DIDOwner is the suffix string
	DIDOwner = "#owner"
)

// Multi-language support
var (
	_dIDDocCmdShorts = map[config.Language]string{
		config.English: "Manage DID Settings IoTeX blockchain",
		config.Chinese: "管理IoTeX区块链上的DID设定",
	}
)

type (
	authenticationStruct struct {
		ID           string `json:"id,omitempty"`
		Type         string `json:"type,omitempty"`
		Controller   string `json:"controller,omitempty"`
		PublicKeyHex string `json:"publicKeyHex,omitempty"`
	}
	// Doc is the DID document struct
	Doc struct {
		Context        string                 `json:"@context,omitempty"`
		ID             string                 `json:"id,omitempty"`
		Authentication []authenticationStruct `json:"authentication,omitempty"`
	}
)

// NewDidDocCmd represents the did doc command
func NewDidDocCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_dIDDocCmdShorts)
	cmd := &cobra.Command{
		Use:   "did doc",
		Short: short,
	}
	return cmd
}

func newDIDDoc() *Doc {
	return &Doc{
		Context: "https://www.w3.org/ns/did/v1",
	}
}

// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/account"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/action"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
)

// Multi-language support
var (
	_generateCmdShorts = map[config.Language]string{
		config.English: "Generate DID document using private key from wallet",
		config.Chinese: "用钱包中的私钥产生DID document",
	}
	_generateCmdUses = map[config.Language]string{
		config.English: "generate [-s SIGNER]",
		config.Chinese: "generate [-s 签署人]",
	}
)

// NewDidGenerateCmd represents the did generate command
func NewDidGenerateCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_generateCmdUses)
	short, _ := client.SelectTranslation(_generateCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			signer, err := cmd.Flags().GetString("signer")
			if err != nil {
				return errors.Wrap(err, "failed to get flag signer")
			}
			addr, err := action.Signer(client, signer)
			if err != nil {
				return errors.Wrap(err, "failed to get signer addr")
			}
			cmd.Printf("Enter password #%s:\n", addr)
			password, err := client.ReadSecret()
			if err != nil {
				return errors.Wrap(err, "failed to get password")
			}
			pri, err := account.PrivateKeyFromSigner(client, cmd, addr, password)
			if err != nil {
				return err
			}
			doc := newDIDDoc()
			ethAddress, err := addrutil.IoAddrToEvmAddr(addr)
			if err != nil {
				return errors.Wrap(err, "")
			}
			doc.ID = DIDPrefix + ethAddress.String()
			authentication := authenticationStruct{
				ID:         doc.ID + DIDOwner,
				Type:       DIDAuthType,
				Controller: doc.ID,
			}
			uncompressed := pri.PublicKey().Bytes()
			if len(uncompressed) == 33 && (uncompressed[0] == 2 || uncompressed[0] == 3) {
				authentication.PublicKeyHex = hex.EncodeToString(uncompressed)
			} else if len(uncompressed) == 65 && uncompressed[0] == 4 {
				lastNum := uncompressed[64]
				authentication.PublicKeyHex = hex.EncodeToString(uncompressed[1:33])
				if lastNum%2 == 0 {
					authentication.PublicKeyHex = "02" + authentication.PublicKeyHex
				} else {
					authentication.PublicKeyHex = "03" + authentication.PublicKeyHex
				}
			} else {
				return errors.New("invalid public key")
			}

			doc.Authentication = append(doc.Authentication, authentication)
			msg, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return errors.Wrap(err, "")
			}

			sum := sha256.Sum256(msg)
			generatedMessage := string(msg) + "\n\nThe hex encoded SHA256 hash of the DID doc is:" + hex.EncodeToString(sum[:])
			if err != nil {
				return errors.Wrap(err, "failed to sign message")
			}
			cmd.Println(generatedMessage)
			return nil
		},
	}
	action.RegisterWriteCommand(client, cmd)
	return cmd
}

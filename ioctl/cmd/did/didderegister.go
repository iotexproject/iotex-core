// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package did

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
)

// Multi-language support
var (
	_deregisterCmdUses = map[config.Language]string{
		config.English: "deregister (RESOLVER_ENDPOINT) [-s SIGNER]",
		config.Chinese: "deregister (Resolver端点) [-s 签署人]",
	}
	_deregisterCmdShorts = map[config.Language]string{
		config.English: "Deregister DID on IoTeX blockchain",
		config.Chinese: "Deregister 在IoTeX链上注销DID",
	}
)

// _didDeregisterCmd represents the contract invoke deregister command
var _didDeregisterCmd = &cobra.Command{
	Use:   config.TranslateInLang(_deregisterCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_deregisterCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := deregisterDID(args)
		return output.PrintError(err)
	},
}

func init() {
	action.RegisterWriteCommand(_didDeregisterCmd)
}

func deregisterDID(args []string) (err error) {
	signer, err := action.Signer()
	if err != nil {
		return output.NewError(output.InputError, "failed to get signer addr", err)
	}
	fmt.Printf("Enter password #%s:\n", signer)
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	pri, err := account.PrivateKeyFromSigner(signer, password)
	if err != nil {
		return output.NewError(output.InputError, "failed to decrypt key", err)
	}

	endpoint := args[0]
	ethAddress, err := addrutil.IoAddrToEvmAddr(signer)
	if err != nil {
		return output.NewError(output.AddressError, "", err)
	}
	permit, err := GetPermit(endpoint, ethAddress.String())
	if err != nil {
		return output.NewError(output.InputError, "failed to fetch permit", err)
	}
	signature, err := SignType(pri.EcdsaPrivateKey().(*ecdsa.PrivateKey), permit.Separator, permit.PermitHash)
	if err != nil {
		return output.NewError(output.InputError, "failed to sign typed permit", err)
	}

	deleteBytes, err := json.Marshal(&signature)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to encode request", err)
	}
	req, err := http.NewRequest("POST", endpoint+"/did/"+ethAddress.String()+"/delete", bytes.NewBuffer(deleteBytes))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to create request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return output.NewError(output.NetworkError, "failed to post request", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to read response", err)
	}
	output.PrintResult(string(body))

	return nil
}

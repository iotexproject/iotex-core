// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package did

import (
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/pkg/util/addrutil"
)

// Multi-language support
var (
	_getCmdUses = map[config.Language]string{
		config.English: "get (RESOLVER_ENDPOINT) ADDRESS",
		config.Chinese: "get (Resolver端点) ADDRESS",
	}
	_getCmdShorts = map[config.Language]string{
		config.English: "Get get DID Document on IoTeX blockchain",
		config.Chinese: "Get 在IoTeX链上获取相应DID的文档",
	}
)

// _didGetCmd represents the DID document for account
var _didGetCmd = &cobra.Command{
	Use:   config.TranslateInLang(_getCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_getCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(get(args))
	},
}

func get(args []string) (err error) {
	endpoint := args[0]
	address := args[1]
	if strings.HasPrefix(address, "io") {
		ethAddress, err := addrutil.IoAddrToEvmAddr(address)
		if err != nil {
			return output.NewError(output.AddressError, "", err)
		}
		address = ethAddress.String()
	}
	resp, err := http.Get(endpoint + "/did/" + address)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to get request", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to read response", err)
	}
	output.PrintResult(string(body))
	return
}

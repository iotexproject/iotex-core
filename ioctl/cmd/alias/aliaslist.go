// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// aliasListCmd represents the alias list command
var aliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List aliases",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		aliasList()
	},
}

type aliasListMessage struct {
	AliasNumber int         `json:"aliasNumber"`
	AliasList   []aliasMeta `json:"aliasList"`
}

type aliasMeta struct {
	Address string `json:"address"`
	Alias   string `json:"alias"`
}

func aliasList() {
	var keys []string
	for alias := range config.ReadConfig.Aliases {
		keys = append(keys, alias)
	}
	sort.Strings(keys)
	aliasListMessage := aliasListMessage{AliasNumber: len(keys)}
	for _, alias := range keys {
		aliasMeta := aliasMeta{Address: config.ReadConfig.Aliases[alias], Alias: alias}
		aliasListMessage.AliasList = append(aliasListMessage.AliasList, aliasMeta)
	}
	printAliasList(aliasListMessage)
}

func printAliasList(message aliasListMessage) {
	switch output.Format {
	default:
		for _, aliasMeta := range message.AliasList {
			fmt.Printf("%s - %s\n", aliasMeta.Address, aliasMeta.Alias)
		}
	case "json":
		out := output.Output{MessageType: output.Result, Message: message}
		byteAsJSON, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			log.Panic(err)
		}
		fmt.Println(string(byteAsJSON))
	}
}

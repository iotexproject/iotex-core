// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

//// nodeDelegateCmd represents the node delegate command
//var nodeDelegateCmd = &cobra.Command{
//	Use:   "delegate [ALIAS|DELEGATE_ADDRESS]",
//	Short: "list delegates and number of blocks produced",
//	Args:  cobra.MaximumNArgs(1),
//	Run: func(cmd *cobra.Command, args []string) {
//		fmt.Println(delegate(args))
//	},
//}
//
//func delegate(args []string) string {
//	delegate := ""
//	var err error
//	if len(args) != 0 {
//		delegate, err = alias.Address(args[0])
//		if err != nil {
//			return err.Error()
//		}
//	}
//	if epochNum == 0 {
//		chainMeta, err := bc.GetChainMeta()
//		if err != nil {
//			return err.Error()
//		}
//		epochNum = chainMeta.Epoch.Num
//	}
//	conn, err := util.ConnectToEndpoint()
//	if err != nil {
//		return err.Error()
//	}
//	defer conn.Close()
//	cli := iotexapi.NewAPIServiceClient(conn)
//	request := &iotexapi.GetEpochMetaRequest{EpochNumber: epochNum}
//	ctx := context.Background()
//	epochResponse, err := cli.GetEpochMeta(ctx, request)
//	if err != nil {
//		return err.Error()
//	}
//	response := epochResponse.Productivity
//	if len(delegate) != 0 {
//		delegateAlias, err := alias.Alias(delegate)
//		if err != nil && err != alias.ErrNoAliasFound {
//			return err.Error()
//		}
//		return fmt.Sprintf("Epoch: %d, Total blocks: %d\n", epochNum, response.TotalBlks) +
//			fmt.Sprintf("%s  %s  %d", delegate, delegateAlias, response.BlksPerDelegate[delegate])
//	}
//
//	aliases := alias.GetAliasMap()
//	formataliasLen := 0
//	for delegate := range response.BlksPerDelegate {
//		if len(aliases[delegate]) > formataliasLen {
//			formataliasLen = len(aliases[delegate])
//		}
//	}
//	lines := make([]string, 0)
//	lines = append(lines, fmt.Sprintf("Epoch: %d, Total blocks: %d\n",
//		epochNum, response.TotalBlks))
//	if formataliasLen == 0 {
//		lines = append(lines, fmt.Sprintf("%-41s  %s", "Address", "Blocks"))
//		for delegate, productivity := range response.BlksPerDelegate {
//			lines = append(lines, fmt.Sprintf("%-41s  %d", delegate, productivity))
//		}
//	} else {
//		formatTitleString := "%-41s  %-" + strconv.Itoa(formataliasLen) + "s  %s"
//		formatDataString := "%-41s  %-" + strconv.Itoa(formataliasLen) + "s  %d"
//		lines = append(lines, fmt.Sprintf(formatTitleString, "Address", "Alias", "Blocks"))
//		for delegate, productivity := range response.BlksPerDelegate {
//			lines = append(lines, fmt.Sprintf(formatDataString, delegate, aliases[delegate], productivity))
//		}
//	}
//	return strings.Join(lines, "\n")
//}

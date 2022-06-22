package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/tools/iomigrater/common"
)

// Multi-language support
var (
	checkHeightCmdShorts = map[string]string{
		"english": "Sub-Command for check IoTeX blockchain db file height.",
		"chinese": "查看IoTeX区块链 db 文件高度的子命令",
	}
	checkHeightCmdLongs = map[string]string{
		"english": "Sub-command for check IoTeX blockchain db file height.",
		"chinese": "查看IoTeX区块链 db 文件高度的子命令",
	}
	checkHeightCmdUse = map[string]string{
		"english": "check",
		"chinese": "check",
	}
)

var (
	// CheckHeight used to Sub command.
	CheckHeight = &cobra.Command{
		Use:   common.TranslateInLang(checkHeightCmdUse),
		Short: common.TranslateInLang(checkHeightCmdShorts),
		Long:  common.TranslateInLang(checkHeightCmdLongs),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			height, err := checkDbFileHeight(args[0])
			if err != nil {
				fmt.Printf("Check db %s height err: %v\n", args[0], err)
				return err
			}
			fmt.Printf("Check db %s height: %d.\n", args[0], height)
			return nil
		},
	}
)

func checkDbFileHeight(filePath string) (uint64, error) {
	cfg, err := config.New([]string{}, []string{})
	if err != nil {
		return uint64(0), fmt.Errorf("failed to new config: %v", err)
	}

	cfg.DB.DbPath = filePath
	cfg.DB.CompressLegacy = cfg.Chain.CompressBlock
	daoCfg, _ := blockdao.CreateModuleConfig(cfg.DB)
	blockDao := blockdao.NewBlockDAO(nil, daoCfg)

	// Load height value.
	ctx := context.Background()
	if err := blockDao.Start(ctx); err != nil {
		return uint64(0), err
	}
	height, err := blockDao.Height()
	if err != nil {
		return uint64(0), err
	}
	if err := blockDao.Stop(ctx); err != nil {
		return uint64(0), err
	}

	return height, nil
}

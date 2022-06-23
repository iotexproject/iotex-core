package cmd

import (
	"context"
	"fmt"

	"github.com/schollz/progressbar/v2"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/tools/iomigrater/common"
)

const (
	// INTMAX Max value of type int.
	INTMAX = int(^uint(0) >> 1)
)

// Multi-language support
var (
	migrateDbCmdShorts = map[string]string{
		"english": "Sub-Command for migration IoTeX blockchain db file.",
		"chinese": "迁移IoTeX区块链 db 文件的子命令",
	}
	migrateDbCmdLongs = map[string]string{
		"english": "Sub-Command for migration IoTeX blockchain db file from 0 to Specified height.",
		"chinese": "迁移IoTeX区块链 db 文件从 0 到指定的高度的子命令",
	}
	migrateDbCmdUse = map[string]string{
		"english": "migrate",
		"chinese": "migrate",
	}
	migrateDbFlagOldFileUse = map[string]string{
		"english": "The file you want to migrate.",
		"chinese": "您要迁移的文件。",
	}
	migrateDbFlagNewFileUse = map[string]string{
		"english": "The path you want to migrate to",
		"chinese": "您要迁移到的路径。",
	}
	migrateDbFlagBlockHeightUse = map[string]string{
		"english": "The height you want to migrate to, Cannot be larger than the height of the migration file (the old file)",
		"chinese": "您要迁移到的高度，不能大于迁移文件（旧文件）的高度。",
	}
)

var (
	// MigrateDb Used to Sub command.
	MigrateDb = &cobra.Command{
		Use:   common.TranslateInLang(migrateDbCmdUse),
		Short: common.TranslateInLang(migrateDbCmdShorts),
		Long:  common.TranslateInLang(migrateDbCmdLongs),
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateDbFile()
		},
	}
)

var (
	oldFile     = ""
	newFile     = ""
	blockHeight = uint64(0)
)

func init() {
	MigrateDb.PersistentFlags().StringVarP(&oldFile, "old-file", "o", "", common.TranslateInLang(migrateDbFlagOldFileUse))
	MigrateDb.PersistentFlags().StringVarP(&newFile, "new-file", "n", "", common.TranslateInLang(migrateDbFlagNewFileUse))
	MigrateDb.PersistentFlags().Uint64VarP(&blockHeight, "block-height", "b", uint64(0), common.TranslateInLang(migrateDbFlagBlockHeightUse))
}

func getProgressMod(num uint64) (int, int) {
	step := 1
	for num/2 >= uint64(INTMAX) {
		step *= 2
		num /= 2
	}
	numInt := int(num)

	for numInt/10 > 100 {
		numInt /= 10
		step *= 10
	}

	return numInt, step
}

func migrateDbFile() error {
	// Check flags
	if oldFile == "" {
		return fmt.Errorf("--old-file is empty")
	}
	if newFile == "" {
		return fmt.Errorf("--new-file is empty")
	}
	if oldFile == newFile {
		return fmt.Errorf("the values of --old-file --new-file flags cannot be the same")
	}
	if blockHeight == 0 {
		return fmt.Errorf("--block-height is 0")
	}

	height, err := checkDbFileHeight(oldFile)
	if err != nil {
		return err
	} else if height < blockHeight {
		return fmt.Errorf("the --block-height cannot be larger than the height of the migration file")
	}

	cfg, err := config.New([]string{}, []string{})
	if err != nil {
		return fmt.Errorf("failed to new config: %v", err)
	}

	cfg.DB.DbPath = oldFile
	cfg.DB.CompressLegacy = cfg.Chain.CompressBlock
	daoCfg, _ := blockdao.CreateConfig(cfg.DB)
	oldDAO := blockdao.NewBlockDAO(nil, daoCfg)

	cfg.DB.DbPath = newFile
	daoCfg, _ = blockdao.CreateConfig(cfg.DB)
	newDAO := blockdao.NewBlockDAO(nil, daoCfg)

	ctx := context.Background()
	if err := oldDAO.Start(ctx); err != nil {
		return fmt.Errorf("failed to start the old db file")
	}
	if err := newDAO.Start(ctx); err != nil {
		return fmt.Errorf("failed to start the new db file")
	}

	defer func() {
		oldDAO.Stop(ctx)
		newDAO.Stop(ctx)
	}()

	// Show the progressbar
	intHeight, step := getProgressMod(blockHeight)
	bar := progressbar.New(intHeight)

	for i := uint64(1); i <= blockHeight; i++ {

		hash, err := oldDAO.GetBlockHash(i)
		if err != nil {
			return fmt.Errorf("failed to get block hash on height %d: %v", i, err)
		}
		blk, err := oldDAO.GetBlock(hash)
		if err != nil {
			return fmt.Errorf("failed to get block on height %d: %v", i, err)
		}
		if err := newDAO.PutBlock(ctx, blk); err != nil {
			return fmt.Errorf("failed to migrate block on height %d: %v", i, err)
		}

		if i%uint64(step) == 0 {
			bar.Add(1)
			intHeight--
		}
	}
	if intHeight > 0 {
		bar.Add(intHeight)
	}

	return nil
}

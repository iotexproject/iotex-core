package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/state/factory/erigonstore"
	"github.com/iotexproject/iotex-core/v2/tools/iomigrater/common"
)

// Multi-language support
var (
	pruneCmdShorts = map[string]string{
		"english": "Sub-Command for prune ErigonDB.",
		"chinese": "清理ErigonDB的子命令",
	}
	pruneCmdLongs = map[string]string{
		"english": "Sub-command for prune ErigonDB, only keep specified number of blocks.",
		"chinese": "清理ErigonDB的子命令，仅保留指定数量的区块。",
	}
	pruneCmdUse = map[string]string{
		"english": "prune",
		"chinese": "prune",
	}
	pruneFlagDbPathUse = map[string]string{
		"english": "The path to ErigonDB.",
		"chinese": "ErigonDB的路径。",
	}
	pruneFlagRetainBlocksUse = map[string]string{
		"english": "The number of blocks to retain.",
		"chinese": "要保留的区块数量。",
	}
	pruneFlagBatchSizeUse = map[string]string{
		"english": "The batch size for pruning (default: 1000).",
		"chinese": "清理的批次大小（默认：1000）。",
	}
)

var (
	// Prune used to Sub command.
	Prune = &cobra.Command{
		Use:   common.TranslateInLang(pruneCmdUse),
		Short: common.TranslateInLang(pruneCmdShorts),
		Long:  common.TranslateInLang(pruneCmdLongs),
		RunE: func(_ *cobra.Command, _ []string) error {
			return pruneErigonDB()
		},
	}
)

var (
	dbPath       = ""
	retainBlocks = uint64(0)
	batchSize    = uint64(1000)
)

func init() {
	Prune.PersistentFlags().StringVarP(&dbPath, "db-path", "d", "", common.TranslateInLang(pruneFlagDbPathUse))
	Prune.PersistentFlags().Uint64VarP(&retainBlocks, "retain-blocks", "r", uint64(0), common.TranslateInLang(pruneFlagRetainBlocksUse))
	Prune.PersistentFlags().Uint64VarP(&batchSize, "batch-size", "b", uint64(1000), common.TranslateInLang(pruneFlagBatchSizeUse))
}

func pruneErigonDB() error {
	// Check flags
	if dbPath == "" {
		return fmt.Errorf("--db-path is empty")
	}
	if retainBlocks == 0 {
		return fmt.Errorf("--retain-blocks must be greater than 0")
	}
	if batchSize == 0 {
		return fmt.Errorf("--batch-size must be greater than 0")
	}

	fmt.Printf("Starting ErigonDB prune...\n")
	fmt.Printf("  DB Path: %s\n", dbPath)
	fmt.Printf("  Retain Blocks: %d\n", retainBlocks)
	fmt.Printf("  Batch Size: %d\n", batchSize)

	// Create ErigonDB instance
	db := erigonstore.NewErigonDB(dbPath)

	// Start the database
	ctx := context.Background()
	if err := db.Start(ctx); err != nil {
		return fmt.Errorf("failed to start ErigonDB: %w", err)
	}
	defer db.Stop(ctx)

	// Get current height
	currentHeight, err := db.Height()
	if err != nil {
		return fmt.Errorf("failed to get ErigonDB height: %w", err)
	}
	fmt.Printf("  Current Height: %d\n", currentHeight)

	// Calculate prune range
	if currentHeight <= retainBlocks {
		fmt.Printf("No need to prune, current height (%d) <= retain blocks (%d)\n", currentHeight, retainBlocks)
		return nil
	}

	pruneFrom := currentHeight - retainBlocks
	pruneTo := currentHeight

	fmt.Printf("  Prune From: %d\n", pruneFrom)
	fmt.Printf("  Prune To: %d\n", pruneTo)
	fmt.Printf("\nPruning ErigonDB...\n")

	// Execute batch prune
	if err := db.BatchPrune(ctx, pruneFrom, pruneTo, batchSize); err != nil {
		return fmt.Errorf("failed to prune ErigonDB: %w", err)
	}

	fmt.Printf("\nErigonDB prune completed successfully!\n")
	fmt.Printf("  Pruned blocks: 0 to %d\n", pruneTo)
	fmt.Printf("  Retained blocks: %d to %d (%d blocks)\n", pruneTo, currentHeight, retainBlocks)

	return nil
}

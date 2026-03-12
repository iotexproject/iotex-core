package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/tools/iomigrater/common"
)

// Multi-language support
var (
	checkShardsCmdShorts = map[string]string{
		"english": "Sub-command for listing filedao shard height ranges.",
		"chinese": "查看 filedao 分片高度范围的子命令",
	}
	checkShardsCmdLongs = map[string]string{
		"english": "Sub-command for listing each filedao shard file and its stored height range.",
		"chinese": "查看每个 filedao 分片文件及其存储高度范围的子命令",
	}
	checkShardsCmdUse = map[string]string{
		"english": "check-shards",
		"chinese": "check-shards",
	}
)

var (
	// CheckShards lists shard height ranges for a chain db.
	CheckShards = &cobra.Command{
		Use:   common.TranslateInLang(checkShardsCmdUse),
		Short: common.TranslateInLang(checkShardsCmdShorts),
		Long:  common.TranslateInLang(checkShardsCmdLongs),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return printShardRanges(args[0])
		},
	}
)

func printShardRanges(filePath string) error {
	cfg, err := config.New([]string{}, []string{})
	if err != nil {
		return fmt.Errorf("failed to new config: %v", err)
	}

	cfg.DB.DbPath = filePath
	ranges, err := filedao.InspectShardRanges(cfg.DB, block.NewDeserializer(cfg.Chain.EVMNetworkID))
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "FILE\tVERSION\tSTART\tEND")
	for _, r := range ranges {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", r.FilePath, r.Version, r.StartHeight, r.EndHeight)
	}
	return w.Flush()
}

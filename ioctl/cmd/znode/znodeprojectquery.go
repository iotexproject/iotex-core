package znode

import (
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/spf13/cobra"
)

var (
	// znodeProjectQuery represents the query znode project command
	znodeProjectQuery = &cobra.Command{
		Use:   "query",
		Short: config.TranslateInLang(znodeProjectQueryShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := queryProject(id)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// znodeProjectQueryShorts query znode project shorts multi-lang support
	znodeProjectQueryShorts = map[config.Language]string{
		config.English: "query znode project",
		config.Chinese: "查询项目",
	}
)

func init() {

}

func queryProject(id uint64) (string, error) {
	return "", nil
}

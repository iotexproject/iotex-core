package znode

import (
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/spf13/cobra"
)

var (
	// znodeProjectUpdate represents the update znode project command
	znodeProjectUpdate = &cobra.Command{
		Use:   "update",
		Short: config.TranslateInLang(znodeProjectCreateShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return output.PrintError(err)
			}
			uri, err := cmd.Flags().GetString("project-uri")
			if err != nil {
				return output.PrintError(err)
			}
			hash, err := cmd.Flags().GetString("project-hash")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := updateProject(id, uri, hash)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// znodeProjectUpdateShorts update znode project shorts multi-lang support
	znodeProjectUpdateShorts = map[config.Language]string{
		config.English: "update znode project",
		config.Chinese: "更新项目",
	}
)

func init() {
	znodeProjectCreate.Flags().Uint64P("project-id", "p", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	znodeProjectCreate.Flags().StringP("project-uri", "u", "", config.TranslateInLang(_flagProjectUriUsages, config.UILanguage))
	znodeProjectCreate.Flags().StringP("project-hash", "h", "", config.TranslateInLang(_flagProjectHashUsages, config.UILanguage))

	_ = znodeProjectCreate.MarkFlagRequired("project-id")
	_ = znodeProjectCreate.MarkFlagRequired("project-uri")
	_ = znodeProjectCreate.MarkFlagRequired("project-hash")
}

func updateProject(id uint64, uri, hash string) (string, error) {
	return "", nil
}

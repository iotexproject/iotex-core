package znode

import (
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/spf13/cobra"
)

var (
	// znodeProjectCreate represents the create znode project command
	znodeProjectCreate = &cobra.Command{
		Use:   "create",
		Short: config.TranslateInLang(znodeProjectCreateShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			uri, err := cmd.Flags().GetString("project-uri")
			if err != nil {
				return output.PrintError(err)
			}
			hash, err := cmd.Flags().GetString("project-hash")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := createProject(uri, hash)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// znodeProjectSendShorts create znode project shorts multi-lang support
	znodeProjectCreateShorts = map[config.Language]string{
		config.English: "create znode project",
		config.Chinese: "创建项目",
	}

	_flagProjectUriUsages = map[config.Language]string{
		config.English: "project config fetch uri",
		config.Chinese: "项目配置拉取地址",
	}
	_flagProjectHashUsages = map[config.Language]string{
		config.English: "project config hash for validating",
		config.Chinese: "项目配置hash",
	}
)

func init() {
	znodeProjectCreate.Flags().StringP("project-uri", "u", "", config.TranslateInLang(_flagProjectUriUsages, config.UILanguage))
	znodeProjectCreate.Flags().StringP("project-hash", "h", "", config.TranslateInLang(_flagProjectHashUsages, config.UILanguage))

	_ = znodeProjectCreate.MarkFlagRequired("project-uri")
	_ = znodeProjectCreate.MarkFlagRequired("project-hash")
}

func createProject(uri, hash string) (string, error) {
	return "", nil
}

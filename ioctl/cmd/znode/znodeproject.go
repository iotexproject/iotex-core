package znode

import (
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/spf13/cobra"
)

var (
	// znodeProject represents the znode project management command
	znodeProject = &cobra.Command{
		Use:   "project",
		Short: config.TranslateInLang(znodeProjectShorts, config.UILanguage),
	}

	// znodeProjectShorts znode project shorts multi-lang support
	znodeProjectShorts = map[config.Language]string{
		config.English: "znode project management",
		config.Chinese: "znode项目管理",
	}
)

func init() {
	znodeProject.AddCommand(znodeProjectCreate)
	znodeProject.AddCommand(znodeProjectUpdate)
	znodeProject.AddCommand(znodeProjectQuery)
}

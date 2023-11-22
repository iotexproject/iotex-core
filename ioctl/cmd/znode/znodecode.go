package znode

import (
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/spf13/cobra"
)

var (
	// znodeCode represents the znode code command
	znodeCode = &cobra.Command{
		Use:   "code",
		Short: config.TranslateInLang(znodeCodeShorts, config.UILanguage),
	}

	// znodeCodeShorts znode code shorts multi-lang support
	znodeCodeShorts = map[config.Language]string{
		config.English: "znode code operations",
		config.Chinese: "znode代码操作",
	}
)

func init() {
	znodeCode.AddCommand(znodeCodeConvert)
}

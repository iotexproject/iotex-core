package ws

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

var (
	// wsCode represents the w3bstream code command
	wsCode = &cobra.Command{
		Use:   "code",
		Short: config.TranslateInLang(wsCodeShorts, config.UILanguage),
	}

	// wsCodeShorts w3bstream code shorts multi-lang support
	wsCodeShorts = map[config.Language]string{
		config.English: "ws code operations",
		config.Chinese: "ws代码操作",
	}
)

func init() {
	wsCode.AddCommand(wsCodeConvert)
}

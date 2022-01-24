package alias

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

var (
	removeShorts = map[config.Language]string{
		config.English: "Remove alias",
		config.Chinese: "移除别名",
	}
	removeUses = map[config.Language]string{
		config.English: "remove",
		config.Chinese: "remove",
	}
	removeInvalidAlias = map[config.Language]string{
		config.English: "invalid alias %s",
		config.Chinese: "不可用别名 %s",
	}
	removeMarshalError = map[config.Language]string{
		config.English: "failed to marshal config",
		config.Chinese: "无法序列化配置",
	}
	removeWriteError = map[config.Language]string{
		config.English: "failed to write to config file %s",
		config.Chinese: "无法写入配置文件 %s",
	}
	removeResult = map[config.Language]string{
		config.English: "%s is removed",
		config.Chinese: "%s 已移除",
	}
)

// NewAliasRemove represents the removes alias command
func NewAliasRemove(c ioctl.Client) *cobra.Command {
	use, _ := c.SelectTranslation(removeUses)
	short, _ := c.SelectTranslation(removeShorts)
	invalidAlias, _ := c.SelectTranslation(removeInvalidAlias)
	marshalError, _ := c.SelectTranslation(removeMarshalError)
	writeError, _ := c.SelectTranslation(removeWriteError)
	result, _ := c.SelectTranslation(removeResult)

	ec := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]
			if err := validator.ValidateAlias(alias); err != nil {
				return fmt.Errorf(invalidAlias, alias)
			}
			conf := c.Config()
			delete(conf.Aliases, alias)
			out, err := yaml.Marshal(&conf)
			if err != nil {
				return output.NewError(output.SerializationError, marshalError, err)
			}
			if err := os.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
				return output.NewError(output.WriteFileError, fmt.Sprintf(writeError, config.DefaultConfigFile), err)
			}
			fmt.Println(fmt.Sprintf(result, alias))
			return nil
		},
	}
	return ec
}

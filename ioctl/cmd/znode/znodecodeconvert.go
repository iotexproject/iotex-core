package znode

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var (
	// znodeCodeConvert represents the znode code convert command
	znodeCodeConvert = &cobra.Command{
		Use:   "convert",
		Short: config.TranslateInLang(znodeCodeConvertShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmType, err := cmd.Flags().GetString("vm-type")
			if err != nil {
				return errors.Wrap(err, "failed to get flag vm-type")
			}
			codeFile, err := cmd.Flags().GetString("code-file")
			if err != nil {
				return errors.Wrap(err, "failed to get flag code-file")
			}
			confFile, err := cmd.Flags().GetString("conf-file")
			if err != nil {
				return errors.Wrap(err, "failed to get flag conf-file")
			}
			expParam, err := cmd.Flags().GetString("expand-param")
			if err != nil {
				return errors.Wrap(err, "failed to get flag expand-param")
			}

			out, err := convertCodeToZlibHex(vmType, codeFile, confFile, expParam)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// znodeCodeConvertShorts znode code convert shorts multi-lang support
	znodeCodeConvertShorts = map[config.Language]string{
		config.English: "convert zkp code to hex string compressed with zlib",
		config.Chinese: "将zkp代码通过zlib进行压缩之后转成hex字符串",
	}

	_flagVmTypeUsages = map[config.Language]string{
		config.English: "vm type",
		config.Chinese: "虚拟机类型",
	}
	_flagCodeFileUsages = map[config.Language]string{
		config.English: "code file",
		config.Chinese: "代码文件",
	}
	_flagConfFileUsages = map[config.Language]string{
		config.English: "conf file",
		config.Chinese: "配置文件",
	}
	_flagExpandParamUsages = map[config.Language]string{
		config.English: "expand param",
		config.Chinese: "扩展参数",
	}
)

func init() {
	znodeCodeConvert.Flags().StringP("vm-type", "t", "", config.TranslateInLang(_flagVmTypeUsages, config.UILanguage))
	znodeCodeConvert.Flags().StringP("code-file", "i", "", config.TranslateInLang(_flagCodeFileUsages, config.UILanguage))
	znodeCodeConvert.Flags().StringP("conf-file", "c", "", config.TranslateInLang(_flagConfFileUsages, config.UILanguage))
	znodeCodeConvert.Flags().StringP("expand-param", "e", "", config.TranslateInLang(_flagExpandParamUsages, config.UILanguage))

	_ = znodeCodeConvert.MarkFlagRequired("vm-type")
	_ = znodeCodeConvert.MarkFlagRequired("code-file")
}

func convertCodeToZlibHex(vmType string, codeFile string, confFile string, expParam string) (string, error) {
	content, err := os.ReadFile(codeFile)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("failed to read code-file %s", codeFile))
	}
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	_, err = w.Write(content)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("failed to zlib code-file %s", codeFile))
	}
	w.Close()

	hexString := hex.EncodeToString(b.Bytes())

	confMap := make(map[string]interface{})
	if expParam != "" {
		confMap["codeExpParam"] = expParam
	}
	confMap["vmType"] = vmType
	confMap["code"] = hexString

	jsonConf, err := json.MarshalIndent(confMap, "", "  ")

	if confFile == "" {
		confFile = fmt.Sprintf("./%s-config.json", vmType)
	}
	err = os.WriteFile(confFile, jsonConf, fs.FileMode(0666))
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("failed to write conf-file %s", confFile))
	}

	return fmt.Sprintf("conf file: %s", confFile), err
}

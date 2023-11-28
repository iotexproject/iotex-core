package ws

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
	// wsCodeConvert represents the w3bstream code convert command
	wsCodeConvert = &cobra.Command{
		Use:   "convert",
		Short: config.TranslateInLang(wsCodeConvertShorts, config.UILanguage),
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

			out, err := generateProjectFile(vmType, codeFile, confFile, expParam)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// wsCodeConvertShorts w3bstream code convert shorts multi-lang support
	wsCodeConvertShorts = map[config.Language]string{
		config.English: "convert zkp code to hex string compressed with zlib",
		config.Chinese: "将zkp代码通过zlib进行压缩之后转成hex字符串",
	}

	_flagVmTypeUsages = map[config.Language]string{
		config.English: "vm type, support risc0, halo2",
		config.Chinese: "虚拟机类型，目前支持risc0和halo2",
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
		config.English: "expand param, if you use risc0 vm, need it.",
		config.Chinese: "扩展参数，risc0虚拟机需要此参数",
	}
)

func init() {
	wsCodeConvert.Flags().StringP("vm-type", "t", "", config.TranslateInLang(_flagVmTypeUsages, config.UILanguage))
	wsCodeConvert.Flags().StringP("code-file", "i", "", config.TranslateInLang(_flagCodeFileUsages, config.UILanguage))
	wsCodeConvert.Flags().StringP("conf-file", "c", "", config.TranslateInLang(_flagConfFileUsages, config.UILanguage))
	wsCodeConvert.Flags().StringP("expand-param", "e", "", config.TranslateInLang(_flagExpandParamUsages, config.UILanguage))

	_ = wsCodeConvert.MarkFlagRequired("vm-type")
	_ = wsCodeConvert.MarkFlagRequired("code-file")
}

func generateProjectFile(vmType string, codeFile string, confFile string, expParam string) (string, error) {
	hexString, err := convertCodeToZlibHex(codeFile)
	if err != nil {
		return "", err
	}

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

func convertCodeToZlibHex(codeFile string) (string, error) {
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

	return hexString, err
}

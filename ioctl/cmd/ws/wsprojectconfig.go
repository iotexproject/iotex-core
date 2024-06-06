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
	// wsProjectConfig represents the generate w3bstream project configuration command
	wsProjectConfig = &cobra.Command{
		Use:   "config",
		Short: config.TranslateInLang(wsProjectConfigShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			dataSource, err := cmd.Flags().GetString("data-source")
			if err != nil {
				return errors.Wrap(err, "failed to get flag data-source")
			}
			defaultVersion, err := cmd.Flags().GetString("default-version")
			if err != nil {
				return errors.Wrap(err, "failed to get flag default-version")
			}
			version, err := cmd.Flags().GetString("version")
			if err != nil {
				return errors.Wrap(err, "failed to get flag version")
			}
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
			outputFile, err := cmd.Flags().GetString("output-file")
			if err != nil {
				return errors.Wrap(err, "failed to get flag output-file")
			}

			out, err := generateProjectFile(dataSource, defaultVersion, version, vmType, codeFile, confFile, expParam, outputFile)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// wsProjectConfigShorts w3bstream project configuration shorts multi-lang support
	wsProjectConfigShorts = map[config.Language]string{
		config.English: "generate w3bstream project configuration file",
		config.Chinese: "生成项目的配置文件",
	}

	_flagDataSourceUsages = map[config.Language]string{
		config.English: "data source of the project",
		config.Chinese: "该project的数据源",
	}
	_flagDefaultVersionUsages = map[config.Language]string{
		config.English: "default version for the project config",
		config.Chinese: "该project config的默认版本号",
	}
	_flagVersionUsages = map[config.Language]string{
		config.English: "version for the project config",
		config.Chinese: "该project config的版本号",
	}
	_flagVMTypeUsages = map[config.Language]string{
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
	_flagOutputFileUsages = map[config.Language]string{
		config.English: "output file, default is stdout.",
		config.Chinese: "output的值所在文件，指定proof的输出",
	}
)

func init() {
	wsProjectConfig.Flags().StringP("data-source", "s", "", config.TranslateInLang(_flagDataSourceUsages, config.UILanguage))
	wsProjectConfig.Flags().StringP("default-version", "d", "0.1", config.TranslateInLang(_flagDefaultVersionUsages, config.UILanguage))
	wsProjectConfig.Flags().StringP("version", "v", "", config.TranslateInLang(_flagVersionUsages, config.UILanguage))
	wsProjectConfig.Flags().StringP("vm-type", "t", "", config.TranslateInLang(_flagVMTypeUsages, config.UILanguage))
	wsProjectConfig.Flags().StringP("code-file", "i", "", config.TranslateInLang(_flagCodeFileUsages, config.UILanguage))
	wsProjectConfig.Flags().StringP("conf-file", "c", "", config.TranslateInLang(_flagConfFileUsages, config.UILanguage))
	wsProjectConfig.Flags().StringP("expand-param", "e", "", config.TranslateInLang(_flagExpandParamUsages, config.UILanguage))
	wsProjectConfig.Flags().StringP("output-file", "u", "", config.TranslateInLang(_flagOutputFileUsages, config.UILanguage))

	_ = wsProjectConfig.MarkFlagRequired("data-source")
	_ = wsProjectConfig.MarkFlagRequired("vm-type")
	_ = wsProjectConfig.MarkFlagRequired("code-file")

	wsProject.AddCommand(wsProjectConfig)
}

type projectConfig struct {
	Version      string      `json:"version"`
	VMType       string      `json:"vmType"`
	Output       interface{} `json:"output"`
	CodeExpParam string      `json:"codeExpParam,omitempty"`
	Code         string      `json:"code"`
}

func generateProjectFile(dataSource, defaultVersion, version, vmType, codeFile, confFile, expParam, outputFile string) (string, error) {
	tye, err := stringToVMType(vmType)
	if err != nil {
		return "", err
	}

	hexString, err := convertCodeToZlibHex(codeFile)
	if err != nil {
		return "", err
	}

	var (
		confMap   = make(map[string]interface{})
		verMap    projectConfig
		verMaps   = make([]projectConfig, 0)
		outputMap = make(map[string]interface{})
	)

	if expParam != "" {
		verMap.CodeExpParam = expParam
	}
	verMap.VMType = string(tye)
	verMap.Code = hexString
	if version == "" {
		version = "0.1"
	}
	verMap.Version = version

	output := []byte(`{
  "type": "stdout"
}`)
	if outputFile != "" {
		output, err = os.ReadFile(outputFile)
		if err != nil {
			return "", errors.Wrap(err, "failed to read output file")
		}
	}
	if err := json.Unmarshal(output, &outputMap); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal output file")
	}
	verMap.Output = outputMap
	verMaps = append(verMaps, verMap)

	confMap["datasourceURI"] = dataSource
	confMap["defaultVersion"] = defaultVersion
	confMap["versions"] = verMaps
	jsonConf, err := json.MarshalIndent(confMap, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal config maps")
	}

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

// zkp vm type
type vmType string

const (
	risc0  vmType = "risc0"  // risc0 vm
	halo2  vmType = "halo2"  // halo2 vm
	zkWasm vmType = "zkwasm" // zkwasm vm
	wasm   vmType = "wasm"   // wasm vm
)

// TODO move this to sprout
func stringToVMType(vmType string) (vmType, error) {
	switch vmType {
	case string(risc0):
		return risc0, nil
	case string(halo2):
		return halo2, nil
	case string(zkWasm):
		return zkWasm, nil
	case string(wasm):
		return wasm, nil
	default:
		return "", errors.New(fmt.Sprintf("not support %s type, just support %s, %s, %s, and %s", vmType, risc0, halo2, zkWasm, wasm))
	}
}

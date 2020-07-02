// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var iotexIDE string

// Multi-language support
var (
	contractShareCmdUses = map[config.Language]string{
		config.English: "share LOCAL_FOLDER_PATH [--iotex-ide YOUR_IOTEX_IDE_URL_INSTANCE]",
		config.Chinese: "share 本地文件路径 [--iotex-ide 你的IOTEX_IDE的URL]",
	}
	contractShareCmdShorts = map[config.Language]string{
		config.English: "share a folder from your local computer to the IoTex smart contract dev.(default to https://ide.iotex.io/)",
		config.Chinese: "share 将本地文件夹内容分享到IoTex在线智能合约IDE(默认为https://ide.iotex.io/)",
	}
	flagIoTexIDEUrlUsage = map[config.Language]string{
		config.English: "set your IoTex IDE url instance",
		config.Chinese: "设置自定义IoTexIDE Url",
	}
)

// contractShareCmd represents the contract share command
var contractShareCmd = &cobra.Command{
	Use:   config.TranslateInLang(contractShareCmdUses, config.UILanguage),
	Short: config.TranslateInLang(contractShareCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := share(args)
		return output.PrintError(err)
	},
}

func init() {
	contractShareCmd.Flags().StringVar(&iotexIDE, "iotex-ide", "https://ide.iotex.io/", config.TranslateInLang(flagIoTexIDEUrlUsage, config.UILanguage))
}

func share(args []string) error {
	var err error
	path := args[0]
	if len(path) == 0 {
		return output.NewError(output.ReadFileError, "failed to get directory", nil)
	}

	adsPath, _ := filepath.Abs(path)

	info, err := os.Stat(adsPath)
	if err != nil {
		return output.NewError(output.ReadFileError, "failed to get directory", nil)
	}
	if !info.IsDir() {
		return output.NewError(output.InputError, "input file rather than directory", nil)
	}

	err = checkRemixdReady()
	if err != nil {
		output.NewError(output.RuntimeError, "remixd not ready, please run 'npm -g install remixd' and try again ", err)
	}

	if len(iotexIDE) == 0 {
		return output.NewError(output.FlagError, "failed to get iotex ide url instance", nil)
	}

	cmdString := "remixd -s " + adsPath + " --remix-ide " + iotexIDE

	fmt.Println(cmdString)

	cmd := exec.Command("bash", "-c", cmdString)
	stdout, err := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	if err != nil {
		return err
	}

	if err = cmd.Start(); err != nil {
		return output.NewError(output.RuntimeError, "failed to link-local folder(restart terminal and try again)", err)
	}
	for {
		tmp := make([]byte, 1024)
		_, err := stdout.Read(tmp)
		fmt.Print(string(tmp))
		if err != nil {
			break
		}
	}

	if err = cmd.Wait(); err != nil {
		return output.NewError(output.RuntimeError, "failed to link-local folder(restart terminal and try again)", err)
	}

	return nil

}

func checkRemixdReady() error {
	cmdString := "remixd -h"
	cmd := exec.Command("bash", "-c", cmdString)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

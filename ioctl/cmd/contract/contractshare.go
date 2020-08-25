// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var (
	iotexIDE  string
	fileList  []string
	givenPath string

	addr = flag.String("addr", "localhost:65520", "http service address")

	upgrade = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			if iotexIDE == r.Header["Origin"][0] {
				return true
			}
			return false
		},
	}
)

// Multi-language support
var (
	contractShareCmdUses = map[config.Language]string{
		config.English: "share LOCAL_FOLDER_PATH [--iotex-ide YOUR_IOTEX_IDE_URL_INSTANCE]",
		config.Chinese: "share 本地文件路径 [--iotex-ide 你的IOTEX_IDE的URL]",
	}
	contractShareCmdShorts = map[config.Language]string{
		config.English: "share a folder from your local computer to the IoTex smart contract dev.(default to https://ide.iotex.io)",
		config.Chinese: "share 将本地文件夹内容分享到IoTex在线智能合约IDE(默认为https://ide.iotex.io)",
	}
	flagIoTexIDEUrlUsage = map[config.Language]string{
		config.English: "set your IoTeX IDE url instance",
		config.Chinese: "设置自定义IoTeX IDE Url",
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
	contractShareCmd.Flags().StringVar(&iotexIDE, "iotex-ide", "https://ide.iotex.io", config.TranslateInLang(flagIoTexIDEUrlUsage, config.UILanguage))
}

func isDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {

		return false

	}
	return s.IsDir()
}

func share(args []string) error {
	givenPath = args[0]
	if len(givenPath) == 0 {
		return output.NewError(output.ReadFileError, "failed to get directory", nil)
	}

	if !isDir(givenPath) {
		return output.NewError(output.InputError, "given file rather than directory", nil)
	}

	if len(iotexIDE) == 0 {
		return output.NewError(output.FlagError, "failed to get IoTeX ide url instance", nil)
	}

	filepath.Walk(givenPath, func(path string, info os.FileInfo, err error) error {
		if !isDir(path) {
			relPath, err := filepath.Rel(givenPath, path)
			if err != nil {
				return err
			}

			if !strings.HasPrefix(relPath, ".") {
				fileList = append(fileList, relPath)
			}
		}

		return nil
	})

	log.Println("Listening on 127.0.0.1:65520, Please open your IDE ( " + iotexIDE + " ) to connect to local files")

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrade.Upgrade(writer, request, nil)

		if err != nil {
			log.Println("websocket error:", err)
			return
		}

		for {
			requestInfo := make(map[string]interface{})
			response := make(map[string]interface{})

			if err := conn.ReadJSON(&requestInfo); err != nil {
				log.Println("read json", err)
				return
			}

			response["action"] = "response"
			response["id"] = requestInfo["id"]

			switch requestInfo["key"] {
			case "handshake":
				response["key"] = "handshake"
				if err := conn.WriteJSON(response); err != nil {
					log.Println("send handshake response", err)
					break
				}
			case "list":
				response["key"] = "list"
				payload := make(map[string]bool)
				for _, ele := range fileList {
					payload[ele] = false
				}
				response["payload"] = payload
				if err := conn.WriteJSON(response); err != nil {
					log.Println("send response with file list", err)
					break
				}
			case "get":
				payload := map[string]interface{}{}
				t := requestInfo["payload"]
				s := reflect.ValueOf(t)
				for i := 0; i < s.Len(); i++ {
					p, _ := s.Index(i).Interface().(map[string]interface{})
					for _, v := range p {
						upload, err := ioutil.ReadFile(givenPath + "/" + v.(string))
						log.Println("share :" + givenPath + "/" + v.(string))
						if err != nil {
							log.Println("read file failed", err)
							break
						}
						payload["content"] = string(upload)
						payload["readonly"] = true
						response["key"] = "get"
						response["payload"] = payload
						if err := conn.WriteJSON(response); err != nil {
							log.Println("send response with file", err)
							break
						}
					}
				}
			default:
				log.Println("Don't support this IDE yet. Can not handle websocket method: " + requestInfo["key"].(string))
			}
		}
	})
	log.Fatal(http.ListenAndServe(*addr, nil))

	return nil

}

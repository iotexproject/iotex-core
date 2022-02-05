// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"flag"
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
			return iotexIDE == r.Header["Origin"][0]
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

type requestMessage struct {
	ID      string        `json:"id"`
	Action  string        `json:"action"`
	Key     string        `json:"key"`
	Payload []interface{} `json:"payload"`
}

type responseMessage struct {
	ID      string      `json:"id"`
	Action  string      `json:"action"`
	Key     string      `json:"key"`
	Payload interface{} `json:"payload"`
}

func isDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

func isReadOnly(path string) bool {
	var readOnly = false
	file, err := os.OpenFile(path, os.O_WRONLY, 0666)
	if err != nil {
		if os.IsPermission(err) {
			log.Println("Error: Write permission denied.")
		}
		if os.IsNotExist(err) {
			log.Println("Error: File doesn't exist.")
		}
		readOnly = true
	}
	file.Close()
	return readOnly
}

func isExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func rename(oldPath string, newPath string, c chan bool) {
	if isExist(givenPath + "/" + oldPath) {
		if err := os.Rename(oldPath, newPath); err != nil {
			log.Println("Rename file failed: ", err)
		}
		c <- false
	}
	c <- true
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
			var request requestMessage
			var response responseMessage

			if err := conn.ReadJSON(&request); err != nil {
				log.Println("read json error: ", err)
				return
			}
			response.ID = request.ID
			response.Action = "response"
			response.Key = request.Key

			switch request.Key {

			case "handshake":
				response.Payload = nil
				if err := conn.WriteJSON(&response); err != nil {
					log.Println("send handshake response", err)
				}
			case "list":
				payload := make(map[string]bool)
				for _, ele := range fileList {
					payload[ele] = isReadOnly(givenPath + "/" + ele)
				}
				response.Payload = payload
				if err := conn.WriteJSON(&response); err != nil {
					log.Println("send list response", err)
				}
			case "get":
				payload := map[string]interface{}{}

				t := request.Payload
				getPayload := reflect.ValueOf(t).Index(0).Interface().(map[string]interface{})
				getPayloadPath := getPayload["path"].(string)
				upload, err := os.ReadFile(givenPath + "/" + getPayloadPath)
				if err != nil {
					log.Println("read file failed: ", err)
				}
				payload["content"] = string(upload)
				payload["readonly"] = isReadOnly(givenPath + "/" + getPayloadPath)
				response.Payload = payload
				if err := conn.WriteJSON(&response); err != nil {
					log.Println("send get response: ", err)
					break
				}
				log.Println("share: " + givenPath + "/" + getPayloadPath)

			case "rename":
				c := make(chan bool)
				t := request.Payload
				renamePayload := reflect.ValueOf(t).Index(0).Interface().(map[string]interface{})
				oldPath := renamePayload["oldPath"].(string)
				newPath := renamePayload["newPath"].(string)
				go rename(oldPath, newPath, c)
				response.Payload = <-c
				if err := conn.WriteJSON(&response); err != nil {
					log.Println("send get response: ", err)
					break
				}
				log.Println("rename: " + givenPath + "/" + oldPath + " to " + givenPath + "/" + newPath)

			case "set":
				t := request.Payload
				setPayload := reflect.ValueOf(t).Index(0).Interface().(map[string]interface{})
				setPath := setPayload["path"].(string)
				content := setPayload["content"].(string)
				err := os.WriteFile(givenPath+"/"+setPath, []byte(content), 0777)
				if err != nil {
					log.Println("set file failed: ", err)
				}
				if err := conn.WriteJSON(&response); err != nil {
					log.Println("send set response: ", err)
					break
				}
				log.Println("set: " + givenPath + "/" + setPath)

			default:
				log.Println("Don't support this IDE yet. Can not handle websocket method: " + request.Key)

			}
		}
	})
	log.Fatal(http.ListenAndServe(*addr, nil))

	return nil

}

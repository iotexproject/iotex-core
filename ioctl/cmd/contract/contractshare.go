// Copyright (c) 2022 IoTeX Foundation
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
	_iotexIDE  string
	_fileList  []string
	_givenPath string

	_addr = flag.String("_addr", "localhost:65520", "http service _address")

	_upgrade = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return _iotexIDE == r.Header["Origin"][0]
		},
	}
)

// Multi-language support
var (
	_contractShareCmdUses = map[config.Language]string{
		config.English: "share LOCAL_FOLDER_PATH [--iotex-ide YOUR_IOTEX_IDE_URL_INSTANCE]",
		config.Chinese: "share 本地文件路径 [--iotex-ide 你的IOTEX_IDE的URL]",
	}
	_contractShareCmdShorts = map[config.Language]string{
		config.English: "share a folder from your local computer to the IoTex smart contract dev.(default to https://ide.iotex.io)",
		config.Chinese: "share 将本地文件夹内容分享到IoTex在线智能合约IDE(默认为https://ide.iotex.io)",
	}
	_flagIoTexIDEUrlUsage = map[config.Language]string{
		config.English: "set your IoTeX IDE url instance",
		config.Chinese: "设置自定义IoTeX IDE Url",
	}
)

// _contractShareCmd represents the contract share command
var _contractShareCmd = &cobra.Command{
	Use:   config.TranslateInLang(_contractShareCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_contractShareCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := share(args)
		return output.PrintError(err)
	},
}

func init() {
	_contractShareCmd.Flags().StringVar(&_iotexIDE, "iotex-ide", "https://ide.iotex.io", config.TranslateInLang(_flagIoTexIDEUrlUsage, config.UILanguage))
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
	if isExist(oldPath) {
		if err := os.Rename(oldPath, newPath); err != nil {
			log.Println("Rename file failed: ", err)
		}
		c <- false
	}
	c <- true
}

func share(args []string) error {
	_givenPath = filepath.Clean(args[0])
	if len(_givenPath) == 0 {
		return output.NewError(output.ReadFileError, "failed to get directory", nil)
	}

	if !isDir(_givenPath) {
		return output.NewError(output.InputError, "given file rather than directory", nil)
	}

	if len(_iotexIDE) == 0 {
		return output.NewError(output.FlagError, "failed to get IoTeX ide url instance", nil)
	}

	filepath.Walk(_givenPath, func(path string, info os.FileInfo, err error) error {
		if !isDir(path) {
			relPath, err := filepath.Rel(_givenPath, path)
			if err != nil {
				return err
			}

			if !strings.HasPrefix(relPath, ".") {
				_fileList = append(_fileList, relPath)
			}
		}
		return nil
	})

	log.Println("Listening on 127.0.0.1:65520, Please open your IDE ( " + _iotexIDE + " ) to connect to local files")

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := _upgrade.Upgrade(writer, request, nil)

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
				for _, ele := range _fileList {
					payload[ele] = isReadOnly(filepath.Join(_givenPath, ele))
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
				payloadPath := filepath.Join(_givenPath, filepath.Clean(getPayloadPath))
				upload, err := os.ReadFile(payloadPath)
				if err != nil {
					log.Println("read file failed: ", err)
				}
				payload["content"] = string(upload)
				payload["readonly"] = isReadOnly(payloadPath)
				response.Payload = payload
				if err := conn.WriteJSON(&response); err != nil {
					log.Println("send get response: ", err)
					break
				}
				log.Println("share: " + easpcapeString(payloadPath))

			case "rename":
				c := make(chan bool)
				t := request.Payload
				renamePayload := reflect.ValueOf(t).Index(0).Interface().(map[string]interface{})
				oldPath := filepath.Join(_givenPath, filepath.Clean(renamePayload["oldPath"].(string)))
				newPath := filepath.Join(_givenPath, filepath.Clean(renamePayload["newPath"].(string)))
				go rename(oldPath, newPath, c)
				response.Payload = <-c
				if err := conn.WriteJSON(&response); err != nil {
					log.Println("send get response: ", err)
					break
				}
				log.Println("rename: " + easpcapeString(oldPath) + " to " + easpcapeString(newPath))

			case "set":
				t := request.Payload
				setPayload := reflect.ValueOf(t).Index(0).Interface().(map[string]interface{})
				setPath := setPayload["path"].(string)
				content := setPayload["content"].(string)
				newPath := filepath.Join(_givenPath, filepath.Clean(setPath))
				if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
					log.Println("mkdir failed: ", err)
				}
				if err := os.WriteFile(newPath, []byte(content), 0644); err != nil {
					log.Println("set file failed: ", err)
				}
				if err := conn.WriteJSON(&response); err != nil {
					log.Println("send set response: ", err)
					break
				}
				log.Println("set: " + easpcapeString(newPath))

			default:
				log.Println("Don't support this IDE yet. Can not handle websocket method: " + easpcapeString(request.Key))

			}
		}
	})
	log.Fatal(http.ListenAndServe(*_addr, nil))

	return nil
}

func easpcapeString(str string) string {
	escaped := strings.Replace(str, "\n", "", -1)
	return strings.Replace(escaped, "\r", "", -1)
}

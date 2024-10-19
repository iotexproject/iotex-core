package ws

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
)

var (
	// wsMessage represents the w3bstream message command
	wsMessage = &cobra.Command{
		Use:   "message",
		Short: config.TranslateInLang(wsMessageShorts, config.UILanguage),
	}

	// wsMessageShorts w3bstream message shorts multi-lang support
	wsMessageShorts = map[config.Language]string{
		config.English: "w3bstream message operations",
		config.Chinese: "w3bstream消息操作",
	}

	_flagDIDVCTokenUsages = map[config.Language]string{
		config.English: "DID VC token",
		config.Chinese: "DID VC 令牌",
	}
)

func init() {
	wsMessage.AddCommand(wsMessageSend)
	wsMessage.AddCommand(wsMessageQuery)

	WsCmd.AddCommand(wsMessage)
}

type sendMessageReq struct {
	ProjectID      uint64 `json:"projectID"`
	ProjectVersion string `json:"projectVersion"`
	Data           string `json:"data"`
}

type sendMessageRsp struct {
	MessageID string `json:"messageID"`
}

type stateLog struct {
	State   string    `json:"state"`
	Time    time.Time `json:"time"`
	Comment string    `json:"comment"`
	Result  string    `json:"result"`
}

type queryMessageRsp struct {
	MessageID string      `json:"messageID"`
	States    []*stateLog `json:"states"`
}

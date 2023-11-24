package ws

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
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
)

func init() {
	wsMessage.AddCommand(wsMessageSend)
	wsMessage.AddCommand(wsMessageQuery)
}

type sendMessageReq struct {
	ProjectID      uint64 `json:"projectID"`
	ProjectVersion string `json:"projectVersion"`
	Data           string `json:"data"`
}

type sendMessageRsp struct {
	MessageID string `json:"messageID"`
}

type queryMessageRsp struct {
	MessageID string `json:"messageID"`
	States    []struct {
		State       string    `json:"state"`
		Time        time.Time `json:"time"`
		Description string    `json:"description"`
	} `json:"states"`
}

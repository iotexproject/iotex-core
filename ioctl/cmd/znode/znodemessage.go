package znode

import (
	"github.com/spf13/cobra"
	"time"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

var (
	// znodeMessage represents the znode message command
	znodeMessage = &cobra.Command{
		Use:   "message",
		Short: config.TranslateInLang(znodeMessageShorts, config.UILanguage),
	}

	// znodeMessageShorts znode message shorts multi-lang support
	znodeMessageShorts = map[config.Language]string{
		config.English: "znode message operations",
		config.Chinese: "znode消息操作",
	}
)

func init() {
	znodeMessage.AddCommand(znodeMessageSend)
	znodeMessage.AddCommand(znodeMessageQuery)
}

type sendMessageReq struct {
	ProjectID      uint64 `json:"projectID"`
	ProjectVersion string `json:"projectVersion"`
	Data           string `json:"data"`
}

type sendMessageRsp struct {
	MessageID string `json:"taskID"`
}

type queryMessageRsp struct {
	ID                   string     `json:"id"`
	ProjectID            uint64     `json:"projectID"`
	ProjectVersion       string     `json:"projectVersion"`
	Data                 string     `json:"data"`
	ReceivedAt           *time.Time `json:"receivedAt"`
	SubmitProvingAt      *time.Time `json:"submitProvingAt,omitempty"`
	ProofResult          string     `json:"proofResult,omitempty"`
	SubmitToBlockchainAt *time.Time `json:"SubmitToBlockchainAt,omitempty"`
	TxHash               string     `json:"txHash,omitempty"`
	Succeed              bool       `json:"succeed"`
	ErrorMessage         string     `json:"errorMessage,omitempty"`
	// TODO field `Status`
}

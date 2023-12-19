package ws

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var (
	// wsMessageSend represents the w3bstream send message command
	wsMessageSend = &cobra.Command{
		Use:   "send",
		Short: config.TranslateInLang(wsMessageSendShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return output.PrintError(err)
			}
			projectVersion, err := cmd.Flags().GetString("project-version")
			if err != nil {
				return output.PrintError(err)
			}
			data, err := cmd.Flags().GetString("data")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := sendMessage(projectID, projectVersion, data)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// wsMessageSendShorts w3bstream message send shorts multi-lang support
	wsMessageSendShorts = map[config.Language]string{
		config.English: "send message to w3bstream for zk proofing",
		config.Chinese: "向w3bstream发送消息请求zk证明",
	}
)

func init() {
	wsMessageSend.Flags().Uint64P("project-id", "p", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	wsMessageSend.Flags().StringP("project-version", "v", "", config.TranslateInLang(_flagProjectVersionUsages, config.UILanguage))
	wsMessageSend.Flags().StringP("data", "d", "", config.TranslateInLang(_flagSendDataUsages, config.UILanguage))

	_ = wsMessageSend.MarkFlagRequired("project-id")
	_ = wsMessageSend.MarkFlagRequired("project-version")
	_ = wsMessageSend.MarkFlagRequired("data")
}

func sendMessage(projectID uint64, projectVersion string, data string) (string, error) {
	reqbody, err := json.Marshal(&sendMessageReq{
		ProjectID:      projectID,
		ProjectVersion: projectVersion,
		Data:           data,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to build call message")
	}

	u := url.URL{
		Scheme: "http",
		Host:   config.ReadConfig.WsEndpoint,
		Path:   "/message",
	}

	rsp, err := http.Post(u.String(), "application/json", bytes.NewReader(reqbody))
	if err != nil {
		return "", errors.Wrap(err, "call w3bsteam failed")
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return "", errors.Errorf("call w3bsteam failed: %s", rsp.Status)
	}

	rspbody, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read responded content")
	}
	rspdata := &sendMessageRsp{}
	if err = json.Unmarshal(rspbody, rspdata); err != nil {
		return "", errors.Wrap(err, "failed to parse responded content")
	}
	out, err := json.MarshalIndent(rspdata, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "failed to serialize output")
	}
	return string(out), err
}

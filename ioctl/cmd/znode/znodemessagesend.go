package znode

import (
	"bytes"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"net/url"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var (
	// znodeMessageSend represents the znode send message command
	znodeMessageSend = &cobra.Command{
		Use:   "send",
		Short: config.TranslateInLang(znodeMessageSendShorts, config.UILanguage),
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
			out, err := sendMessageToZnode(projectID, projectVersion, data)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// znodeMessageSendShorts znode message send shorts multi-lang support
	znodeMessageSendShorts = map[config.Language]string{
		config.English: "send message to znode for zk proofing",
		config.Chinese: "向znode发送消息请求zk证明",
	}

	_flagProjectIDUsages = map[config.Language]string{
		config.English: "project id",
		config.Chinese: "项目ID",
	}
	_flagProjectVersionUsages = map[config.Language]string{
		config.English: "project version",
		config.Chinese: "项目版本",
	}
	_flagSendDataUsages = map[config.Language]string{
		config.English: "send data",
		config.Chinese: "要发送的数据",
	}
)

func init() {
	znodeMessageSend.Flags().Uint64P("project-id", "p", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	znodeMessageSend.Flags().StringP("project-version", "v", "", config.TranslateInLang(_flagProjectVersionUsages, config.UILanguage))
	znodeMessageSend.Flags().StringP("data", "d", "", config.TranslateInLang(_flagSendDataUsages, config.UILanguage))

	_ = znodeMessageSend.MarkFlagRequired("project-id")
	_ = znodeMessageSend.MarkFlagRequired("project-version")
	_ = znodeMessageSend.MarkFlagRequired("data")
}

func sendMessageToZnode(projectID uint64, projectVersion string, data string) (string, error) {
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
		Host:   config.ReadConfig.ZnodeEndpoint,
		Path:   "/message",
	}

	rsp, err := http.Post(u.String(), "application/json", bytes.NewReader(reqbody))
	if err != nil {
		return "", errors.Wrap(err, "call znode failed")
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return "", errors.Errorf("call znode failed: %s", rsp.Status)
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

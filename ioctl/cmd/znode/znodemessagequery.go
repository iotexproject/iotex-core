package znode

import (
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
	// znodeMessageQuery represents the znode query message command
	znodeMessageQuery = &cobra.Command{
		Use:   "query",
		Short: config.TranslateInLang(znodeMessageQueryShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetString("message-id")
			if err != nil {
				return errors.Wrap(err, "failed to get flag message-id")
			}

			out, err := queryMessageStatus(id)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// znodeMessageSendShorts znode message query shorts multi-lang support
	znodeMessageQueryShorts = map[config.Language]string{
		config.English: "query message status from znode",
		config.Chinese: "向znode查询消息状态",
	}

	_flagMessageIDUsages = map[config.Language]string{
		config.English: "message id",
		config.Chinese: "消息ID",
	}
)

func init() {
	znodeMessageQuery.Flags().StringP("message-id", "i", "", config.TranslateInLang(_flagMessageIDUsages, config.UILanguage))
	_ = znodeMessageQuery.MarkFlagRequired("message-id")
}

func queryMessageStatus(id string) (string, error) {
	u := url.URL{
		Scheme: "http",
		Host:   config.ReadConfig.ZnodeEndpoint,
		Path:   "/message/" + id,
	}

	rsp, err := http.Get(u.String())
	if err != nil {
		return "", errors.Wrap(err, "call znode failed")
	}
	defer rsp.Body.Close()

	switch sc := rsp.StatusCode; sc {
	case http.StatusNotFound:
		return "", errors.Errorf("the message [%s] is not found or expired", id)
	case http.StatusOK:
	default:
		return "", errors.Errorf("responded status code: %d", sc)
	}

	rspbody, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", errors.Wrap(err, "read responded body failed")
	}

	rspdata := &queryMessageRsp{}
	if err = json.Unmarshal(rspbody, rspdata); err != nil {
		return "", errors.Wrap(err, "parse responded body failed")
	}
	out, err := json.MarshalIndent(rspdata, "", "  ")
	return string(out), err
}

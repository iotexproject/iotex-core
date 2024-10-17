package ws

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var (
	// wsMessageQuery represents the w3bstream query message command
	wsMessageQuery = &cobra.Command{
		Use:   "query",
		Short: config.TranslateInLang(wsMessageQueryShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetString("message-id")
			if err != nil {
				return errors.Wrap(err, "failed to get flag message-id")
			}
			tok, err := cmd.Flags().GetString("did-vc-token")
			if err != nil {
				return errors.Wrap(err, "failed to get flag did-vc-token")
			}

			out, err := queryMessageStatus(id, tok)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// wsMessageSendShorts w3bstream message query shorts multi-lang support
	wsMessageQueryShorts = map[config.Language]string{
		config.English: "query message status from w3bstream",
		config.Chinese: "向w3bstream查询消息状态",
	}

	_flagMessageIDUsages = map[config.Language]string{
		config.English: "message id",
		config.Chinese: "消息ID",
	}
)

func init() {
	wsMessageQuery.Flags().StringP("message-id", "i", "", config.TranslateInLang(_flagMessageIDUsages, config.UILanguage))
	wsMessageQuery.Flags().StringP("did-vc-token", "t", "", config.TranslateInLang(_flagDIDVCTokenUsages, config.UILanguage))

	_ = wsMessageQuery.MarkFlagRequired("message-id")
}

func queryMessageStatus(id string, tok string) (string, error) {
	u := url.URL{
		Scheme: "http",
		Host:   config.ReadConfig.WsEndpoint,
		Path:   "/message/" + id,
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to new http request")
	}

	if tok != "" {
		req.Header.Set("Authorization", tok)
	}

	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "call w3bstream failed")
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

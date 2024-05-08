package ws

import (
	"encoding/hex"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var wsProjectQueryCmd = &cobra.Command{
	Use: "query",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "query w3bstream project",
		config.Chinese: "查询项目",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := queryProject(flagProjectID.Value().(uint64))
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	flagProjectID.RegisterCommand(wsProjectQueryCmd)
	flagProjectID.MarkFlagRequired(wsProjectQueryCmd)

	wsProject.AddCommand(wsProjectQueryCmd)
}

func queryProject(projectID uint64) (any, error) {
	caller, err := NewContractCaller(projectStoreABI, projectStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to new contract caller")
	}
	value := new(contracts.W3bstreamProjectProjectConfig)
	result := NewContractResult(&projectStoreABI, funcQueryProject, value)
	if err = caller.Read(funcQueryProject, []any{big.NewInt(int64(projectID))}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProject)
	}
	if _, err = result.Result(); err != nil {
		return nil, err
	}

	isPaused := new(bool)
	result2 := NewContractResult(&projectStoreABI, funcIsProjectPaused, isPaused)
	if err = caller.Read(funcIsProjectPaused, []any{big.NewInt(int64(projectID))}, result2); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProject)
	}
	if _, err = result2.Result(); err != nil {
		return nil, err
	}

	return &struct {
		ProjectID uint64 `json:"projectID"`
		URI       string `json:"uri"`
		Hash      string `json:"hash"`
		IsPaused  bool   `json:"isPaused"`
	}{
		ProjectID: projectID,
		URI:       value.Uri,
		Hash:      hex.EncodeToString(value.Hash[:]),
		IsPaused:  *isPaused,
	}, nil
}

package ws

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

var wsProjectQueryCmd = &cobra.Command{
	Use: "query",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "query w3bstream project",
		config.Chinese: "查询项目",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := queryProject(big.NewInt(int64(projectID.Value().(uint64))))
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	projectID.RegisterCommand(wsProjectQueryCmd)
	projectID.MarkFlagRequired(wsProjectQueryCmd)

	wsProject.AddCommand(wsProjectQueryCmd)
}

func queryProject(projectID *big.Int) (any, error) {
	caller, err := NewContractCaller(projectStoreABI, projectStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to new contract caller")
	}

	result := NewContractResult(&projectStoreABI, funcQueryProject, new(contracts.W3bstreamProjectProjectConfig))
	if err = caller.Read(funcQueryProject, []any{projectID}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProject)
	}
	value, err := result.Result()
	if err != nil {
		return nil, err
	}

	result = NewContractResult(&projectStoreABI, funcIsProjectPaused, new(bool))
	if err = caller.Read(funcIsProjectPaused, []any{projectID}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProject)
	}
	isPaused, err := result.Result()
	if err != nil {
		return nil, err
	}

	result = NewContractResult(&projectStoreABI, funcQueryProjectOwner, new(common.Address))
	if err = caller.Read(funcQueryProjectOwner, []any{projectID}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProject)
	}
	owner, err := result.Result()
	if err != nil {
		return nil, err
	}

	ownerAddr, err := util.Address((*(owner.(*common.Address))).String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert owner address")
	}

	_projectConfig := value.(*contracts.W3bstreamProjectProjectConfig)

	return &struct {
		ProjectID uint64 `json:"projectID"`
		Owner     string `json:"owner"`
		URI       string `json:"uri"`
		Hash      string `json:"hash"`
		IsPaused  bool   `json:"isPaused"`
	}{
		ProjectID: projectID.Uint64(),
		Owner:     ownerAddr,
		URI:       _projectConfig.Uri,
		Hash:      hex.EncodeToString(_projectConfig.Hash[:]),
		IsPaused:  *(isPaused.(*bool)),
	}, nil
}

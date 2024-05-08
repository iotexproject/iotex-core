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

var wsProjectUpdateCmd = &cobra.Command{
	Use: "update",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "update w3bstream project",
		config.Chinese: "更新项目",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := updateProject(
			big.NewInt(int64(flagProjectID.Value().(uint64))),
			flagProjectFile.Value().(string),
			flagProjectHash.Value().(string),
		)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult("project updated")
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	flagProjectID.RegisterCommand(wsProjectUpdateCmd)
	flagProjectID.MarkFlagRequired(wsProjectUpdateCmd)
	flagProjectFile.RegisterCommand(wsProjectUpdateCmd)
	flagProjectFile.MarkFlagRequired(wsProjectUpdateCmd)
	flagProjectHash.RegisterCommand(wsProjectUpdateCmd)

	wsProject.AddCommand(wsProjectUpdateCmd)
}

func updateProject(projectID *big.Int, filename, hashstr string) (any, error) {
	uri, hashval, err := uploadToIPFS(ipfsEndpoint, filename, hashstr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upload project file to ipfs")
	}

	caller, err := NewContractCaller(projectStoreABI, projectStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract caller")
	}

	result := NewContractResult(&projectStoreABI, eventProjectConfigUpdated, new(contracts.W3bstreamProjectProjectConfigUpdated))
	_, err = caller.CallAndRetrieveResult(funcUpdateProjectConfig, []any{projectID, uri, hashval}, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update project config")
	}

	_v, err := result.Result()
	if err != nil {
		return nil, err
	}
	v := _v.(*contracts.W3bstreamProjectProjectConfigUpdated)
	return &struct {
		ProjectID uint64 `json:"projectID"`
		URI       string `json:"uri"`
		Hash      string `json:"hash"`
	}{
		ProjectID: v.ProjectId.Uint64(),
		URI:       v.Uri,
		Hash:      hex.EncodeToString(v.Hash[:]),
	}, nil
}

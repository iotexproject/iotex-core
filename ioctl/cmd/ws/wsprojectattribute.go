package ws

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var (
	wsProjectAttributeCmd = &cobra.Command{
		Use: "attributes",
		Short: config.TranslateInLang(map[config.Language]string{
			config.English: "project attributes operations",
			config.Chinese: "项目属性配置操作",
		}, config.UILanguage),
	}

	wsProjectAttributeSetCmd = &cobra.Command{
		Use: "set",
		Short: config.TranslateInLang(map[config.Language]string{
			config.English: "set project attributes",
			config.Chinese: "配置项目属性",
		}, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := setAttribute(
				big.NewInt(int64(projectID.Value().(uint64))),
				projectAttrKey.Value().(string),
				projectAttrVal.Value().(string),
			)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(output.JSONString(out))
			return nil
		},
	}

	wsProjectAttributeGetCmd = &cobra.Command{
		Use: "get",
		Short: config.TranslateInLang(map[config.Language]string{
			config.English: "get project attributes",
			config.Chinese: "获取项目属性",
		}, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := getAttribute(
				big.NewInt(int64(projectID.Value().(uint64))),
				projectAttrKey.Value().(string),
			)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(output.JSONString(out))
			return nil
		},
	}
)

func init() {
	projectID.RegisterCommand(wsProjectAttributeSetCmd)
	projectID.MarkFlagRequired(wsProjectAttributeSetCmd)
	projectAttrKey.RegisterCommand(wsProjectAttributeSetCmd)
	projectAttrKey.MarkFlagRequired(wsProjectAttributeSetCmd)
	projectAttrVal.RegisterCommand(wsProjectAttributeSetCmd)
	projectAttrVal.MarkFlagRequired(wsProjectAttributeSetCmd)

	projectID.RegisterCommand(wsProjectAttributeGetCmd)
	projectID.MarkFlagRequired(wsProjectAttributeGetCmd)
	projectAttrKey.RegisterCommand(wsProjectAttributeGetCmd)
	projectAttrKey.MarkFlagRequired(wsProjectAttributeGetCmd)

	wsProjectAttributeCmd.AddCommand(wsProjectAttributeSetCmd)
	wsProjectAttributeCmd.AddCommand(wsProjectAttributeGetCmd)

	wsProject.AddCommand(wsProjectAttributeCmd)
}

func getAttribute(projectID *big.Int, key string) (any, error) {
	caller, err := NewContractCaller(projectStoreABI, projectStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract caller")
	}
	result := NewContractResult(&projectStoreABI, funcGetProjectAttr, new([]byte))
	keysig := crypto.Keccak256Hash([]byte(key))
	if err = caller.Read(funcGetProjectAttr, []any{projectID, keysig}, result); err != nil {
		return nil, errors.Wrap(err, "failed to read contract")
	}
	v, err := result.Result()
	if err != nil {
		return nil, err
	}
	return &struct {
		ProjectID uint64 `json:"projectID"`
		Key       string `json:"key"`
		KeySig    string `json:"keySig"`
		Val       string `json:"val"`
	}{
		ProjectID: projectID.Uint64(),
		Key:       key,
		KeySig:    hex.EncodeToString(keysig[:]),
		Val:       string(*v.(*[]byte)),
	}, nil
}

func setAttribute(projectID *big.Int, key, val string) (any, error) {
	caller, err := NewContractCaller(projectStoreABI, projectStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract caller")
	}

	value := new(contracts.W3bstreamProjectAttributeSet)
	result := NewContractResult(&projectStoreABI, eventProjectAttrSet, value)
	keysig := crypto.Keccak256Hash([]byte(key))
	if _, err = caller.CallAndRetrieveResult(funcSetProjectAttr, []any{
		projectID,
		[][32]byte{keysig},
		[][]byte{[]byte(val)},
	}, result); err != nil {
		return nil, errors.Wrap(err, "failed to read contract")
	}
	if _, err = result.Result(); err != nil {
		return nil, err
	}
	return &struct {
		ProjectID uint64 `json:"projectID"`
		Key       string `json:"key"`
		KeySig    string `json:"keySig"`
		Val       string `json:"val"`
	}{
		ProjectID: projectID.Uint64(),
		Key:       key,
		KeySig:    hex.EncodeToString(value.Key[:]),
		Val:       string(value.Value),
	}, nil
}

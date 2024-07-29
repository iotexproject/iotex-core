package ws

import (
	"bytes"
	_ "embed" // used to embed contract abi

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
)

var wsProverCmd = &cobra.Command{
	Use: "prover",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "w3bstream prover management",
		config.Chinese: "w3bstream prover 节点管理",
	}, config.UILanguage),
}

var (
	proverID       = flag.NewUint64VarP("id", "", 0, config.TranslateInLang(_flagProverIDUsages, config.UILanguage))
	proverNodeType = flag.NewUint64VarP("node-type", "", 0, config.TranslateInLang(_flagProverNodeTypeUsages, config.UILanguage))
	proverOperator = flag.NewStringVarP("operator", "", "", config.TranslateInLang(_flagProverOperatorUsages, config.UILanguage))
)

var (
	_flagProverIDUsages = map[config.Language]string{
		config.English: "prover id",
		config.Chinese: "prover(计算节点) ID",
	}
	_flagProverNodeTypeUsages = map[config.Language]string{
		config.English: "prover node type",
		config.Chinese: "prover(计算节点) 节点类型",
	}
	_flagProverOperatorUsages = map[config.Language]string{
		config.English: "prover node operator",
		config.Chinese: "prover(计算节点)操作者",
	}
)

var (
	//go:embed contracts/abis/W3bstreamProver.json
	proverStoreJSON    []byte
	proverStoreAddress string
	proverStoreABI     abi.ABI

	//go:embed contracts/abis/FleetManagement.json
	fleetManagementJSON    []byte
	fleetManagementAddress string
	fleetManagementABI     abi.ABI
)

const (
	funcProverRegister      = "register"
	funcAddProverType       = "addNodeType"
	funcDelProverType       = "delNodeType"
	funcQueryProverNodeType = "hasNodeType"
	funcQueryProverIsPaused = "isPaused"
	funcQueryProverOperator = "operator"
	funcQueryProverOwner    = "prover"
	funcPauseProver         = "pause"
	funcResumeProver        = "resume"
	funcChangeProverOwner   = "changeOperator"
)

const (
	eventOnProverRegistered   = "Transfer"
	eventOnProverTypeAdded    = "NodeTypeAdded"
	eventOnProverTypeDeleted  = "NodeTypeDeleted"
	eventOnProverPaused       = "ProverPaused"
	eventOnProverResumed      = "ProverResumed"
	eventOnProverOwnerChanged = "OperatorSet"
)

func init() {
	var err error
	proverStoreABI, err = abi.JSON(bytes.NewReader(proverStoreJSON))
	if err != nil {
		panic(err)
	}
	proverStoreAddress = config.ReadConfig.WsProverStoreContract
	fleetManagementABI, err = abi.JSON(bytes.NewReader(fleetManagementJSON))
	if err != nil {
		panic(err)
	}
	fleetManagementAddress = config.ReadConfig.WsFleetManagementContract

	WsCmd.AddCommand(wsProverCmd)
}

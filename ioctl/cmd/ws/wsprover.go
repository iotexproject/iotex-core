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
	proverStoreJSON []byte
	// proverStoreAddress = "0x57C1B2b85e28A7EEbced2e4ccc397d093D45E50c"
	proverStoreAddress = "0xa9bed62ADB1708E0c501664C9CE6A34BC4Fc246b"
	proverStoreABI     abi.ABI

	//go:embed contracts/abis/FleetManagement.json
	fleetManagementJSON    []byte
	fleetManagementAddress = "0x698D8cEfe0c2E603DCA4B7815cb8E67F251eCF37"
	fleetManagementABI     abi.ABI
)

const (
	funcProverRegister      = "register"
	funcUpdateProver        = "updateNodeType"
	funcQueryProverNodeType = "nodeType"
	funcQueryProverIsPaused = "isPaused"
	funcPauseProver         = "pause"
	funcResumeProver        = "resume"
	funcChangeProverOwner   = "changeOperator"
)

const (
	eventOnProverRegistered   = "Transfer"
	eventOnProverUpdated      = "NodeTypeUpdated"
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
	fleetManagementABI, err = abi.JSON(bytes.NewReader(fleetManagementJSON))
	if err != nil {
		panic(err)
	}

	WsCmd.AddCommand(wsProverCmd)
}

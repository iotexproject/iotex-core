package ws

import (
	"bytes"
	_ "embed" // used to embed contract abi

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
)

var wsVmTypeCmd = &cobra.Command{
	Use: "vmtype",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "w3bstream zkp vm type management",
		config.Chinese: "w3bstream 零知识证明虚拟机类型管理",
	}, config.UILanguage),
}

var (
	vmTypeID   = flag.NewUint64VarP("id", "", 0, config.TranslateInLang(_flagVmTypeIDUsages, config.UILanguage))
	vmTypeName = flag.NewStringVarP("vm-type", "", "", config.TranslateInLang(_flagVmTypeNameUsages, config.UILanguage))
)

var (
	_flagVmTypeIDUsages = map[config.Language]string{
		config.English: "vmType id",
		config.Chinese: "vmType(虚拟机类型) ID",
	}
	_flagVmTypeNameUsages = map[config.Language]string{
		config.English: "vm type",
		config.Chinese: "vmType(虚拟机类型) 名",
	}
)

var (
	//go:embed contracts/abis/W3bstreamVMType.json
	vmTypeJSON    []byte
	vmTypeAddress string
	vmTypeABI     abi.ABI
)

const (
	funcVmTypeMint          = "mint"
	funcQueryVmType         = "vmTypeName"
	funcQueryVmTypeIsPaused = "isPaused"
	funcVmTypePause         = "pause"
	funcVmTypeResume        = "resume"
)

const (
	eventOnVmTypePaused  = "VMTypePaused"
	eventOnVmTypeResumed = "VMTypeResumed"
	eventOnVmTypeSet     = "VMTypeSet"
)

func init() {
	var err error
	vmTypeABI, err = abi.JSON(bytes.NewReader(vmTypeJSON))
	if err != nil {
		panic(err)
	}
	vmTypeAddress = config.ReadConfig.WsVmTypeContract

	WsCmd.AddCommand(wsVmTypeCmd)
}

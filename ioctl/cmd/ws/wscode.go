package ws

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

var (
	// wsCode represents the w3bstream code command
	wsCode = &cobra.Command{
		Use:   "code",
		Short: config.TranslateInLang(wsCodeShorts, config.UILanguage),
	}

	// wsCodeShorts w3bstream code shorts multi-lang support
	wsCodeShorts = map[config.Language]string{
		config.English: "ws code operations",
		config.Chinese: "ws代码操作",
	}
)

func init() {
	wsCode.AddCommand(wsCodeConvert)
}

// zkp vm type
type vmType string

const (
	risc0  vmType = "risc0"  // risc0 vm
	halo2  vmType = "halo2"  // halo2 vm
	zkWasm vmType = "zkwasm" // zkwasm vm
)

func stringToVMType(vmType string) (vmType, error) {
	switch vmType {
	case string(risc0):
		return risc0, nil
	case string(halo2):
		return halo2, nil
	case string(zkWasm):
		return zkWasm, nil
	default:
		return "", errors.New(fmt.Sprintf("not support %s type, just support %s, %s, and %s", vmType, risc0, halo2, zkWasm))
	}
}

package ins

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/sha3"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

// Multi-language support
var (
	_registerUsages = map[config.Language]string{
		config.English: "register NAME",
		config.Chinese: "register 名称",
	}
	_registerShorts = map[config.Language]string{
		config.English: "Register INS name",
		config.Chinese: "注册 INS 域名",
	}
	_registerControllerUsages = map[config.Language]string{
		config.English: "controller contract address",
		config.Chinese: "控制器合约地址",
	}
	_registerResolverUsages = map[config.Language]string{
		config.English: "resolver contract address",
		config.Chinese: "解析器合约地址",
	}
	_registerOwnerUsages = map[config.Language]string{
		config.English: "name owner address",
		config.Chinese: "域名 Owner 地址",
	}
)

var (
	duration   = big.NewInt(365 * 24 * 60 * 60)
	secret     = [32]byte{}
	controller string
	resolver   string
	owner      string

	setAddrABI = `[
		{
      "inputs": [
        {
          "internalType": "bytes32",
          "name": "node",
          "type": "bytes32"
        },
        {
          "internalType": "address",
          "name": "a",
          "type": "address"
        }
      ],
      "name": "setAddr",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    }
	]`
	controllerABI = `[
		{
      "inputs": [
        {
          "internalType": "bytes32",
          "name": "commitment",
          "type": "bytes32"
        }
      ],
      "name": "commit",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
		{
      "inputs": [
        {
          "internalType": "string",
          "name": "name",
          "type": "string"
        },
        {
          "internalType": "uint256",
          "name": "duration",
          "type": "uint256"
        }
      ],
      "name": "rentPrice",
      "outputs": [
        {
          "components": [
            {
              "internalType": "uint256",
              "name": "base",
              "type": "uint256"
            },
            {
              "internalType": "uint256",
              "name": "premium",
              "type": "uint256"
            }
          ],
          "internalType": "struct IPriceOracle.Price",
          "name": "price",
          "type": "tuple"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
		{
      "inputs": [
        {
          "internalType": "string",
          "name": "name",
          "type": "string"
        },
        {
          "internalType": "address",
          "name": "owner",
          "type": "address"
        },
        {
          "internalType": "uint256",
          "name": "duration",
          "type": "uint256"
        },
        {
          "internalType": "bytes32",
          "name": "secret",
          "type": "bytes32"
        },
        {
          "internalType": "address",
          "name": "resolver",
          "type": "address"
        },
        {
          "internalType": "bytes[]",
          "name": "data",
          "type": "bytes[]"
        },
        {
          "internalType": "bool",
          "name": "reverseRecord",
          "type": "bool"
        },
        {
          "internalType": "uint16",
          "name": "ownerControlledFuses",
          "type": "uint16"
        }
      ],
      "name": "register",
      "outputs": [],
      "stateMutability": "payable",
      "type": "function"
    }
	]`
)

// _insRegisterCmd represents the action hash command
var _insRegisterCmd = &cobra.Command{
	Use:   config.TranslateInLang(_registerUsages, config.UILanguage),
	Short: config.TranslateInLang(_registerShorts, config.UILanguage),
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := register(args)
		return output.PrintError(err)
	},
}

func init() {
	_insRegisterCmd.Flags().StringVarP(
		&controller, "controller", "c",
		"0x8aA6acF9BFeEE0243578305706766065180E68d4",
		config.TranslateInLang(_registerControllerUsages, config.UILanguage),
	)
	_insRegisterCmd.Flags().StringVarP(
		&resolver, "resolver", "r",
		"0x41B9132D4661E016A09a61B314a1DFc0038CE3e8",
		config.TranslateInLang(_registerResolverUsages, config.UILanguage),
	)
	_insRegisterCmd.Flags().StringVarP(
		&owner, "owner", "w",
		"",
		config.TranslateInLang(_registerOwnerUsages, config.UILanguage),
	)

	// init secret
	sha := sha3.NewLegacyKeccak256()
	sha.Write([]byte("secret"))
	hash := sha.Sum(nil)
	copy(secret[:], hash)
}

func makeCommitment(
	name string,
	owner common.Address,
	duration *big.Int,
	secret [32]byte,
	data [][]byte,
	reverseRecord bool,
	ownerControlledFuses uint16,
) ([32]byte, error) {
	resolver := common.HexToAddress(resolver)
	sha := sha3.NewLegacyKeccak256()
	sha.Write([]byte(name))
	lable := [32]byte{}
	copy(lable[:], sha.Sum(nil))

	Bytes32, _ := abi.NewType("bytes32", "", nil)
	Address, _ := abi.NewType("address", "", nil)
	Uint256, _ := abi.NewType("uint256", "", nil)
	BytesArray, _ := abi.NewType("bytes[]", "", nil)
	Bool, _ := abi.NewType("bool", "", nil)
	Uint16, _ := abi.NewType("uint16", "", nil)
	args := abi.Arguments{
		{Type: Bytes32, Name: "name"},
		{Type: Address, Name: "owner"},
		{Type: Uint256, Name: "duration"},
		{Type: Bytes32, Name: "secret"},
		{Type: Address, Name: "resolver"},
		{Type: BytesArray, Name: "data"},
		{Type: Bool, Name: "reverseRecord"},
		{Type: Uint16, Name: "ownerControlledFuses"},
	}

	packed, err := args.Pack(lable, owner, duration, secret, resolver, data, reverseRecord, ownerControlledFuses)
	if err != nil {
		return [32]byte{}, err
	}

	hash := sha3.NewLegacyKeccak256()
	hash.Write(packed)
	result := [32]byte{}
	copy(result[:], hash.Sum(nil))

	return result, nil
}

func commit(name string, data []byte, ioController address.Address, controllerAbi abi.ABI) error {
	commitment, err := makeCommitment(
		name,
		common.HexToAddress(owner),
		duration,
		secret,
		[][]byte{data},
		true,
		0,
	)
	if err != nil {
		return err
	}
	fmt.Printf("commit commitment for %s: %s ...\n", name, hex.EncodeToString(commitment[:]))

	commitData, err := controllerAbi.Pack("commit", commitment)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack commit arguments", err)
	}
	err = action.Execute(ioController.String(), big.NewInt(0), commitData)
	if err != nil {
		return output.NewError(output.UpdateError, "failed to commit commitment", err)
	}
	return nil
}

func register(args []string) error {
	name := args[0]

	if owner == "" {
		signer, err := action.Signer()
		if err != nil {
			return output.NewError(output.AddressError, "failed to get signer address", err)
		}
		signerAddr, err := address.FromString(signer)
		if err != nil {
			return output.NewError(output.AddressError, "failed to convert signer address", err)
		}
		owner = signerAddr.Hex()
	}

	hash, err := nameHash(name)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to compute namehash", err)
	}

	setAddrAbi, err := abi.JSON(strings.NewReader(setAddrABI))
	if err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal abi", err)
	}
	setAddrData, err := setAddrAbi.Pack("setAddr", hash, common.HexToAddress(owner))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack setAddr arguments", err)
	}

	ioController, err := address.FromHex(controller)
	if err != nil {
		return output.NewError(output.AddressError, "failed to convert controller address", err)
	}
	controllerAbi, err := abi.JSON(strings.NewReader(controllerABI))
	if err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal abi", err)
	}

	if err = commit(name, setAddrData, ioController, controllerAbi); err != nil {
		return err
	}

	fmt.Printf("sleep for activation commitment ...\n")
	time.Sleep(60 * time.Second)

	priceData, err := controllerAbi.Pack("rentPrice", name, duration)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack price arguments", err)
	}
	priceHex, err := action.Read(ioController, "0", priceData)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	basePrice, _ := hex.DecodeString(priceHex[:64])
	premiumPrice, _ := hex.DecodeString(priceHex[64:])
	price := new(big.Int).Add(new(big.Int).SetBytes(basePrice), new(big.Int).SetBytes(premiumPrice))

	registerData, err := controllerAbi.Pack(
		"register",
		name,
		common.HexToAddress(owner),
		duration,
		secret,
		common.HexToAddress(resolver),
		[][]byte{setAddrData},
		true,
		uint16(0),
	)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack register arguments", err)
	}

	return action.Execute(ioController.String(), price, registerData)
}

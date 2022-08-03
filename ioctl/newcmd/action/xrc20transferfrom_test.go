package action

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

const (
	contractAddrFlagLabel      = "contract-address"
	contractAddrFlagShortLabel = "c"
	contractAddrFlagUsage      = "set contract address"
)

func TestNewXrc20TransferFrom(t *testing.T) {
	var (
		require = require.New(t)
		ctrl    = gomock.NewController(t)

		wrongAddr         = strings.Repeat("a", 41)
		wrongAlias        = "xxx"
		wrongAmount       = "xxx"
		wrongContractData = "xxx"

		dftOwner        = identityset.Address(0).String()
		dftRecipient    = identityset.Address(1).String()
		dftContract     = identityset.Address(2).String()
		dftAmount       = "100"
		dftContractData = "2F"
	)
	tests := []struct {
		name        string
		args        []string
		prepare     func(*gomock.Controller) *cobra.Command
		expectedErr string
	}{
		{
			"WrongArgCount",
			nil,
			prepare,
			"accepts 3 arg(s), received 0",
		}, {
			"WrongOwnerAlias",
			[]string{wrongAlias, "", ""},
			prepare,
			"failed to get owner address: cannot find address for alias " + wrongAlias,
		}, {
			"WrongOwnerAddress",
			[]string{wrongAddr, "", ""},
			prepare,
			"failed to get owner address: " + wrongAddr + ": invalid IoTeX address",
		}, {
			"NoContractAddress",
			[]string{dftOwner, dftRecipient, ""},
			prepare,
			"failed to get contract address: invalid xrc20 address flag: cannot find address for alias ",
		}, {
			"WrongContractAlias",
			[]string{dftOwner, dftRecipient, dftAmount},
			prepareWithContractAddr(wrongAlias, dftOwner, dftContractData),
			"failed to get contract address: invalid xrc20 address flag: cannot find address for alias " + wrongAlias,
		}, {
			"WrongContractAddress",
			[]string{dftOwner, dftRecipient, dftAmount},
			prepareWithContractAddr(wrongAddr, dftOwner, dftContractData),
			"failed to get contract address: invalid xrc20 address flag: " + wrongAddr + ": invalid IoTeX address",
		}, {
			"WrongAmount",
			[]string{dftOwner, dftRecipient, wrongAmount},
			prepareWithContractAddr(dftContract, dftOwner, dftContractData),
			"failed to parse amount: failed to convert string into big int",
		}, {
			"WrongContractDataRsp",
			[]string{dftOwner, dftRecipient, dftAmount},
			prepareWithContractAddr(dftContract, dftOwner, wrongContractData),
			"failed to parse amount: failed to convert string into int64: strconv.ParseInt: parsing \"" + wrongContractData + "\": invalid syntax",
		}, {
			"BalanceNotEnough",
			[]string{dftOwner, dftRecipient, dftAmount},
			prepareWithFullMockClientAndAPI(dftContract, true, true, false),
			"failed to pass balance check: balance is not enough",
		}, {
			"ShouldPass",
			[]string{dftOwner, dftRecipient, dftAmount},
			prepareWithFullMockClientAndAPI(dftContract, true, true, true),
			"",
		}}
	cwd, _ := os.Getwd()
	defer os.RemoveAll(path.Join(cwd, "passwd"))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.prepare(ctrl)
			result, err := util.ExecuteCmd(cmd, test.args...)
			if test.expectedErr == "" {
				require.NoError(err)
				require.Contains(result, "Action has been sent to blockchain.\nWait for several seconds and query this action by hash:")
			} else {
				require.Equal(test.expectedErr, err.Error())
			}
		})
	}
}

type prepareFn func(*gomock.Controller) *cobra.Command

func prepare(ctrl *gomock.Controller) *cobra.Command {
	cli := mock_ioctlclient.NewMockClient(ctrl)
	api := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	cli.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
	cli.EXPECT().APIServiceClient().Return(api, nil).AnyTimes()
	cli.EXPECT().Xrc20ContractAddr().Return("")
	return NewXrc20TransferFrom(cli)
}

func prepareWithContractAddr(addr, owner, data string) prepareFn {
	return func(ctrl *gomock.Controller) *cobra.Command {
		cli := mock_ioctlclient.NewMockClient(ctrl)
		api := mock_iotexapi.NewMockAPIServiceClient(ctrl)

		cli.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
		cli.EXPECT().APIServiceClient().Return(api, nil).AnyTimes()
		cli.EXPECT().Xrc20ContractAddr().Return(addr)

		if owner != "" {
			cli.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(owner, nil)
			api.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&iotexapi.ReadContractResponse{Data: data}, nil)
		}

		cmd := NewXrc20TransferFrom(cli)
		xrc20ContractAddress := ""
		cmd.PersistentFlags().StringVarP(&xrc20ContractAddress,
			contractAddrFlagLabel,
			contractAddrFlagShortLabel,
			addr,
			contractAddrFlagUsage,
		)
		return cmd
	}
}

func prepareWithFullMockClientAndAPI(
	contractAddr string,
	validContractData bool,
	validOwner bool,
	validBalance bool,
) prepareFn {
	return func(ctrl *gomock.Controller) *cobra.Command {
		passwd := "passwd"
		ks := keystore.NewKeyStore(passwd, 2, 1)
		account, err := ks.NewAccount(passwd)
		if err != nil {
			return nil
		}
		accountAddr, err := address.FromBytes(account.Address.Bytes())
		owner := accountAddr.String()
		if !validOwner {
			owner = "aaaa"
		}

		cli := mock_ioctlclient.NewMockClient(ctrl)
		api := mock_iotexapi.NewMockAPIServiceClient(ctrl)

		cli.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
		cli.EXPECT().APIServiceClient().Return(api, nil).AnyTimes()

		cli.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(owner, nil).AnyTimes()
		cli.EXPECT().NewKeyStore().Return(ks).AnyTimes()
		cli.EXPECT().PackABI(gomock.Any(), gomock.Any()).Return([]byte(""), nil)
		cli.EXPECT().IsCryptoSm2().Return(false).AnyTimes()
		cli.EXPECT().ReadSecret().Return(passwd, nil).AnyTimes()
		cli.EXPECT().Address(gomock.Any()).Return(owner, nil).AnyTimes()
		cli.EXPECT().Alias(gomock.Any()).Return("", nil).AnyTimes()
		cli.EXPECT().AskToConfirm(gomock.Any()).Return(true, nil).AnyTimes()

		contractData := "2f"
		if !validContractData {
			contractData = "xxx"
		}

		api.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&iotexapi.ReadContractResponse{Data: contractData}, nil)
		api.EXPECT().GetAccount(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&iotexapi.GetAccountResponse{
				AccountMeta: &iotextypes.AccountMeta{PendingNonce: 100},
			}, nil)
		api.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).
			Return(&iotexapi.GetChainMetaResponse{}, nil)

		balance := "20000000000000100000"
		if !validBalance {
			balance = "0"
		}
		api.EXPECT().GetAccount(gomock.Any(), gomock.Any()).
			Return(&iotexapi.GetAccountResponse{
				AccountMeta: &iotextypes.AccountMeta{Balance: balance},
			}, nil)
		cli.EXPECT().Xrc20ContractAddr().Return(contractAddr)
		api.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(&iotexapi.SendActionResponse{}, nil)

		cli.EXPECT().Config().Return(config.Config{
			Explorer: "iotexscan",
			Endpoint: "testnet1",
		}).Times(2)

		cmd := NewXrc20TransferFrom(cli)
		xrc20ContractAddress := ""
		cmd.PersistentFlags().StringVarP(&xrc20ContractAddress,
			contractAddrFlagLabel,
			contractAddrFlagShortLabel,
			contractAddr,
			contractAddrFlagUsage,
		)
		return cmd
	}
}

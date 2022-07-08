package action

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestNewXrc20TransferFrom(t *testing.T) {
	var (
		req = require.New(t)
		ctr = gomock.NewController(t)

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
	cases := []struct {
		name    string
		args    []string
		prepare func(*gomock.Controller) *cobra.Command
		err     string
	}{{
		"WrongArgCount", nil, prepare, "accepts 3 arg(s), received 0",
	}, {
		"WrongOwnerAlias", []string{wrongAlias, "", ""}, prepare,
		"failed to get owner address: cannot find address for alias " + wrongAlias,
	}, {
		"WrongOwnerAddress", []string{wrongAddr, "", ""}, prepare,
		"failed to get owner address: " + wrongAddr + ": invalid IoTeX address",
	}, {
		"NoContractAddress",
		[]string{dftOwner, dftRecipient, ""},
		prepare,
		"failed to get contract address: flag accessed but not defined: contract-address",
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

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := c.prepare(ctr)
			_, err := util.ExecuteCmd(cmd, c.args...)
			if c.err == "" {
				req.Nil(err)
			} else {
				req.Equal(err.Error(), c.err)
			}

		})
	}
}

type prepareFn func(*gomock.Controller) *cobra.Command

func prepare(ctr *gomock.Controller) *cobra.Command {
	cli := mock_ioctlclient.NewMockClient(ctr)
	api := mock_iotexapi.NewMockAPIServiceClient(ctr)
	cli.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
	cli.EXPECT().APIServiceClient().Return(api, nil).AnyTimes()
	return NewXrc20TransferFrom(cli)
}

func prepareWithContractAddr(addr, owner, data string) prepareFn {
	return func(ctr *gomock.Controller) *cobra.Command {
		cli := mock_ioctlclient.NewMockClient(ctr)
		api := mock_iotexapi.NewMockAPIServiceClient(ctr)

		cli.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
		cli.EXPECT().APIServiceClient().Return(api, nil).AnyTimes()
		if owner != "" {
			cli.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).
				Return(owner, nil)
			api.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&iotexapi.ReadContractResponse{Data: data}, nil)
		}

		cmd := NewXrc20TransferFrom(cli)
		flag := ""
		cmd.PersistentFlags().StringVarP(&flag,
			contractAddrFlagLabel,
			contractAddrFlagShortLabel,
			addr,
			selectTranslation(cli, _flagContractAddressUsages),
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
	return func(ctr *gomock.Controller) *cobra.Command {
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

		cli := mock_ioctlclient.NewMockClient(ctr)
		api := mock_iotexapi.NewMockAPIServiceClient(ctr)

		cli.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
		cli.EXPECT().APIServiceClient().Return(api, nil).AnyTimes()

		cli.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(owner, nil).AnyTimes()
		cli.EXPECT().NewKeyStore().Return(ks).AnyTimes()
		cli.EXPECT().PackABI(gomock.Any(), gomock.Any()).Return([]byte(""), nil)
		cli.EXPECT().IsCryptoSm2().Return(false).AnyTimes()
		cli.EXPECT().ReadSecret().Return(passwd, nil).AnyTimes()
		cli.EXPECT().Address(gomock.Any()).Return(owner, nil).AnyTimes()
		cli.EXPECT().Alias(gomock.Any()).Return("", nil).AnyTimes()
		cli.EXPECT().AskToConfirm(gomock.Any()).Return(true).AnyTimes()

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

		cmd := NewXrc20TransferFrom(cli)
		_contractAddr := ""
		cmd.PersistentFlags().StringVarP(&_contractAddr,
			contractAddrFlagLabel,
			contractAddrFlagShortLabel,
			contractAddr,
			selectTranslation(cli, _flagContractAddressUsages),
		)
		_ = cmd.Flags().Set(signerFlagLabel, owner)

		return cmd
	}
}

package action

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

var (
	_signByte     = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	_pubKeyString = "04403d3c0dbd3270ddfc248c3df1f9aafd60f1d8e7456961c9ef262" + "92262cc68f0ea9690263bef9e197a38f06026814fc70912c2b98d2e90a68f8ddc5328180a01"
)

func TestNewActionHashCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(6)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(2)

	_pubKeyByte, err := hex.DecodeString(_pubKeyString)
	require.NoError(err)

	t.Run("get action receipt", func(t *testing.T) {
		getActionResponse := &iotexapi.GetActionsResponse{
			ActionInfo: []*iotexapi.ActionInfo{
				{
					Index:   0,
					ActHash: "test",
					Action: &iotextypes.Action{
						SenderPubKey: _pubKeyByte,
						Signature:    _signByte,
						Core:         createEnvelope(0).Proto(),
					},
				},
			},
		}
		getReceiptResponse := &iotexapi.GetReceiptByActionResponse{
			ReceiptInfo: &iotexapi.ReceiptInfo{
				Receipt: &iotextypes.Receipt{
					Status:             1,
					BlkHeight:          12,
					ActHash:            []byte("9b1d77d8b8902e8d4e662e7cd07d8a74179e032f030d92441ca7fba1ca68e0f4"),
					GasConsumed:        123,
					ContractAddress:    "test",
					TxIndex:            1,
					ExecutionRevertMsg: "balance not enough",
				},
			},
		}
		apiServiceClient.EXPECT().GetActions(context.Background(), gomock.Any()).Return(getActionResponse, nil)
		apiServiceClient.EXPECT().GetReceiptByAction(context.Background(), gomock.Any()).Return(getReceiptResponse, nil)

		cmd := NewActionHashCmd(client)
		result, err := util.ExecuteCmd(cmd, "test")
		require.NoError(err)
		require.Contains(result, "status: 1 (Success)\n")
		require.Contains(result, "blkHeight: 12\n")
		require.Contains(result, "gasConsumed: 123\n")
		require.Contains(result, "senderPubKey: "+_pubKeyString+"\n")
		require.Contains(result, "signature: 010203040506070809\n")
	})

	t.Run("no action info returned", func(t *testing.T) {
		getActionResponse := &iotexapi.GetActionsResponse{
			ActionInfo: []*iotexapi.ActionInfo{},
		}
		expectedErr := errors.New("no action info returned")
		apiServiceClient.EXPECT().GetActions(context.Background(), gomock.Any()).Return(getActionResponse, nil)

		cmd := NewActionHashCmd(client)
		_, err := util.ExecuteCmd(cmd, "test")
		require.Equal(expectedErr.Error(), err.Error())
	})

	t.Run("failed to dial grpc connection", func(t *testing.T) {
		expectedErr := errors.New("failed to dial grpc connection")
		client.EXPECT().APIServiceClient().Return(nil, expectedErr).Times(1)

		cmd := NewActionHashCmd(client)
		_, err := util.ExecuteCmd(cmd, "test")
		require.Equal(expectedErr, err)
	})
}

func createEnvelope(chainID uint32) action.Envelope {
	tsf, _ := action.NewTransfer(
		uint64(10),
		unit.ConvertIotxToRau(1000+int64(10)),
		identityset.Address(10%identityset.Size()).String(),
		nil,
		20000+uint64(10),
		unit.ConvertIotxToRau(1+int64(10)),
	)
	eb := action.EnvelopeBuilder{}
	return eb.
		SetAction(tsf).
		SetGasLimit(tsf.GasLimit()).
		SetGasPrice(tsf.GasPrice()).
		SetNonce(tsf.Nonce()).
		SetVersion(1).
		SetChainID(chainID).Build()
}

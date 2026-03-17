package blockchain

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
)

func TestNewBlockchainSkipsWitnessStoreForCSSyncConsumer(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dao := mock_blockdao.NewMockBlockDAO(ctrl)
	cfg := DefaultConfig
	cfg.WitnessDBPath = t.TempDir() + "/witness.db"
	cfg.EnableExperimentalCSSync = true

	bc := NewBlockchain(cfg, genesis.Default, dao, nil)
	chain, ok := bc.(*blockchain)
	require.True(ok)
	require.Nil(chain.witnessStore)
}

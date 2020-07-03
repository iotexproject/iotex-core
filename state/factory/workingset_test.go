package factory

import (
	"context"
	"errors"
	"testing"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/stretchr/testify/require"
)


func TestReadView(t *testing.T){
	require := require.New(t)
	sf, err := NewFactory(config.Default, InMemTrieOption())
	require.NoError(err)
	ctx := protocol.WithBlockchainCtx(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		protocol.BlockchainCtx{Genesis: config.Default.Genesis},
	)
	require.NoError(sf.Start(ctx))
	ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 1)
	retrdf,retErr := ws.ReadView("")
	expectedErrors := errors.New("name : name does not exist")
	require.EqualError(retErr, expectedErrors.Error())
	require.Equal(retrdf, "")
}

// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package indexservice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
)

func TestServer(t *testing.T) {
	require := require.New(t)

	// create chain
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemDaoOption())

	svr := NewServer(config.Default, bc)
	err := svr.Start(context.Background())
	require.Nil(err)

	db := svr.idx.store.GetDB()

	// get
	_, err = db.Prepare("SELECT * FROM index_history_transfer WHERE node_address=?")
	require.Nil(err)

	err = svr.Stop(context.Background())
	require.Nil(err)
}

// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package indexservice

import (
	"testing"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	t.Skip("Skipping when RDS credentail not provided.")

	require := require.New(t)

	// create chain
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemDaoOption())

	svr := NewServer(&config.Default, bc)
	err := svr.Start(nil)
	require.Nil(err)

	db := svr.idx.rds.GetDB()

	// get
	_, err = db.Prepare("SELECT * FROM transfer_history WHERE node_address=?")
	require.Nil(err)

	err = svr.Stop(nil)
	require.Nil(err)
}

package filedao

import (
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

func TestBlockStore(t *testing.T) {
	r := require.New(t)
	builder := block.NewTestingBuilder()
	blk := createTestingBlock(builder, 1, hash.ZeroHash256)
	bs, err := convertToBlockStore(blk)
	r.NoError(err)
	data := bs.Serialize()
	dbs := new(blockstore)
	r.NoError(dbs.Deserialize(data))
	r.Equal(bs[:], (*dbs)[:], "serialized block store should be equal to deserialized block store")
	// check deserialized block
	deser := block.NewDeserializer(0)
	dBlk, err := dbs.Block(deser)
	r.NoError(err)
	dTxLogs, err := dbs.TransactionLogs()
	r.NoError(err)
	r.NoError(fillTransactionLog(dBlk.Receipts, dTxLogs.Logs))
	r.Equal(blk.Header, dBlk.Header)
	r.Equal(blk.Body, dBlk.Body)
	r.Equal(blk.Footer, dBlk.Footer)
	r.Equal(len(blk.Receipts), len(dBlk.Receipts))
	for i := range blk.Receipts {
		r.Equal(blk.Receipts[i].Hash(), dBlk.Receipts[i].Hash())
		r.Equal(len(blk.Receipts[i].TransactionLogs()), len(dBlk.Receipts[i].TransactionLogs()))
		for j := range blk.Receipts[i].TransactionLogs() {
			r.Equal(blk.Receipts[i].TransactionLogs()[j], dBlk.Receipts[i].TransactionLogs()[j])
		}
	}
}

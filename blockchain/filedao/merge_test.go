package filedao

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
)

func TestMergeDao(t *testing.T) {
	r := require.New(t)
	cfg := db.DefaultConfig
	cfg.DbPath = t.TempDir() + "/filedao"
	deser := block.NewDeserializer(4689)
	fdao, err := NewFileDAO(cfg, deser)
	r.NoError(err)
	ctx := context.Background()
	r.NoError(fdao.Start(ctx))
	r.NoError(testCommitBlocks(t, fdao, 1, 100, hash.ZeroHash256))
	r.NoError(fdao.Stop(ctx))

	fdao, err = NewFileDAO(cfg, deser)
	r.NoError(err)
	sdao, err := NewSizedFileDao(10, t.TempDir(), deser)
	r.NoError(err)
	mdao := NewMergeDao(fdao, sdao, 10)
	r.NoError(mdao.Start(ctx))
	_, err = mdao.GetBlockByHeight(1)
	r.ErrorIs(err, db.ErrNotExist)
	_, err = mdao.GetBlockByHeight(90)
	r.ErrorIs(err, db.ErrNotExist)
	_, err = mdao.GetBlockByHeight(91)
	r.NoError(err)
	r.NoError(mdao.Stop(ctx))
}

package p2p

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBlockList(t *testing.T) {
	r := require.New(t)

	now := time.Now()
	name := "alfa"
	withinBlockTTL := now.Add(blockListTTL / 2)
	pastBlockTTL := now.Add(blockListTTL * 2)

	blockTests := []struct {
		curTime time.Time
		blocked bool
	}{
		{now, false},
		{now, false},
		{withinBlockTTL, true},
		{withinBlockTTL, true},
		{pastBlockTTL, false},
		{withinBlockTTL, false},
		{withinBlockTTL, false},
		{pastBlockTTL, false},
	}

	list := NewBlockList(10)
	for _, v := range blockTests {
		list.Add(name, now)
		r.Equal(v.blocked, list.Blocked(name, v.curTime))
	}
}

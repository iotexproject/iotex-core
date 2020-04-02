// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDock(t *testing.T) {
	r := require.New(t)

	dk := NewDock()
	r.NotNil(dk)

	testDocks := []struct {
		name string
		v    int
	}{
		{
			"test1", 1,
		},
		{
			"test2", 2,
		},
		{
			"test3", 3,
		},
		{
			"test4", 1,
		},
	}

	for _, e := range testDocks {
		r.NoError(dk.Load(e.name, e.v))
	}
	r.True(dk.Dirty())
	r.NoError(dk.Push())

	for _, e := range testDocks {
		r.True(dk.ProtocolDirty(e.name))
		v, err := dk.Unload(e.name)
		r.NoError(err)
		r.Equal(e.v, v.(int))
	}

	// overwrite one, and add a new one
	r.NoError(dk.Load(testDocks[1].name, 5))
	r.NoError(dk.Load("test5", 5))
	r.NoError(dk.Push())
	v, err := dk.Unload(testDocks[1].name)
	r.NoError(err)
	r.Equal(5, v.(int))
	v, err = dk.Unload("test5")
	r.NoError(err)
	r.Equal(5, v.(int))

	dk.Reset()
	r.False(dk.Dirty())
	r.False(dk.ProtocolDirty(testDocks[1].name))
}

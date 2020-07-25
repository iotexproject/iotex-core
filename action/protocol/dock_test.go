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

type testString struct {
	s string
}

func (ts *testString) Serialize() ([]byte, error) {
	return []byte(ts.s), nil
}

func (ts *testString) Deserialize(v []byte) error {
	ts.s = string(v)
	return nil
}

func TestDock(t *testing.T) {
	r := require.New(t)

	dk := NewDock()
	r.NotNil(dk)

	testDocks := []struct {
		name, key string
		v         *testString
	}{
		{
			"ns", "test1", &testString{"v1"},
		},
		{
			"ns", "test2", &testString{"v2"},
		},
		{
			"vs", "test3", &testString{"v3"},
		},
		{
			"ts", "test4", &testString{"v4"},
		},
	}

	for _, e := range testDocks {
		r.NoError(dk.Load(e.name, e.key, e.v))
	}

	for _, e := range testDocks {
		r.True(dk.ProtocolDirty(e.name))
		ts := &testString{}
		r.NoError(dk.Unload(e.name, e.key, ts))
		r.Equal(e.v, ts)
	}

	// overwrite one, and add a new one
	v5 := &testString{"v5"}
	r.NoError(dk.Load(testDocks[1].name, testDocks[1].key, v5))
	v6 := &testString{"v6"}
	r.NoError(dk.Load("ts", "test6", v6))
	ts := &testString{}
	r.NoError(dk.Unload(testDocks[1].name, testDocks[1].key, ts))
	r.Equal(v5, ts)
	r.NoError(dk.Unload("ts", "test6", ts))
	r.Equal(v6, ts)

	dk.Reset()
	for _, e := range testDocks {
		r.False(dk.ProtocolDirty(e.name))
	}
}

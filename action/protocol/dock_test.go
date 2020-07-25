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

	// empty dock does not contain a thing
	ts := &testString{}
	for _, e := range testDocks {
		r.False(dk.ProtocolDirty(e.name))
		r.Equal(ErrNoName, dk.Unload(e.name, e.key, ts))
	}

	// populate the dock, and verify existence
	for _, e := range testDocks {
		r.NoError(dk.Load(e.name, e.key, e.v))
		r.True(dk.ProtocolDirty(e.name))
		r.NoError(dk.Unload(e.name, e.key, ts))
		r.Equal(e.v, ts)
		// test key that does not exist
		r.Equal(ErrNoName, dk.Unload(e.name, "notexist", ts))
	}

	// overwrite one
	v5 := &testString{"v5"}
	r.NoError(dk.Load(testDocks[1].name, testDocks[1].key, v5))
	r.NoError(dk.Unload(testDocks[1].name, testDocks[1].key, ts))
	r.Equal(v5, ts)

	// add a new one
	v6 := &testString{"v6"}
	r.NoError(dk.Load("as", "test6", v6))
	r.True(dk.ProtocolDirty("as"))
	r.NoError(dk.Unload("as", "test6", ts))
	r.Equal(v6, ts)

	dk.Reset()
	for _, e := range testDocks {
		r.False(dk.ProtocolDirty(e.name))
	}
	r.False(dk.ProtocolDirty("as"))
}

// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHdwPath(t *testing.T) {
	r := require.New(t)

	tests := []struct {
		addressOrAlias string
		a, b, c        uint32
		err            string
	}{
		{"hdw::0/1/2", 0, 1, 2, ""},
		{"hdw::0/1", 0, 1, 0, ""},
		{"hdw::0", 0, 0, 0, "derivation path error"},
		{"hdw::", 0, 0, 0, "derivation path error"},
		{"hdw::0/1/2/3", 0, 0, 0, "derivation path error"},
		{"hdw::a/3", 0, 0, 0, "must be integer value"},
		{"hdw::a/b", 0, 0, 0, "must be integer value"},
		{"hdw::1/23b", 0, 0, 0, "must be integer value"},
	}
	for _, v := range tests {
		a, b, c, err := parseHdwPath(v.addressOrAlias)
		r.Equal(a, v.a)
		r.Equal(b, v.b)
		r.Equal(c, v.c)
		if err != nil {
			r.Contains(err.Error(), v.err)
		}
	}
}

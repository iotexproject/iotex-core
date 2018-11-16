// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package prometheustimer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrometheusTimer(t *testing.T) {
	require := require.New(t)
	factory, err := New("test", "test", []string{"default"}, nil)
	require.Error(err)
	require.Nil(factory)
	timer := factory.NewTimer("test")
	require.NotNil(timer)
	factory, err = New("test", "test", []string{"default"}, []string{"default"})
	require.NoError(err)
	require.NotNil(factory)
	timer = factory.NewTimer("label")
	require.NotNil(timer)
}

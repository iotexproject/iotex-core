// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package lifecycle

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReady(t *testing.T) {
	r := require.New(t)

	ready := Readiness{}
	r.False(ready.IsReady())
	r.Equal(ErrWrongState, ready.TurnOff())

	// ready after turn on
	r.NoError(ready.TurnOn())
	r.True(ready.IsReady())
	r.Equal(ErrWrongState, ready.TurnOn()) // cannot turn on again

	// not ready after turn off
	r.NoError(ready.TurnOff())
	r.False(ready.IsReady())
	r.Equal(ErrWrongState, ready.TurnOff()) // cannot turn off again
}

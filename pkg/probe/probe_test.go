// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package probe

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/testutil"
)

type testCase struct {
	endpoint string
	code     int
}

func testFunc(t *testing.T, ts []testCase) {
	for _, tt := range ts {
		resp, err := http.Get("http://localhost:7788" + tt.endpoint)
		require.NoError(t, err)
		assert.Equal(t, tt.code, resp.StatusCode)
	}
}

func TestBasicProbe(t *testing.T) {
	s := New(7788)
	ctx := context.Background()
	require.NoError(t, s.Start(ctx))
	require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 2*time.Second, func() (b bool, e error) {
		_, err := http.Get("http://localhost:7788/liveness")
		return err == nil, nil
	}))
	test1 := []testCase{
		{
			endpoint: "/liveness",
			code:     http.StatusOK,
		},
		{
			endpoint: "/readiness",
			code:     http.StatusServiceUnavailable,
		},
		{
			endpoint: "/health",
			code:     http.StatusServiceUnavailable,
		},
	}
	testFunc(t, test1)

	test2 := []testCase{
		{
			endpoint: "/liveness",
			code:     http.StatusOK,
		},
		{
			endpoint: "/readiness",
			code:     http.StatusOK,
		},
		{
			endpoint: "/health",
			code:     http.StatusOK,
		},
	}
	s.Ready()
	testFunc(t, test2)
	s.NotReady()
	testFunc(t, test1)

	require.NoError(t, s.Stop(ctx))
	_, err := http.Get("http://localhost:7788/liveness")
	require.Error(t, err)
}

func TestReadniessHandler(t *testing.T) {
	ctx := context.Background()
	s := New(7788, WithReadinessHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})))
	defer s.Stop(ctx)

	require.NoError(t, s.Start(ctx))
	require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 2*time.Second, func() (b bool, e error) {
		_, err := http.Get("http://localhost:7788/liveness")
		return err == nil, nil
	}))
	test := []testCase{
		{
			endpoint: "/liveness",
			code:     http.StatusOK,
		},
		{
			endpoint: "/readiness",
			code:     http.StatusAccepted,
		},
		{
			endpoint: "/health",
			code:     http.StatusAccepted,
		},
	}
	s.Ready()
	testFunc(t, test)
}

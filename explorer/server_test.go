// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"github.com/iotexproject/iotex-core/logger"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	require := require.New(t)
	StartJSONServer(nil, nil, nil, nil, true, "14004", 0)

	timeout := time.Duration(20 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get("http://127.0.0.1:14004")
	if err != nil {
		logger.Error().Err(err).Msg("Error:")
	} else {
		require.Equal("200 OK", resp.Status)
	}
}

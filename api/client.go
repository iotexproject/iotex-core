// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"github.com/coopernurse/barrister-go"

	"github.com/iotexproject/iotex-core/api/idl/api"
)

// NewAPIProxy accepts an URL to the endpoint of the API server
// and returns a proxy that implements the API interface
func NewAPIProxy(url string) api.API {
	trans := &barrister.HttpTransport{Url: url}

	client := barrister.NewRemoteClient(trans, true)

	// calc.NewExplorerProxy() is provided by the idl2go
	// generated calc.go file
	return api.NewAPIProxy(client)
}

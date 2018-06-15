// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"net/http"

	"github.com/coopernurse/barrister-go"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/logger"
)

// LogFilter example of Filter implementation
type LogFilter struct{}

// PreInvoke implement empty preinvoke
func (f LogFilter) PreInvoke(r *barrister.RequestResponse) bool {
	logger.Debug().Msgf("LogFilter: PreInvoke of method:", r.Method)
	return true
}

// PostInvoke implement empty postinvoke
func (f LogFilter) PostInvoke(r *barrister.RequestResponse) bool {
	logger.Debug().Msgf("LogFilter: PostInvoke of method:", r.Method)
	return true
}

// StartJSONServer start the json server for explorer
func StartJSONServer(blockchain blockchain.Blockchain, isTest bool, port string) {
	svc := Service{
		bc: blockchain,
	}
	idl := barrister.MustParseIdlJson([]byte(explorer.IdlJsonRaw))
	svr := explorer.NewJSONServer(idl, true, &svc)
	if isTest {
		svr = explorer.NewJSONServer(idl, true, &TestExplorer{})
	}
	svr.AddFilter(LogFilter{})
	http.Handle("/", &svr)

	logger.Info().Msg("Starting Explorer JSON-RPC server on localhost:" + port)
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			logger.Error().Msg(err.Error())
			panic(err)
		}
	}()
}

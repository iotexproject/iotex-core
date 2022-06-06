package coretypes

import (
	abci "github.com/iotexproject/iotex-core/api"
)

// ResultCheckTx wraps abci.ResponseCheckTx.
type ResultCheckTx struct {
	abci.ResponseCheckTx
}

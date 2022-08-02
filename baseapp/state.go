package baseapp

import (
	sdk "github.com/iotexproject/iotex-sdk/types"
)

type state struct {
	ctx sdk.Context
}

// Context returns the Context of the state.
func (st *state) Context() sdk.Context {
	return st.ctx
}

// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithTipBlockContext(t *testing.T) {
	require := require.New(t)
	tbc := TipBlockContext{}
	require.NotNil(WithTipBlockContext(context.Background(), tbc))
}

func TestExtractTipBlockContext(t *testing.T) {
	require := require.New(t)
	tbc := TipBlockContext{}
	ctx := WithTipBlockContext(context.Background(), tbc)
	require.NotNil(ctx)
	_, ok := ExtractTipBlockContext(ctx)
	require.True(ok)
}

func TestMustExtractTipBlockContext(t *testing.T) {
	require := require.New(t)
	tbc := TipBlockContext{}
	ctx := WithTipBlockContext(context.Background(), tbc)
	require.NotNil(ctx)
	// Case I: Normal
	ret := MustExtractTipBlockContext(ctx)
	require.NotNil(ret)
	// Case II: Panic
	require.Panics(func() { MustExtractTipBlockContext(context.Background()) }, "Miss tip block context")
}

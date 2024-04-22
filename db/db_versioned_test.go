// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPb(t *testing.T) {
	r := require.New(t)

	vn := &versionedNamespace{
		name:   "3jfsp5@(%)EW*#)_#¡ªº–ƒ˚œade∆…",
		keyLen: 5}
	data := vn.serialize()
	r.Equal("0a29336a667370354028252945572a23295f23c2a1c2aac2bae28093c692cb9ac593616465e28886e280a61005", hex.EncodeToString(data))
	vn1, err := deserializeVersionedNamespace(data)
	r.NoError(err)
	r.Equal(vn, vn1)
}

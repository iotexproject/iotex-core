// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txvm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/iotxaddress"
)

func TestPayToAddrScriptTrue(t *testing.T) {
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)

	locks, err := PayToAddrScript(addr.RawAddress)
	assert.Nil(t, err)

	t.Log(locks)

	parsed, err := printScript(locks)
	assert.Nil(t, err)

	assert.True(t, strings.HasPrefix(parsed, "OpDup OpHash160"))
	assert.True(t, strings.HasSuffix(parsed, "OpEqualVerify OpCheckSig"))

	txin := []byte{0x11, 0x22, 0x33, 0x44}
	unlocks, err := SignatureScript(txin, addr.PublicKey, addr.PrivateKey)
	assert.Nil(t, err)

	t.Log(append(unlocks, locks...))

	v, err := NewIVM(txin, append(unlocks, locks...))
	assert.Nil(t, err)

	err = v.Execute()
	assert.Nil(t, err)
}

func TestPayToAddrScriptFalse(t *testing.T) {
	addr, _ := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	locks, _ := PayToAddrScript(addr.RawAddress)

	newAddr, _ := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	txin := []byte{0x11, 0x22, 0x33, 0x44}
	unlocks, _ := SignatureScript(txin, newAddr.PublicKey, newAddr.PrivateKey)

	v, err := NewIVM(txin, append(unlocks, locks...))
	assert.Nil(t, err)

	err = v.Execute()
	assert.Equal(t, "invalid signature", err.Error())
}

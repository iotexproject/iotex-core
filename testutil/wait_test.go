// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitUntil(t *testing.T) {
	assert := assert.New(t)
	threshold := time.Now()
	check1 := CheckCondition(func() (bool, error) {
		return time.Now().UnixNano() > threshold.UnixNano()+int64(1000), nil
	})
	err := WaitUntil(time.Millisecond, time.Second, check1)
	assert.Nil(err)
	check2 := CheckCondition(func() (bool, error) {
		return time.Now().UnixNano() > threshold.UnixNano()+int64(1000000000), nil
	})
	err = WaitUntil(time.Millisecond, time.Millisecond*10, check2)
	assert.Equal(ErrTimeout, err)
}

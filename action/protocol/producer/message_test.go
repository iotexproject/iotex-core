// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disset epoch rewarded. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producer

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDonateToProducerFund(t *testing.T) {
	b := DonateToProducerFundBuilder{}
	d1 := b.SetAmount(big.NewInt(1)).
		SetData([]byte{2}).
		Build()
	dProto := d1.Proto()
	d2 := DonateToProducerFund{}
	d2.LoadProto(dProto)
	assert.Equal(t, d1, d2)
}

func TestClaimFromProducerFund(t *testing.T) {

}

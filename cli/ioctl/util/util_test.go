// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringToRau(t *testing.T) {
	require := require.New(t)
	inputString := []string{"123", "123.321", ".123", "0.123", "123.", "+123.0", "0.",
		".0", "00.00", ".00", "00."}
	expectedString := []string{"123000000000000", "123321000000000", "123000000000",
		"123000000000", "123000000000000", "123000000000000", "0", "0", "0", "0", "0"}
	invalidString := []string{"  .123", "123. ", "0.12345678912345678900000",
		"1..2", "1.+2", "-1.2", ".", ". ", " .", " . ", ""}
	for i, teststring := range inputString {
		res, err := StringToRau(teststring, GasPriceDecimalNum)
		require.NoError(err)
		require.Equal(res.String(), expectedString[i])
	}
	for _, teststring := range invalidString {
		_, err := StringToRau(teststring, GasPriceDecimalNum)
		require.Error(err)
	}
}

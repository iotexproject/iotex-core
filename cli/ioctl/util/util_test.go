// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"math/big"
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

func TestRauToString(t *testing.T) {
	require := require.New(t)
	inputString := []string{"1", "0", "1000000000000", "200000000000", "30000000000",
		"1004000000000", "999999999999999999999939987", "100090907000030000100"}
	expectedString := []string{"0.000000000001", "0", "1", "0.2", "0.03", "1.004",
		"999999999999999.999999939987", "100090907.0000300001"}
	for i, teststring := range inputString {
		testBigInt, ok := big.NewInt(0).SetString(teststring, 10)
		require.True(ok)
		res := RauToString(testBigInt, GasPriceDecimalNum)
		require.Equal(expectedString[i], res)
	}
}

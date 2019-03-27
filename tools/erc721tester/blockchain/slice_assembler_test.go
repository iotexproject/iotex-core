package blockchain

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDynamicTypeNumber(t *testing.T) {
	assert := assert.New(t)

	sa := NewSliceAssembler()
	assert.False(sa.IsDynamicType(25))
	// test byte slice
	assert.False(sa.IsDynamicType([]byte{1, 2, 3, 4, 5}))
	assert.False(sa.IsDynamicType([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2}))
	assert.True(sa.IsDynamicType([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3}))
	// test other type slice
	assert.True(sa.IsDynamicType([]uint32{1, 2, 3, 4, 5}))
	assert.True(sa.IsDynamicType([]int32{1, 2, 3, 4, 5}))
	assert.True(sa.IsDynamicType([]uint64{1, 2, 3, 4, 5}))
	assert.True(sa.IsDynamicType([]int64{1, 2, 3, 4, 5}))
	assert.True(sa.IsDynamicType([]uint16{1, 2, 3, 4, 5}))
	assert.True(sa.IsDynamicType([]int16{1, 2, 3, 4, 5}))
	// test string
	assert.True(sa.IsDynamicType("hello world"))
}

func TestExample(t *testing.T) {
	assert := assert.New(t)

	// example taken from https://solidity.readthedocs.io/en/v0.5.3/abi-spec.html
	// call sam with the arguments "dave", true and [1,2,3]
	expected, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000464617665000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003")
	sa := NewSliceAssembler()
	data, err := sa.PackArguments("dave", true, []uint{1, 2, 3})
	assert.NoError(err)
	assert.Equal(expected, data)
}

func TestSliceAssembler_PackArguments_BigInt(t *testing.T) {
	sa := NewSliceAssembler()
	target := big.NewInt(25)
	t.Log(sa.PackArguments(target, target, target))
}

func TestSliceAssembler_PackArguments_BigInts(t *testing.T) {
	sa := NewSliceAssembler()
	target := big.NewInt(25)
	t.Log(sa.PackArguments([]*big.Int{target, target, target}))
}

func TestSliceAssembler_PackArguments_Ints(t *testing.T) {
	sa := NewSliceAssembler()
	target := int32(25)
	t.Log(sa.PackArguments([]int32{target, target, target}))
}

func TestSliceAssembler_PackArguments_Params(t *testing.T) {
	sa := NewSliceAssembler()
	target := 25
	t.Log(sa.PackArguments(5, 6, []int{target, target, target}))
}

func TestSliceAssembler_PackArguments_Strings(t *testing.T) {
	sa := NewSliceAssembler()
	target := "Hello, world!"
	t.Log(sa.PackArguments(target, target, target))
}

func TestSliceAssembler_PackArguments_String(t *testing.T) {
	sa := NewSliceAssembler()
	target := "Hello, world!"
	t.Log(sa.PackArguments(target))
}

func TestSliceAssembler_PackArguments_Bytes(t *testing.T) {
	sa := NewSliceAssembler()
	target := "Hello, world!Hello, world!Hello, world!"
	minTarget := "Hello"
	t.Log(sa.PackArguments([]byte(minTarget), []byte(target)))
}

package blockchain

import (
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

func packByte(b []byte) ([]byte, error) {
	length := len(b)
	bs := packNum(reflect.ValueOf(length))
	bs = append(bs, rightPadBytes(b, (length+31)/32*32)...)
	return bs, nil
}

// packNum packs the given number (using the reflect value) and will cast it to appropriate number representation
func packNum(value reflect.Value) []byte {
	switch kind := value.Kind(); kind {
	case reflect.Bool:
		b := hash.ZeroHash256
		if value.Bool() {
			b[31] = 1
		}
		return b[:]
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return U256(new(big.Int).SetUint64(value.Uint()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return U256(big.NewInt(value.Int()))
	case reflect.Ptr:
		return U256(value.Interface().(*big.Int))
	default:
		panic("abi: fatal error")
	}
}

func rightPadBytes(slice []byte, l int) []byte {
	if l <= len(slice) {
		return slice
	}
	padded := make([]byte, l)
	copy(padded, slice)
	return padded
}

// U256 converts a big Int into a 256bit EVM number.
func U256(n *big.Int) []byte {
	return math.PaddedBigBytes(math.U256(n), 32)
}

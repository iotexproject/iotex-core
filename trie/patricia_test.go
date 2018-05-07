package trie

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/iotexproject/iotex-core-internal/common"
	"github.com/stretchr/testify/assert"
)

func TestPatricia(t *testing.T) {
	assert := assert.New(t)

	b := branch{}
	b.Value = []byte{1, 6}
	root, _ := hex.DecodeString("90e0967d54b5f6f898c95404d0818f3f7a332ee6d5d7439666dd1e724771cb5e")
	hash1, _ := hex.DecodeString("9595c9df90075148eb06860365df33584b75bff782a510c6cd4883a419833d50")
	hash2, _ := hex.DecodeString("901c60ffffd77f743729f8fea0233c0b00223428b5192c2015f853562b45ce59")
	b.Path[0] = root
	b.Path[3] = hash1
	b.Path[11] = hash2

	stream, err := b.serialize()
	assert.Nil(err)
	assert.NotNil(stream)
	assert.Equal(byte(0), stream[0])
	b1 := branch{}
	err = b1.deserialize(stream)
	assert.Nil(err)
	assert.Equal(0, bytes.Compare(root, b1.Path[0]))
	assert.Equal(0, bytes.Compare(hash1, b1.Path[3]))
	assert.Equal(0, bytes.Compare(hash2, b1.Path[11]))
	assert.Equal(byte(1), b1.Value[0])
	assert.Equal(byte(6), b1.Value[1])

	e := ext{}
	e.Path = []byte{2, 3, 5, 7}
	copy(e.Value[:], hash1)
	stream, err = e.serialize()
	assert.Nil(err)
	assert.NotNil(stream)
	assert.Equal(byte(1), stream[0])
	e1 := ext{}
	err = e1.deserialize(stream)
	assert.Nil(err)
	assert.Equal(hash1, e1.Value[:])
	assert.Equal(byte(2), e1.Path[0])
	assert.Equal(byte(3), e1.Path[1])
	assert.Equal(byte(5), e1.Path[2])
	assert.Equal(byte(7), e1.Path[3])

	l := leaf{Value: make([]byte, common.HashSize)}
	l.Path = []byte{4, 6, 8, 9}
	copy(l.Value[:], hash2)
	stream, err = l.serialize()
	assert.Nil(err)
	assert.NotNil(stream)
	assert.Equal(byte(2), stream[0])
	l1 := leaf{}
	err = l1.deserialize(stream)
	assert.Nil(err)
	assert.Equal(hash2, l1.Value[:])
	assert.Equal(byte(4), l1.Path[0])
	assert.Equal(byte(6), l1.Path[1])
	assert.Equal(byte(8), l1.Path[2])
	assert.Equal(byte(9), l1.Path[3])
}

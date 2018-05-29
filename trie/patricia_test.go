package trie

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/iotexproject/iotex-core/common"
	"github.com/stretchr/testify/assert"
)

var (
	root, _  = hex.DecodeString("90e0967d54b5f6f898c95404d0818f3f7a332ee6d5d7439666dd1e724771cb5e")
	hash1, _ = hex.DecodeString("9595c9df90075148eb06860365df33584b75bff782a510c6cd4883a419833d50")
	hash2, _ = hex.DecodeString("901c60ffffd77f743729f8fea0233c0b00223428b5192c2015f853562b45ce59")

	ham = []byte{1, 2, 3, 4, 2, 3, 4, 5}
	car = []byte{1, 2, 3, 4, 5, 6, 7, 7}
	cat = []byte{1, 2, 3, 4, 5, 6, 7, 8}
	rat = []byte{1, 2, 3, 4, 5, 6, 7, 9}
	egg = []byte{1, 2, 3, 4, 5, 8, 1, 0}
	dog = []byte{1, 2, 3, 4, 6, 7, 1, 0}
	fox = []byte{1, 2, 3, 5, 6, 7, 8, 9}
	cow = []byte{1, 2, 5, 6, 7, 8, 9, 0}
	ant = []byte{2, 3, 4, 5, 6, 7, 8, 9}

	testV = [8][]byte{[]byte("ham"), []byte("car"), []byte("cat"), []byte("dog"), []byte("egg"), []byte("fox"), []byte("cow"), []byte("ant")}
)

func TestPatricia(t *testing.T) {
	assert := assert.New(t)

	b := branch{}
	b.Value = []byte{1, 6}
	b.Path[0] = root
	b.Path[2] = hash1
	b.Path[11] = hash2

	stream, err := b.serialize()
	assert.Nil(err)
	assert.NotNil(stream)
	assert.Equal(byte(2), stream[0])
	b1 := branch{}
	err = b1.deserialize(stream)
	assert.Nil(err)
	assert.Equal(0, bytes.Compare(root, b1.Path[0]))
	assert.Equal(0, bytes.Compare(hash1, b1.Path[2]))
	assert.Equal(0, bytes.Compare(hash2, b1.Path[11]))
	assert.Equal(byte(1), b1.Value[0])
	assert.Equal(byte(6), b1.Value[1])
	assert.Equal(444, len(stream))

	e := leaf{1, nil, make([]byte, common.HashSize)}
	e.Path = []byte{2, 3, 5, 7}
	copy(e.Value, hash1)
	stream, err = e.serialize()
	assert.Nil(err)
	assert.NotNil(stream)
	assert.Equal(byte(1), stream[0])
	e1 := leaf{}
	err = e1.deserialize(stream)
	assert.Nil(err)
	assert.Equal(hash1, e1.Value)
	assert.Equal(byte(2), e1.Path[0])
	assert.Equal(byte(3), e1.Path[1])
	assert.Equal(byte(5), e1.Path[2])
	assert.Equal(byte(7), e1.Path[3])
	assert.Equal(93, len(stream))

	l := leaf{Value: make([]byte, common.HashSize)}
	l.Path = []byte{4, 6, 8, 9}
	copy(l.Value, hash2)
	stream, err = l.serialize()
	assert.Nil(err)
	assert.NotNil(stream)
	assert.Equal(byte(0), stream[0])
	l1 := leaf{}
	err = l1.deserialize(stream)
	assert.Nil(err)
	assert.Equal(hash2, l1.Value)
	assert.Equal(byte(4), l1.Path[0])
	assert.Equal(byte(6), l1.Path[1])
	assert.Equal(byte(8), l1.Path[2])
	assert.Equal(byte(9), l1.Path[3])
	assert.Equal(91, len(stream))
}

func TestDescend(t *testing.T) {
	assert := assert.New(t)

	// testing branch
	br := branch{}
	br.Value = []byte{1, 6}
	br.Path[0] = root
	br.Path[2] = hash1
	br.Path[11] = hash2
	b, match, err := br.descend(cat)
	assert.Nil(b)
	assert.Equal(0, match)
	assert.NotNil(err)
	b, match, err = br.descend(ant)
	assert.Equal(b, hash1)
	assert.Equal(1, match)
	assert.Nil(err)

	// testing ext
	e := leaf{1, nil, make([]byte, common.HashSize)}
	e.Path = []byte{1, 2, 3, 5, 6}
	copy(e.Value, hash1)
	b, match, err = e.descend(ant)
	assert.Nil(b)
	assert.Equal(0, match)
	assert.Equal(ErrPathDiverge, err)
	b, match, err = e.descend(cow)
	assert.Nil(b)
	assert.Equal(2, match)
	assert.Equal(ErrPathDiverge, err)
	b, match, err = e.descend(cat)
	assert.Nil(b)
	assert.Equal(3, match)
	assert.Equal(ErrPathDiverge, err)
	b, match, err = e.descend(fox)
	assert.Equal(hash1, b)
	assert.Equal(5, match)
	assert.Nil(err)
}

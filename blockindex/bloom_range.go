package blockindex

import (
	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/iotexproject/go-pkgs/byteutil"
	"github.com/pkg/errors"
)

type bloomRange struct {
	start, end uint64
	bloom.BloomFilter
}

func newBloomRange(bfSize, bfNumHash uint64) (*bloomRange, error) {
	bf, err := bloom.NewBloomFilter(bfSize, bfNumHash)
	if err != nil {
		return nil, err
	}
	return &bloomRange{
		BloomFilter: bf,
	}, nil
}

func (br *bloomRange) Start() uint64 {
	return br.start
}

func (br *bloomRange) SetStart(start uint64) {
	br.start = start
}

func (br *bloomRange) End() uint64 {
	return br.end
}

func (br *bloomRange) SetEnd(end uint64) {
	br.end = end
}

func (br *bloomRange) Bytes() ([]byte, error) {
	if br.end < br.start {
		return nil, errors.New("end should be larger than start")
	}

	b := br.BloomFilter.Bytes()
	b = append(b, byteutil.Uint64ToBytesBigEndian(br.start)...)
	b = append(b, byteutil.Uint64ToBytesBigEndian(br.end)...)
	return b, nil
}

func (br *bloomRange) FromBytes(data []byte) error {
	length := len(data)
	if length <= 16 {
		return errors.New("not enough data")
	}
	// data = bf.Bytes() + start (8-byte) + end (8-byte)
	if br.BloomFilter == nil {
		return errors.New("the bloomFilter of bloomRange is nil")
	}

	if err := br.BloomFilter.FromBytes(data[:length-16]); err != nil {
		return err
	}
	br.end = byteutil.BytesToUint64BigEndian(data[length-8:])
	br.start = byteutil.BytesToUint64BigEndian(data[length-16 : length-8])
	return nil
}

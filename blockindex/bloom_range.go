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

func newBloomRange(start uint64, bf bloom.BloomFilter) *bloomRange {
	return &bloomRange{
		start:       start,
		BloomFilter: bf,
	}
}

func (br *bloomRange) Start() uint64 {
	return br.start
}

func (br *bloomRange) End() uint64 {
	return br.end
}

func (br *bloomRange) SetEnd(end uint64) *bloomRange {
	br.end = end
	return br
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

func bloomRangeFromBytes(data []byte) (*bloomRange, error) {
	length := len(data)
	if length <= 16 {
		return nil, errors.New("not enough data")
	}

	// data = bf.Bytes() + start (8-byte) + end (8-byte)
	end := byteutil.BytesToUint64BigEndian(data[length-8:])
	start := byteutil.BytesToUint64BigEndian(data[length-16 : length-8])
	bf, err := bloom.BloomFilterFromBytes(data[:length-16])
	if err != nil {
		return nil, err
	}

	return &bloomRange{
		start:       start,
		end:         end,
		BloomFilter: bf,
	}, nil
}

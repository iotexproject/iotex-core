package rolldpos

import "github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"

type (
	kvStore interface {
		Put(string, []byte, []byte) error
		Get(string, []byte) ([]byte, error)
	}

	roundStore struct {
		underlying kvStore
		height     uint64
		roundNum   uint32
	}
)

func newRoundStore(kv kvStore, height uint64, roundNum uint32) *roundStore {
	return &roundStore{underlying: kv, height: height, roundNum: roundNum}
}

func (rs *roundStore) Put(ns string, key []byte, value []byte) error {
	return rs.underlying.Put(ns, rs.realKey(key), value)
}

func (rs *roundStore) Get(ns string, key []byte) ([]byte, error) {
	return rs.underlying.Get(ns, rs.realKey(key))
}

func (rs *roundStore) ChangeRound(height uint64, roundNum uint32) {
	rs.height = height
	rs.roundNum = roundNum
}

func (rs *roundStore) realKey(key []byte) []byte {
	keys := byteutil.Uint64ToBytes(rs.height)
	keys = append(keys, byteutil.Uint32ToBytes(rs.roundNum)...)
	keys = append(keys, key...)
	return keys
}

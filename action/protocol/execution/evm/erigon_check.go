package evm

import (
	"bytes"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type contractAdapterCheck struct {
	*contractAdapter
}

func newContractAdapterCheck(adapter *contractAdapter) *contractAdapterCheck {
	return &contractAdapterCheck{
		contractAdapter: adapter,
	}
}

func (c *contractAdapterCheck) GetCommittedState(addr hash.Hash256) ([]byte, error) {
	org, err := c.contractAdapter.Contract.GetCommittedState(addr)
	erigon, err2 := c.contractAdapter.erigon.GetCommittedState(addr)
	if e := consistentEqualStateE(org, erigon, err, err2, func(v1, v2 []byte) bool {
		if bytes.Equal(v1, v2) {
			return true
		}
		if !bytes.Equal(erigon, hash.ZeroHash256[:]) {
			return false
		}
		erigonCur, _ := c.contractAdapter.erigon.GetState(addr)
		return bytes.Equal(v1, erigonCur)
	}); e != nil {
		return nil, e
	}
	return org, err
}

func (c *contractAdapterCheck) GetState(addr hash.Hash256) ([]byte, error) {
	org, err := c.contractAdapter.Contract.GetState(addr)
	erigon, err2 := c.contractAdapter.erigon.GetState(addr)
	if e := consistentEqualStateE(org, erigon, err, err2, func(v1, v2 []byte) bool {
		return bytes.Equal(v1, v2)
	}); e != nil {
		return nil, e
	}
	return org, err
}

func (c *contractAdapterCheck) GetCode() ([]byte, error) {
	org, err := c.contractAdapter.Contract.GetCode()
	erigon, err2 := c.contractAdapter.erigon.GetCode()
	if e := consistentEqualVE(org, erigon, err, err2, func(v1, v2 []byte) bool {
		return bytes.Equal(v1, v2)
	}); e != nil {
		return nil, e
	}
	return org, err
}
func (c *contractAdapterCheck) SelfState() *state.Account {
	org := c.contractAdapter.Contract.SelfState()
	erigon := c.contractAdapter.erigon.SelfState()
	if e := consistentEqual(org, erigon, func(a, b *state.Account) bool {
		pa := a.ToProto()
		pa.Root = nil
		pa.VotingWeight = nil
		pa.IsCandidate = false
		d1, err := proto.Marshal(pa)
		if err != nil {
			log.S().Panicf("failed to serialize account %+v: %v", a, err)
		}
		pb := b.ToProto()
		pb.Root = nil
		pb.VotingWeight = nil
		pb.IsCandidate = false
		d2, err := proto.Marshal(pb)
		if err != nil {
			log.S().Panicf("failed to serialize account %+v: %v", b, err)
		}
		return bytes.Equal(d1, d2)
	}); e != nil {
		log.S().Panicf("inconsistent SelfState: %+v vs %+v", org, erigon)
	}
	return org
}

func consistentEqual[T any](a, b T, equal func(T, T) bool) error {
	if !equal(a, b) {
		log.S().Panicf("inconsistent value: %+v vs %+v", a, b)
		return errors.Errorf("inconsistent value: %v vs %v", a, b)
	}
	return nil
}

func consistentError(err1, err2 error) error {
	return consistentEqual(err1, err2, func(a, b error) bool {
		return errors.Cause(a) == errors.Cause(b)
	})
}

func consistentEqualVE[V any](a, b V, err1, err2 error, equal func(V, V) bool) error {
	if e := consistentError(err1, err2); e != nil {
		return e
	}
	if err1 != nil {
		return err1
	}
	return consistentEqual(a, b, equal)
}

func consistentEqualStateE(a, b []byte, err1, err2 error, equal func([]byte, []byte) bool) error {
	// special case: erigon returns zero value instead of not exist error
	if err2 == nil && errors.Is(err1, trie.ErrNotExist) && bytes.Equal(b, hash.ZeroHash256[:]) {
		return err1
	}
	if e := consistentError(err1, err2); e != nil {
		return e
	}
	if err1 != nil {
		return err1
	}
	return consistentEqual(a, b, equal)
}

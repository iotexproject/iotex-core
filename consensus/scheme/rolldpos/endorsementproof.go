package rolldpos

import (
	"bytes"

	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/pkg/errors"
)

type endorsementProof struct {
	blkHash      hash.Hash32B
	endorsements []*endorsement
}

func (l *endorsementProof) addEndorsement(en *endorsement) error {
	if !bytes.Equal(l.blkHash[:], en.blkHash[:]) {
		return errors.New("the endorsement block hash is different from lock")
	}
	if !en.VerifySignature() {
		return errors.New("invalid signature in endorsement")
	}
	l.endorsements = append(l.endorsements, en)

	return nil
}

func (l *endorsementProof) blockHash() hash.Hash32B {
	return l.blkHash
}

func (l *endorsementProof) numOfValidEndorsements(topics []uint8, endorsers []*iotxaddress.Address) uint16 {
	topicSet := map[uint8]bool{}
	for _, topic := range topics {
		topicSet[topic] = true
	}
	endorserSet := map[keypair.PublicKey]bool{}
	for _, endorser := range endorsers {
		endorserSet[endorser.PublicKey] = true
	}
	cnt := uint16(0)
	for _, endorsement := range l.endorsements {
		if _, ok := topicSet[endorsement.topic]; !ok {
			continue
		}
		if _, ok := endorserSet[endorsement.endorserPubkey]; !ok {
			continue
		}
		cnt++
	}

	return cnt
}

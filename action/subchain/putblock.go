package subchain

import (
	"sort"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

func (p *Protocol) handlePutBlock(pb *action.PutBlock, ws state.WorkingSet) error {
	if err := p.validatePutBlock(pb, ws); err != nil {
		return err
	}
	return p.putBlockProof(putBlockToBlockProof(pb), ws)
}

func (p *Protocol) validatePutBlock(pb *action.PutBlock, ws state.WorkingSet) error {
	// use owner address TODO

	// can only emit on one height
	if _, exist := p.getBlockProof(pb.SubChainAddress(), pb.Height()); exist {
		return errors.New("block already exist")
	}
	return nil
}

func (p *Protocol) getBlockProof(addr string, height uint64) (BlockProof, bool) {
	var bp BlockProof
	if _, err := p.sf.State(blockProofKey(addr, height), &bp); err != nil {
		return BlockProof{}, false
	}
	return bp, true
}

func (p *Protocol) putBlockProof(bp BlockProof, ws state.WorkingSet) error {
	return ws.PutState(blockProofKey(bp.SubChainAddress, bp.Height), &bp)
}

func blockProofKey(addr string, height uint64) hash.PKHash {
	stream := []byte{}
	stream = append(stream, addr...)
	temp := make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, height)
	stream = append(stream, temp...)
	return byteutil.BytesTo20B(hash.Hash160b(stream))
}

func putBlockToBlockProof(pb *action.PutBlock) BlockProof {
	roots := pb.Roots()
	keys := make([]string, 0, len(roots))
	for k := range roots {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	bpRoots := make([]MerkleRoot, 0, len(keys))
	for _, k := range keys {
		v := roots[k]
		mr := MerkleRoot{
			Name: k,
		}
		copy(mr.Value[:], v[:])
		bpRoots = append(bpRoots, mr)
	}

	return BlockProof{
		SubChainAddress:   pb.SubChainAddress(),
		Roots:             bpRoots,
		Height:            pb.Height(),
		ProducerPublicKey: pb.ProducerPublicKey(),
		ProducerAddress:   pb.ProducerAddress(),
	}
}

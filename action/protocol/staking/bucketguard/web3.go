package bucketguard

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Web3 entry: one ABI method per operation, so wallets can submit guarded
// actions as ordinary eth transactions to the native staking address. The ABI
// is only an envelope. Decoding yields the same BindBucketGuard /
// GuardedBucketOp values as the native encoding, which then go through the
// same Validate / CheckOp / VerifyOp / Next path; size limits and gas are
// computed over C(m) of the decoded value, never over the ABI bytes, so both
// entries price an action identically. At integration these methods join
// action.NativeStakingContractABI.
const web3ABI = `[
{"type":"function","name":"bindBucketGuard","inputs":[
 {"name":"bucketIndex","type":"uint64"},{"name":"algID","type":"uint32"},
 {"name":"publicKey","type":"bytes"},{"name":"acceptSig","type":"bytes"}]},
{"type":"function","name":"guardedUnstake","inputs":[
 {"name":"bucketIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"nonce","type":"uint64"},{"name":"deadline","type":"uint64"},
 {"name":"sig","type":"bytes"}]},
{"type":"function","name":"guardedWithdraw","inputs":[
 {"name":"bucketIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"nonce","type":"uint64"},{"name":"deadline","type":"uint64"},
 {"name":"recipient","type":"address"},{"name":"sig","type":"bytes"}]},
{"type":"function","name":"guardedTransfer","inputs":[
 {"name":"bucketIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"nonce","type":"uint64"},{"name":"deadline","type":"uint64"},
 {"name":"newOwner","type":"address"},{"name":"clearGuard","type":"bool"},
 {"name":"newAlgID","type":"uint32"},{"name":"newPublicKey","type":"bytes"},{"name":"newAcceptSig","type":"bytes"},
 {"name":"sig","type":"bytes"}]},
{"type":"function","name":"guardedChangeCandidate","inputs":[
 {"name":"bucketIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"nonce","type":"uint64"},{"name":"deadline","type":"uint64"},
 {"name":"candidateName","type":"string"},{"name":"expectedCandidateID","type":"address"},{"name":"sig","type":"bytes"}]},
{"type":"function","name":"guardedRestake","inputs":[
 {"name":"bucketIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"nonce","type":"uint64"},{"name":"deadline","type":"uint64"},
 {"name":"duration","type":"uint32"},{"name":"autoStake","type":"bool"},{"name":"sig","type":"bytes"}]},
{"type":"function","name":"guardedRotate","inputs":[
 {"name":"bucketIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"nonce","type":"uint64"},{"name":"deadline","type":"uint64"},
 {"name":"newAlgID","type":"uint32"},{"name":"newPublicKey","type":"bytes"},{"name":"newAcceptSig","type":"bytes"},
 {"name":"sig","type":"bytes"}]}
]`

const bindMethod = "bindBucketGuard"

var (
	// ErrWeb3Decode is returned for eth data that is not a well-formed call of
	// one of the guarded staking methods.
	ErrWeb3Decode = errors.New("cannot decode guarded staking call")

	web3Contract abi.ABI
	opMethods    = map[Op]string{
		OpUnstake:         "guardedUnstake",
		OpWithdraw:        "guardedWithdraw",
		OpTransfer:        "guardedTransfer",
		OpChangeCandidate: "guardedChangeCandidate",
		OpRestake:         "guardedRestake",
		OpRotate:          "guardedRotate",
	}
)

func init() {
	var err error
	if web3Contract, err = abi.JSON(strings.NewReader(web3ABI)); err != nil {
		panic(err)
	}
}

// EthData returns the ABI-encoded call of bindBucketGuard.
func (m *BindBucketGuard) EthData() ([]byte, error) {
	return pack(bindMethod, m.BucketIndex, uint32(m.AlgID), m.PublicKey, m.AcceptSig)
}

// BindBucketGuardFromEthData decodes a bindBucketGuard call.
func BindBucketGuardFromEthData(data []byte) (*BindBucketGuard, error) {
	v, err := unpack(bindMethod, data)
	if err != nil {
		return nil, err
	}
	return &BindBucketGuard{
		BucketIndex: v.u64("bucketIndex"),
		AlgID:       pqcrypto.AlgID(v.u32("algID")),
		PublicKey:   v.bytes("publicKey"),
		AcceptSig:   v.bytes("acceptSig"),
	}, v.err
}

// EthData returns the ABI-encoded call of the method for m.Op.
func (m *GuardedBucketOp) EthData() ([]byte, error) {
	method, ok := opMethods[m.Op]
	if !ok {
		return nil, malformed("unknown op %d", uint32(m.Op))
	}
	head := []any{m.BucketIndex, m.Generation, m.Nonce, m.Deadline}
	var ng NewGuard
	if m.NewGuard != nil {
		ng = *m.NewGuard
	}
	var args []any
	switch m.Op {
	case OpUnstake:
	case OpWithdraw:
		recipient, err := toAddr(m.Op, "recipient", m.Recipient)
		if err != nil {
			return nil, err
		}
		args = []any{recipient}
	case OpTransfer:
		newOwner, err := toAddr(m.Op, "newOwner", m.NewOwner)
		if err != nil {
			return nil, err
		}
		args = []any{newOwner, m.ClearGuard, uint32(ng.AlgID), ng.PublicKey, ng.AcceptSig}
	case OpChangeCandidate:
		cand, err := toAddr(m.Op, "expectedCandidateID", m.ExpectedCandidateID)
		if err != nil {
			return nil, err
		}
		args = []any{m.CandidateName, cand}
	case OpRestake:
		args = []any{m.Duration, m.AutoStake}
	case OpRotate:
		args = []any{uint32(ng.AlgID), ng.PublicKey, ng.AcceptSig}
	}
	return pack(method, append(append(head, args...), m.Sig)...)
}

// GuardedBucketOpFromEthData decodes a call of one of the guarded methods.
// The result still has to pass Validate.
func GuardedBucketOpFromEthData(data []byte) (*GuardedBucketOp, error) {
	if len(data) < 4 {
		return nil, ErrWeb3Decode
	}
	method, err := web3Contract.MethodById(data[:4])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWeb3Decode, err)
	}
	op, ok := opOf(method.Name)
	if !ok {
		return nil, fmt.Errorf("%w: %s is not a guarded op", ErrWeb3Decode, method.Name)
	}
	v, err := unpack(method.Name, data)
	if err != nil {
		return nil, err
	}
	m := &GuardedBucketOp{
		BucketIndex: v.u64("bucketIndex"),
		Generation:  v.u64("generation"),
		Nonce:       v.u64("nonce"),
		Deadline:    v.u64("deadline"),
		Op:          op,
		Sig:         v.bytes("sig"),
	}
	newGuard := func() *NewGuard {
		g := &NewGuard{AlgID: pqcrypto.AlgID(v.u32("newAlgID")), PublicKey: v.bytes("newPublicKey"), AcceptSig: v.bytes("newAcceptSig")}
		if g.AlgID == 0 && len(g.PublicKey) == 0 && len(g.AcceptSig) == 0 {
			return nil
		}
		return g
	}
	switch op {
	case OpWithdraw:
		m.Recipient = v.addr("recipient")
	case OpTransfer:
		m.NewOwner = v.addr("newOwner")
		m.ClearGuard = v.bool("clearGuard")
		m.NewGuard = newGuard()
	case OpChangeCandidate:
		m.CandidateName = v.string("candidateName")
		m.ExpectedCandidateID = v.addr("expectedCandidateID")
	case OpRestake:
		m.Duration = v.u32("duration")
		m.AutoStake = v.bool("autoStake")
	case OpRotate:
		m.NewGuard = newGuard()
	}
	return m, v.err
}

// toAddr rejects anything but a 20-byte address: common.BytesToAddress would
// silently pad or truncate and change what the ABI call means.
func toAddr(op Op, name string, b []byte) (common.Address, error) {
	if len(b) != addressLen {
		return common.Address{}, malformed("%s %s must be %d bytes, got %d", op, name, addressLen, len(b))
	}
	return common.BytesToAddress(b), nil
}

func opOf(method string) (Op, bool) {
	for op := OpUnstake; op <= OpRotate; op++ {
		if opMethods[op] == method {
			return op, true
		}
	}
	return 0, false
}

func pack(method string, args ...any) ([]byte, error) {
	m := web3Contract.Methods[method]
	data, err := m.Inputs.Pack(args...)
	if err != nil {
		return nil, err
	}
	return append(bytes.Clone(m.ID), data...), nil
}

// abiValues reads typed values out of an unpacked argument map, recording the
// first type mismatch in err.
type abiValues struct {
	m   map[string]any
	err error
}

func unpack(method string, data []byte) (*abiValues, error) {
	m := web3Contract.Methods[method]
	if len(data) < 4 || !bytes.Equal(m.ID, data[:4]) {
		return nil, fmt.Errorf("%w: not a %s call", ErrWeb3Decode, method)
	}
	v := &abiValues{m: map[string]any{}}
	if err := m.Inputs.UnpackIntoMap(v.m, data[4:]); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWeb3Decode, err)
	}
	return v, nil
}

func get[T any](v *abiValues, name string) T {
	x, ok := v.m[name].(T)
	if !ok && v.err == nil {
		v.err = fmt.Errorf("%w: field %s", ErrWeb3Decode, name)
	}
	return x
}

func (v *abiValues) u64(name string) uint64    { return get[uint64](v, name) }
func (v *abiValues) u32(name string) uint32    { return get[uint32](v, name) }
func (v *abiValues) bool(name string) bool     { return get[bool](v, name) }
func (v *abiValues) string(name string) string { return get[string](v, name) }
func (v *abiValues) bytes(name string) []byte  { return bytes.Clone(get[[]byte](v, name)) }
func (v *abiValues) addr(name string) []byte {
	a := get[common.Address](v, name)
	return a.Bytes()
}

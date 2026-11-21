package bucketguard

import (
	"crypto/sha512"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Benchmarks measure the real verification + deterministic handler writes.
// Wallet keygen/signing and independent-state reset are outside the timer;
// no verification is mocked, cached, or omitted from the measured handler.
func BenchmarkVaultCreate(b *testing.B) {
	k, _ := testVaultKeys(b)
	e := freshVaultEngine(b)
	m := createRequest(b, e, 7, k)
	env := VaultEnv{vaultEnv(1, 100), 7}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		iteration := cloneVaultEngine(e)
		b.StartTimer()
		if _, err := iteration.VaultCreate(env, m); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVaultWithdraw(b *testing.B) {
	e, k, _ := vaultFixture(b)
	m := withdrawCall(b, e, k, 100, 101)
	env := vaultEnv(1, 101)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		iteration := cloneVaultEngine(e)
		b.StartTimer()
		if err := iteration.VaultWithdraw(env, m); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVaultRotateGuard(b *testing.B) {
	e, k, k2 := vaultFixture(b)
	m := unsignedVaultCall(e, 0, VaultOpRotateGuard, 101)
	m.NewGuard = &NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: k2.PublicKey().Bytes()}
	call := EncodeVaultCall(e.domain, e.Vault(0).Owner, e.GuardV2(ClassAVault, 0).KeyID(), m)
	m.NewGuard.AcceptSig = vaultSign(b, k2, VaultAcceptContext, EncodeVaultAccept(e.domain, 0, 0, 2, e.Vault(0).Owner, UnbindDelayBlocks, m.NewGuard.AlgID, m.NewGuard.PublicKey, sha512.Sum512(call)))
	signVaultCall(b, e, k, m)
	env := vaultEnv(1, 101)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		iteration := cloneVaultEngine(e)
		b.StartTimer()
		if err := iteration.VaultRotateGuard(env, m); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGuardUnbindCancel(b *testing.B) {
	e, k, _ := vaultFixture(b)
	bindB(b, e, k, 0, UnbindDelayBlocks)
	vaultError(b, e.GuardUnbindInitiate(vaultEnv(1, 101), 0), nil)
	m := cancelCall(b, e, k, 0, 1, 102)
	env := vaultEnv(9, 102)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		iteration := cloneVaultEngine(e)
		b.StartTimer()
		if err := iteration.GuardUnbindCancel(env, m); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVaultSign reports the separate offline signer cost, rather than
// conflating signature generation with an on-chain handler's verification.
func BenchmarkVaultSign(b *testing.B) {
	e, k, _ := vaultFixture(b)
	m := unsignedVaultCall(e, 0, VaultOpWithdraw, 101)
	m.Amount = big.NewInt(100)
	m.Recipient = vaultAddress(3)
	msg := EncodeVaultCall(e.domain, e.Vault(0).Owner, e.GuardV2(ClassAVault, 0).KeyID(), m)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig, err := k.Sign([]byte(VaultCallContext), msg)
		if err != nil || len(sig) != 16224 {
			b.Fatal("SLH-DSA signing failed", err)
		}
	}
}

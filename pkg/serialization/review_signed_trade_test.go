package serialization

import (
	"crypto/ed25519"
	"crypto/rand"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestGeneratedKeyV1CryptographyAndTamper(t *testing.T) {
	_, pk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, ck, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payer, co := solana.PrivateKey(pk), solana.PrivateKey(ck)
	ix := solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{solana.Meta(co.PublicKey()).SIGNER().WRITE(), solana.Meta(payer.PublicKey()).WRITE()}, []byte{1, 2, 3})
	compiled, err := CompileV1Message(payer.PublicKey(), []solana.Instruction{ix}, solana.Hash{}, V1Config{})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := SignV1Transaction(compiled, []solana.PrivateKey{co, payer})
	if err != nil {
		t.Fatal(err)
	}
	if compiled.RequiredSignatures != 2 || compiled.AccountKeys[0] != payer.PublicKey() {
		t.Fatal("signer ordering")
	}
	verify := func(raw []byte) bool {
		n := len(compiled.Message)
		for i, key := range compiled.AccountKeys[:2] {
			if !ed25519.Verify(ed25519.PublicKey(key[:]), raw[:n], raw[n+64*i:n+64*(i+1)]) {
				return false
			}
		}
		return true
	}
	if !verify(wire) {
		t.Fatal("valid signatures rejected")
	}
	for _, index := range []int{len(compiled.Message) - 1, len(compiled.Message)} {
		bad := append([]byte(nil), wire...)
		bad[index] ^= 1
		if verify(bad) {
			t.Fatal("tampering accepted")
		}
	}
	if _, err := SignV1Transaction(compiled, []solana.PrivateKey{payer}); err == nil {
		t.Fatal("missing co-signer accepted")
	}
}

package serialization

import (
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestReviewSignerMetadata(t *testing.T) {
	payer := solana.NewWallet()
	c, err := CompileV1Message(payer.PublicKey(), nil, solana.Hash{}, V1Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = SignV1Transaction(c, []solana.PrivateKey{payer.PrivateKey}); err != nil {
		t.Fatal(err)
	}
	changed := *c
	changed.Message = append([]byte{}, c.Message...)
	changed.Message[42] ^= 1
	if _, err = SignV1Transaction(&changed, []solana.PrivateKey{payer.PrivateKey}); err == nil {
		t.Fatal("stale metadata accepted")
	}
	changed = *c
	changed.Message = append([]byte{}, c.Message...)
	changed.Message[1] = 2
	if _, err = SignV1Transaction(&changed, []solana.PrivateKey{payer.PrivateKey}); err == nil {
		t.Fatal("forged count accepted")
	}
}

package common

import (
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestParseNonceInfo(t *testing.T) {
	authority := solana.PublicKeyFromBytes(bytesOf(7))
	nonceHash := solana.HashFromBytes(bytesOf(9))
	nonceAccount := solana.PublicKeyFromBytes(bytesOf(3))
	data := make([]byte, 80)
	copy(data[8:40], authority[:])
	copy(data[40:72], nonceHash[:])

	got, err := parseNonceInfo(nonceAccount, data)
	if err != nil {
		t.Fatalf("parseNonceInfo returned error: %v", err)
	}
	if got.NonceAccount != nonceAccount {
		t.Fatalf("nonce account mismatch: got %s want %s", got.NonceAccount, nonceAccount)
	}
	if got.Authority != authority {
		t.Fatalf("authority mismatch: got %s want %s", got.Authority, authority)
	}
	if got.NonceHash != nonceHash {
		t.Fatalf("nonce hash mismatch: got %s want %s", got.NonceHash, nonceHash)
	}
	if got.RecentBlockhash != nonceHash {
		t.Fatalf("recent blockhash mismatch: got %s want %s", got.RecentBlockhash, nonceHash)
	}
}

func TestParseNonceInfoRejectsShortData(t *testing.T) {
	if _, err := parseNonceInfo(solana.PublicKey{}, make([]byte, 10)); err == nil {
		t.Fatal("expected an error for short nonce account data")
	}
}

func bytesOf(value byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = value
	}
	return out
}

package instruction

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"testing"
)

func TestPublicCloseAccountUsesSPLABIAndSigns(t *testing.T) {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	payer := solana.PrivateKey(ed25519.NewKeyFromSeed(seed))
	source := solana.PublicKey{1}
	mint := solana.PublicKey{2}
	hash := solana.Hash{}
	if value := os.Getenv("SDK_BANK_BLOCKHASH"); value != "" {
		var err error
		hash, err = solana.HashFromBase58(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	exports := []map[string]any{}
	for _, program := range []solana.PublicKey{solana.TokenProgramID, solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")} {
		ix := BuildCloseAccountInstruction(program, source, payer.PublicKey(), payer.PublicKey())
		data, err := ix.Data()
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 1 || data[0] != 9 {
			t.Fatalf("public close uses invalid SPL opcode: %x", data)
		}
		accounts := ix.Accounts()
		if len(accounts) != 3 || accounts[0].PublicKey != source || !accounts[0].IsWritable || accounts[1].PublicKey != payer.PublicKey() || !accounts[1].IsWritable || accounts[2].PublicKey != payer.PublicKey() || !accounts[2].IsSigner {
			t.Fatal("invalid SPL close account order or flags")
		}
		tx, err := solana.NewTransaction([]solana.Instruction{ix}, hash, solana.TransactionPayer(payer.PublicKey()))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Sign(func(solana.PublicKey) *solana.PrivateKey { return &payer }); err != nil {
			t.Fatal(err)
		}
		wire, err := tx.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := solana.TransactionFromBytes(wire)
		if err != nil {
			t.Fatal(err)
		}
		if err = parsed.VerifySignatures(); err != nil {
			t.Fatal(err)
		}
		exports = append(exports, map[string]any{"wire": base64.StdEncoding.EncodeToString(wire), "program": program.String(), "payer": payer.PublicKey().String(), "source": source.String(), "mint": mint.String(), "blockhash": hash.String()})
	}
	if path := os.Getenv("SDK_CLOSE_EXPORT"); path != "" {
		data, err := json.Marshal(exports)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

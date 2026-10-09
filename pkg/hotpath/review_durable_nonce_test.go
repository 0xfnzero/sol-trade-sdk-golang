package hotpath

import (
	"crypto/ed25519"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"testing"
	"time"
)

func TestDurableNonceGasOrderingAndActualSigning(t *testing.T) {
	payer, authority := solana.NewWallet().PrivateKey, solana.NewWallet().PrivateKey
	nonce := solana.NewWallet().PublicKey()
	hash := solana.Hash(solana.NewWallet().PublicKey())
	cfg := DefaultHotPathConfig()
	cfg.EnablePrefetch = false
	executor := NewHotPathExecutor(nil, cfg)
	executor.state.currentData.Store(&PrefetchedData{Blockhash: &hash, FetchedAt: time.Now()})
	advance := system.NewAdvanceNonceAccountInstruction(nonce, solana.SysVarRecentBlockHashesPubkey, authority.PublicKey()).Build()
	transfer := system.NewTransferInstruction(1, payer.PublicKey(), solana.NewWallet().PublicKey()).Build()
	instructions := []solana.Instruction{advance, transfer}
	tx, err := executor.BuildTransaction(payer.PublicKey(), instructions, []*solana.PrivateKey{&authority, &payer}, &GasFeeConfig{ComputeUnitLimit: 200000, ComputeUnitPrice: 1})
	if err != nil {
		t.Fatal(err)
	}
	first := tx.Message.Instructions[0]
	if tx.Message.AccountKeys[first.ProgramIDIndex] != solana.SystemProgramID || string(first.Data) != string([]byte{4, 0, 0, 0}) {
		t.Fatal("nonce advance moved behind compute budget")
	}
	if tx.Message.AccountKeys[first.Accounts[0]] != nonce || tx.Message.AccountKeys[first.Accounts[2]] != authority.PublicKey() {
		t.Fatal("wrong nonce account/authority roles")
	}
	if len(instructions) != 2 {
		t.Fatal("caller instructions mutated")
	}
	wire, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := solana.TransactionFromBytes(wire)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Message.RecentBlockhash != hash {
		t.Fatal("cached nonce hash replaced")
	}
	message, err := parsed.Message.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	for i, sig := range parsed.Signatures {
		key := parsed.Message.AccountKeys[i]
		if !ed25519.Verify(ed25519.PublicKey(key[:]), message, sig[:]) {
			t.Fatal("invalid Ed25519 signature")
		}
	}
	if _, err := executor.BuildTransaction(payer.PublicKey(), instructions, []*solana.PrivateKey{&payer}, nil); err == nil {
		t.Fatal("missing nonce authority accepted")
	}
}

package hotpath

import (
	"crypto/ed25519"
	"crypto/rand"
	"github.com/gagliardetto/solana-go"
	"testing"
	"time"
)

func TestGeneratedKeyLegacyBuilderSignsCachedHashAndRejectsMissingSigner(t *testing.T) {
	_, pk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, ck, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payer, co := solana.PrivateKey(pk), solana.PrivateKey(ck)
	cfg := DefaultHotPathConfig()
	cfg.EnablePrefetch = false
	executor := NewHotPathExecutor(nil, cfg) // No RPC client exists on this cached builder path.
	hash := solana.Hash{}
	executor.state.currentData.Store(&PrefetchedData{Blockhash: &hash, LastValidHeight: 100, FetchedAt: time.Now()})
	ix := solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{solana.Meta(co.PublicKey()).SIGNER().WRITE(), solana.Meta(payer.PublicKey()).WRITE()}, []byte{1, 2, 3})
	tx, err := executor.BuildTransaction(payer.PublicKey(), []solana.Instruction{ix}, []*solana.PrivateKey{&co, &payer}, nil)
	if err != nil {
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
	if err := parsed.VerifySignatures(); err != nil {
		t.Fatal(err)
	}
	message, err := parsed.Message.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Signatures) != 2 {
		t.Fatal("wrong required signer count")
	}
	for i, signature := range parsed.Signatures {
		key := parsed.Message.AccountKeys[i]
		if !ed25519.Verify(ed25519.PublicKey(key[:]), message, signature[:]) {
			t.Fatal("invalid signature")
		}
	}
	message[len(message)-1] ^= 1
	if ed25519.Verify(ed25519.PublicKey(parsed.Message.AccountKeys[0][:]), message, parsed.Signatures[0][:]) {
		t.Fatal("message tampering accepted")
	}
	if _, err := executor.BuildTransaction(payer.PublicKey(), []solana.Instruction{ix}, []*solana.PrivateKey{&payer}, nil); err == nil {
		t.Fatal("missing signer accepted")
	}
}

func TestCachedSigningRejectsMissingFeePayerNilAndWrongSigners(t *testing.T) {
	payer := solana.NewWallet().PrivateKey
	co := solana.NewWallet().PrivateKey
	wrong := solana.NewWallet().PrivateKey
	cfg := DefaultHotPathConfig()
	cfg.EnablePrefetch = false
	executor := NewHotPathExecutor(nil, cfg)
	hash := solana.Hash{}
	executor.state.currentData.Store(&PrefetchedData{Blockhash: &hash, FetchedAt: time.Now()})
	ix := solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{solana.Meta(co.PublicKey()).SIGNER().WRITE()}, []byte{1, 2, 3})
	for name, signers := range map[string][]*solana.PrivateKey{
		"nil_signer":                         {nil},
		"missing_fee_payer":                  {&co},
		"wrong_instruction_signer":           {&payer, &wrong},
		"nil_and_missing_instruction_signer": {nil, &payer},
	} {
		t.Run(name, func(t *testing.T) {
			tx, err := executor.BuildTransaction(payer.PublicKey(), []solana.Instruction{ix}, signers, nil)
			if err == nil || tx != nil {
				t.Fatal("incomplete signing must fail without returning a submit-ready transaction")
			}
		})
	}
}

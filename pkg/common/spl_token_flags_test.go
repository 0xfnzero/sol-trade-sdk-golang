package common

import (
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestTokenTransferAccountPermissions(t *testing.T) {
	source, destination, owner := solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey()
	ix := NewTokenInstructionBuilder().BuildTransfer(source, destination, owner, 100)
	accounts := ix.Accounts()
	for _, i := range []int{0, 1} {
		if !accounts[i].IsWritable || accounts[i].IsSigner {
			t.Fatal("token account must be writable and not sign")
		}
	}
	if !accounts[2].IsSigner || accounts[2].IsWritable {
		t.Fatal("owner must sign without becoming writable")
	}
}
func TestIdempotentATAAccountPermissions(t *testing.T) {
	payer := solana.NewWallet().PublicKey()
	ix, _, err := BuildCreateIdempotentATA(payer, payer, solana.WrappedSol)
	if err != nil {
		t.Fatal(err)
	}
	accounts := ix.Accounts()
	if !accounts[0].IsWritable || !accounts[0].IsSigner {
		t.Fatal("payer flags")
	}
	if !accounts[1].IsWritable || accounts[1].IsSigner {
		t.Fatal("ATA must be writable and not sign")
	}
	for _, a := range accounts[2:] {
		if a.IsSigner || a.IsWritable {
			t.Fatal("remaining ATA accounts must be readonly")
		}
	}
}

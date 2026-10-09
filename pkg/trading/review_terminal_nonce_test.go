package trading

import (
	"context"
	"crypto/ed25519"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"testing"
)

type signedNonceReceiptClient struct {
	t     *testing.T
	calls int
}

func (c *signedNonceReceiptClient) SendTransaction(_ context.Context, _ soltradesdk.TradeType, wire []byte, wait bool) (solana.Signature, error) {
	c.t.Helper()
	c.calls++
	if wait {
		c.t.Fatal("submission must not poll")
	}
	tx, err := solana.TransactionFromBytes(wire)
	if err != nil {
		c.t.Fatal(err)
	}
	message, err := tx.Message.MarshalBinary()
	if err != nil {
		c.t.Fatal(err)
	}
	for i, sig := range tx.Signatures {
		key := tx.Message.AccountKeys[i]
		if !ed25519.Verify(key[:], message, sig[:]) {
			c.t.Fatal("invalid signature")
		}
	}
	return tx.Signatures[0], nil
}
func (*signedNonceReceiptClient) SendTransactions(context.Context, soltradesdk.TradeType, [][]byte, bool) ([]solana.Signature, error) {
	return nil, nil
}
func (*signedNonceReceiptClient) GetTipAccount() string { return "" }
func (*signedNonceReceiptClient) GetSwqosType() soltradesdk.SwqosType {
	return soltradesdk.SwqosTypeDefault
}
func (*signedNonceReceiptClient) MinTipSol() float64 { return 0 }

func TestConfirmedNonceFailureDoesNotRetryAcknowledgedTransaction(t *testing.T) {
	payer, authority := solana.NewWallet().PrivateKey, solana.NewWallet().PrivateKey
	nonce := solana.NewWallet().PublicKey()
	tx, err := solana.NewTransaction([]solana.Instruction{
		system.NewAdvanceNonceAccountInstruction(nonce, solana.SysVarRecentBlockHashesPubkey, authority.PublicKey()).Build(),
		system.NewTransferInstruction(1, payer.PublicKey(), solana.NewWallet().PublicKey()).Build(),
	}, solana.Hash(solana.NewWallet().PublicKey()), solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key == payer.PublicKey() {
			return &payer
		}
		if key == authority.PublicKey() {
			return &authority
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	status := &regressionStatusRPC{statusErr: map[string]any{"InstructionError": []any{1, map[string]any{"Custom": 6001}}}}
	ex := NewTradeExecutor(rpc.NewWithCustomRPCClient(status), nil, nil)
	ex.confirmationRetry = 1
	client := &signedNonceReceiptClient{t: t}
	ex.AddSwqosClient(client)
	result := ex.Execute(context.Background(), soltradesdk.TradeTypeBuy, tx, ExecuteOptions{WaitConfirmation: true, MaxRetries: 3, RetryDelayMs: 0})
	if result.Success || !result.Submitted || result.Confirmed || result.Signature != tx.Signatures[0] || result.Error == nil {
		t.Fatalf("lost failure receipt: %+v", result)
	}
	if client.calls != 1 {
		t.Fatalf("already confirmed nonce failure submitted %d times", client.calls)
	}
}

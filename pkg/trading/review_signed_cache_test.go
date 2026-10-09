package trading

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestGeneratedKeyCachedRouteSignsWithoutRPCClient(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := solana.PrivateKey(key)
	request := cachedFixture(t, "buy", "dlmm", signer.PublicKey())
	factory := NewTradeExecutorFactory(nil) // A read RPC cannot be called: no client is supplied.
	executor, err := factory.CreateCachedExecutor(request.DexType)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := executor.Prepare(request)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	submit := func(_ context.Context, wire []byte, _ soltradesdk.TradeType) (solana.Signature, error) {
		calls++
		n := len(prepared.Compiled.Message)
		if !ed25519.Verify(ed25519.PublicKey(signer.PublicKey().Bytes()), wire[:n], wire[n:n+64]) {
			t.Fatal("signature invalid")
		}
		damaged := append([]byte(nil), wire...)
		damaged[n-1] ^= 1
		if ed25519.Verify(ed25519.PublicKey(signer.PublicKey().Bytes()), damaged[:n], damaged[n:n+64]) {
			t.Fatal("tamper accepted")
		}
		var signature solana.Signature
		copy(signature[:], wire[n:n+64])
		return signature, nil
	}
	receipt, err := executor.Execute(context.Background(), request, []solana.PrivateKey{signer}, submit)
	if err != nil || !receipt.Submitted || receipt.Confirmed || calls != 1 {
		t.Fatal(receipt, err, calls)
	}
	if _, err := executor.Execute(context.Background(), request, nil, submit); err == nil || calls != 1 {
		t.Fatal("unsigned wire submitted")
	}
}

package soltradesdk_test

import (
	"context"
	"errors"
	"testing"

	solana "github.com/gagliardetto/solana-go"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
)

type rejectingGate struct{ calls int }

func (g *rejectingGate) CheckBuy(params *soltradesdk.TradeBuyParams) error {
	g.calls++
	return errors.New("blocked by risk gate")
}

func TestBuyInvokesRiskGate(t *testing.T) {
	payer := solana.NewWallet().PrivateKey
	client, err := soltradesdk.NewTradingClient(context.Background(), &payer, &soltradesdk.TradeConfig{
		RPCUrl: "http://localhost:8899",
	})
	if err != nil {
		t.Fatal(err)
	}
	gate := &rejectingGate{}
	client.WithRiskGate(gate)

	_, err = client.Buy(context.Background(), soltradesdk.TradeBuyParams{
		InputTokenAmount:    1_000_000,
		SlippageBasisPoints: 100,
	})
	if err == nil || err.Error() != "blocked by risk gate" {
		t.Fatalf("expected risk gate error, got %v", err)
	}
	if gate.calls != 1 {
		t.Fatalf("expected 1 gate call, got %d", gate.calls)
	}
}

package trading

import (
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/subscription"
	"github.com/gagliardetto/solana-go"
	"strings"
	"testing"
)

func TestPumpSearchBoundsDedupAndDirection(t *testing.T) {
	key := func(b byte) solana.PublicKey { var k solana.PublicKey; k[31] = b; return k }
	a, b, c, d, token := key(1), key(2), key(3), key(4), key(5)
	edge := func(p byte, i, o solana.PublicKey) subscription.PoolTradeHint {
		return subscription.PoolTradeHint{Pool: key(p), InputMint: i, OutputMint: o}
	}
	s := NewPumpSwapRouteSearch(key(100), token, a, []subscription.PoolTradeHint{edge(20, b, c), edge(10, a, b), edge(10, b, a), edge(30, a, d), edge(40, d, token)})
	paths, e := pumpConversionPaths(s, a, c, token)
	if e != nil || len(paths) != 1 || len(paths[0]) != 2 || paths[0][0].InputMint != a || paths[0][1].OutputMint != c {
		t.Fatal(paths, e)
	}
	s.MaximumSearchNodes = 3
	if _, e = pumpConversionPaths(s, a, c, token); e == nil || !strings.Contains(e.Error(), "budget") {
		t.Fatal("dead end not counted after valid path", e)
	}
	s.MaximumSearchNodes = 128
	s.ConversionPools = append(s.ConversionPools, edge(10, a, c))
	if _, e = pumpConversionPaths(s, a, c, token); e == nil {
		t.Fatal("accepted conflicting identity")
	}
	s.ConversionPools = []subscription.PoolTradeHint{edge(100, a, b)}
	if _, e = pumpConversionPaths(s, a, c, token); e == nil {
		t.Fatal("accepted anchor reuse")
	}
	s.ConversionPools = nil
	s.MaximumConversionHops = 0
	if p, e := pumpConversionPaths(s, a, a, token); e != nil || len(p) != 1 || len(p[0]) != 0 {
		t.Fatal(p, e)
	}
	s.ConversionPools = make([]subscription.PoolTradeHint, 129)
	if _, e = pumpConversionPaths(s, a, a, token); e == nil {
		t.Fatal("hint bound checked after dedup")
	}
}

func TestPumpRouteTieUsesPubkeyBytes(t *testing.T) {
	var a, b solana.PublicKey
	a[31] = 1
	b[0] = 1
	x := []PumpSwapQuotedStep{{PoolTradeHint: subscription.PoolTradeHint{Pool: a}}}
	y := []PumpSwapQuotedStep{{PoolTradeHint: subscription.PoolTradeHint{Pool: b}}}
	if !pumpRouteLess(x, y) || pumpRouteLess(y, x) {
		t.Fatal("noncanonical tie")
	}
	if !pumpRouteLess(x, append(y, y...)) {
		t.Fatal("hop count tie")
	}
}

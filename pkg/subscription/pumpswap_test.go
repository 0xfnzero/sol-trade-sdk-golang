package subscription

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"strconv"
	"testing"
)

type pumpSwapFixture struct {
	Pool      string
	Input     string   `json:"input_mint"`
	Output    string   `json:"output_mint"`
	Fees      []string `json:"expected_fees"`
	BuyQuote  string   `json:"expected_buy_quote"`
	SellQuote string   `json:"expected_sell_quote"`
	Accounts  []struct{ Address, Owner, Data string }
}

func pumpSwapFixtures(t *testing.T) []pumpSwapFixture {
	t.Helper()
	d, e := os.ReadFile("testdata/cached_pumpswap_state.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct{ Cases []pumpSwapFixture }
	if e = json.Unmarshal(d, &f); e != nil {
		t.Fatal(e)
	}
	return f.Cases
}
func pumpSwapSnapshot(t *testing.T, c pumpSwapFixture) (map[solana.PublicKey]CachedAccount, PoolTradeHint) {
	t.Helper()
	m := map[solana.PublicKey]CachedAccount{}
	for _, a := range c.Accounts {
		d, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			t.Fatal(e)
		}
		m[solana.MustPublicKeyFromBase58(a.Address)] = CachedAccount{Owner: solana.MustPublicKeyFromBase58(a.Owner), Data: d, Slot: 100, WriteVersion: 1}
	}
	return m, PoolTradeHint{Pool: solana.MustPublicKeyFromBase58(c.Pool), InputMint: solana.MustPublicKeyFromBase58(c.Input), OutputMint: solana.MustPublicKeyFromBase58(c.Output)}
}
func TestPumpSwapCurrentState(t *testing.T) {
	for _, c := range pumpSwapFixtures(t) {
		m, h := pumpSwapSnapshot(t, c)
		s, e := (&AccountCacheSnapshot{accounts: m}).PumpSwap(h, CacheReadContext{Slot: 100, Epoch: 10})
		if e != nil {
			t.Fatal(e)
		}
		f := s.FeeBasisPoints
		got := []uint64{f.LPFeeBasisPoints, f.ProtocolFeeBasisPoints, f.CoinCreatorFeeBasisPoints}
		for i, w := range c.Fees {
			if strconv.FormatUint(got[i], 10) != w {
				t.Fatal(got, c.Fees)
			}
		}
		if s.BaseReserve != 1000000 || s.QuoteReserve != 500000 || s.Pool.VirtualQuoteReserves.Int64() != 1000 {
			t.Fatal(s)
		}
		if s.ProtocolFeeRecipients[0][0] != 8 {
			t.Fatal(s.ProtocolFeeRecipients)
		}
	}
}
func TestPumpSwapRejectCorruptState(t *testing.T) {
	c := pumpSwapFixtures(t)[0]
	for _, pair := range [][2]int{{0, 0}, {0, 243}, {0, 8}, {1, 45}, {3, 0}, {3, 108}, {5, 0}, {5, 417}, {6, 0}, {6, 8}} {
		m, h := pumpSwapSnapshot(t, c)
		key := solana.MustPublicKeyFromBase58(c.Accounts[pair[0]].Address)
		a := m[key]
		if a.Data[pair[1]] == 1 {
			a.Data[pair[1]] = 2
		} else {
			a.Data[pair[1]] ^= 255
		}
		m[key] = a
		if _, e := (&AccountCacheSnapshot{accounts: m}).PumpSwap(h, CacheReadContext{Slot: 100, Epoch: 10}); e == nil {
			t.Fatal("accepted corruption", pair)
		}
	}
}
func TestPumpSwapUnavailableConfig(t *testing.T) {
	c := pumpSwapFixtures(t)[0]
	for _, mode := range []string{"missing", "owner", "stale"} {
		m, h := pumpSwapSnapshot(t, c)
		key := solana.MustPublicKeyFromBase58(c.Accounts[6].Address)
		a := m[key]
		switch mode {
		case "missing":
			delete(m, key)
		case "owner":
			a.Owner = solana.PublicKey{}
			m[key] = a
		case "stale":
			a.Slot = 99
			m[key] = a
		}
		if _, e := (&AccountCacheSnapshot{accounts: m}).PumpSwap(h, CacheReadContext{Slot: 100, Epoch: 10}); e == nil {
			t.Fatal("accepted", mode)
		}
	}
}

func TestPumpSwapIndependentPreparation(t *testing.T) {
	var payer solana.PublicKey
	for i := range payer {
		payer[i] = 42
	}
	for _, c := range pumpSwapFixtures(t) {
		m, h := pumpSwapSnapshot(t, c)
		s := &AccountCacheSnapshot{accounts: m}
		ctx := CacheReadContext{Slot: 100, Epoch: 10}
		for _, buy := range []bool{true, false} {
			leg := h
			want := c.BuyQuote
			if !buy {
				leg.InputMint, leg.OutputMint = h.OutputMint, h.InputMint
				want = c.SellQuote
			}
			p, e := s.PreparePumpSwap(leg, ctx, payer, 10000, 100)
			if e != nil {
				t.Fatal(e)
			}
			out, e := strconv.ParseUint(want, 10, 64)
			if e != nil {
				t.Fatal(e)
			}
			if p.Quote.AmountOut != out || p.Quote.MinimumAmountOut != out-out/100 {
				t.Fatal(p.Quote, want)
			}
			d, e := p.Instruction.Data()
			if e != nil {
				t.Fatal(e)
			}
			if binary.LittleEndian.Uint64(d[8:16]) != 10000 || binary.LittleEndian.Uint64(d[16:24]) != p.Quote.MinimumAmountOut {
				t.Fatal(d)
			}
			keys := p.Instruction.Accounts()
			n := 24
			if buy {
				n = 26
			}
			if len(keys) != n || keys[9].PublicKey[0] != 8 || keys[len(keys)-2].PublicKey[0] != 10 {
				t.Fatal(keys)
			}
			route, e := s.PrepareRoute([]PoolTradeHint{leg}, ctx, 1, payer, 10000, 100, 8)
			if e != nil {
				t.Fatal(e)
			}
			actual, e := route.SwapInstructions[0].Data()
			if e != nil || !bytes.Equal(actual, d) {
				t.Fatal(actual, e)
			}
		}
	}
}
func TestPumpSwapUnavailablePreparation(t *testing.T) {
	c := pumpSwapFixtures(t)[0]
	var payer solana.PublicKey
	for i := range payer {
		payer[i] = 42
	}
	for _, change := range [][3]int{{5, 56, 8}, {5, 56, 16}, {0, 244, 1}, {5, 57, 0}, {5, 643, 0}} {
		m, h := pumpSwapSnapshot(t, c)
		key := solana.MustPublicKeyFromBase58(c.Accounts[change[0]].Address)
		a := m[key]
		if change[1] == 57 || change[1] == 643 {
			clear(a.Data[change[1] : change[1]+32])
		} else {
			a.Data[change[1]] = byte(change[2])
		}
		m[key] = a
		if change[1] == 56 && change[2] == 16 {
			h.InputMint, h.OutputMint = h.OutputMint, h.InputMint
		}
		if _, e := (&AccountCacheSnapshot{accounts: m}).PreparePumpSwap(h, CacheReadContext{Slot: 100, Epoch: 10}, payer, 10000, 100); e == nil {
			t.Fatal("accepted unavailable context", change)
		}
	}
	m, h := pumpSwapSnapshot(t, c)
	a := m[h.Pool]
	a.Data[243] = 1
	m[h.Pool] = a
	p, e := (&AccountCacheSnapshot{accounts: m}).PreparePumpSwap(h, CacheReadContext{Slot: 100, Epoch: 10}, payer, 10000, 100)
	if e != nil {
		t.Fatal(e)
	}
	if p.Instruction.Accounts()[9].PublicKey[0] != 9 {
		t.Fatal("wrong mayhem recipient")
	}
}

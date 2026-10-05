package subscription

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"os"
	"strconv"
	"testing"
)

var token = solana.TokenProgramID
var ctx = CacheReadContext{100, 1047, 0}

func key() solana.PublicKey { return solana.NewWallet().PublicKey() }
func account(data string, slot, version uint64) CachedAccount {
	return CachedAccount{token, []byte(data), slot, version}
}
func fixture(t *testing.T) (*SubscriptionAccountCache, PoolTradeHint, map[string]json.RawMessage) {
	t.Helper()
	raw, e := os.ReadFile("../../examples/fixtures/stonkfun_curve_mainnet_20261002.json")
	if e != nil {
		t.Fatal(e)
	}
	var data map[string]json.RawMessage
	if e = json.Unmarshal(raw, &data); e != nil {
		t.Fatal(e)
	}
	c := &SubscriptionAccountCache{}
	keys := map[string]solana.PublicKey{}
	for _, name := range []string{"pool", "global", "platform", "base_mint", "quote_mint"} {
		var a struct{ Pubkey, Owner, Data string }
		if e = json.Unmarshal(data[name], &a); e != nil {
			t.Fatal(e)
		}
		b, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			t.Fatal(e)
		}
		keys[name] = solana.MustPublicKeyFromBase58(a.Pubkey)
		if _, e = c.Update(keys[name], CachedAccount{solana.MustPublicKeyFromBase58(a.Owner), b, 100, 1}); e != nil {
			t.Fatal(e)
		}
	}
	return c, PoolTradeHint{keys["pool"], keys["base_mint"], keys["quote_mint"]}, data
}
func TestOrderingAtomicBatchAndSnapshot(t *testing.T) {
	c := &SubscriptionAccountCache{}
	k, other := key(), key()
	a := account("one", 100, 1)
	if changed, e := c.Update(k, a); e != nil || !changed {
		t.Fatal(changed, e)
	}
	a.Data[0] = 0
	frozen := c.Snapshot()
	if changed, e := c.Update(k, account("one", 100, 1)); e != nil || changed {
		t.Fatal(changed, e)
	}
	if changed, e := c.Update(k, account("old", 99, 999)); e != nil || changed {
		t.Fatal(changed, e)
	}
	if _, e := c.UpdateMany([]AccountUpdate{{other, account("new", 100, 1)}, {k, account("conflict", 100, 1)}}); e == nil {
		t.Fatal("accepted conflict")
	}
	if _, e := c.Snapshot().Get(other, ctx, nil); e == nil {
		t.Fatal("batch partially committed")
	}
	if err := frozen.AssertUsable(); err == nil {
		t.Fatal("conflicted frozen snapshot still usable")
	}
	if _, err := c.Update(k, account("two", 100, 2)); err == nil {
		t.Fatal("new write version implicitly resolved fork conflict")
	}
	c = &SubscriptionAccountCache{}
	c.Update(k, account("one", 100, 1))
	frozen = c.Snapshot()
	c.Update(k, account("two", 100, 2))
	returned, e := frozen.Get(k, ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	returned.Data[0] = 0
	again, _ := frozen.Get(k, ctx, nil)
	if string(again.Data) != "one" {
		t.Fatal("mutated snapshot")
	}
	current, _ := c.Snapshot().Get(k, ctx, nil)
	if string(current.Data) != "two" {
		t.Fatal("update missing")
	}
	for _, read := range []CacheReadContext{{99, 0, 10}, {102, 0, 1}} {
		if _, e := c.Snapshot().Get(k, read, nil); e == nil {
			t.Fatal("accepted invalid age")
		}
	}
	wrong := key()
	if _, e := c.Snapshot().Get(k, ctx, &wrong); e == nil {
		t.Fatal("accepted wrong owner")
	}
}
func TestParserClosure(t *testing.T) {
	c := &SubscriptionAccountCache{}
	k := key()
	c.UpdateRaw(k, token, []byte("old"), 0, 100, 1)
	if changed, _ := c.Update(k, account("resurrect", 99, 999)); changed {
		t.Fatal("resurrected")
	}
	if _, e := c.Snapshot().Get(k, ctx, nil); e == nil {
		t.Fatal("accepted closed")
	}
}
func TestCurrentCurvePreparation(t *testing.T) {
	c, h, _ := fixture(t)
	payer := key()
	sell, e := c.Snapshot().PrepareStonkFunCurve(h, ctx, payer, 1000000000, 100)
	if e != nil {
		t.Fatal(e)
	}
	if sell.Quote.MinimumAmountOut != 1374 {
		t.Fatal(sell.Quote)
	}
	if sell.Instruction.Accounts()[0].PublicKey != payer {
		t.Fatal("payer copied")
	}
	buy, e := c.Snapshot().PrepareStonkFunCurve(PoolTradeHint{h.Pool, h.OutputMint, h.InputMint}, ctx, payer, 10000, 100)
	if e != nil {
		t.Fatal(e)
	}
	if buy.Quote.MinimumAmountOut != 6817301666 {
		t.Fatal(buy.Quote)
	}
	if _, _, e := c.Snapshot().StonkFunCurve(PoolTradeHint{h.Pool, key(), h.OutputMint}, ctx); e == nil {
		t.Fatal("accepted wrong mint")
	}
	c.Update(h.OutputMint, account("", 101, 1))
	if _, _, e := c.Snapshot().StonkFunCurve(h, CacheReadContext{101, 1047, 1}); e == nil {
		t.Fatal("accepted closed mint")
	}
}
func TestRouteIdentity(t *testing.T) {
	_, h, _ := fixture(t)
	input, output := h.InputMint.String(), h.OutputMint.String()
	leg := ParserRouteIdentity{"LaunchLab", protocols["LaunchLab"], h.Pool.String(), &input, &output}
	got, e := PoolTradeHintFromRouteLeg(leg)
	if e != nil || got != h {
		t.Fatal(got, e)
	}
	leg.Program = token.String()
	if _, e := PoolTradeHintFromIdentity(leg); e == nil {
		t.Fatal("accepted wrong program")
	}
	leg.Program = protocols["LaunchLab"]
	leg.InputMint = nil
	if _, e := PoolTradeHintFromIdentity(leg); e == nil {
		t.Fatal("accepted unresolved mint")
	}
}
func TestCpmmRustGolden(t *testing.T) {
	raw, e := os.ReadFile("testdata/cpmm_rust_5_0_6.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixture struct {
		Cases []struct {
			CreatorFeeOn uint8  `json:"creator_fee_on"`
			BaseIn       bool   `json:"base_in"`
			Enabled      bool   `json:"enabled"`
			Amount       string `json:"amount"`
			Bps          uint16 `json:"slippage_bps"`
			FeeBps       uint16 `json:"fee_basis_points"`
			MaxFee       string `json:"maximum_fee"`
			Out          string `json:"amount_out"`
			Min          string `json:"minimum_amount_out"`
			Trade        string `json:"trade_fee"`
		}
	}
	if e = json.Unmarshal(raw, &fixture); e != nil {
		t.Fatal(e)
	}
	parse := func(s string) uint64 {
		n, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}
	for _, c := range fixture.Cases {
		fee := calc.TokenTransferFee{BasisPoints: c.FeeBps, MaximumFee: parse(c.MaxFee)}
		p := CachedCpmmState{BaseReserve: 1000000, QuoteReserve: 2000000, TradeFeeRate: 2500, ProtocolFeeRate: 120000, FundFeeRate: 40000, CreatorFeeRate: 10000, CreatorFeeOn: c.CreatorFeeOn, EnableCreatorFee: c.Enabled, BaseTransferFee: fee, QuoteTransferFee: fee}
		q, e := QuoteCachedCpmmExactIn(p, parse(c.Amount), c.BaseIn, c.Bps)
		if e != nil {
			t.Fatal(e)
		}
		if q.AmountOut != parse(c.Out) || q.MinimumAmountOut != parse(c.Min) || q.TradeFee != parse(c.Trade) {
			t.Fatalf("case %+v: %+v", c, q)
		}
	}
}
func TestCpmmCurrentStateAndValidation(t *testing.T) {
	c := &SubscriptionAccountCache{}
	pool, config, base, quote, bv, qv, obs := key(), key(), key(), key(), key(), key(), key()
	d := make([]byte, 637)
	copy(d, []byte{247, 237, 227, 245, 215, 195, 222, 70})
	for o, k := range map[int]solana.PublicKey{8: config, 72: bv, 104: qv, 168: base, 200: quote, 232: token, 264: token, 296: obs} {
		copy(d[o:o+32], k[:])
	}
	for o, n := range map[int]uint64{341: 10, 357: 20, 397: 5, 373: 100} {
		binary.LittleEndian.PutUint64(d[o:], n)
	}
	d[389] = 2
	d[390] = 1
	f := make([]byte, 236)
	copy(f, []byte{218, 244, 33, 104, 203, 203, 43, 111})
	for o, n := range map[int]uint64{12: 4321, 20: 12345, 28: 54321, 108: 987} {
		binary.LittleEndian.PutUint64(f[o:], n)
	}
	c.Update(pool, CachedAccount{CpmmProgram, d, 100, 1})
	c.Update(config, CachedAccount{CpmmProgram, f, 100, 1})
	for _, v := range []struct {
		vault, mint solana.PublicKey
		amount      uint64
	}{{bv, base, 1000}, {qv, quote, 2000}} {
		b := make([]byte, 165)
		copy(b, v.mint[:])
		copy(b[32:64], instruction.RAYDIUM_CPMM_AUTHORITY[:])
		binary.LittleEndian.PutUint64(b[64:], v.amount)
		b[108] = 1
		m := make([]byte, 82)
		m[45] = 1
		c.Update(v.vault, CachedAccount{token, b, 100, 1})
		c.Update(v.mint, CachedAccount{token, m, 100, 1})
	}
	h := PoolTradeHint{pool, quote, base}
	p, e := c.Snapshot().Cpmm(h, ctx, 100)
	if e != nil {
		t.Fatal(e)
	}
	if p.BaseReserve != 965 || p.QuoteReserve != 2000 || p.TradeFeeRate != 4321 || p.CreatorFeeRate != 987 || p.CreatorFeeOn != 2 {
		t.Fatal(p)
	}
	prepared, e := c.Snapshot().PrepareCpmm(h, ctx, 100, key(), 100, 100)
	if e != nil {
		t.Fatal(e)
	}
	if prepared.Instruction.Accounts()[10].PublicKey != quote {
		t.Fatal("orientation lost")
	}
	if _, e := c.Snapshot().Cpmm(h, ctx, 99); e == nil {
		t.Fatal("accepted unopened")
	}
	d[329] = 4
	c.Update(pool, CachedAccount{CpmmProgram, d, 100, 2})
	if _, e := c.Snapshot().Cpmm(h, ctx, 100); e == nil {
		t.Fatal("accepted disabled")
	}
	d[329] = 0
	c.Update(pool, CachedAccount{CpmmProgram, d, 100, 3})
	bad := make([]byte, 165)
	copy(bad, base[:])
	bad[108] = 1
	c.Update(bv, CachedAccount{token, bad, 100, 2})
	if _, e := c.Snapshot().Cpmm(h, ctx, 100); e == nil {
		t.Fatal("accepted underflow")
	}
}
func TestCpmmMainnetSimulationQuotes(t *testing.T) {
	parse := func(s string) uint64 {
		n, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}
	raw, e := os.ReadFile("../../examples/fixtures/cpmm_mainnet_20261002.json")
	if e != nil {
		t.Fatal(e)
	}
	var saved map[string]json.RawMessage
	if e = json.Unmarshal(raw, &saved); e != nil {
		t.Fatal(e)
	}
	cache := &SubscriptionAccountCache{}
	keys := map[string]solana.PublicKey{}
	for _, n := range []string{"pool", "config", "base_mint", "quote_mint", "base_vault", "quote_vault"} {
		var a struct{ Pubkey, Owner, Data, Slot string }
		if e = json.Unmarshal(saved[n], &a); e != nil {
			t.Fatal(e)
		}
		d, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			t.Fatal(e)
		}
		keys[n] = solana.MustPublicKeyFromBase58(a.Pubkey)
		cache.Update(keys[n], CachedAccount{solana.MustPublicKeyFromBase58(a.Owner), d, parse(a.Slot), 0})
	}
	field := func(n string) uint64 {
		var s string
		if e = json.Unmarshal(saved[n], &s); e != nil {
			t.Fatal(e)
		}
		return parse(s)
	}
	state, e := cache.Snapshot().Cpmm(PoolTradeHint{keys["pool"], keys["base_mint"], keys["quote_mint"]}, CacheReadContext{field("read_slot"), field("epoch"), 32}, field("unix_timestamp"))
	if e != nil {
		t.Fatal(e)
	}
	raw, e = os.ReadFile("../../examples/fixtures/cpmm_mainnet_simulations_20261002.json")
	if e != nil {
		t.Fatal(e)
	}
	var evidence struct {
		Cases []struct {
			Side   string
			Input  string `json:"input_reserve_at_simulation"`
			Output string `json:"output_reserve_at_simulation"`
			Actual string `json:"actual_net_output"`
		}
	}
	if e = json.Unmarshal(raw, &evidence); e != nil {
		t.Fatal(e)
	}
	for _, c := range evidence.Cases {
		p := state
		buy := c.Side == "buy"
		amount := uint64(1000000000)
		p.BaseReserve, p.QuoteReserve = parse(c.Output), parse(c.Input)
		if buy {
			amount = 1000000
			p.BaseReserve, p.QuoteReserve = parse(c.Input), parse(c.Output)
		}
		q, e := QuoteCachedCpmmExactIn(p, amount, buy, 100)
		if e != nil || q.AmountOut != parse(c.Actual) {
			t.Fatal(c, q, e)
		}
	}
}

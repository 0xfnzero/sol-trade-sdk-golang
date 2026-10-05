package subscription

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"strconv"
	"testing"
)

type clmmFixtureAccount struct {
	Pubkey, Owner, Data, Slot string
	WriteVersion              string `json:"write_version"`
}
type clmmFixture struct {
	Pool, Payer, Amount, Epoch string
	InputMint                  string `json:"input_mint"`
	OutputMint                 string `json:"output_mint"`
	ReadSlot                   string `json:"read_slot"`
	UnixTimestamp              string `json:"unix_timestamp"`
	Slippage                   uint16 `json:"slippage_bps"`
	Budget                     int    `json:"maximum_arrays"`
	Accounts                   []clmmFixtureAccount
	Legs                       []struct {
		Pool       string
		InputMint  string `json:"input_mint"`
		OutputMint string `json:"output_mint"`
	}
	Expected map[string]json.RawMessage
}

func clmmLoad(t *testing.T, name string) clmmFixture {
	t.Helper()
	data, e := os.ReadFile("../../examples/fixtures/" + name + "_mainnet_20261002.json")
	if e != nil {
		t.Fatal(e)
	}
	var v clmmFixture
	if e = json.Unmarshal(data, &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func clmmNumber(t *testing.T, s string) uint64 {
	t.Helper()
	v, e := strconv.ParseUint(s, 10, 64)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func clmmSnapshot(t *testing.T, v clmmFixture) *AccountCacheSnapshot {
	t.Helper()
	c := &SubscriptionAccountCache{}
	for _, a := range v.Accounts {
		b, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = c.Update(solana.MustPublicKeyFromBase58(a.Pubkey), CachedAccount{Owner: solana.MustPublicKeyFromBase58(a.Owner), Data: b, Slot: clmmNumber(t, a.Slot), WriteVersion: clmmNumber(t, a.WriteVersion)}); e != nil {
			t.Fatal(e)
		}
	}
	return c.Snapshot()
}
func TestCachedClmmMainnet(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		v := clmmLoad(t, "clmm")
		im, om := v.InputMint, v.OutputMint
		k := "stock_to_usdc"
		if reverse {
			im, om = om, im
			k = "usdc_to_stock"
		}
		a, q, _, e := clmmSnapshot(t, v).PrepareClmm(PoolTradeHint{solana.MustPublicKeyFromBase58(v.Pool), solana.MustPublicKeyFromBase58(im), solana.MustPublicKeyFromBase58(om)}, CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), 10000, 100, 8)
		if e != nil {
			t.Fatal(e)
		}
		var want string
		json.Unmarshal(v.Expected[k], &want)
		if strconv.FormatUint(q.EstimatedNetAmountOut, 10) != want || q.MinimumAmountOut != q.EstimatedNetAmountOut*99/100 || len(a.TickArrays) != 1 {
			t.Fatal(q)
		}
	}
}
func TestCachedClmmInvalid(t *testing.T) {
	for _, kind := range []string{"stale", "owner", "missing_array", "pool_identity", "tick_index", "closed", "budget"} {
		t.Run(kind, func(t *testing.T) {
			v := clmmLoad(t, "clmm")
			age := uint64(0)
			budget := 8
			if kind == "stale" {
				age = 1
			}
			if kind == "budget" {
				budget = 0
			}
			kept := []clmmFixtureAccount{}
			for _, a := range v.Accounts {
				d, _ := base64.StdEncoding.DecodeString(a.Data)
				if a.Pubkey == v.Pool {
					if kind == "owner" {
						a.Owner = solana.PublicKey{}.String()
					}
					if kind == "closed" {
						a.Data = ""
					}
				}
				if len(d) == 10240 {
					if kind == "missing_array" {
						continue
					}
					if kind == "pool_identity" {
						copy(d[8:40], make([]byte, 32))
					}
					if kind == "tick_index" {
						for i := 0; i < 60; i++ {
							o := 44 + i*168
							initialized := false
							for _, b := range append(append([]byte{}, d[o+20:o+36]...), d[o+124:o+140]...) {
								initialized = initialized || b != 0
							}
							if initialized {
								binary.LittleEndian.PutUint32(d[o:], 443636)
							}
						}
					}
					a.Data = base64.StdEncoding.EncodeToString(d)
				}
				kept = append(kept, a)
			}
			v.Accounts = kept
			_, _, _, e := clmmSnapshot(t, v).PrepareClmm(PoolTradeHint{solana.MustPublicKeyFromBase58(v.Pool), solana.MustPublicKeyFromBase58(v.InputMint), solana.MustPublicKeyFromBase58(v.OutputMint)}, CacheReadContext{clmmNumber(t, v.ReadSlot) + age, clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), 10000, 100, budget)
			if e == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
func TestCachedRouteMainnet(t *testing.T) {
	for _, direction := range []string{"buy", "sell"} {
		v := clmmLoad(t, "route_"+direction)
		hints := []PoolTradeHint{}
		for _, h := range v.Legs {
			hints = append(hints, PoolTradeHint{solana.MustPublicKeyFromBase58(h.Pool), solana.MustPublicKeyFromBase58(h.InputMint), solana.MustPublicKeyFromBase58(h.OutputMint)})
		}
		r, e := clmmSnapshot(t, v).PrepareRoute(hints, CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), clmmNumber(t, v.Amount), v.Slippage, v.Budget)
		if e != nil {
			t.Fatal(e)
		}
		var want string
		json.Unmarshal(v.Expected["minimum_amount_out"], &want)
		if strconv.FormatUint(r.MinimumNetAmountOut, 10) != want || len(r.SetupInstructions) != 3 || len(r.SwapInstructions) != 2 || r.Legs[1].AmountIn > r.Legs[0].MinimumNetAmountOut {
			t.Fatal(r)
		}
		var legs []map[string]string
		json.Unmarshal(v.Expected["legs"], &legs)
		for i, l := range r.Legs {
			if strconv.FormatUint(l.AmountIn, 10) != legs[i]["amount_in"] || strconv.FormatUint(*l.EstimatedNetAmountOut, 10) != legs[i]["amount_out"] || strconv.FormatUint(l.MinimumNetAmountOut, 10) != legs[i]["minimum_amount_out"] {
				t.Fatal(l)
			}
		}
	}
}
func TestCachedRouteInvalid(t *testing.T) {
	for _, kind := range []string{"disconnected", "reused", "cycle", "stale", "unsupported", "zero", "slippage"} {
		t.Run(kind, func(t *testing.T) {
			v := clmmLoad(t, "route_buy")
			age := uint64(0)
			switch kind {
			case "disconnected":
				v.Legs[1].InputMint = v.Legs[0].InputMint
			case "reused":
				v.Legs[1].Pool = v.Legs[0].Pool
			case "cycle":
				v.Legs[1].OutputMint = v.Legs[0].InputMint
			case "stale":
				age = 1
			case "unsupported":
				for i, a := range v.Accounts {
					if a.Pubkey == v.Legs[0].Pool {
						v.Accounts[i].Owner = solana.PublicKey{}.String()
					}
				}
			case "zero":
				v.Amount = "0"
			case "slippage":
				v.Slippage = 10000
			}
			hints := []PoolTradeHint{}
			for _, h := range v.Legs {
				hints = append(hints, PoolTradeHint{solana.MustPublicKeyFromBase58(h.Pool), solana.MustPublicKeyFromBase58(h.InputMint), solana.MustPublicKeyFromBase58(h.OutputMint)})
			}
			_, e := clmmSnapshot(t, v).PrepareRoute(hints, CacheReadContext{clmmNumber(t, v.ReadSlot) + age, clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), clmmNumber(t, v.Amount), v.Slippage, v.Budget)
			if e == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

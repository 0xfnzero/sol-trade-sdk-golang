package subscription

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"math/big"
	"os"
	"testing"
)

type ammVector struct {
	CoinReserve                                   string `json:"coin_reserve"`
	PcReserve                                     string `json:"pc_reserve"`
	Amount, Out, Min, Fee, Numerator, Denominator string
	CoinIn                                        bool   `json:"coin_in"`
	Slippage                                      uint16 `json:"slippage_bps"`
}

func TestAmmV4RustAndExecutionEvidence(t *testing.T) {
	for _, name := range []string{"amm_v4_rust_5_0_6.json", "amm_v4_execution_replays_20261002.json"} {
		d, e := os.ReadFile("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		var vectors []ammVector
		if e = json.Unmarshal(d, &vectors); e != nil {
			t.Fatal(e)
		}
		for i, v := range vectors {
			num, denom := uint64(25), uint64(10000)
			if v.Numerator != "" {
				num, denom = clmmNumber(t, v.Numerator), clmmNumber(t, v.Denominator)
			}
			p := CachedAmmV4State{CoinReserve: clmmNumber(t, v.CoinReserve), PcReserve: clmmNumber(t, v.PcReserve), SwapFeeNumerator: num, SwapFeeDenominator: denom}
			q, e := QuoteCachedAmmV4ExactIn(p, clmmNumber(t, v.Amount), v.CoinIn, v.Slippage)
			if e != nil || q.AmountOut != clmmNumber(t, v.Out) {
				t.Fatalf("%s vector %d: %+v %v", name, i, q, e)
			}
			if v.Min != "" && (q.MinimumAmountOut != clmmNumber(t, v.Min) || q.SwapFee != clmmNumber(t, v.Fee)) {
				t.Fatal(q, v)
			}
		}
	}
}
func TestCachedAmmV4MainnetRoute(t *testing.T) {
	for _, name := range []string{"buy", "sell", "route_buy", "route_sell"} {
		v := clmmLoad(t, "amm_v4_"+name)
		hints := []PoolTradeHint{}
		for _, h := range v.Legs {
			hints = append(hints, PoolTradeHint{solana.MustPublicKeyFromBase58(h.Pool), solana.MustPublicKeyFromBase58(h.InputMint), solana.MustPublicKeyFromBase58(h.OutputMint)})
		}
		r, e := clmmSnapshot(t, v).PrepareRoute(hints, CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), clmmNumber(t, v.Amount), 100, 8)
		if e != nil {
			t.Fatal(e)
		}
		var want string
		json.Unmarshal(v.Expected["minimum_amount_out"], &want)
		if r.MinimumNetAmountOut != clmmNumber(t, want) {
			t.Fatal(r)
		}
		for _, ix := range r.SwapInstructions {
			if ix.ProgramID() == AmmV4Program {
				data, _ := ix.Data()
				keys := ix.Accounts()
				if len(keys) != 8 || data[0] != 16 || !keys[7].IsSigner || keys[7].IsWritable {
					t.Fatal(ix)
				}
			}
		}
	}
}
func TestAmmV4DecoderFullU128(t *testing.T) {
	v := clmmLoad(t, "amm_v4_buy")
	for _, a := range v.Accounts {
		if a.Owner != AmmV4Program.String() {
			continue
		}
		d, _ := base64.StdEncoding.DecodeString(a.Data)
		counters := []*big.Int{}
		for i, o := range []int{256, 272, 296, 312} {
			n := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 100), big.NewInt(int64(i)))
			counters = append(counters, n)
			binary.LittleEndian.PutUint64(d[o:], uint64(i))
			binary.LittleEndian.PutUint64(d[o+8:], 1<<36)
		}
		p := instruction.DecodeAmmInfo(d)
		for i, n := range []*big.Int{p.Output.SwapCoinInAmount, p.Output.SwapPcOutAmount, p.Output.SwapPcInAmount, p.Output.SwapCoinOutAmount} {
			if n.Cmp(counters[i]) != 0 {
				t.Fatal(n, counters[i])
			}
		}
		var vault, mint solana.PublicKey
		copy(vault[:], d[336:368])
		copy(mint[:], d[400:432])
		if p.TokenCoin != vault || p.CoinMint != mint {
			t.Fatal(p)
		}
		return
	}
	t.Fatal("pool missing")
}
func TestAmmV4InvalidCurrentState(t *testing.T) {
	for _, kind := range []string{"disabled", "future_open", "fee", "pnl", "nonce", "vault_owner", "vault_mint", "frozen", "decimals", "stale", "missing"} {
		t.Run(kind, func(t *testing.T) {
			v := clmmLoad(t, "amm_v4_buy")
			h := v.Legs[0]
			var vault, mint solana.PublicKey
			for i, a := range v.Accounts {
				if a.Pubkey != h.Pool {
					continue
				}
				d, _ := base64.StdEncoding.DecodeString(a.Data)
				copy(vault[:], d[336:368])
				copy(mint[:], d[400:432])
				switch kind {
				case "disabled":
					binary.LittleEndian.PutUint64(d, 2)
				case "future_open":
					binary.LittleEndian.PutUint64(d, 7)
					binary.LittleEndian.PutUint64(d[224:], clmmNumber(t, v.UnixTimestamp)+1)
				case "fee":
					binary.LittleEndian.PutUint64(d[184:], 0)
				case "pnl":
					binary.LittleEndian.PutUint64(d[192:], ^uint64(0))
				case "nonce":
					binary.LittleEndian.PutUint64(d[8:], 256)
				}
				v.Accounts[i].Data = base64.StdEncoding.EncodeToString(d)
			}
			kept := []clmmFixtureAccount{}
			for _, a := range v.Accounts {
				d, _ := base64.StdEncoding.DecodeString(a.Data)
				if a.Pubkey == vault.String() {
					if kind == "missing" {
						continue
					}
					switch kind {
					case "vault_owner":
						clear(d[32:64])
					case "vault_mint":
						clear(d[:32])
					case "frozen":
						d[108] = 2
					}
				}
				if a.Pubkey == mint.String() && kind == "decimals" {
					d[44] = 255
				}
				a.Data = base64.StdEncoding.EncodeToString(d)
				kept = append(kept, a)
			}
			v.Accounts = kept
			ctx := CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}
			if kind == "stale" {
				ctx.Slot++
			}
			_, e := clmmSnapshot(t, v).PrepareAmmV4(PoolTradeHint{solana.MustPublicKeyFromBase58(h.Pool), solana.MustPublicKeyFromBase58(h.InputMint), solana.MustPublicKeyFromBase58(h.OutputMint)}, ctx, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), 10000, 100)
			if e == nil {
				t.Fatal("invalid state accepted")
			}
		})
	}
}

package subscription

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"os"
	"strconv"
	"testing"
)

func TestCachedDlmmMainnet(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		v := clmmLoad(t, "dlmm")
		im, om := v.InputMint, v.OutputMint
		amount := uint64(10000)
		key := "usdc_to_wsol"
		if reverse {
			im, om = om, im
			amount = 1000000
			key = "wsol_to_usdc"
		}
		a, q, ix, e := clmmSnapshot(t, v).PrepareDlmm(PoolTradeHint{solana.MustPublicKeyFromBase58(v.Pool), solana.MustPublicKeyFromBase58(im), solana.MustPublicKeyFromBase58(om)}, CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), amount, 100, 8)
		if e != nil {
			t.Fatal(e)
		}
		var want string
		json.Unmarshal(v.Expected[key], &want)
		d, _ := ix.Data()
		if strconv.FormatUint(q.EstimatedNetAmountOut, 10) != want || q.MinimumAmountOut != q.EstimatedNetAmountOut*99/100 || len(a.BinArrays) == 0 || !bytes.Equal(d[:8], []byte{65, 75, 63, 76, 235, 91, 91, 136}) {
			t.Fatal(q)
		}
	}
}
func TestCachedDlmmInvalid(t *testing.T) {
	for _, kind := range []string{"stale", "owner", "missing", "pool_identity", "zero_price", "closed", "budget", "future_time", "mode", "power", "activation", "function"} {
		t.Run(kind, func(t *testing.T) {
			v := clmmLoad(t, "dlmm")
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
					switch kind {
					case "owner":
						a.Owner = solana.PublicKey{}.String()
					case "closed":
						d = nil
					case "future_time":
						binary.LittleEndian.PutUint64(d[56:], clmmNumber(t, v.UnixTimestamp)+1)
					case "mode":
						d[36] = 2
					case "power":
						d[34] = 19
					case "activation":
						d[86] = 0
						binary.LittleEndian.PutUint64(d[816:], clmmNumber(t, v.ReadSlot)+1)
					case "function":
						d[35] = 3
					}
				}
				if len(d) >= 8 && bytes.Equal(d[:8], []byte{92, 142, 92, 220, 5, 148, 70, 181}) {
					switch kind {
					case "missing":
						continue
					case "pool_identity":
						copy(d[24:56], make([]byte, 32))
					case "zero_price":
						binary.LittleEndian.PutUint64(d[56:], 1)
						copy(d[72:88], make([]byte, 16))
					}
				}
				a.Data = base64.StdEncoding.EncodeToString(d)
				kept = append(kept, a)
			}
			v.Accounts = kept
			_, _, _, e := clmmSnapshot(t, v).PrepareDlmm(PoolTradeHint{solana.MustPublicKeyFromBase58(v.Pool), solana.MustPublicKeyFromBase58(v.InputMint), solana.MustPublicKeyFromBase58(v.OutputMint)}, CacheReadContext{clmmNumber(t, v.ReadSlot) + age, clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), 10000, 100, budget)
			if e == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
func TestDlmmSolRouteReplay(t *testing.T) {
	for _, direction := range []string{"buy", "sell"} {
		v := clmmLoad(t, "dlmm_sol_route_"+direction)
		hints := []PoolTradeHint{}
		for _, h := range v.Legs {
			hints = append(hints, PoolTradeHint{solana.MustPublicKeyFromBase58(h.Pool), solana.MustPublicKeyFromBase58(h.InputMint), solana.MustPublicKeyFromBase58(h.OutputMint)})
		}
		r, e := clmmSnapshot(t, v).PrepareRoute(hints, CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), clmmNumber(t, v.Amount), 100, 8)
		if e != nil {
			t.Fatal(e)
		}
		var minimum string
		json.Unmarshal(v.Expected["minimum_amount_out"], &minimum)
		if strconv.FormatUint(r.MinimumNetAmountOut, 10) != minimum || len(r.Legs) != 3 {
			t.Fatal(r)
		}
		for i := 1; i < 3; i++ {
			if r.Legs[i].AmountIn > r.Legs[i-1].MinimumNetAmountOut {
				t.Fatal("overfunded leg")
			}
		}
	}
}

func TestDlmmExtendedBitmap(t *testing.T) {
	data, e := os.ReadFile("testdata/dlmm_bitmap_synthetic.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []json.RawMessage
	if e = json.Unmarshal(data, &cases); e != nil {
		t.Fatal(e)
	}
	for _, raw := range cases {
		var v clmmFixture
		var expected struct {
			Array  string `json:"expected_array"`
			Bitmap string `json:"expected_bitmap"`
		}
		json.Unmarshal(raw, &v)
		json.Unmarshal(raw, &expected)
		prepare := func(v clmmFixture) (instruction.MeteoraDlmmSwap2Accounts, CachedDlmmQuote, error) {
			a, q, _, e := clmmSnapshot(t, v).PrepareDlmm(PoolTradeHint{solana.MustPublicKeyFromBase58(v.Pool), solana.MustPublicKeyFromBase58(v.InputMint), solana.MustPublicKeyFromBase58(v.OutputMint)}, CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), 10000, 100, 8)
			return a, q, e
		}
		a, q, e := prepare(v)
		if e != nil {
			t.Fatal(e)
		}
		if q.EstimatedNetAmountOut != 10000 || len(a.BinArrays) != 1 || a.BinArrays[0].String() != expected.Array || a.BitmapExtension == nil || a.BitmapExtension.String() != expected.Bitmap {
			t.Fatal("incorrect bitmap traversal", a, q)
		}
		for i, x := range v.Accounts {
			if x.Pubkey == expected.Bitmap {
				v.Accounts[i].Data = ""
			}
		}
		if _, _, e = prepare(v); e == nil {
			t.Fatal("closed bitmap accepted")
		}
	}
}

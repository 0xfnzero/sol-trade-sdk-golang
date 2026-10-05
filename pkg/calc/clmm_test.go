package calc

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"testing"
)

func TestClmmRustGolden(t *testing.T) {
	data, e := os.ReadFile("testdata/clmm_rust_5_0_6.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus []struct {
		Case struct {
			Kind                                      string
			Tick                                      int32
			Spacing                                   uint16
			Current, Target, Liquidity, Amount, Limit string
			Fee                                       uint32
			FeeOn                                     uint8 `json:"fee_on"`
			Down                                      bool
			Dynamic                                   []uint8
			Timestamp                                 uint64
			Ticks                                     []struct {
				Tick                        int32
				Net, Gross, Orders, Partial string
			}
		}
		Expected map[string]string
	}
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	n := func(s string) *big.Int {
		v, ok := new(big.Int).SetString(s, 10)
		if !ok {
			t.Fatal(s)
		}
		return v
	}
	for i, v := range corpus {
		t.Run(fmt.Sprintf("%d_%s", i, v.Case.Kind), func(t *testing.T) {
			c := v.Case
			var got map[string]string
			var err error
			switch c.Kind {
			case "tick":
				var p *big.Int
				p, err = ClmmSqrtPriceAtTick(c.Tick)
				if err == nil {
					got = map[string]string{"price": p.String()}
					if p.Cmp(clmmMaxPrice) < 0 {
						tick, e := ClmmTickAtSqrtPrice(p)
						if e != nil || tick != c.Tick {
							t.Fatalf("inverse tick %d %v", tick, e)
						}
					}
				}
			case "step":
				var r ClmmSwapStepResult
				r, err = ClmmSwapStep(n(c.Current), n(c.Target), n(c.Liquidity), n(c.Amount).Uint64(), c.Fee, c.Down)
				if err == nil {
					got = map[string]string{"price": r.SqrtPrice.String(), "input": fmt.Sprint(r.AmountIn), "output": fmt.Sprint(r.AmountOut), "fee": fmt.Sprint(r.Fee)}
				}
			case "swap":
				ticks := make([]ClmmTick, len(c.Ticks))
				for i, x := range c.Ticks {
					ticks[i] = ClmmTick{x.Tick, n(x.Net), n(x.Gross), n(x.Orders).Uint64(), n(x.Partial).Uint64()}
				}
				before := fmt.Sprint(ticks)
				p := ClmmPool{n(c.Current), n(c.Liquidity), c.Tick, c.Spacing, c.Fee}
				var r ClmmSwapResult
				r, err = ClmmSwapExactIn(p, ticks, n(c.Amount).Uint64(), n(c.Limit), c.FeeOn, c.Dynamic, c.Timestamp, c.Down)
				if fmt.Sprint(ticks) != before {
					t.Fatal("mutated input ticks")
				}
				if err == nil {
					got = map[string]string{"consumed": fmt.Sprint(r.Consumed), "output": fmt.Sprint(r.AmountOut)}
				}
			}
			if _, bad := v.Expected["error"]; bad {
				if err == nil {
					t.Fatal("expected rejection")
				}
			} else if err != nil || !reflect.DeepEqual(got, v.Expected) {
				t.Fatalf("got %v (%v), expected %v", got, err, v.Expected)
			}
		})
	}
}

package calc

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"testing"
)

func TestWhirlpoolRustGolden(t *testing.T) {
	data, e := os.ReadFile("testdata/whirlpool_rust_5_0_6.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		Case struct {
			Kind                       string
			Tick                       int32
			Current, Liquidity, Amount string
			Spacing                    uint16
			Fee                        uint32
			Down                       bool
			Timestamp                  uint64
			Starts                     []int32
			Ticks                      []struct {
				Tick int32
				Net  string
			}
			Adaptive *struct {
				FilterPeriod           uint16 `json:"filter_period"`
				DecayPeriod            uint16 `json:"decay_period"`
				ReductionFactor        uint16 `json:"reduction_factor"`
				ControlFactor          uint32 `json:"control_factor"`
				MaximumVolatility      uint32 `json:"maximum_volatility"`
				TickGroupSize          uint16 `json:"tick_group_size"`
				LastReferenceTimestamp uint64 `json:"last_reference_timestamp"`
				LastMajorSwapTimestamp uint64 `json:"last_major_swap_timestamp"`
				VolatilityReference    uint32 `json:"volatility_reference"`
				ReferenceGroup         int32  `json:"reference_group"`
				Volatility             uint32
			}
		}
		Expected map[string]json.RawMessage
	}
	if e = json.Unmarshal(data, &cases); e != nil {
		t.Fatal(e)
	}
	n := func(s string) *big.Int {
		v, ok := new(big.Int).SetString(s, 10)
		if !ok {
			t.Fatal(s)
		}
		return v
	}
	for i, v := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			c := v.Case
			var got map[string]any
			var err error
			if c.Kind == "tick" {
				p, e := WhirlpoolSqrtPriceAtTick(c.Tick)
				err = e
				if err == nil {
					got = map[string]any{"price": p.String()}
					tick, e := WhirlpoolTickAtSqrtPrice(p)
					if e != nil || tick != c.Tick {
						t.Fatal("tick inverse", tick, e)
					}
				}
			} else {
				var adaptive *WhirlpoolAdaptiveFee
				if a := c.Adaptive; a != nil {
					adaptive = &WhirlpoolAdaptiveFee{FilterPeriod: a.FilterPeriod, DecayPeriod: a.DecayPeriod, ReductionFactor: a.ReductionFactor, ControlFactor: a.ControlFactor, MaximumVolatility: a.MaximumVolatility, TickGroupSize: a.TickGroupSize, LastReferenceTimestamp: a.LastReferenceTimestamp, LastMajorSwapTimestamp: a.LastMajorSwapTimestamp, VolatilityReference: a.VolatilityReference, ReferenceGroup: a.ReferenceGroup, Volatility: a.Volatility}
				}
				ticks := []ClmmTick{}
				for _, t := range c.Ticks {
					ticks = append(ticks, ClmmTick{Tick: t.Tick, LiquidityNet: n(t.Net), LiquidityGross: big.NewInt(0)})
				}
				before := fmt.Sprint(ticks)
				r, e := WhirlpoolSwapExactIn(ClmmPool{n(c.Current), n(c.Liquidity), c.Tick, c.Spacing, c.Fee}, ticks, c.Starts, n(c.Amount).Uint64(), c.Timestamp, c.Down, adaptive, nil)
				err = e
				if fmt.Sprint(ticks) != before {
					t.Fatal("mutated ticks")
				}
				if e == nil {
					got = map[string]any{"consumed": fmt.Sprint(r.Consumed), "output": fmt.Sprint(r.AmountOut), "fee": fmt.Sprint(r.Fee), "minimum_fee": float64(r.MinimumFeeRate), "maximum_fee": float64(r.MaximumFeeRate)}
				}
			}
			if _, bad := v.Expected["error"]; bad {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			want := map[string]any{}
			for k, b := range v.Expected {
				var x any
				json.Unmarshal(b, &x)
				want[k] = x
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got%v (%v) want%v", got, err, want)
			}
		})
	}
}

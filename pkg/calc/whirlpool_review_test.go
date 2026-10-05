package calc

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestWhirlpoolFullStateReference(t *testing.T) {
	data, err := os.ReadFile("testdata/whirlpool_review_reference_20261005.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Name string
		Pool struct {
			Price, Liquidity string
			Tick             int32
			Spacing          uint16
			Fee              uint32
		}
		Ticks []struct {
			Tick int32
			Net  string
		}
		Starts        []int32
		Amount, Limit string
		Timestamp     uint64
		Down          bool
		Adaptive      *struct {
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
		Expected map[string]string
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	n := func(s string) *big.Int {
		v, ok := new(big.Int).SetString(s, 10)
		if !ok {
			t.Fatal(s)
		}
		return v
	}
	for _, v := range vectors {
		for _, order := range []string{"shuffled", "ascending", "descending"} {
			t.Run(v.Name+order, func(t *testing.T) {
				p := v.Pool
				pool := ClmmPool{n(p.Price), n(p.Liquidity), p.Tick, p.Spacing, p.Fee}
				ticks := []ClmmTick{}
				for _, x := range v.Ticks {
					ticks = append(ticks, ClmmTick{Tick: x.Tick, LiquidityNet: n(x.Net), LiquidityGross: big.NewInt(10)})
				}
				if order != "shuffled" {
					sort.Slice(ticks, func(i, j int) bool {
						if order == "ascending" {
							return ticks[i].Tick < ticks[j].Tick
						}
						return ticks[i].Tick > ticks[j].Tick
					})
				}
				before, _ := json.Marshal(ticks)
				var adaptive *WhirlpoolAdaptiveFee
				if a := v.Adaptive; a != nil {
					adaptive = &WhirlpoolAdaptiveFee{FilterPeriod: a.FilterPeriod, DecayPeriod: a.DecayPeriod, ReductionFactor: a.ReductionFactor, ControlFactor: a.ControlFactor, MaximumVolatility: a.MaximumVolatility, TickGroupSize: a.TickGroupSize, LastReferenceTimestamp: a.LastReferenceTimestamp, LastMajorSwapTimestamp: a.LastMajorSwapTimestamp, VolatilityReference: a.VolatilityReference, ReferenceGroup: a.ReferenceGroup, Volatility: a.Volatility}
				}
				r, err := WhirlpoolSwapExactIn(pool, ticks, v.Starts, n(v.Amount).Uint64(), v.Timestamp, v.Down, adaptive, n(v.Limit))
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]string{"consumed": fmt.Sprint(r.Consumed), "amount_out": fmt.Sprint(r.AmountOut), "fee": fmt.Sprint(r.Fee), "minimum_fee_rate": fmt.Sprint(r.MinimumFeeRate), "maximum_fee_rate": fmt.Sprint(r.MaximumFeeRate), "sqrt_price": r.SqrtPrice.String(), "tick_current": fmt.Sprint(r.TickCurrent), "liquidity": r.Liquidity.String()}
				if !reflect.DeepEqual(got, v.Expected) {
					t.Fatal(got, v.Expected)
				}
				after, _ := json.Marshal(ticks)
				if string(before) != string(after) {
					t.Fatal("mutated ticks")
				}
			})
		}
	}
}
func TestWhirlpoolArrayProtocolRange(t *testing.T) {
	price, _ := WhirlpoolSqrtPriceAtTick(0)
	pool := ClmmPool{price, big.NewInt(1000000), 0, 1, 3000}
	for _, start := range []int32{-443784, 443696, -2147483616, 2147483616} {
		if _, e := WhirlpoolSwapExactIn(pool, nil, []int32{start}, 100, 105, true, nil, nil); e == nil {
			t.Fatal("accepted out-of-range start", start)
		}
	}
}

func TestWhirlpoolSortedDuplicatesAndBoundaryArrays(t *testing.T) {
	price, _ := WhirlpoolSqrtPriceAtTick(0)
	pool := ClmmPool{price, big.NewInt(1000000), 0, 1, 3000}
	ticks := []ClmmTick{{Tick: 2, LiquidityNet: big.NewInt(0)}, {Tick: 0, LiquidityNet: big.NewInt(0)}, {Tick: 2, LiquidityNet: big.NewInt(0)}}
	if _, err := WhirlpoolSwapExactIn(pool, ticks, []int32{-88, 0}, 100, 105, true, nil, nil); err == nil {
		t.Fatal("duplicate tick accepted")
	}
	for _, c := range []struct {
		tick, start int32
		down        bool
	}{{-443635, -443696, true}, {443635, 443608, false}} {
		price, _ := WhirlpoolSqrtPriceAtTick(c.tick)
		pool := ClmmPool{price, big.NewInt(0), c.tick, 1, 3000}
		result, err := WhirlpoolSwapExactIn(pool, nil, []int32{c.start}, 100, 105, c.down, nil, nil)
		if err != nil || result.Consumed != 0 {
			t.Fatal("valid boundary array rejected", c, err)
		}
	}
}

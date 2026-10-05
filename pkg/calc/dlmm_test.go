package calc

import (
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"strconv"
	"testing"
)

type dlmmFixture struct {
	Active int32  `json:"active_id"`
	Step   uint16 `json:"bin_step"`
	Mode   uint8  `json:"fee_mode"`
	Static struct {
		Base      uint16 `json:"base_factor"`
		Power     uint8
		Control   uint32
		Max       uint32 `json:"maximum_volatility"`
		Filter    uint16 `json:"filter_period"`
		Decay     uint16 `json:"decay_period"`
		Reduction uint16 `json:"reduction_factor"`
	}
	Variable struct {
		Vol       uint32 `json:"volatility"`
		Reference uint32
		Index     int32  `json:"index_reference"`
		Time      string `json:"last_timestamp"`
	}
	Bins []struct {
		ID        int32  `json:"bin_id"`
		X         string `json:"amount_x"`
		Y         string `json:"amount_y"`
		Price     string
		Open      string `json:"open_order"`
		Processed string `json:"processed_order"`
		Ask       uint8  `json:"ask_side"`
	}
	Arrays                   []int64 `json:"loaded_arrays"`
	Amount, Timestamp        string
	Down, Orders, Exhaustive bool
	Expected                 struct {
		Out       string `json:"amount_out"`
		Remaining string `json:"remaining_in"`
		Crossed   uint32 `json:"bins_crossed"`
		Complete  bool
		Missing   *int32 `json:"missing_bin_id"`
		Error     bool
	}
}

func dlmmCases(t *testing.T) []dlmmFixture {
	t.Helper()
	b, e := os.ReadFile("testdata/dlmm_rust_5_0_6.json")
	if e != nil {
		t.Fatal(e)
	}
	var cs []dlmmFixture
	if e = json.Unmarshal(b, &cs); e != nil {
		t.Fatal(e)
	}
	return cs
}
func dlmmUint(t *testing.T, s string) uint64 {
	t.Helper()
	v, e := strconv.ParseUint(s, 10, 64)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func dlmmArgs(t *testing.T, c dlmmFixture) (DlmmPool, []DlmmBin, uint64, int64) {
	sp, vp := c.Static, c.Variable
	p := DlmmPool{c.Active, c.Step, c.Mode, DlmmStaticFee{sp.Base, sp.Power, sp.Control, sp.Max, sp.Filter, sp.Decay, sp.Reduction}, DlmmVariableFee{vp.Vol, vp.Reference, vp.Index, int64(dlmmUint(t, vp.Time))}}
	bins := []DlmmBin{}
	for _, b := range c.Bins {
		price, ok := new(big.Int).SetString(b.Price, 10)
		if !ok {
			t.Fatal("price")
		}
		bins = append(bins, DlmmBin{b.ID, dlmmUint(t, b.X), dlmmUint(t, b.Y), price, dlmmUint(t, b.Open), dlmmUint(t, b.Processed), b.Ask})
	}
	return p, bins, dlmmUint(t, c.Amount), int64(dlmmUint(t, c.Timestamp))
}
func TestDlmmRustVectors(t *testing.T) {
	for i, c := range dlmmCases(t) {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			p, bins, amount, ts := dlmmArgs(t, c)
			q, e := DlmmSwapExactIn(p, bins, c.Arrays, amount, ts, c.Down, c.Orders, true, c.Exhaustive)
			var partial *InsufficientDlmmArrays
			missing := errors.As(e, &partial)
			if e != nil && !missing {
				t.Fatal(e)
			}
			w := c.Expected
			if strconv.FormatUint(q.AmountOut, 10) != w.Out || strconv.FormatUint(q.RemainingIn, 10) != w.Remaining || q.BinsCrossed != w.Crossed || q.Complete != w.Complete || missing != w.Error {
				t.Fatal(q, w, e)
			}
			if (q.MissingBinID == nil) != (w.Missing == nil) || (q.MissingBinID != nil && *q.MissingBinID != *w.Missing) {
				t.Fatal("missing bin mismatch")
			}
		})
	}
}
func TestDlmmInvalid(t *testing.T) {
	cs := dlmmCases(t)
	var c dlmmFixture
	for _, x := range cs {
		if len(x.Bins) > 0 {
			c = x
			break
		}
	}
	for _, kind := range []string{"duplicate_array", "unloaded", "future_time", "zero_price", "duplicate_bin", "invalid_fee"} {
		t.Run(kind, func(t *testing.T) {
			p, bins, amount, ts := dlmmArgs(t, c)
			arrays := append([]int64{}, c.Arrays...)
			switch kind {
			case "duplicate_array":
				arrays = append(arrays, arrays[0])
			case "unloaded":
				arrays = nil
			case "future_time":
				ts = 0
			case "zero_price":
				bins = []DlmmBin{{BinID: bins[0].BinID, AmountX: 1, Price: new(big.Int)}}
			case "duplicate_bin":
				bins = append(bins, bins[0])
			case "invalid_fee":
				p.FeeMode = 2
			}
			_, e := DlmmSwapExactIn(p, bins, arrays, amount, ts, c.Down, c.Orders, true, c.Exhaustive)
			if e == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

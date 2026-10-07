package calc

import (
	"encoding/json"
	"math/big"
	"os"
	"strconv"
	"testing"
)

func TestPumpV3OfficialQuotes(t *testing.T) {
	raw, err := os.ReadFile("../../tests/fixtures/pump_upgrade/quotes.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Cases []struct {
			State                  map[string]string
			Mode, Amount, Expected string
		}
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.Cases {
		n := func(k string) uint64 {
			x, e := strconv.ParseUint(c.State[k], 10, 64)
			if e != nil {
				t.Fatal(e)
			}
			return x
		}
		s := PumpV3QuoteState{VirtualBase: n("virtual_base"), VirtualQuote: n("virtual_quote"), RemainingBase: n("remaining_base"), RealQuote: n("real_quote"), CurveBaseBalance: n("curve_base_balance"), MigrationFee: n("migration_fee"), ProtocolBps: n("protocol_bps"), CreatorBps: n("creator_bps")}
		a, _ := strconv.ParseUint(c.Amount, 10, 64)
		var q PumpV3Quote
		var e error
		if c.Mode == "out" {
			q, e = QuotePumpBuyV3ExactOut(s, a, false)
		} else {
			q, e = QuotePumpBuyV3ExactIn(s, a)
		}
		if e != nil {
			t.Fatal(e)
		}
		actual := q.BaseOut
		if c.Mode == "out" {
			actual = q.QuoteIn
		}
		want, _ := strconv.ParseUint(c.Expected, 10, 64)
		if actual != want {
			t.Fatalf("%s %d: got %d want %d", c.Mode, a, actual, want)
		}
		if c.Mode == "in" && q.QuoteIn > a {
			t.Fatal("exceeds budget")
		}
	}
}

func TestPumpChildInitialReserves(t *testing.T) {
	r, e := PumpCoinInitialQuoteReserves(123, 1000, big.NewInt(10), 1000, 100, 2000, 0, 1)
	if e != nil || r != 12300 {
		t.Fatal(r, e)
	}
	if _, e = PumpCoinInitialQuoteReserves(123, 1000, big.NewInt(10), 1000, 100, 100, 0, 1); e == nil {
		t.Fatal("supply cap ignored")
	}
}

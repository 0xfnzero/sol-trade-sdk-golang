package subscription

import (
	"encoding/base64"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestCachedHolderBank(t *testing.T) {
	for _, side := range []string{"buy", "sell"} {
		b, e := os.ReadFile("testdata/pump_holder_cached_" + side + "_20261009.json")
		if e != nil {
			t.Fatal(e)
		}
		var v struct {
			Accounts []struct {
				Pubkey, Owner, Data, Slot string
				Write                     string `json:"write_version"`
			}
			Legs []struct {
				Pool   string
				Input  string `json:"input_mint"`
				Output string `json:"output_mint"`
			}
			Payer, Amount, Epoch string
			Slot                 string `json:"read_slot"`
			Slippage             uint16 `json:"slippage_bps"`
			Expected             struct {
				Out     string `json:"estimated_out"`
				Minimum string `json:"minimum_out"`
			}
		}
		if e = json.Unmarshal(b, &v); e != nil {
			t.Fatal(e)
		}
		pk := solana.MustPublicKeyFromBase58
		n := func(s string) uint64 {
			v, e := strconv.ParseUint(s, 10, 64)
			if e != nil {
				t.Fatal(e)
			}
			return v
		}
		invoke := func(mut int) (PreparedPumpFun, error) {
			c := &SubscriptionAccountCache{}
			for _, a := range v.Accounts {
				data, e := base64.StdEncoding.DecodeString(a.Data)
				if e != nil {
					t.Fatal(e)
				}
				if a.Pubkey == v.Legs[0].Pool && mut >= 0 {
					if mut == 0 {
						copy(data[49:81], make([]byte, 32))
					} else {
						data[124] = byte(mut)
					}
				}
				if _, e = c.Update(pk(a.Pubkey), CachedAccount{Owner: pk(a.Owner), Data: data, Slot: n(a.Slot), WriteVersion: n(a.Write)}); e != nil {
					t.Fatal(e)
				}
			}
			h := v.Legs[0]
			return c.Snapshot().PreparePumpFun(PoolTradeHint{pk(h.Pool), pk(h.Input), pk(h.Output)}, CacheReadContext{Slot: n(v.Slot), Epoch: n(v.Epoch), MaximumSlotAge: 32}, pk(v.Payer), n(v.Amount), v.Slippage)
		}
		p, e := invoke(-1)
		if e != nil {
			t.Fatal(e)
		}
		if p.Quote.EstimatedNetAmountOut != n(v.Expected.Out) || p.Quote.MinimumNetAmountOut != n(v.Expected.Minimum) {
			t.Fatal("bank snapshot quote differs")
		}
		for _, mut := range []int{0, 2} {
			if _, e = invoke(mut); e == nil || !strings.Contains(e.Error(), "holder-reward") {
				t.Fatal("invalid holder accepted", e)
			}
		}
	}
}

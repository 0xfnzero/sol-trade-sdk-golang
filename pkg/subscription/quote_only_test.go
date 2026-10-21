package subscription

import (
	"github.com/gagliardetto/solana-go"
	"reflect"
	"testing"
)

func TestConcentratedQuoteOnlyParity(t *testing.T) {
	for _, name := range []string{"clmm", "dlmm", "whirlpool"} {
		for _, reverse := range []bool{false, true} {
			t.Run(name+map[bool]string{true: "/reverse", false: "/forward"}[reverse], func(t *testing.T) {
				v := clmmLoad(t, name)
				s := clmmSnapshot(t, v)
				key := solana.MustPublicKeyFromBase58
				h := PoolTradeHint{key(v.Pool), key(v.InputMint), key(v.OutputMint)}
				amount := uint64(10000)
				if reverse {
					h.InputMint, h.OutputMint = h.OutputMint, h.InputMint
					if name == "dlmm" {
						amount = 1000000
					}
				}
				ctx := CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}
				timestamp := clmmNumber(t, v.UnixTimestamp)
				payer := key(v.Payer)
				var expected, actual any
				var err, errorQuote error
				switch name {
				case "clmm":
					_, expected, _, err = s.PrepareClmm(h, ctx, timestamp, payer, amount, 100, 8)
					actual, errorQuote = s.QuoteClmm(h, ctx, timestamp, payer, amount, 100, 8)
				case "dlmm":
					_, expected, _, err = s.PrepareDlmm(h, ctx, timestamp, payer, amount, 100, 8)
					actual, errorQuote = s.QuoteDlmm(h, ctx, timestamp, payer, amount, 100, 8)
				case "whirlpool":
					_, expected, _, err = s.PrepareWhirlpool(h, ctx, timestamp, payer, amount, 100, 8)
					actual, errorQuote = s.QuoteWhirlpool(h, ctx, timestamp, payer, amount, 100, 8)
				}
				if err != nil || errorQuote != nil || !reflect.DeepEqual(expected, actual) {
					t.Fatal(expected, actual, err, errorQuote)
				}
			})
		}
	}
}

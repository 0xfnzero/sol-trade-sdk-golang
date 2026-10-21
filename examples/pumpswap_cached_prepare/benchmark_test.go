package main

import (
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"testing"
)

// Local cache preparation / transaction construction only. This measures neither
// network submission nor production end-to-end latency. Run with -benchmem.
func BenchmarkPumpSwapCached(b *testing.B) {
	for _, buy := range []bool{true, false} {
		name := "buy"
		if !buy {
			name = "sell"
		}
		b.Run(name, func(b *testing.B) {
			signer := solana.NewWallet()
			search, r, err := loadSnapshot("../fixtures/pumpswap_ttwo_route_accounts.json", signer.PublicKey(), buy)
			if err != nil {
				b.Fatal(err)
			}
			prepared, err := trading.PrepareBestPumpSwapTrade(search, r)
			if err != nil {
				b.Fatal(err)
			}
			signers := []solana.PrivateKey{signer.PrivateKey}
			wire, err := prepared.Trade.BuildTransaction(r.Context, signers)
			if err != nil {
				b.Fatal(err)
			}
			addresses := solana.PublicKeySlice{}
			for i := 0; i < int(wire[41]); i++ {
				addresses = append(addresses, solana.PublicKeyFromBytes(wire[42+32*i:74+32*i]))
			}
			options := trading.PumpSwapV0Options{AddressLookupTables: map[solana.PublicKey]solana.PublicKeySlice{{199}: addresses}}
			b.Run("prepare-search", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, e := trading.PrepareBestPumpSwapTrade(search, r); e != nil {
						b.Fatal(e)
					}
				}
			})
			b.Run("sign-v0-alt", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, e := prepared.Trade.BuildV0Transaction(r.Context, signers, options); e != nil {
						b.Fatal(e)
					}
				}
			})
			b.Run("sign-v1", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, e := prepared.Trade.BuildTransaction(r.Context, signers); e != nil {
						b.Fatal(e)
					}
				}
			})
		})
	}
}

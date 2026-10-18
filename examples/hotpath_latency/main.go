// go run ./examples/hotpath_latency [config.json]; local cache/CPU work, no RPC.
package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/latency"
	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/pumpswapreplay"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"os"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	path := "examples/hotpath_latency/config.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	config, e := latency.Load(path)
	if e != nil {
		return e
	}
	// Public deterministic local test key: identical PDA work in every run.
	key := solana.PrivateKey(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{117}, 32)))
	signers := []solana.PrivateKey{key}
	results := []latency.Result{}
	for _, buy := range []bool{true, false} {
		name := "buy"
		if !buy {
			name = "sell"
		}
		search, r, e := pumpswapreplay.LoadSnapshot("examples/fixtures/pumpswap_ttwo_route_accounts.json", key.PublicKey(), buy)
		if e != nil {
			return e
		}
		prepared, e := trading.PrepareBestPumpSwapTrade(search, r)
		if e != nil {
			return e
		}
		original, e := prepared.Trade.BuildTransaction(r.Context, signers)
		if e != nil {
			return e
		}
		addresses := solana.PublicKeySlice{}
		for i := 0; i < int(original[41]); i++ {
			addresses = append(addresses, solana.PublicKeyFromBytes(original[42+32*i:74+32*i]))
		}
		options := trading.PumpSwapV0Options{AddressLookupTables: map[solana.PublicKey]solana.PublicKeySlice{{199}: addresses}}
		operations := []struct {
			name string
			work func() error
		}{
			{"prepare-search/" + name, func() error {
				p, e := trading.PrepareBestPumpSwapTrade(search, r)
				if e == nil && p.Trade.MinimumOutput() != prepared.Trade.MinimumOutput() {
					return fmt.Errorf("protected output changed")
				}
				return e
			}},
			{"sign-v0-alt/" + name, func() error {
				wire, e := prepared.Trade.BuildV0Transaction(r.Context, signers, options)
				if e == nil && len(wire) == 0 {
					return fmt.Errorf("empty signed transaction")
				}
				return e
			}},
			{"sign-v1/" + name, func() error {
				wire, e := prepared.Trade.BuildTransaction(r.Context, signers)
				if e == nil && len(wire) != len(original) {
					return fmt.Errorf("signed wire size changed")
				}
				return e
			}},
		}
		for _, operation := range operations {
			result, e := latency.Measure(operation.name, config, operation.work)
			if e != nil {
				return e
			}
			results = append(results, result)
		}
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Config   latency.Config   `json:"config"`
		Results  []latency.Result `json:"results"`
		RPCCalls int              `json:"rpc_calls"`
	}{config, results, 0})
}

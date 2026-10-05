// go run ./examples/cached_damm_v2 snapshot.json [--simulate]
// Frozen state -> caller's explicit swap2 threshold -> unsigned V1; no autoquote/send.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
	sdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/subscription"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"os"
	"strconv"
)

type input struct {
	Payer, Amount, Epoch string
	TradeType            string `json:"trade_type"`
	NativeInput          bool   `json:"native_input"`
	NativeOutput         bool   `json:"native_output"`
	TemporaryWsolSeed    string `json:"temporary_wsol_seed"`
	RentLamports         string `json:"rent_lamports"`
	Minimum              string `json:"fixed_output_amount"`
	ReadSlot             string `json:"read_slot"`
	Age                  string `json:"maximum_slot_age"`
	Time                 string `json:"unix_timestamp"`
	Hash                 string `json:"recent_blockhash"`
	Legs                 []struct {
		Pool   string
		Input  string `json:"input_mint"`
		Output string `json:"output_mint"`
	}
	Accounts []struct {
		Pubkey, Owner, Data, Slot string
		Version                   string `json:"write_version"`
	}
}

func number(s string) uint64 {
	n, e := strconv.ParseUint(s, 10, 64)
	if e != nil {
		panic(e)
	}
	return n
}
func build(v input) ([]byte, error) {
	cache := &subscription.SubscriptionAccountCache{}
	for _, a := range v.Accounts {
		d, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			return nil, e
		}
		if _, e = cache.Update(solana.MustPublicKeyFromBase58(a.Pubkey), subscription.CachedAccount{Owner: solana.MustPublicKeyFromBase58(a.Owner), Data: d, Slot: number(a.Slot), WriteVersion: number(a.Version)}); e != nil {
			return nil, e
		}
	}
	if len(v.Legs) != 1 {
		return nil, fmt.Errorf("one independent DAMM v2 swap required")
	}
	l := v.Legs[0]
	h := subscription.PoolTradeHint{Pool: solana.MustPublicKeyFromBase58(l.Pool), InputMint: solana.MustPublicKeyFromBase58(l.Input), OutputMint: solana.MustPublicKeyFromBase58(l.Output)}
	direction := sdk.TradeTypeBuy
	if v.TradeType == "Sell" {
		direction = sdk.TradeTypeSell
	} else if v.TradeType != "" && v.TradeType != "Buy" {
		return nil, fmt.Errorf("independent Buy or Sell required")
	}
	rent := uint64(0)
	if v.RentLamports != "" {
		rent = number(v.RentLamports)
	}
	minimum := number(v.Minimum)
	prepared, e := trading.PrepareCachedTrade(trading.CachedTradeRequest{DexType: sdk.DexTypeMeteoraDammV2, TradeType: direction, Snapshot: cache.Snapshot(), Hints: []subscription.PoolTradeHint{h}, Context: subscription.CacheReadContext{Slot: number(v.ReadSlot), Epoch: number(v.Epoch), MaximumSlotAge: number(v.Age)}, UnixTimestamp: number(v.Time), Payer: solana.MustPublicKeyFromBase58(v.Payer), Amount: number(v.Amount), FixedOutputAmount: &minimum, RecentBlockhash: solana.MustHashFromBase58(v.Hash), NativeInput: v.NativeInput, NativeOutput: v.NativeOutput, TemporaryWsolSeed: v.TemporaryWsolSeed, RentLamports: rent})
	if e != nil {
		return nil, e
	}
	message := prepared.Compiled
	return append(message.Message, make([]byte, 64*int(message.RequiredSignatures))...), nil
}
func main() {
	if len(os.Args) < 2 {
		panic("provide snapshot JSON")
	}
	d, e := os.ReadFile(os.Args[1])
	if e != nil {
		panic(e)
	}
	var v input
	if e = json.Unmarshal(d, &v); e != nil {
		panic(e)
	}
	wire, e := build(v)
	if e != nil {
		panic(e)
	}
	encoded := base64.StdEncoding.EncodeToString(wire)
	out, _ := json.Marshal(map[string]any{"transaction": encoded, "wire_bytes": len(wire)})
	fmt.Println(string(out))
	if e := exampleutil.SimulateIfRequested(os.Args[2:], wire, number(v.ReadSlot)); e != nil {
		panic(e)
	}
}

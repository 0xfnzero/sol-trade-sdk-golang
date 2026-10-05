// go run ./examples/damm_v2_snapshot snapshot.json [--simulate]
// Historical wire: WSOL input uses the legacy SOL-funding helper.
// Use examples/cached_damm_v2 for explicit SOL/WSOL endpoints.
// Frozen state -> caller's explicit swap2 threshold -> unsigned V1; no autoquote/send.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/serialization"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/subscription"
	"github.com/gagliardetto/solana-go"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type input struct {
	Payer, Amount, Epoch string
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
	state, e := cache.Snapshot().DammV2(h, subscription.CacheReadContext{Slot: number(v.ReadSlot), Epoch: number(v.Epoch), MaximumSlotAge: number(v.Age)}, number(v.Time))
	if e != nil {
		return nil, e
	}
	for _, f := range state.TransferFees {
		if f.BasisPoints != 0 && f.MaximumFee != 0 {
			return nil, fmt.Errorf("nonzero transfer-fee thresholds are not verified by this example")
		}
	}
	minimum := number(v.Minimum)
	if minimum == 0 {
		return nil, fmt.Errorf("explicit positive u64 minimum required")
	}
	p := state.Pool
	payer := solana.MustPublicKeyFromBase58(v.Payer)
	mode := uint8(0)
	ixs, e := instruction.MeteoraDammV2BuildBuyInstructions(&instruction.MeteoraDammV2BuildBuyParams{Payer: payer, InputMint: h.InputMint, OutputMint: h.OutputMint, InputAmount: number(v.Amount), FixedOutputAmount: &minimum, CreateInputMintAta: true, CreateOutputMintAta: true, ProtocolParams: &instruction.MeteoraDammV2Params{Pool: h.Pool, TokenAMint: p.TokenAMint, TokenBMint: p.TokenBMint, TokenAVault: p.TokenAVault, TokenBVault: p.TokenBVault, TokenAProgram: state.TokenAProgram, TokenBProgram: state.TokenBProgram, SwapMode: &mode, IncludeRateLimiterSysvar: p.PoolFees.BaseFee.FeeSchedulerMode == 2}})
	if e != nil {
		return nil, e
	}
	compute, loaded := uint32(300000), uint32(64*1024*1024)
	message, e := serialization.CompileV1Message(payer, ixs, solana.MustHashFromBase58(v.Hash), serialization.V1Config{ComputeUnitLimit: &compute, LoadedAccountsDataSizeLimit: &loaded})
	if e != nil {
		return nil, e
	}
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
	if len(os.Args) > 2 && os.Args[2] == "--simulate" {
		url := os.Getenv("RPC_URL")
		if url == "" {
			url = "https://api.mainnet-beta.solana.com"
		}
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "simulateTransaction", "params": []any{encoded, map[string]any{"encoding": "base64", "sigVerify": false, "replaceRecentBlockhash": true, "innerInstructions": true, "commitment": "confirmed"}}})
		client := &http.Client{Timeout: 60 * time.Second}
		r, e := client.Post(url, "application/json", bytes.NewReader(body))
		if e != nil {
			panic(e)
		}
		defer r.Body.Close()
		d, e := io.ReadAll(r.Body)
		if e != nil {
			panic(e)
		}
		fmt.Println(string(d))
		if r.StatusCode != 200 {
			panic("simulation HTTP failure")
		}
		var result struct {
			Error  any
			Result struct{ Value struct{ Err json.RawMessage } }
		}
		if e = json.Unmarshal(d, &result); e != nil || result.Error != nil || string(result.Result.Value.Err) != "null" {
			panic("simulation failed")
		}
	}
}

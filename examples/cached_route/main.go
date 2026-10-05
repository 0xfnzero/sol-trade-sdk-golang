// go run ./examples/cached_route snapshots.json [--simulate]
// Same JSON schema as Node/Python. Quote/build are entirely local.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
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

type snapshotAccount struct {
	Pubkey, Owner, Data string
	Slot                string
	WriteVersion        string `json:"write_version"`
}
type input struct {
	NativeInput       bool    `json:"native_input"`
	NativeOutput      bool    `json:"native_output"`
	RentLamports      string  `json:"rent_lamports"`
	TemporaryWsolSeed string  `json:"temporary_wsol_seed"`
	ReadSlot          string  `json:"read_slot"`
	MaximumSlotAge    *uint64 `json:"maximum_slot_age"`
	Payer             string
	RecentBlockhash   string `json:"recent_blockhash"`
	Epoch             string
	UnixTimestamp     string `json:"unix_timestamp"`
	Amount            string
	SlippageBps       uint16 `json:"slippage_bps"`
	MaximumArrays     *int   `json:"maximum_arrays"`
	Legs              []struct {
		Pool       string
		InputMint  string `json:"input_mint"`
		OutputMint string `json:"output_mint"`
	}
	Accounts []snapshotAccount
}

func account(a snapshotAccount) instruction.LaunchLabAccountBytes {
	data, e := base64.StdEncoding.DecodeString(a.Data)
	if e != nil {
		panic(e)
	}
	return instruction.LaunchLabAccountBytes{Pubkey: solana.MustPublicKeyFromBase58(a.Pubkey), Owner: solana.MustPublicKeyFromBase58(a.Owner), Data: data}
}
func number(s string) uint64 {
	n, e := strconv.ParseUint(s, 10, 64)
	if e != nil {
		panic(e)
	}
	return n
}
func ata(payer, mint, program solana.PublicKey) solana.Instruction {
	ix, _, err := common.BuildCreateIdempotentATA(payer, payer, mint, program)
	if err != nil {
		panic(err)
	}
	return ix
}

func main() {
	if len(os.Args) < 2 {
		panic("provide subscription snapshots JSON")
	}
	data, e := os.ReadFile(os.Args[1])
	if e != nil {
		panic(e)
	}
	var in input
	if e = json.Unmarshal(data, &in); e != nil {
		panic(e)
	}

	cache := &subscription.SubscriptionAccountCache{}
	for _, value := range in.Accounts {
		a := account(value)
		if _, e = cache.Update(a.Pubkey, subscription.CachedAccount{Owner: a.Owner, Data: a.Data, Slot: number(value.Slot), WriteVersion: number(value.WriteVersion)}); e != nil {
			panic(e)
		}
	}
	maxAge := uint64(32)
	if in.MaximumSlotAge != nil {
		maxAge = *in.MaximumSlotAge
	}
	arrays := 8
	if in.MaximumArrays != nil {
		arrays = *in.MaximumArrays
	}
	ctx := subscription.CacheReadContext{Slot: number(in.ReadSlot), Epoch: number(in.Epoch), MaximumSlotAge: maxAge}
	payer := solana.MustPublicKeyFromBase58(in.Payer)
	hints := make([]subscription.PoolTradeHint, len(in.Legs))
	for i, h := range in.Legs {
		hints[i] = subscription.PoolTradeHint{Pool: solana.MustPublicKeyFromBase58(h.Pool), InputMint: solana.MustPublicKeyFromBase58(h.InputMint), OutputMint: solana.MustPublicKeyFromBase58(h.OutputMint)}
	}
	route, e := cache.Snapshot().PrepareRoute(hints, ctx, number(in.UnixTimestamp), payer, number(in.Amount), in.SlippageBps, arrays)
	if e != nil {
		panic(e)
	}
	instructions := append(append([]solana.Instruction{}, route.SetupInstructions...), route.SwapInstructions...)

	if in.NativeInput || in.NativeOutput {
		settled, err := subscription.SettleCachedRouteWithNativeSol(route, payer, in.TemporaryWsolSeed, number(in.RentLamports), in.NativeInput, in.NativeOutput)
		if err != nil {
			panic(err)
		}
		instructions = settled.Instructions
	}
	compute := uint32(300000)
	loaded := uint32(64 * 1024 * 1024)
	compiled, e := serialization.CompileV1Message(payer, instructions, solana.MustHashFromBase58(in.RecentBlockhash), serialization.V1Config{ComputeUnitLimit: &compute, LoadedAccountsDataSizeLimit: &loaded})
	if e != nil {
		panic(e)
	}
	raw := append(append([]byte{}, compiled.Message...), make([]byte, compiled.RequiredSignatures*64)...)
	encoded := base64.StdEncoding.EncodeToString(raw)
	legs := []map[string]any{}
	for _, l := range route.Legs {
		var estimated any
		if l.EstimatedNetAmountOut != nil {
			estimated = strconv.FormatUint(*l.EstimatedNetAmountOut, 10)
		}
		legs = append(legs, map[string]any{"amount_in": strconv.FormatUint(l.AmountIn, 10), "amount_out": estimated, "minimum_amount_out": strconv.FormatUint(l.MinimumNetAmountOut, 10)})
	}
	residuals := []map[string]string{}
	for _, r := range route.EstimatedIntermediateResiduals {
		residuals = append(residuals, map[string]string{"mint": r.Mint.String(), "amount": strconv.FormatUint(r.Amount, 10)})
	}
	out, _ := json.Marshal(map[string]any{"minimum_amount_out": strconv.FormatUint(route.MinimumNetAmountOut, 10), "legs": legs, "intermediate_residuals": residuals, "wire_bytes": len(raw), "transaction": encoded})
	fmt.Println(string(out))
	if len(os.Args) > 2 && os.Args[2] == "--simulate" {
		endpoint := os.Getenv("RPC_URL")
		if endpoint == "" {
			endpoint = "https://api.mainnet-beta.solana.com"
		}
		payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "simulateTransaction", "params": []any{encoded, map[string]any{"encoding": "base64", "sigVerify": false, "replaceRecentBlockhash": true, "commitment": "confirmed"}}})
		client := http.Client{Timeout: 30 * time.Second}
		response, e := client.Post(endpoint, "application/json", bytes.NewReader(payload))
		if e != nil {
			panic(e)
		}
		defer response.Body.Close()
		body, e := io.ReadAll(response.Body)
		if e != nil {
			panic(e)
		}
		fmt.Println(string(body))
		if response.StatusCode != 200 {
			panic("simulation HTTP failure")
		}
		var result struct {
			Error  json.RawMessage
			Result struct{ Value struct{ Err json.RawMessage } }
		}
		if e = json.Unmarshal(body, &result); e != nil {
			panic(e)
		}
		if len(result.Error) > 0 && string(result.Error) != "null" || len(result.Result.Value.Err) > 0 && string(result.Result.Value.Err) != "null" {
			panic("simulation failed; inspect returned logs")
		}
	}
}

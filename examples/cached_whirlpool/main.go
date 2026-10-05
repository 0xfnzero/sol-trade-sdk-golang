// go run ./examples/cached_whirlpool snapshots.json [--simulate]
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
	ReadSlot        string  `json:"read_slot"`
	MaximumSlotAge  *uint64 `json:"maximum_slot_age"`
	Payer           string
	RecentBlockhash string `json:"recent_blockhash"`
	Epoch           string
	UnixTimestamp   string `json:"unix_timestamp"`
	Amount          string
	SlippageBps     uint16 `json:"slippage_bps"`
	MaximumArrays   *int   `json:"maximum_arrays"`
	Pool            string
	InputMint       string `json:"input_mint"`
	OutputMint      string `json:"output_mint"`
	Accounts        []snapshotAccount
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
	hint := subscription.PoolTradeHint{Pool: solana.MustPublicKeyFromBase58(in.Pool), InputMint: solana.MustPublicKeyFromBase58(in.InputMint), OutputMint: solana.MustPublicKeyFromBase58(in.OutputMint)}
	snapshot := cache.Snapshot()
	_, result, swap, e := snapshot.PrepareWhirlpool(hint, ctx, number(in.UnixTimestamp), payer, number(in.Amount), in.SlippageBps, arrays)
	if e != nil {
		panic(e)
	}
	im, e := snapshot.Get(hint.InputMint, ctx, nil)
	if e != nil {
		panic(e)
	}
	om, e := snapshot.Get(hint.OutputMint, ctx, nil)
	if e != nil {
		panic(e)
	}

	compute := uint32(300000)
	loaded := uint32(64 * 1024 * 1024)
	compiled, e := serialization.CompileV1Message(payer, []solana.Instruction{ata(payer, hint.InputMint, im.Owner), ata(payer, hint.OutputMint, om.Owner), swap}, solana.MustHashFromBase58(in.RecentBlockhash), serialization.V1Config{ComputeUnitLimit: &compute, LoadedAccountsDataSizeLimit: &loaded})
	if e != nil {
		panic(e)
	}
	raw := append(append([]byte{}, compiled.Message...), make([]byte, compiled.RequiredSignatures*64)...)
	encoded := base64.StdEncoding.EncodeToString(raw)
	out, _ := json.Marshal(map[string]any{"amount_in": strconv.FormatUint(result.AmountIn, 10), "amount_out": strconv.FormatUint(result.EstimatedNetAmountOut, 10), "minimum_amount_out": strconv.FormatUint(result.MinimumAmountOut, 10), "version": 1, "wire_bytes": len(raw), "transaction": encoded})
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

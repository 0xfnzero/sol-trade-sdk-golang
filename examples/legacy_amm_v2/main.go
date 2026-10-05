// go run ./examples/legacy_amm_v2 snapshot.json [--simulate] [--exact-output=N]
// Buy wraps SOL in payer WSOL ATA; sell receives WSOL. Never closes existing ATAs.
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
	"slices"
	"strconv"
	"strings"
	"time"
)

type snapshotAccount struct {
	Pubkey, Owner, Data string
	Slot                string
	WriteVersion        string `json:"write_version"`
}
type input struct {
	DexType           string  `json:"dex_type"`
	TradeType         string  `json:"trade_type"`
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

	ctx := subscription.CacheReadContext{Slot: number(in.ReadSlot), Epoch: number(in.Epoch), MaximumSlotAge: maxAge}
	if len(in.Legs) != 1 {
		panic("single AMM pool required; use cached_trade for routes")
	}
	h := in.Legs[0]
	hint := subscription.PoolTradeHint{Pool: solana.MustPublicKeyFromBase58(h.Pool), InputMint: solana.MustPublicKeyFromBase58(h.InputMint), OutputMint: solana.MustPublicKeyFromBase58(h.OutputMint)}
	p, e := cache.Snapshot().AmmV4(hint, ctx, number(in.UnixTimestamp))
	if e != nil {
		panic(e)
	}
	pp := &instruction.RaydiumAmmV4Params{Amm: p.Pool, CoinMint: p.CoinMint, PcMint: p.PcMint, TokenCoin: p.CoinVault, TokenPc: p.PcVault, CoinReserve: p.CoinReserve, PcReserve: p.PcReserve, SwapFeeNumerator: &p.SwapFeeNumerator, SwapFeeDenominator: &p.SwapFeeDenominator}
	payer := solana.MustPublicKeyFromBase58(in.Payer)
	var fixed *uint64
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "--exact-output=") {
			n := number(strings.TrimPrefix(arg, "--exact-output="))
			fixed = &n
		}
	}
	var ixs []solana.Instruction
	switch in.TradeType {
	case "Buy":
		ixs, e = instruction.RaydiumAmmV4BuildBuyInstructions(&instruction.RaydiumAmmV4BuildBuyParams{Payer: payer, InputMint: hint.InputMint, OutputMint: hint.OutputMint, InputAmount: number(in.Amount), SlippageBasisPoints: uint64(in.SlippageBps), ProtocolParams: pp, CreateInputMintAta: in.NativeInput, CreateOutputMintAta: true, FixedOutputAmount: fixed})
	case "Sell":
		ixs, e = instruction.RaydiumAmmV4BuildSellInstructions(&instruction.RaydiumAmmV4BuildSellParams{Payer: payer, InputMint: hint.InputMint, OutputMint: hint.OutputMint, InputAmount: number(in.Amount), SlippageBasisPoints: uint64(in.SlippageBps), ProtocolParams: pp, CreateOutputMintAta: true, FixedOutputAmount: fixed})
	default:
		panic("explicit Buy/Sell required")
	}
	if e != nil {
		panic(e)
	}
	compute, loaded := uint32(300000), uint32(64*1024*1024)
	compiled, e := serialization.CompileV1Message(payer, ixs, solana.MustHashFromBase58(in.RecentBlockhash), serialization.V1Config{ComputeUnitLimit: &compute, LoadedAccountsDataSizeLimit: &loaded})
	if e != nil {
		panic(e)
	}
	raw := append(append([]byte{}, compiled.Message...), make([]byte, compiled.RequiredSignatures*64)...)
	encoded := base64.StdEncoding.EncodeToString(raw)
	out, _ := json.Marshal(map[string]any{"wire_bytes": len(raw), "transaction": encoded})
	fmt.Println(string(out))
	if slices.Contains(os.Args[2:], "--simulate") {
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

// go run ./examples/cached_trade snapshots.json [--simulate]
// Same JSON schema as Node/Python. Quote/build are entirely local.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/subscription"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"os"
	"strconv"
)

type snapshotAccount struct {
	Pubkey, Owner, Data string
	Slot                string
	WriteVersion        string `json:"write_version"`
}
type routeHintInput struct {
	Pool       string
	InputMint  string `json:"input_mint"`
	OutputMint string `json:"output_mint"`
}

type input struct {
	FixedOutputAmount *string         `json:"fixed_output_amount"`
	DexType           string          `json:"dex_type"`
	TradeType         string          `json:"trade_type"`
	NativeInput       bool            `json:"native_input"`
	NativeOutput      bool            `json:"native_output"`
	RentLamports      string          `json:"rent_lamports"`
	TipAccount        string          `json:"tip_account"`
	TipLamports       string          `json:"tip_lamports"`
	TemporaryWsolSeed string          `json:"temporary_wsol_seed"`
	ReadSlot          string          `json:"read_slot"`
	MaximumSlotAge    json.RawMessage `json:"maximum_slot_age"`
	Payer             string
	RecentBlockhash   string `json:"recent_blockhash"`
	Epoch             string
	UnixTimestamp     string `json:"unix_timestamp"`
	Amount            string
	SlippageBps       uint16 `json:"slippage_bps"`
	MaximumArrays     *int   `json:"maximum_arrays"`
	Legs              []routeHintInput
	Candidates        []routeHintInput
	InputMint         string `json:"input_mint"`
	OutputMint        string `json:"output_mint"`
	Accounts          []snapshotAccount
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
		maxAge, e = jsonU64(in.MaximumSlotAge)
		if e != nil {
			panic(e)
		}
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
	candidates := make([]subscription.PoolTradeHint, len(in.Candidates))
	for i, h := range in.Candidates {
		candidates[i] = subscription.PoolTradeHint{Pool: solana.MustPublicKeyFromBase58(h.Pool), InputMint: solana.MustPublicKeyFromBase58(h.InputMint), OutputMint: solana.MustPublicKeyFromBase58(h.OutputMint)}
	}
	var inputMint, outputMint solana.PublicKey
	if in.InputMint != "" {
		inputMint = solana.MustPublicKeyFromBase58(in.InputMint)
	}
	if in.OutputMint != "" {
		outputMint = solana.MustPublicKeyFromBase58(in.OutputMint)
	}

	dex, ok := map[string]soltradesdk.DexType{"PumpFun": soltradesdk.DexTypePumpFun, "MeteoraDammV2": soltradesdk.DexTypeMeteoraDammV2, "PumpSwap": soltradesdk.DexTypePumpSwap, "RaydiumAmmV4": soltradesdk.DexTypeRaydiumAmmV4, "LaunchLab": soltradesdk.DexTypeLaunchLab, "Bonk": soltradesdk.DexTypeBonk, "StonkFun": soltradesdk.DexTypeStonkFun, "RaydiumCpmm": soltradesdk.DexTypeRaydiumCpmm, "RaydiumClmm": soltradesdk.DexTypeRaydiumClmm, "OrcaWhirlpool": soltradesdk.DexTypeOrcaWhirlpool, "MeteoraDlmm": soltradesdk.DexTypeMeteoraDlmm}[in.DexType]
	if !ok {
		panic("explicit supported dex_type required")
	}
	direction := soltradesdk.TradeTypeBuy
	switch in.TradeType {
	case "Buy":
	case "Sell":
		direction = soltradesdk.TradeTypeSell
	default:
		panic("explicit Buy/Sell trade_type required")
	}
	factory := trading.NewTradeExecutorFactory(nil)
	executor, err := factory.CreateCachedExecutor(dex)
	if err != nil {
		panic(err)
	}
	rent := uint64(0)
	if in.RentLamports != "" {
		rent = number(in.RentLamports)
	}
	var tipAccount solana.PublicKey
	if in.TipAccount != "" {
		tipAccount = solana.MustPublicKeyFromBase58(in.TipAccount)
	}
	tipLamports := uint64(0)
	if in.TipLamports != "" {
		tipLamports = number(in.TipLamports)
	}
	var minimum *uint64
	if in.FixedOutputAmount != nil {
		value := number(*in.FixedOutputAmount)
		minimum = &value
	}
	prepared, err := executor.Prepare(trading.CachedTradeRequest{DexType: dex, TradeType: direction, Snapshot: cache.Snapshot(), Hints: hints, Candidates: candidates, InputMint: inputMint, OutputMint: outputMint, Context: ctx, UnixTimestamp: number(in.UnixTimestamp), Payer: payer, Amount: number(in.Amount), FixedOutputAmount: minimum, RecentBlockhash: solana.MustHashFromBase58(in.RecentBlockhash), SlippageBps: &in.SlippageBps, MaximumArrays: arrays, NativeInput: in.NativeInput, NativeOutput: in.NativeOutput, TemporaryWsolSeed: in.TemporaryWsolSeed, RentLamports: rent, TipAccount: tipAccount, TipLamports: tipLamports})
	if err != nil {
		panic(err)
	}
	route, compiled := prepared.Route, prepared.Compiled
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
	out, _ := json.Marshal(map[string]any{"minimum_amount_out": strconv.FormatUint(route.MinimumNetAmountOut, 10), "legs": legs, "intermediate_residuals": residuals, "estimated_native_residual_lamports": strconv.FormatUint(prepared.EstimatedNativeResidualLamports, 10), "wire_bytes": len(raw), "transaction": encoded})
	fmt.Println(string(out))
	if e := exampleutil.SimulateIfRequested(os.Args[2:], raw, number(in.ReadSlot)); e != nil {
		panic(e)
	}
}

// jsonU64 accepts the same exact decimal string/integer representation as the other examples.
func jsonU64(raw json.RawMessage) (uint64, error) {
	value := string(raw)
	if len(raw) > 0 && raw[0] == '"' {
		if e := json.Unmarshal(raw, &value); e != nil {
			return 0, e
		}
	}
	if len(value) == 0 {
		return 0, fmt.Errorf("empty u64 value")
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("u64 requires unsigned decimal digits")
		}
	}
	return strconv.ParseUint(value, 10, 64)
}

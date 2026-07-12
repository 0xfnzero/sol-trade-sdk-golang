package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
)

// ParsedPoolEvent is the small contract that sol-parser-sdk output should be
// adapted to inside the solana-streamer callback.
type ParsedPoolEvent struct {
	ReceivedAt   time.Time
	DexType      soltradesdk.DexType
	Mint         solana.PublicKey
	Pool         solana.PublicKey
	TokenProgram solana.PublicKey
	BuyState     interface{}
}

type TradeExecutor interface {
	Buy(context.Context, soltradesdk.TradeBuyParams) (*soltradesdk.TradeResult, error)
	Sell(context.Context, soltradesdk.TradeSellParams) (*soltradesdk.TradeResult, error)
}

type LiveAdapters interface {
	TokenBalance(context.Context, solana.PublicKey, solana.PublicKey, solana.PublicKey) (uint64, error)
	LatestBlockhash(context.Context) (solana.Hash, error)
	RefreshSellState(context.Context, ParsedPoolEvent) (interface{}, error)
}

func envUint64(name string, fallback uint64) uint64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		log.Fatalf("invalid %s: %v", name, err)
	}
	return parsed
}

func HandleParsedEvent(
	ctx context.Context,
	executor TradeExecutor,
	adapters LiveAdapters,
	owner solana.PublicKey,
	event ParsedPoolEvent,
	targetMint, targetPool *solana.PublicKey,
) error {
	maxAge := time.Duration(envUint64("MAX_EVENT_AGE_MS", 500)) * time.Millisecond
	inputAmount := envUint64("INPUT_AMOUNT", 100_000)
	slippage := envUint64("SLIPPAGE_BPS", 300)
	if !exampleutil.IsEventFresh(event.ReceivedAt, maxAge, time.Now()) {
		return nil
	}
	if !exampleutil.MatchesTarget(event.Mint, targetMint) || !exampleutil.MatchesTarget(event.Pool, targetPool) {
		return nil
	}
	if err := exampleutil.ValidateTradeIntent(inputAmount, slippage, nil); err != nil {
		return err
	}

	before, err := adapters.TokenBalance(ctx, owner, event.Mint, event.TokenProgram)
	if err != nil {
		return err
	}
	buyBlockhash, err := adapters.LatestBlockhash(ctx)
	if err != nil {
		return err
	}
	buy := soltradesdk.TradeBuyParams{
		DexType: event.DexType, InputTokenType: soltradesdk.TradeTokenTypeWSOL,
		Mint: event.Mint, InputTokenAmount: inputAmount, SlippageBasisPoints: slippage,
		RecentBlockhash: &buyBlockhash, ExtensionParams: event.BuyState, WaitTxConfirmed: true,
	}
	bought, err := executor.Buy(ctx, buy)
	if err != nil {
		return err
	}
	if bought == nil || !bought.Success {
		return errors.New("buy was not confirmed")
	}

	after, err := adapters.TokenBalance(ctx, owner, event.Mint, event.TokenProgram)
	if err != nil {
		return err
	}
	acquired, err := exampleutil.CheckedPositionDelta(before, after)
	if err != nil {
		return err
	}

	// Confirmation changes both the pool state and usable blockhash window.
	sellState, err := adapters.RefreshSellState(ctx, event)
	if err != nil {
		return err
	}
	sellBlockhash, err := adapters.LatestBlockhash(ctx)
	if err != nil {
		return err
	}
	sell := soltradesdk.TradeSellParams{
		DexType: event.DexType, OutputTokenType: soltradesdk.TradeTokenTypeWSOL,
		Mint: event.Mint, InputTokenAmount: acquired, SlippageBasisPoints: slippage,
		RecentBlockhash: &sellBlockhash, ExtensionParams: sellState, WaitTxConfirmed: true,
	}
	sold, err := executor.Sell(ctx, sell)
	if err != nil {
		return err
	}
	if sold == nil || !sold.Success {
		return errors.New("sell failed")
	}
	return nil
}

func main() {
	ctx := context.Background()
	client, err := exampleutil.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Bot adapter ready for wallet:", client.GetPayer())
	fmt.Println("Register HandleParsedEvent in solana-streamer and provide an instruction-builder execution adapter.")
	fmt.Println("The root TradingClient and protocol factory intentionally do not submit protocol trades.")
}

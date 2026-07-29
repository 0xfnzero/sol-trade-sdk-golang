<div align="center">
    <h1>🚀 Sol Trade SDK for Go</h1>
    <h3><em>A comprehensive Go SDK for seamless Solana DEX trading</em></h3>
</div>

<p align="center">
    <strong>A high-performance Go SDK for low-latency Solana DEX trading bots. Built for speed and efficiency, it enables seamless, high-throughput interaction with PumpFun, Pump AMM (PumpSwap), Bonk, Meteora DAMM v2, Raydium AMM v4, and Raydium CPMM for latency-critical trading strategies.</strong>
</p>

<p align="center">
    <a href="https://pkg.go.dev/github.com/0xfnzero/sol-trade-sdk-golang">
        <img src="https://pkg.go.dev/badge/github.com/0xfnzero/sol-trade-sdk-golang.svg" alt="Go Reference">
    </a>
    <a href="LICENSE">
        <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License">
    </a>
</p>

<p align="center">
    <img src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
    <img src="https://img.shields.io/badge/Solana-9945FF?style=for-the-badge&logo=solana&logoColor=white" alt="Solana">
    <img src="https://img.shields.io/badge/DEX-4B8BBE?style=for-the-badge&logo=bitcoin&logoColor=white" alt="DEX Trading">
</p>

<p align="center">
    <a href="https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/README_CN.md">中文</a> |
    <a href="https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/README.md">English</a> |
    <a href="https://fnzero.dev/">Website</a> |
    <a href="https://t.me/fnzero_group">Telegram</a> |
    <a href="https://discord.gg/vuazbGkqQE">Discord</a>
</p>

## 📋 Table of Contents

- [✨ Features](#-features)
- [📦 Installation](#-installation)
- [🛠️ Usage Examples](#️-usage-examples)
  - [📋 Example Usage](#-example-usage)
  - [⚡ Trading Parameters](#-trading-parameters)
  - [📊 Usage Examples Summary Table](#-usage-examples-summary-table)
  - [⚙️ SWQoS Service Configuration](#️-swqos-service-configuration)
  - [🔧 Middleware System](#-middleware-system)
  - [🔍 Address Lookup Tables](#-address-lookup-tables)
  - [🔍 Nonce Cache](#-nonce-cache)
- [💰 Cashback Support (PumpFun / PumpSwap)](#-cashback-support-pumpfun--pumpswap)
- [🛡️ MEV Protection Services](#️-mev-protection-services)
- [📁 Project Structure](#-project-structure)
- [📄 License](#-license)
- [💬 Contact](#-contact)
- [⚠️ Important Notes](#️-important-notes)

---

## 📦 SDK Versions

This SDK is available in multiple languages:

| Language | Repository | Description |
|----------|------------|-------------|
| **Rust** | [sol-trade-sdk](https://github.com/0xfnzero/sol-trade-sdk) | Ultra-low latency with zero-copy optimization |
| **Node.js** | [sol-trade-sdk-nodejs](https://github.com/0xfnzero/sol-trade-sdk-nodejs) | TypeScript/JavaScript for Node.js |
| **Python** | [sol-trade-sdk-python](https://github.com/0xfnzero/sol-trade-sdk-python) | Async/await native support |
| **Go** | [sol-trade-sdk-golang](https://github.com/0xfnzero/sol-trade-sdk-golang) | Concurrent-safe with goroutine support |

## What This SDK Is For

`sol-trade-sdk-golang` brings the FnZero Solana trading SDK to Go. It is built for Go trading services, copy-trading systems, sniper bots, backend workers, and DEX integrations that need typed builders, reusable execution components, and Rust SDK behavior parity.

| Area | Coverage |
|------|----------|
| DEX protocols | PumpFun, PumpSwap, Bonk, Meteora DAMM v2, Raydium AMM v4, Raydium CPMM |
| Submit lanes | Default Solana RPC plus Jito, ZeroSlot, Temporal, Bloxroute, FlashBlock, BlockRazor, Node1, Astralane, Stellium, Lightspeed, Soyas, Speedlanding, Helius, and Solami; NextBlock remains Rust-blacklisted by default |
| Trading workflows | `BuySimple` / `SellSimple`, legacy buy/sell params, copy trading, sniper trading, address lookup tables, durable nonce, middleware, prebuilt transaction execution |
| Runtime | Go 1.24+, backend services, workers, and latency-sensitive bot infrastructure |

## 🔖 Current Release

**Go module tag:** `github.com/0xfnzero/sol-trade-sdk-golang@v0.1.5`

This release refreshes PumpFun V2 and USDC quote-pool handling, keeps the default RPC submit lane active alongside SWQoS lanes, and aligns Raydium CPMM fixed-output swaps with the on-chain `swap_base_out` instruction. Trade execution requires a caller-supplied recent blockhash or durable nonce; hot-path execution does not query RPC for blockhash, account, or balance data.

## Rust v4.0.21 Parity

This SDK now tracks the Rust SDK `v4.0.21` public behavior for high-level trade intent APIs and SWQoS provider coverage. New code can use `BuySimple` / `SellSimple` with `AccountPolicy`, `BuyAmount`, and `SellAmount`; these convert to the existing `Buy` / `Sell` params without removing the legacy API. The root `TradingClient` still documents its execution boundary and does not claim to build or submit trades by itself. SWQoS coverage includes the Rust `Solami` type and defaults (`beam.solami.dev:11000`, min tip `0.0001 SOL`); live Solami submit uses the main QUIC client path and requires the same base58 Solana keypair api token model as Rust. Explicit SWQoS routes still keep the default RPC lane appended. NextBlock is still filtered by the Rust parity blacklist unless Rust changes that behavior. Legacy extended provider classes such as `Triton`, `QuickNode`, `Syndica`, `Figment`, and `Alchemy` are kept only for source compatibility and are not part of Rust `v4.0.21` trading provider parity.

## ✨ Features

1. **PumpFun Builders**: Instruction builders and params for legacy/V2 PumpFun SOL and USDC quote-pool flows
2. **PumpSwap Builders**: Instruction builders and params for PumpSwap pool operations
3. **Bonk Builders**: Instruction builders and params for Bonk routing
4. **Raydium CPMM Builders**: Instruction builders and params for Raydium CPMM fixed-input and fixed-output swaps
5. **Raydium AMM V4 Builders**: Instruction builders and params for Raydium AMM V4 swaps
6. **Meteora DAMM V2 Builders**: Instruction builders and params for Meteora DAMM V2 operations
7. **Multiple MEV Protection**: Support for the Rust v4.0.21 SWQoS set, including Jito, ZeroSlot, Temporal, Bloxroute, FlashBlock, BlockRazor, Node1, Astralane, Stellium, Lightspeed, Soyas, Speedlanding, Helius, Solami, and Default RPC
8. **Concurrent Trading**: Submit through every configured SWQoS provider plus the default RPC lane; the first accepted result can return early while slower routes continue submitting
9. **Unified Trading Interface**: Use unified trading protocol types for trading operations, including Rust-parity `BuySimple` / `SellSimple` intent params
10. **Middleware System**: Support for custom instruction middleware in lower-level instruction/executor paths
11. **Reusable Execution Components**: Build and submit prebuilt transactions through `pkg/trading`, `pkg/trading/core`, or `pkg/hotpath`
12. **Hot-Path RPC Boundary**: Trade execution uses caller-supplied blockhash or durable nonce and never queries RPC for blockhash, account, or balance data

## 📦 Installation

### Direct Clone (Recommended)

Clone this project to your project directory:

```bash
cd your_project_root_directory
git clone https://github.com/0xfnzero/sol-trade-sdk-golang
```

Add the dependency to your `go.mod`:

```go
// Add to your go.mod
require github.com/0xfnzero/sol-trade-sdk-golang v0.1.5

replace github.com/0xfnzero/sol-trade-sdk-golang => ./sol-trade-sdk-golang
```

Then run:

```bash
go mod tidy
```

### Use Go Modules

```bash
go get github.com/0xfnzero/sol-trade-sdk-golang@v0.1.5
```

## 🛠️ Usage Examples

### 📋 Example Usage

For the high-level intent API, see [Simple Trading](examples/simple_trading/main.go). It shows `AccountPolicyAuto`, `BuyWithMaxInput`, and the conversion to legacy `TradeBuyParams`.

#### 1. Create Root TradingClient Instance

You can refer to [Example: Create TradingClient Instance](examples/trading_client/main.go).

**Method 1: Simple (single wallet)**
```go
package main

import (
    "context"
    "fmt"
    "log"

    soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
    "github.com/gagliardetto/solana-go"
)

func main() {
    ctx := context.Background()

    // Wallet
    wallet := solana.NewWallet()
    payer := wallet.PrivateKey

    // RPC URL
    rpcURL := "https://mainnet.helius-rpc.com/?api-key=xxxxxx"

    // Multiple SWQoS services can be configured
    swqosConfigs := []soltradesdk.SwqosConfig{
        {Type: soltradesdk.SwqosTypeJito, APIKey: "your_uuid", Region: soltradesdk.SwqosRegionFrankfurt},
        {Type: soltradesdk.SwqosTypeBloxroute, APIKey: "your_api_token", Region: soltradesdk.SwqosRegionFrankfurt},
        {Type: soltradesdk.SwqosTypeAstralane, APIKey: "your_api_key", Region: soltradesdk.SwqosRegionFrankfurt},
    }

    config := soltradesdk.NewTradeConfig(rpcURL, swqosConfigs)

    client, err := soltradesdk.NewTradingClient(ctx, &payer, config)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("wallet:", client.GetPayer())
}
```

**Method 2: Prebuilt transaction executor**

The root `TradingClient` converts high-level simple params but intentionally does not build or submit protocol trades in Go. Submit already-built transactions with `pkg/trading.TradeExecutor`, `pkg/trading/core`, or `pkg/hotpath`.

```go
rpcClient := rpc.New(rpcURL)
executor := trading.NewTradeExecutor(rpcClient, config, soltradesdk.NewGasFeeStrategy())
executor.AddSwqosClient(swqos.NewDefaultClient(rpcURL))

result := executor.Execute(ctx, soltradesdk.TradeTypeBuy, signedTx, trading.DefaultExecuteOptions())
if !result.Success {
    log.Fatal(result.Error)
}
```

#### 2. Configure Gas Fee Strategy

```go
// Create GasFeeStrategy instance
gasStrategy := common.NewGasFeeStrategy()
// Set global strategy
gasStrategy.SetGlobalFeeStrategy(150000, 150000, 500000, 500000, 0.001, 0.001)
```

#### 3. Build Trading Parameters

```go
simple := soltradesdk.NewSimpleBuyParams(
    soltradesdk.DexTypePumpSwap,
    soltradesdk.TradeTokenTypeWSOL,
    mintPubkey,
    soltradesdk.BuyWithMaxInput(buySolAmount),
    pumpSwapParams,
    recentBlockhash,
    gasStrategy,
).
    SetSlippageBasisPoints(500).
    SetAccountPolicy(soltradesdk.AccountPolicyAuto)
buyParams := client.BuildBuyParamsFromSimple(simple)
```

#### 4. Execute Boundary

```go
result, err := client.Buy(ctx, buyParams)
if errors.Is(err, soltradesdk.ErrTradingExecutionUnavailable) {
    log.Fatal("root TradingClient converted params only; use a lower-level executor for live submission")
}
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Transaction signature: %s\n", result.Signature)
```

### ⚡ Trading Parameters

For comprehensive information about all trading parameters including `TradeBuyParams` and `TradeSellParams`, see the Trading Parameters documentation.

#### About ShredStream

When using shred to subscribe to events, due to the nature of shreds, you cannot get complete information about transaction events.
Please ensure that the parameters your trading logic depends on are available in shreds when using them.

### 📊 Usage Examples Summary Table

| Description | Run Command | Source Code |
|-------------|-------------|-------------|
| Create and configure TradingClient instance | `go run ./examples/trading_client` | [examples/trading_client](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/trading_client/main.go) |
| Share infrastructure across multiple wallets | `go run ./examples/shared_infrastructure` | [examples/shared_infrastructure](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/shared_infrastructure/main.go) |
| PumpFun token sniping trading | `go run ./examples/pumpfun_sniper_trading` | [examples/pumpfun_sniper_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/pumpfun_sniper_trading/main.go) |
| PumpFun token copy trading | `go run ./examples/pumpfun_copy_trading` | [examples/pumpfun_copy_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/pumpfun_copy_trading/main.go) |
| PumpSwap trading operations | `go run ./examples/pumpswap_trading` | [examples/pumpswap_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/pumpswap_trading/main.go) |
| PumpSwap direct trading (via RPC) | `go run ./examples/pumpswap_direct_trading` | [examples/pumpswap_direct_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/pumpswap_direct_trading/main.go) |
| Raydium CPMM trading operations | `go run ./examples/raydium_cpmm_trading` | [examples/raydium_cpmm_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/raydium_cpmm_trading/main.go) |
| Raydium AMM V4 trading operations | `go run ./examples/raydium_amm_v4_trading` | [examples/raydium_amm_v4_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/raydium_amm_v4_trading/main.go) |
| Meteora DAMM V2 trading operations | `go run ./examples/meteora_damm_v2_trading` | [examples/meteora_damm_v2_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/meteora_damm_v2_trading/main.go) |
| Bonk token sniping trading | `go run ./examples/bonk_sniper_trading` | [examples/bonk_sniper_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/bonk_sniper_trading/main.go) |
| Bonk token copy trading | `go run ./examples/bonk_copy_trading` | [examples/bonk_copy_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/bonk_copy_trading/main.go) |
| Custom instruction middleware example | `go run ./examples/middleware_system` | [examples/middleware_system](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/middleware_system/main.go) |
| Address lookup table example | `go run ./examples/address_lookup` | [examples/address_lookup](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/address_lookup/main.go) |
| Nonce cache (durable nonce) example | `go run ./examples/nonce_cache` | [examples/nonce_cache](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/nonce_cache/main.go) |
| Wrap/unwrap SOL to/from WSOL example | `go run ./examples/wsol_wrapper` | [examples/wsol_wrapper](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/wsol_wrapper/main.go) |
| Seed trading example | `go run ./examples/seed_trading` | [examples/seed_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/seed_trading/main.go) |
| Gas fee strategy example | `go run ./examples/gas_fee_strategy` | [examples/gas_fee_strategy](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/gas_fee_strategy/main.go) |
| Hot path trading (zero-RPC) | `go run ./examples/hot_path_trading` | [examples/hot_path_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/hot_path_trading/main.go) |

### ⚙️ SWQoS Service Configuration

When configuring SWQoS services, use `APIKey` for provider credentials:

- **Jito**: `APIKey` is the Jito UUID (or `""` if not required)
- **Other MEV services**: `APIKey` is the provider API token

#### Custom URL Support

Each SWQoS service supports an optional custom URL parameter:

```go
// Using custom URL
jitoConfig := soltradesdk.SwqosConfig{
    Type:      soltradesdk.SwqosTypeJito,
    APIKey:    "your_uuid",
    Region:    soltradesdk.SwqosRegionFrankfurt,
    CustomURL: "https://custom-jito-endpoint.com",
}

// Using default regional endpoint
bloxrouteConfig := soltradesdk.SwqosConfig{
    Type:   soltradesdk.SwqosTypeBloxroute,
    APIKey: "your_api_token",
    Region: soltradesdk.SwqosRegionNewYork,
}
```

**URL Priority Logic**:
- If a custom URL is provided, it will be used instead of the regional endpoint
- If no custom URL is provided, the system will use the default endpoint for the specified region
- This allows for maximum flexibility while maintaining backward compatibility

When using multiple MEV services, you need to use `Durable Nonce`. You need to use the `FetchNonceInfo` function to get the latest `nonce` value, and use it as the `DurableNonce` when trading.

---

### 🔧 Middleware System

The SDK provides a powerful middleware system that allows you to modify, add, or remove instructions before transaction execution. Middleware executes in the order they are added:

```go
import "github.com/0xfnzero/sol-trade-sdk-golang/pkg/middleware"

manager := middleware.NewManager()
manager.Add(FirstMiddleware{})   // Executes first
manager.Add(SecondMiddleware{})  // Executes second
manager.Add(ThirdMiddleware{})   // Executes last
```

### 🔍 Address Lookup Tables

Address Lookup Tables (ALT) allow you to optimize transaction size and reduce fees by storing frequently used addresses in a compact table format.

```go
import "github.com/0xfnzero/sol-trade-sdk-golang/pkg/addresslookup"

// Fetch ALT from chain
alt, err := addresslookup.FetchAddressLookupTableAccount(ctx, rpcClient, altAddress)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("ALT contains %d addresses\n", len(alt.Addresses))
```

### 🔍 Durable Nonce

Use Durable Nonce to implement transaction replay protection and optimize transaction processing.

```go
import "github.com/0xfnzero/sol-trade-sdk-golang/pkg/nonce"

// Fetch nonce info
nonceInfo, err := nonce.FetchNonceInfo(ctx, rpcClient, nonceAccount)
if err != nil {
    log.Fatal(err)
}
```

## 💰 Cashback Support (PumpFun / PumpSwap)

PumpFun and PumpSwap support **cashback** for eligible tokens: part of the trading fee can be returned to the user. The SDK **must know** whether the token has cashback enabled so that buy/sell instructions include the correct accounts.

- **When params come from RPC**: If you use `PumpFunParams.FromMintByRPC` or `PumpSwapParams.FromPoolAddressByRPC`, the SDK reads `IsCashbackCoin` from chain—no extra step.
- **When params come from decoded events**: If you build params from already-decoded trade events (for example from a parser service), you **must** pass the cashback flag into the SDK:
  - **PumpFun**: Set `IsCashbackCoin` when building params from decoded events.
  - **PumpSwap**: Set `IsCashbackCoin` field when constructing params manually.

## 🛡️ MEV Protection Services

You can apply for a key through the official website: [Community Website](https://fnzero.dev/swqos)

- **Jito**: High-performance block space
- **ZeroSlot**: Zero-latency transactions
- **Temporal**: Time-sensitive transactions
- **Bloxroute**: Blockchain network acceleration
- **FlashBlock**: High-speed transaction execution with API key authentication
- **BlockRazor**: High-speed transaction execution with API key authentication
- **Node1**: High-speed transaction execution with API key authentication
- **Astralane**: Blockchain network acceleration

## 📁 Project Structure

```
pkg/
├── addresslookup/    # Address Lookup Table support
├── cache/            # LRU, TTL, and sharded caches
├── calc/             # AMM calculations with overflow detection
├── common/           # Core types, gas strategies, errors
├── execution/        # Branch optimization, prefetching
├── hotpath/          # Zero-RPC hot path execution
├── instruction/      # Instruction builders for all DEXes
├── middleware/       # Instruction middleware system
├── perf/             # Performance optimizations
├── pool/             # Connection and worker pools
├── rpc/              # High-performance RPC clients
├── seed/             # PDA derivation for all protocols
├── security/         # Secure key storage, validators
├── swqos/            # MEV provider clients
└── trading/          # High-performance trade executor
```

## 📄 License

MIT License

## 💬 Contact

- Official Website: https://fnzero.dev/
- Project Repository: https://github.com/0xfnzero/sol-trade-sdk-golang
- Telegram Group: https://t.me/fnzero_group
- Discord: https://discord.gg/vuazbGkqQE

## ⚠️ Important Notes

1. Test thoroughly before using on mainnet
2. Properly configure private keys and API tokens
3. Pay attention to slippage settings to avoid transaction failures
4. Monitor balances and transaction fees
5. Comply with relevant laws and regulations

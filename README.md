<div align="center">
    <h1>🚀 Sol Trade SDK for Go</h1>
    <h3><em>A comprehensive Go SDK for seamless Solana DEX trading</em></h3>
</div>

<p align="center">
    <strong>A high-performance Go SDK for low-latency Solana DEX trading bots. Built for speed and efficiency, it enables seamless, high-throughput interaction with PumpFun, Pump AMM (PumpSwap), Bonk, Meteora DAMM v2, Raydium AMM v4, and Raydium CPMM for latency-critical trading strategies.</strong>
</p>

<p align="center">
    <a href="https://pkg.go.dev/github.com/0xfnzero/sol-trade-sdk-golang/v9">
        <img src="https://pkg.go.dev/badge/github.com/0xfnzero/sol-trade-sdk-golang/v9.svg" alt="Go Reference">
    </a>
    <a href="#license">
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

Current version: **9.0.1**. Read the [migration notes](RELEASE_NOTES.md) before upgrading; see the [changelog](CHANGELOG.md) for historical changes.

## Contents

Pump.fun defaults to V3 with per-request V1/V2/V3 overrides. See [version configuration, multi-hop examples and simulation limits](docs/PUMPFUN_VERSIONS.md).

- [SDK Versions and Related SDKs](#sdk-versions-and-related-sdks)
- [Installation](#installation)
- [What This SDK Is For](#what-this-sdk-is-for)
- [Features](#features)
- [Usage Examples](#usage-examples)
- [Documentation](#documentation)
- [PumpSwap cached routes](#pumpswap-custom-quote-tokens-and-cached-routes)

<a id="sdk-versions-and-related-sdks"></a>

## 📦 SDK Versions and Related SDKs

Repositories are versioned independently; follow each SDK’s installation guide for compatible dependencies.

| Language | Repository | Purpose | Version |
|----------|------------|-------------|---------|
| Rust | [sol-parser-sdk](https://github.com/0xfnzero/sol-parser-sdk) | DEX transaction and account event parsing | `v0.9.1` |
| Node.js | [sol-parser-sdk-nodejs](https://github.com/0xfnzero/sol-parser-sdk-nodejs) | DEX transaction and account event parsing | `v0.9.1` |
| Python | [sol-parser-sdk-python](https://github.com/0xfnzero/sol-parser-sdk-python) | DEX transaction and account event parsing | `v0.9.1` |
| Go | [sol-parser-sdk-golang](https://github.com/0xfnzero/sol-parser-sdk-golang) | DEX transaction and account event parsing | `v0.9.1` |
| Rust | [sol-trade-sdk](https://github.com/0xfnzero/sol-trade-sdk) | Transaction building, signing and multi-channel submission | `v9.0.1` |
| Node.js | [sol-trade-sdk-nodejs](https://github.com/0xfnzero/sol-trade-sdk-nodejs) | Transaction building, signing and multi-channel submission | `v9.0.1` |
| Python | [sol-trade-sdk-python](https://github.com/0xfnzero/sol-trade-sdk-python) | Transaction building, signing and multi-channel submission | `v9.0.1` |
| Go | [sol-trade-sdk-golang](https://github.com/0xfnzero/sol-trade-sdk-golang) | Transaction building, signing and multi-channel submission | `v9.0.1` |
| Rust | [sol-shred-sdk](https://github.com/0xfnzero/sol-shred-sdk) | Raw shred decoding and ShredStream instruction events | `v6.0.1` |
| Rust | [solana-streamer](https://github.com/0xfnzero/solana-streamer) | gRPC / ShredStream subscriptions and event streaming | `v5.0.0` |
| Rust | [sol-trade-router-sdk](https://github.com/0xfnzero/sol-trade-router-sdk) | On-chain multi-hop router and client SDK | `v0.3.0` |

<a id="installation"></a>

## Installation

```sh
go get github.com/0xfnzero/sol-trade-sdk-golang/v9@v9.0.1
```

To run repository examples or contribute, clone the source:

```sh
git clone https://github.com/0xfnzero/sol-trade-sdk-golang
cd sol-trade-sdk-golang
go mod download
```

See [the example index](examples/README.md) for configuration and offline versus live workflows.

<a id="what-this-sdk-is-for"></a>

## What This SDK Is For

`sol-trade-sdk-golang` brings the FnZero Solana trading SDK to Go. It is built for Go trading services, copy-trading systems, sniper bots, backend workers, and DEX integrations that need typed builders, reusable execution components, and Rust SDK behavior parity.

| Area | Coverage |
|------|----------|
| DEX protocols | PumpFun, PumpSwap, Bonk, Meteora DAMM v2, Raydium AMM v4, Raydium CPMM |
| Submit lanes | Default Solana RPC plus Jito, ZeroSlot, Temporal, Bloxroute, FlashBlock, BlockRazor, Astralane, Stellium, Lightspeed, Soyas, Speedlanding, Helius, and Solami; NextBlock remains Rust-blacklisted by default |
| Trading workflows | `BuySimple` / `SellSimple`, legacy buy/sell params, copy trading, sniper trading, address lookup tables, durable nonce, middleware, prebuilt transaction execution |
| Runtime | Go 1.25+, backend services, workers, and latency-sensitive bot infrastructure |

<a id="features"></a>

## ✨ Features

1. **PumpFun Builders**: Instruction builders and params for legacy/V2 PumpFun SOL and USDC quote-pool flows
2. **PumpSwap Builders**: Instruction builders and params for PumpSwap pool operations
3. **Bonk Builders**: Instruction builders and params for Bonk routing
4. **Raydium CPMM Builders**: Instruction builders and params for Raydium CPMM fixed-input and fixed-output swaps
5. **Raydium AMM V4 Builders**: Instruction builders and params for Raydium AMM V4 swaps
6. **Meteora DAMM V2 Builders**: Instruction builders and params for Meteora DAMM V2 operations
7. **Multiple MEV Protection**: Support for the Rust v5.0.2 SWQoS set, including Jito, ZeroSlot, Temporal, Bloxroute, FlashBlock, BlockRazor, Astralane, Stellium, Lightspeed, Soyas, Speedlanding, Helius, Solami, LunarLander, Glaive, and Default RPC
8. **Concurrent Trading**: Submit through every configured SWQoS provider plus the default RPC lane; the first accepted result can return early while slower routes continue submitting
9. **Unified Trading Interface**: Use unified trading protocol types for trading operations, including Rust-parity `BuySimple` / `SellSimple` intent params
10. **Middleware System**: Support for custom instruction middleware in lower-level instruction/executor paths
11. **Reusable Execution Components**: Build and submit prebuilt transactions through `pkg/trading`, `pkg/trading/core`, or `pkg/hotpath`
12. **Hot-Path RPC Boundary**: Trade execution uses caller-supplied blockhash or durable nonce and never queries RPC for blockhash, account, or balance data

<a id="usage-examples"></a>

## 🛠️ Usage Examples

### 📋 Trading workflow

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

    soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg"
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

Temporal uses HTTP/3 QUIC first and Binary Batch HTTP as its default fallback. BlockRazor uses gRPC `SendBinaryTransaction` first and JSON HTTP as fallback. Astralane uses persistent QUIC first and Binary HTTP as fallback. Explicit transport settings force one protocol; a custom URL without a transport remains an explicit HTTP route.

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

### Example index

See the [example index](examples/README.md) for workflows, commands and configuration requirements.

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
import "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/middleware"

manager := middleware.NewManager()
manager.Add(FirstMiddleware{})   // Executes first
manager.Add(SecondMiddleware{})  // Executes second
manager.Add(ThirdMiddleware{})   // Executes last
```

### 🔍 Address Lookup Tables

Address Lookup Tables (ALT) allow you to optimize transaction size and reduce fees by storing frequently used addresses in a compact table format.

```go
import "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/addresslookup"

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
import "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/nonce"

// Fetch nonce info
nonceInfo, err := nonce.FetchNonceInfo(ctx, rpcClient, nonceAccount)
if err != nil {
    log.Fatal(err)
}
```

<a id="pumpswap-custom-quote-tokens-and-cached-routes"></a>

## PumpSwap custom quote tokens and cached routes

Direct PumpSwap builders match explicit input/output mints, including custom
Token-2022 quote tokens. Cache preparation supports independent buy/sell routes,
bounded best-protected path selection, quote freshness checks and offline V0/V1
signing with explicit nonce/tip/ALT policies. Every hop spends protected credit;
preparation and signing require no RPC.

Run `go run ./examples/pumpswap_cached_prepare` and
`go test ./examples/pumpswap_cached_prepare -count=2` for historical TTWO replay.
See [the API, limits and native tests](docs/PUMPSWAP_QUOTE_ROUTING.md).

Run `go run ./examples/hotpath_latency` from the repository root to measure
cached three-hop buy/sell search and preparation, V0+ALT build/sign and V1
build/sign. Edit `examples/hotpath_latency/config.json` for warmup, samples and
batch size; an alternate config path is optional. The native benchmark uses a
public deterministic test key and frozen accounts, reports p50/p95/p99
nanoseconds and allocations, and makes no RPC calls or submissions. Keep real
wallets out of this example. Local CPU results do not measure network landing.

<a id="cashback-support-pumpfun-pumpswap"></a>

## 💰 Cashback Support (PumpFun / PumpSwap)

PumpFun and PumpSwap support **cashback** for eligible tokens: part of the trading fee can be returned to the user. The SDK **must know** whether the token has cashback enabled so that buy/sell instructions include the correct accounts.

- **When params come from RPC**: If you use `PumpFunParams.FromMintByRPC` or `PumpSwapParams.FromPoolAddressByRPC`, the SDK reads `IsCashbackCoin` from chain—no extra step.
- **When params come from decoded events**: If you build params from already-decoded trade events (for example from a parser service), you **must** pass the cashback flag into the SDK:
  - **PumpFun**: Set `IsCashbackCoin` when building params from decoded events.
  - **PumpSwap**: Set `IsCashbackCoin` field when constructing params manually.

<a id="mev-protection-services"></a>

## 🛡️ MEV Protection Services

You can apply for a key through the official website: [Community Website](https://fnzero.dev/swqos)

- **Jito**: High-performance block space
- **ZeroSlot**: Zero-latency transactions
- **Temporal**: Time-sensitive transactions
- **Bloxroute**: Blockchain network acceleration
- **FlashBlock**: High-speed transaction execution with API key authentication
- **BlockRazor**: High-speed transaction execution with API key authentication
- **Astralane**: Blockchain network acceleration

<a id="project-structure"></a>

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

<a id="important-notes"></a>

## ⚠️ Important Notes

1. Test thoroughly before using on mainnet
2. Properly configure private keys and API tokens
3. Pay attention to slippage settings to avoid transaction failures
4. Monitor balances and transaction fees
5. Comply with relevant laws and regulations

<a id="api-compatibility"></a>

## API compatibility

Full cross-language API parity is still in progress. See the [API migration guide](docs/USAGE.md#api-compatibility) for public behavior and support boundaries, and the examples directory for usage.


Live parser-to-trade integration uses Yellowstone **gRPC**. See [gRPC cache integration and language examples](docs/USAGE.md#grpc-cache). Quoting and construction read cached state without hot-path RPC.


See [cached_damm_v2](examples/cached_damm_v2/main.go) for single-hop DAMM v2 cached preparation and explicit simulation. Callers supply a minimum output; expected net receipts remain unknown. Native SOL and existing WSOL settle through different accounts. Reading PumpFun configuration alone does not provide complete fee quotes or a cached factory.

### Offline deployed-program validation

[Native Go PumpSwap bank replay](examples/pumpswap_offline_bank/README.md) executes
12 direct/three-hop buy/sell scenarios using real signatures and frozen public
SBF programs, including ALT rejection and final-swap rollback. Run
`go test ./examples/pumpswap_offline_bank -count=1 -v` from this repository.
Local preparation/signing benchmarks:
`go test ./examples/pumpswap_cached_prepare -run '^$' -bench BenchmarkPumpSwapCached -benchmem`.
Replay and construction make no RPC calls. Mixed historical captures and synthetic
local funding are documented; these checks do not measure production latency.

<a id="documentation"></a>

## Documentation

- [Release and migration](RELEASE_NOTES.md)
- [Usage guide](docs/USAGE.md)
- [Examples](examples/README.md)
- [Cross-language support and boundaries](docs/cross-language-sync.md)


<a id="license"></a>

## 📄 License

MIT License

<a id="contact"></a>

## 💬 Contact

- Official Website: https://fnzero.dev/
- Project Repository: https://github.com/0xfnzero/sol-trade-sdk-golang
- Telegram Group: https://t.me/fnzero_group
- Discord: https://discord.gg/vuazbGkqQE


Shared stock bridges: 33 official stock/ETF mints and 58 pools. See [eleven-SDK catalog and routing alignment](docs/STOCK_BRIDGE_SYNC.md) for configuration, USDC routing and cache-only boundaries.

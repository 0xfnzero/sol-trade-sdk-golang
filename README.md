# Sol Trade SDK for Go

<p align="center">
    <strong>A high-performance Go SDK for low-latency Solana DEX trading</strong>
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
    <a href="README_CN.md">中文</a> |
    <a href="https://fnzero.dev/">Website</a> |
    <a href="https://t.me/fnzero_group">Telegram</a> |
    <a href="https://discord.gg/vuazbGkqQE">Discord</a>
</p>

---

## 📦 SDK Versions

This SDK is available in multiple languages:

| Language | Repository | Description |
|----------|------------|-------------|
| **Rust** | [sol-trade-sdk](https://github.com/0xfnzero/sol-trade-sdk) | Ultra-low latency with zero-copy optimization |
| **Node.js** | [sol-trade-sdk-nodejs](https://github.com/0xfnzero/sol-trade-sdk-nodejs) | TypeScript/JavaScript for Node.js |
| **Python** | [sol-trade-sdk-python](https://github.com/0xfnzero/sol-trade-sdk-python) | Async/await native support |
| **Go** | [sol-trade-sdk-golang](https://github.com/0xfnzero/sol-trade-sdk-golang) | Concurrent-safe with goroutine support |

---

A comprehensive, high-performance Go SDK for Solana DEX trading with support for multiple protocols and MEV providers.

## Features

- **Multiple DEX Support**: PumpFun, PumpSwap, Bonk, Raydium AMM V4, Raydium CPMM, Meteora DAMM V2
- **SWQoS Integration**: Multiple MEV providers for transaction submission
- **High Performance**: LRU/TTL/Sharded caching, connection pooling, parallel execution
- **Low Latency**: Optimized for sub-second trade execution
- **Security First**: Integer overflow protection with `math/bits`, proper error handling
- **Zero-RPC Hot Path**: All RPC calls happen BEFORE trading execution
- **Type Safety**: Strongly typed with comprehensive error returns
- **Modular Design**: Use only what you need

## Installation

### Direct Clone (Recommended)

Clone this project to your project directory:

```bash
cd your_project_root_directory
git clone https://github.com/0xfnzero/sol-trade-sdk-golang
```

Add the dependency to your `go.mod`:

```go
// Add to your go.mod
require github.com/0xfnzero/sol-trade-sdk-golang v0.0.0

replace github.com/0xfnzero/sol-trade-sdk-golang => ./sol-trade-sdk-golang
```

Then run:

```bash
go mod tidy
```

### Use Go Modules

```bash
go get github.com/0xfnzero/sol-trade-sdk-golang
```

## Quick Start

### Basic Trading

```go
package main

import (
    "context"
    "fmt"
    "log"

    soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang"
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
    "github.com/gagliardetto/solana-go/rpc"
)

func main() {
    ctx := context.Background()

    // Create gas strategy
    gasStrategy := common.NewGasFeeStrategy()
    gasStrategy.SetGlobalFeeStrategy(200000, 200000, 100000, 100000, 0.001, 0.001)

    // Create RPC client
    rpcClient := rpc.New("https://api.mainnet-beta.solana.com")

    // Create trade executor
    config := &soltradesdk.TradeConfig{
        SwqosConfigs: []soltradesdk.SwqosConfig{
            {Type: soltradesdk.SwqosTypeJito},
        },
    }
    executor := trading.NewTradeExecutor(rpcClient, config, gasStrategy)

    // Execute trade
    result, err := executor.Execute(ctx, soltradesdk.TradeTypeBuy, transaction, nil)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Transaction signature: %s\n", result.Signature)
}
```

### High-Performance Executor

```go
package main

import (
    "context"
    "fmt"

    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
    soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang"
)

func main() {
    ctx := context.Background()

    config := &trading.HighPerfTradeConfig{
        RPCUrl: "https://api.mainnet-beta.solana.com",
        SWQoSConfigs: []soltradesdk.SwqosConfig{
            {Type: soltradesdk.SwqosTypeJito},
            {Type: soltradesdk.SwqosTypeBloxroute},
        },
        MaxWorkers:             10,
        ConfirmationTimeoutMs:  30000,
        ConfirmationRetryCount: 30,
        RateLimitPerSecond:     100.0,
    }

    executor, err := trading.NewHighPerfTradeExecutor(config)
    if err != nil {
        panic(err)
    }
    defer executor.Close()

    // Execute with parallel submission
    result := executor.Execute(ctx, soltradesdk.TradeTypeBuy, txBytes, nil)
    fmt.Printf("Success: %v, Signature: %s\n", result.Success, result.Signature)
}
```

### Trading with Factory

```go
package main

import (
    "context"
    "fmt"

    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
    soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang"
)

func main() {
    ctx := context.Background()

    // Create factory with base executor
    factory := trading.NewTradeExecutorFactory(baseExecutor)

    // Get DEX-specific executor
    pumpfunExecutor, err := factory.GetExecutor(soltradesdk.DexTypePumpFun)
    if err != nil {
        panic(err)
    }

    // Create trading client
    client := trading.NewTradingClient(factory)

    // Execute buy on PumpFun
    result, err := client.Buy(ctx, soltradesdk.DexTypePumpFun, params)
    fmt.Printf("Result: %+v\n", result)
}
```

## Usage Examples Summary

| Description | Directory | Run Command |
|-------------|-----------|-------------|
| Create and configure TradingClient instance | [examples/trading_client](examples/trading_client/main.go) | `go run ./examples/trading_client` |
| Share infrastructure across multiple wallets | [examples/shared_infrastructure](examples/shared_infrastructure/main.go) | `go run ./examples/shared_infrastructure` |
| PumpFun token sniping trading | [examples/pumpfun_sniper_trading](examples/pumpfun_sniper_trading/main.go) | `go run ./examples/pumpfun_sniper_trading` |
| PumpFun token copy trading | [examples/pumpfun_copy_trading](examples/pumpfun_copy_trading/main.go) | `go run ./examples/pumpfun_copy_trading` |
| PumpSwap trading operations | [examples/pumpswap_trading](examples/pumpswap_trading/main.go) | `go run ./examples/pumpswap_trading` |
| PumpSwap direct trading (via RPC) | [examples/pumpswap_direct_trading](examples/pumpswap_direct_trading/main.go) | `go run ./examples/pumpswap_direct_trading` |
| Raydium CPMM trading operations | [examples/raydium_cpmm_trading](examples/raydium_cpmm_trading/main.go) | `go run ./examples/raydium_cpmm_trading` |
| Raydium AMM V4 trading operations | [examples/raydium_amm_v4_trading](examples/raydium_amm_v4_trading/main.go) | `go run ./examples/raydium_amm_v4_trading` |
| Meteora DAMM V2 trading operations | [examples/meteora_damm_v2_trading](examples/meteora_damm_v2_trading/main.go) | `go run ./examples/meteora_damm_v2_trading` |
| Bonk token sniping trading | [examples/bonk_sniper_trading](examples/bonk_sniper_trading/main.go) | `go run ./examples/bonk_sniper_trading` |
| Bonk token copy trading | [examples/bonk_copy_trading](examples/bonk_copy_trading/main.go) | `go run ./examples/bonk_copy_trading` |
| Custom instruction middleware example | [examples/middleware_system](examples/middleware_system/main.go) | `go run ./examples/middleware_system` |
| Address lookup table example | [examples/address_lookup](examples/address_lookup/main.go) | `go run ./examples/address_lookup` |
| Nonce cache (durable nonce) example | [examples/nonce_cache](examples/nonce_cache/main.go) | `go run ./examples/nonce_cache` |
| Wrap/unwrap SOL to/from WSOL example | [examples/wsol_wrapper](examples/wsol_wrapper/main.go) | `go run ./examples/wsol_wrapper` |
| Seed trading example | [examples/seed_trading](examples/seed_trading/main.go) | `go run ./examples/seed_trading` |
| Gas fee strategy example | [examples/gas_fee_strategy](examples/gas_fee_strategy/main.go) | `go run ./examples/gas_fee_strategy` |

## Address Lookup Tables

```go
package main

import (
    "context"
    "fmt"
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/addresslookup"
    "github.com/gagliardetto/solana-go"
)

func main() {
    ctx := context.Background()

    // Fetch ALT from chain
    alt, err := addresslookup.FetchAddressLookupTableAccount(
        ctx,
        rpcClient,
        solana.MustPublicKeyFromBase58("..."),
    )
    if err != nil {
        panic(err)
    }

    fmt.Printf("ALT contains %d addresses\n", len(alt.Addresses))
}
```

## Security Features

```go
package main

import (
    "fmt"
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/security"
)

func main() {
    // Integer overflow protection
    fee, err := calc.ComputeFee(amount, feeBasisPoints)
    if err != nil {
        if err == calc.ErrOverflow {
            // Handle overflow
            fmt.Println("Integer overflow detected")
        }
    }

    // Input validation
    err := security.ValidateAmount(amount, "amount")
    if err != nil {
        panic(err)
    }

    // Secure key storage
    storage, err := security.NewSecureKeyStorage(keypair)
    if err != nil {
        panic(err)
    }
    defer storage.Clear()
}
```

## Architecture

| Package | Description |
|---------|-------------|
| `pkg/addresslookup` | Address Lookup Table support |
| `pkg/cache` | LRU, TTL, and sharded caches |
| `pkg/calc` | AMM calculations with overflow detection |
| `pkg/common` | Core types, gas strategies, errors |
| `pkg/execution` | Branch optimization, prefetching |
| `pkg/hotpath` | Zero-RPC hot path execution |
| `pkg/instruction` | Instruction builders for all DEXes |
| `pkg/middleware` | Instruction middleware system |
| `pkg/perf` | Performance optimizations (SIMD, kernel bypass, etc.) |
| `pkg/pool` | Connection and worker pools |
| `pkg/rpc` | High-performance RPC clients |
| `pkg/seed` | PDA derivation for all protocols |
| `pkg/security` | Secure key storage, validators |
| `pkg/swqos` | MEV provider clients (19 providers) |
| `pkg/trading` | High-performance trade executor |

## Performance Optimizations

### Kernel Bypass (Linux io_uring)
```go
import "github.com/0xfnzero/sol-trade-sdk-golang/pkg/perf"

ring := perf.NewIOUring(256)
defer ring.Close()
```

### SIMD Vectorization
```go
import "github.com/0xfnzero/sol-trade-sdk-golang/pkg/perf"

// Use SIMD-optimized hash functions
hashes := perf.VectorizedHash(dataList)
```

### Compiler Optimizations
```go
import "github.com/0xfnzero/sol-trade-sdk-golang/pkg/perf"

// Likely/unlikely hints for branch prediction
if perf.Likely(condition) {
    // Fast path
}
```

## Supported Protocols

### PumpFun
- Bonding curve calculations with creator fee support
- Buy/Sell instruction building
- PDA derivation for bonding curve and associated accounts

### PumpSwap
- Pool calculations with LP/protocol/creator fees
- Buy/Sell instruction building
- Mayhem mode support

### Bonk
- Virtual/real reserve calculations
- Protocol fee handling

### Raydium
- AMM V4 calculations with constant product
- CPMM calculations
- Authority PDA derivation

### Meteora
- DAMM V2 swap calculations
- Pool PDA derivation

## Middleware System

```go
package main

import (
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/middleware"
)

func main() {
    manager := middleware.NewMiddlewareManager()
    manager.AddMiddleware(&middleware.ValidationMiddleware{
        MaxInstructions: 100,
        MaxDataSize:     10000,
    })
    manager.AddMiddleware(&middleware.LoggingMiddleware{})

    // Apply middlewares to instructions
    processed, err := manager.ApplyMiddlewaresProcessProtocolInstructions(
        instructions, "PumpFun", true,
    )
}
```

## Requirements

- Go >= 1.21
- github.com/gagliardetto/solana-go >= v1.9.0

## Error Handling

All calculation functions return errors for overflow and invalid inputs:

```go
var (
    ErrOverflow       = errors.New("integer overflow")
    ErrDivisionByZero = errors.New("division by zero")
    ErrInvalidInput   = errors.New("invalid input")
)

// Always check errors
result, err := calc.ComputeFee(amount, feeBasisPoints)
if err != nil {
    // Handle error appropriately
}
```

## Testing

```bash
go test ./...

# With coverage
go test -cover ./...

# Verbose
go test -v ./...
```

## License

MIT License

## Contact

- Official Website: https://fnzero.dev/
- Project Repository: https://github.com/0xfnzero/sol-trade-sdk-golang
- Telegram Group: https://t.me/fnzero_group
- Discord: https://discord.gg/vuazbGkqQE

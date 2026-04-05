# Sol Trade SDK for Go

<p align="center">
    <strong>A high-performance Go SDK for low-latency Solana DEX trading</strong>
</p>

<p align="center">
    <a href="https://pkg.go.dev/github.com/sol-trade-sdk/sol-trade-sdk-golang">
        <img src="https://pkg.go.dev/badge/github.com/sol-trade-sdk/sol-trade-sdk-golang.svg" alt="Go Reference">
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

A comprehensive, high-performance Go SDK for Solana DEX trading with support for multiple protocols and MEV providers.

## Features

- **Multiple DEX Support**: PumpFun, PumpSwap, Bonk, Raydium AMM V4, Raydium CPMM, Meteora DAMM V2
- **SWQoS Integration**: Jito, Bloxroute, ZeroSlot, Temporal, FlashBlock, Helius, and more
- **High Performance**: LRU/TTL/Sharded caching, connection pooling, parallel execution
- **Low Latency**: Optimized for sub-second trade execution
- **Security First**: Integer overflow protection with `math/bits`, proper error handling
- **Type Safety**: Strongly typed with comprehensive error returns
- **Modular Design**: Use only what you need

## Installation

```bash
go get github.com/sol-trade-sdk/sol-trade-sdk-golang
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    soltradesdk "github.com/sol-trade-sdk/sol-trade-sdk-golang"
    "github.com/sol-trade-sdk/sol-trade-sdk-golang/pkg/common"
    "github.com/sol-trade-sdk/sol-trade-sdk-golang/pkg/trading"
)

func main() {
    ctx := context.Background()

    // Create gas strategy
    gasStrategy := common.NewGasFeeStrategy()
    gasStrategy.SetGlobalFeeStrategy(200000, 200000, 100000, 100000, 0.001, 0.001)

    // Create RPC client
    rpcClient := common.NewRPCClient("https://api.mainnet-beta.solana.com")

    // Create trade executor
    config := &trading.Config{
        SwqosConfigs: []common.SwqosConfig{
            {Type: common.SwqosTypeJito},
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

## Security Features

```go
import "github.com/sol-trade-sdk/sol-trade-sdk-golang/pkg/calc"

// Integer overflow protection
fee, err := calc.ComputeFee(amount, feeBasisPoints)
if err != nil {
    if err == calc.ErrOverflow {
        // Handle overflow
    }
}

// Safe calculations with error handling
output, err := calc.CalculateOutputAmount(input, inputReserve, outputReserve)
if err != nil {
    log.Fatal(err)
}
```

## Architecture

| Package | Description |
|---------|-------------|
| `pkg/cache` | LRU, TTL, and sharded caches |
| `pkg/calc` | AMM calculations with overflow detection |
| `pkg/common` | Core types, gas strategies, errors |
| `pkg/hotpath` | Zero-RPC hot path execution |
| `pkg/instruction` | Instruction builders |
| `pkg/pool` | Connection and worker pools |
| `pkg/rpc` | High-performance RPC clients |
| `pkg/seed` | PDA derivation for all protocols |
| `pkg/swqos` | MEV provider clients |
| `pkg/trading` | High-performance trade executor |

## Supported Protocols

### PumpFun
- Bonding curve calculations
- Buy/Sell instruction building
- PDA derivation

### PumpSwap
- Pool calculations
- Fee breakdown (LP, protocol, curve)
- Instruction building

### Raydium
- AMM V4 calculations
- CPMM calculations
- Authority PDA derivation

### Meteora
- DAMM V2 support
- Pool PDA derivation

## SWQoS Providers

| Provider | Min Tip | Features |
|----------|---------|----------|
| Jito | 0.001 SOL | Bundle support, gRPC |
| Bloxroute | 0.0003 SOL | High reliability |
| ZeroSlot | 0.0001 SOL | Low latency |
| Temporal | 0.0001 SOL | Fast confirmation |
| FlashBlock | 0.0001 SOL | Competitive pricing |
| Helius | 0.000005 SOL | SWQoS-only mode |

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
- Project Repository: https://github.com/sol-trade-sdk/sol-trade-sdk-golang
- Telegram Group: https://t.me/fnzero_group
- Discord: https://discord.gg/vuazbGkqQE

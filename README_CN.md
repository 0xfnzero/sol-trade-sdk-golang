# Sol Trade SDK for Go

<p align="center">
    <strong>高性能 Go SDK，用于低延迟 Solana DEX 交易</strong>
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
    <a href="README.md">English</a> |
    <a href="https://fnzero.dev/">官网</a> |
    <a href="https://t.me/fnzero_group">Telegram</a> |
    <a href="https://discord.gg/vuazbGkqQE">Discord</a>
</p>

---

一个全面的高性能 Go SDK，用于 Solana DEX 交易，支持多种协议和 MEV 提供商。

## 特性

- **多 DEX 支持**: PumpFun、PumpSwap、Bonk、Raydium AMM V4、Raydium CPMM、Meteora DAMM V2
- **SWQoS 集成**: Jito、Bloxroute、ZeroSlot、Temporal、FlashBlock、Helius 等
- **高性能**: LRU/TTL/分片缓存、连接池、并行执行
- **低延迟**: 针对亚秒级交易执行优化
- **安全优先**: 使用 `math/bits` 的整数溢出保护、正确的错误处理
- **类型安全**: 强类型，带全面的错误返回
- **模块化设计**: 按需使用

## 安装

```bash
go get github.com/sol-trade-sdk/sol-trade-sdk-golang
```

## 快速开始

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

    // 创建 Gas 策略
    gasStrategy := common.NewGasFeeStrategy()
    gasStrategy.SetGlobalFeeStrategy(200000, 200000, 100000, 100000, 0.001, 0.001)

    // 创建 RPC 客户端
    rpcClient := common.NewRPCClient("https://api.mainnet-beta.solana.com")

    // 创建交易执行器
    config := &trading.Config{
        SwqosConfigs: []common.SwqosConfig{
            {Type: common.SwqosTypeJito},
        },
    }
    executor := trading.NewTradeExecutor(rpcClient, config, gasStrategy)

    // 执行交易
    result, err := executor.Execute(ctx, soltradesdk.TradeTypeBuy, transaction, nil)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("交易签名: %s\n", result.Signature)
}
```

## 安全特性

```go
import "github.com/sol-trade-sdk/sol-trade-sdk-golang/pkg/calc"

// 整数溢出保护
fee, err := calc.ComputeFee(amount, feeBasisPoints)
if err != nil {
    if err == calc.ErrOverflow {
        // 处理溢出
    }
}

// 带错误处理的安全计算
output, err := calc.CalculateOutputAmount(input, inputReserve, outputReserve)
if err != nil {
    log.Fatal(err)
}
```

## 架构

| 包 | 描述 |
|-----|------|
| `pkg/cache` | LRU、TTL 和分片缓存 |
| `pkg/calc` | 带溢出检测的 AMM 计算 |
| `pkg/common` | 核心类型、Gas 策略、错误 |
| `pkg/hotpath` | 零-RPC 热路径执行 |
| `pkg/instruction` | 指令构建器 |
| `pkg/pool` | 连接池和工作池 |
| `pkg/rpc` | 高性能 RPC 客户端 |
| `pkg/seed` | 所有协议的 PDA 派生 |
| `pkg/swqos` | MEV 提供商客户端 |
| `pkg/trading` | 高性能交易执行器 |

## 支持的协议

### PumpFun
- 联合曲线计算
- 买卖指令构建
- PDA 派生

### PumpSwap
- 池计算
- 费用分解 (LP、协议、曲线)
- 指令构建

### Raydium
- AMM V4 计算
- CPMM 计算
- 权限 PDA 派生

### Meteora
- DAMM V2 支持
- 池 PDA 派生

## SWQoS 提供商

| 提供商 | 最低小费 | 特性 |
|----------|---------|----------|
| Jito | 0.001 SOL | 捆绑支持、gRPC |
| Bloxroute | 0.0003 SOL | 高可靠性 |
| ZeroSlot | 0.0001 SOL | 低延迟 |
| Temporal | 0.0001 SOL | 快速确认 |
| FlashBlock | 0.0001 SOL | 有竞争力的价格 |
| Helius | 0.000005 SOL | 仅 SWQoS 模式 |

## 环境要求

- Go >= 1.21
- github.com/gagliardetto/solana-go >= v1.9.0

## 错误处理

所有计算函数都会为溢出和无效输入返回错误：

```go
var (
    ErrOverflow       = errors.New("integer overflow")
    ErrDivisionByZero = errors.New("division by zero")
    ErrInvalidInput   = errors.New("invalid input")
)

// 始终检查错误
result, err := calc.ComputeFee(amount, feeBasisPoints)
if err != nil {
    // 适当处理错误
}
```

## 测试

```bash
go test ./...

# 带覆盖率
go test -cover ./...

# 详细输出
go test -v ./...
```

## 许可证

MIT License

## 联系方式

- 官方网站: https://fnzero.dev/
- 项目仓库: https://github.com/sol-trade-sdk/sol-trade-sdk-golang
- Telegram 群组: https://t.me/fnzero_group
- Discord: https://discord.gg/vuazbGkqQE

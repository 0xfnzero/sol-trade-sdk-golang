# Sol Trade SDK for Go

<p align="center">
    <strong>高性能 Go SDK，用于低延迟 Solana DEX 交易</strong>
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
    <a href="README.md">English</a> |
    <a href="https://fnzero.dev/">官网</a> |
    <a href="https://t.me/fnzero_group">Telegram</a> |
    <a href="https://discord.gg/vuazbGkqQE">Discord</a>
</p>

---

## 📦 SDK 版本

本 SDK 提供多种语言版本：

| 语言 | 仓库 | 描述 |
|------|------|------|
| **Node.js** | [sol-trade-sdk-nodejs](https://github.com/0xfnzero/sol-trade-sdk-nodejs) | TypeScript/JavaScript，Node.js 支持 |
| **Python** | [sol-trade-sdk-python](https://github.com/0xfnzero/sol-trade-sdk-python) | 原生 async/await 支持 |
| **Go** | [sol-trade-sdk-golang](https://github.com/0xfnzero/sol-trade-sdk-golang) | 并发安全，goroutine 支持 |

---

一个全面的高性能 Go SDK，用于 Solana DEX 交易，支持多种协议和 MEV 提供商。

## 特性

- **多 DEX 支持**: PumpFun、PumpSwap、Bonk、Raydium AMM V4、Raydium CPMM、Meteora DAMM V2
- **SWQoS 集成**: 多个 MEV 提供商用于交易提交
- **高性能**: LRU/TTL/分片缓存、连接池、并行执行
- **低延迟**: 针对亚秒级交易执行优化
- **安全优先**: 使用 `math/bits` 的整数溢出保护、正确的错误处理
- **零-RPC 热路径**: 所有 RPC 调用在交易执行前完成
- **类型安全**: 强类型，带全面的错误返回
- **模块化设计**: 按需使用

## 安装

```bash
go get github.com/0xfnzero/sol-trade-sdk-golang
```

## 快速开始

### 基本交易

```go
package main

import (
    "context"
    "fmt"
    "log"

    soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang"
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
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

### PumpFun 交易

```go
import (
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction/pumpfun"
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
)

// 计算输入 SOL 可获得的代币数量
tokens, err := calc.PumpFunBuyTokenAmountFromSolAmount(
    1_073_000_000_000_000,  // virtualTokenReserves
    30_000_000_000,          // virtualSolReserves
    793_000_000_000_000,     // realTokenReserves
    true,                    // hasCreator
    1_000_000_000,           // amount (1 SOL)
)
if err != nil {
    log.Fatal(err)
}

// 构建买入指令
builder := pumpfun.NewInstructionBuilder()
instructions, err := builder.BuildBuyInstructions(
    payer,
    tokenMint,
    1_000_000_000,          // inputAmount
    500,                     // slippageBasisPoints (5%)
    bondingCurve,
    creatorVault,
    associatedBondingCurve,
)
```

### 热路径执行（零-RPC 交易）

```go
import (
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/hotpath"
)

// 使用预取数据初始化热路径状态
state := hotpath.NewState()
err := state.PrefetchBlockhash(rpcClient)
if err != nil {
    log.Fatal(err)
}
err = state.CacheAccount(tokenAccountPubkey)
if err != nil {
    log.Fatal(err)
}

// 在交易期间无需任何 RPC 调用即可执行
executor := hotpath.NewExecutor(state)
result, err := executor.ExecuteTrade(ctx, transaction)
```

## 安全特性

```go
import (
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
)

// 整数溢出保护
fee, err := calc.ComputeFee(amount, feeBasisPoints)
if err != nil {
    if errors.Is(err, calc.ErrOverflow) {
        // 处理溢出
    }
    if errors.Is(err, calc.ErrDivisionByZero) {
        // 处理除零
    }
}

// 带错误处理的安全计算
output, err := calc.CalculateOutputAmount(input, inputReserve, outputReserve)
if err != nil {
    log.Fatal(err)
}
```

## 地址查找表

```go
import (
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/addresslookup"
)

// 从链上获取 ALT
alt, err := addresslookup.FetchAddressLookupTableAccount(ctx, rpcClient, altAddress)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("ALT 包含 %d 个地址\n", len(alt.Addresses))
```

## 架构

| 包 | 描述 |
|-----|------|
| `pkg/cache` | LRU、TTL 和分片缓存 |
| `pkg/calc` | 带 `math/bits` 溢出检测的所有 DEX AMM 计算 |
| `pkg/common` | 核心类型、Gas 策略、联合曲线、错误 |
| `pkg/hotpath` | 零-RPC 热路径执行 |
| `pkg/instruction` | 所有 DEX 的指令构建器 |
| `pkg/middleware` | 指令中间件系统 |
| `pkg/pool` | 连接池和工作池 |
| `pkg/rpc` | 高性能 RPC 客户端（同步/异步） |
| `pkg/seed` | 所有协议的 PDA 派生 |
| `pkg/swqos` | MEV 提供商客户端 |
| `pkg/trading` | 高性能交易执行器 |

## 支持的协议

### PumpFun
- 带创建者费用支持的联合曲线计算
- 买卖指令构建
- 联合曲线和关联账户的 PDA 派生

### PumpSwap
- 带 LP/协议/创建者费用的池计算
- 买卖指令构建
- Mayhem 模式支持

### Bonk
- 虚拟/真实储备计算
- 协议费用处理

### Raydium
- 恒定乘积的 AMM V4 计算
- CPMM 计算
- 权限 PDA 派生

### Meteora
- DAMM V2 交换计算
- 池 PDA 派生

## 中间件系统

```go
import (
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/middleware"
)

manager := middleware.NewManager()
manager.Add(middleware.NewValidationMiddleware(100)) // max 100 instructions
manager.Add(middleware.NewTimerMiddleware())

// 将中间件应用于指令
processed, err := manager.Apply(instructions, "PumpFun", true)
if err != nil {
    log.Fatal(err)
}
```

## 高性能交易执行器

```go
import (
    "github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
)

// 创建带并行 SWQoS 提交的高性能执行器
config := &trading.Config{
    SwqosConfigs: []common.SwqosConfig{
        {Type: common.SwqosTypeJito},
        {Type: common.SwqosTypeBloxroute},
        {Type: common.SwqosTypeZeroSlot},
    },
    ParallelSubmission: true,
    FirstSuccessWins:   true,
}

executor := trading.NewHighPerfExecutor(rpcClient, config, gasStrategy)
result, err := executor.Execute(ctx, tradeType, transaction, opts)
```

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
- 项目仓库: https://github.com/0xfnzero/sol-trade-sdk-golang
- Telegram 群组: https://t.me/fnzero_group
- Discord: https://discord.gg/vuazbGkqQE

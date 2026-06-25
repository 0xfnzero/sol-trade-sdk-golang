<div align="center">
    <h1>🚀 Sol Trade SDK for Go</h1>
    <h3><em>全面的 Go SDK，用于无缝 Solana DEX 交易</em></h3>
</div>

<p align="center">
    <strong>一个面向低延迟 Solana DEX 交易机器人的高性能 Go SDK。该 SDK 以速度和效率为核心设计，支持与 PumpFun、Pump AMM（PumpSwap）、Bonk、Meteora DAMM v2、Raydium AMM v4 以及 Raydium CPMM 进行无缝、高吞吐量的交互，适用于对延迟高度敏感的交易策略。</strong>
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
    <a href="https://fnzero.dev/">官网</a> |
    <a href="https://t.me/fnzero_group">Telegram</a> |
    <a href="https://discord.gg/vuazbGkqQE">Discord</a>
</p>

> ☕ **支持本项目**
>
> 本 SDK 完全免费且开源。但维护和持续更新需要消耗大量 AI 算力与 Token。如果这个 SDK 对您的开发有帮助，欢迎每月捐赠任意数量的 SOL，您的支持将帮助这个项目持续运行！
>
> **捐赠钱包：** `6oW7AXz1yRb57pYSxysuXnMs2aR1ha5rzGzReZ1MjPV8`

## 📋 目录

- [✨ 项目特性](#-项目特性)
- [📦 安装](#-安装)
- [🛠️ 使用示例](#️-使用示例)
  - [📋 使用示例](#-使用示例)
  - [⚡ 交易参数](#-交易参数)
  - [📊 使用示例汇总表格](#-使用示例汇总表格)
  - [⚙️ SWQoS 服务配置说明](#️-swqos-服务配置说明)
  - [🔧 中间件系统说明](#-中间件系统说明)
  - [🔍 地址查找表](#-地址查找表)
  - [🔍 Nonce 缓存](#-nonce-缓存)
- [💰 Cashback 支持（PumpFun / PumpSwap）](#-cashback-支持pumpfun--pumpswap)
- [🛡️ MEV 保护服务](#️-mev-保护服务)
- [📁 项目结构](#-项目结构)
- [📄 许可证](#-许可证)
- [💬 联系方式](#-联系方式)
- [⚠️ 重要注意事项](#️-重要注意事项)

---

## 📦 SDK 版本

本 SDK 提供多种语言版本：

| 语言 | 仓库 | 描述 |
|------|------|------|
| **Rust** | [sol-trade-sdk](https://github.com/0xfnzero/sol-trade-sdk) | 超低延迟，零拷贝优化 |
| **Node.js** | [sol-trade-sdk-nodejs](https://github.com/0xfnzero/sol-trade-sdk-nodejs) | TypeScript/JavaScript，Node.js 支持 |
| **Python** | [sol-trade-sdk-python](https://github.com/0xfnzero/sol-trade-sdk-python) | 原生 async/await 支持 |
| **Go** | [sol-trade-sdk-golang](https://github.com/0xfnzero/sol-trade-sdk-golang) | 并发安全，goroutine 支持 |

## 这个 SDK 适合什么场景

`sol-trade-sdk-golang` 将 FnZero Solana 交易 SDK 带到 Go 生态，适合 Go 交易服务、跟单系统、狙击机器人、后端 worker 和 DEX 集成场景，提供强类型构建器、可复用执行组件，并保持 Rust SDK 行为对齐。

| 方向 | 覆盖范围 |
|------|----------|
| DEX 协议 | PumpFun、PumpSwap、Bonk、Meteora DAMM v2、Raydium AMM v4、Raydium CPMM |
| 提交通道 | 默认 Solana RPC，以及 Jito、ZeroSlot、Temporal、Bloxroute、FlashBlock、BlockRazor、Node1、Astralane、Stellium、Lightspeed、Soyas、Speedlanding、Helius、Solami；NextBlock 默认仍按 Rust 黑名单跳过 |
| 交易流程 | `BuySimple` / `SellSimple`、旧版 buy/sell 参数、跟单交易、狙击交易、地址查找表、durable nonce、中间件、预构建交易执行 |
| 运行环境 | Go 1.24+、后端服务、worker 和低延迟 Bot 基础设施 |

## 🔖 当前版本

**Go module tag:** `github.com/0xfnzero/sol-trade-sdk-golang@v0.1.3`

本版本刷新 PumpFun V2 与 USDC quote 池处理逻辑，确保默认 RPC 提交通道会和 SWQoS 通道一起发出，并将 Raydium CPMM fixed-output 交易对齐到链上 `swap_base_out` 指令。交易执行必须由调用方传入 recent blockhash 或 durable nonce；热路径不会查询 RPC 获取 blockhash、账户或余额数据。

## Rust v4.0.21 对齐

本 SDK 现在按 Rust SDK `v4.0.21` 对齐高层交易 intent API 和 SWQoS provider 覆盖。新代码可以使用 `BuySimple` / `SellSimple`，并通过 `AccountPolicy`、`BuyAmount`、`SellAmount` 描述意图；内部会转换到现有 `Buy` / `Sell` 参数，不移除旧 API。根 `TradingClient` 仍明确其执行边界，不宣称自己会构建或提交交易。SWQoS 已包含 Rust 的 `Solami` 类型与默认配置（`beam.solami.dev:11000`，最小 tip `0.0001 SOL`）；真实 Solami 提交走主 QUIC client 路径，并需要和 Rust 相同的 base58 Solana keypair api token。配置显式 SWQoS 时仍会自动追加默认 RPC 通道。NextBlock 仍按 Rust parity 黑名单过滤，除非 Rust 后续改变该行为。`Triton`、`QuickNode`、`Syndica`、`Figment`、`Alchemy` 等历史 extended provider 类仅为源码兼容保留，不属于 Rust `v4.0.21` 交易 provider 对齐范围。

## ✨ 项目特性

1. **PumpFun 构建器**: 提供 legacy/V2 PumpFun SOL 与 USDC quote 池流程的指令构建器和参数
2. **PumpSwap 构建器**: 提供 PumpSwap 池操作的指令构建器和参数
3. **Bonk 构建器**: 提供 Bonk 路由的指令构建器和参数
4. **Raydium CPMM 构建器**: 提供 Raydium CPMM fixed-input 和 fixed-output swap 的指令构建器和参数
5. **Raydium AMM V4 构建器**: 提供 Raydium AMM V4 swap 的指令构建器和参数
6. **Meteora DAMM V2 构建器**: 提供 Meteora DAMM V2 操作的指令构建器和参数
7. **多种 MEV 保护**: 支持 Rust v4.0.21 SWQoS 集合，包括 Jito、ZeroSlot、Temporal、Bloxroute、FlashBlock、BlockRazor、Node1、Astralane、Stellium、Lightspeed、Soyas、Speedlanding、Helius、Solami 和默认 RPC
8. **并发交易**: 所有已配置的 SWQoS 通道和默认 RPC 通道都会发出提交；首个成功只影响返回，较慢通道会继续提交
9. **统一交易接口**: 使用统一的交易协议类型进行交易操作，并支持 Rust 对齐的 `BuySimple` / `SellSimple` intent 参数
10. **中间件系统**: 在低层指令/执行器路径支持自定义指令中间件
11. **可复用执行组件**: 通过 `pkg/trading`、`pkg/trading/core` 或 `pkg/hotpath` 构建并提交预构建交易
12. **热路径 RPC 边界**: 交易执行使用调用方传入的 blockhash 或 durable nonce，不在热路径查询 blockhash、账户或余额

## 📦 安装

### 直接克隆（推荐）

将此项目克隆到您的项目目录：

```bash
cd your_project_root_directory
git clone https://github.com/0xfnzero/sol-trade-sdk-golang
```

在您的 `go.mod` 中添加依赖：

```go
// 添加到您的 go.mod
require github.com/0xfnzero/sol-trade-sdk-golang v0.1.3

replace github.com/0xfnzero/sol-trade-sdk-golang => ./sol-trade-sdk-golang
```

然后运行：

```bash
go mod tidy
```

### 使用 Go Modules

```bash
go get github.com/0xfnzero/sol-trade-sdk-golang@v0.1.3
```

## 🛠️ 使用示例

### 📋 使用示例

高层 intent API 可参考 [Simple Trading](examples/simple_trading/main.go)，示例展示 `AccountPolicyAuto`、`BuyWithMaxInput` 以及到旧版 `TradeBuyParams` 的转换。

#### 1. 创建根 TradingClient 实例

您可以参考 [示例：创建 TradingClient 实例](examples/trading_client/main.go)。

**方法一：简单方式（单钱包）**
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

    // 钱包
    wallet := solana.NewWallet()
    payer := wallet.PrivateKey

    // RPC URL
    rpcURL := "https://mainnet.helius-rpc.com/?api-key=xxxxxx"

    // 可配置多个 SWQoS 服务
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

**方法二：预构建交易执行器**

Go 根 `TradingClient` 只负责高层 simple 参数转换，不在根 facade 中构建或提交协议交易。已构建好的交易可通过 `pkg/trading.TradeExecutor`、`pkg/trading/core` 或 `pkg/hotpath` 提交。

```go
rpcClient := rpc.New(rpcURL)
executor := trading.NewTradeExecutor(rpcClient, config, soltradesdk.NewGasFeeStrategy())
executor.AddSwqosClient(swqos.NewDefaultClient(rpcURL))

result := executor.Execute(ctx, soltradesdk.TradeTypeBuy, signedTx, trading.DefaultExecuteOptions())
if !result.Success {
    log.Fatal(result.Error)
}
```

#### 2. 配置 Gas 费策略

```go
// 创建 GasFeeStrategy 实例
gasStrategy := common.NewGasFeeStrategy()
// 设置全局策略
gasStrategy.SetGlobalFeeStrategy(150000, 150000, 500000, 500000, 0.001, 0.001)
```

#### 3. 构建交易参数

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

#### 4. 执行边界

```go
result, err := client.Buy(ctx, buyParams)
if errors.Is(err, soltradesdk.ErrTradingExecutionUnavailable) {
    log.Fatal("根 TradingClient 只完成参数转换；真实提交请使用低层 executor")
}
if err != nil {
    log.Fatal(err)
}
fmt.Printf("交易签名: %s\n", result.Signature)
```

### ⚡ 交易参数

关于所有交易参数（包括 `TradeBuyParams` 和 `TradeSellParams`）的详细信息，请参阅交易参数文档。

#### 关于 ShredStream

使用 shred 订阅事件时，由于 shred 的特性，您无法获取交易事件的完整信息。
在使用时，请确保您的交易逻辑所依赖的参数在 shred 中可用。

### 📊 使用示例汇总表格

| 描述 | 运行命令 | 源码 |
|------|----------|------|
| 创建并配置 TradingClient 实例 | `go run ./examples/trading_client` | [examples/trading_client](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/trading_client/main.go) |
| 多钱包共享基础设施 | `go run ./examples/shared_infrastructure` | [examples/shared_infrastructure](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/shared_infrastructure/main.go) |
| PumpFun 代币狙击交易 | `go run ./examples/pumpfun_sniper_trading` | [examples/pumpfun_sniper_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/pumpfun_sniper_trading/main.go) |
| PumpFun 代币跟单交易 | `go run ./examples/pumpfun_copy_trading` | [examples/pumpfun_copy_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/pumpfun_copy_trading/main.go) |
| PumpSwap 交易操作 | `go run ./examples/pumpswap_trading` | [examples/pumpswap_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/pumpswap_trading/main.go) |
| PumpSwap 直接交易（通过 RPC） | `go run ./examples/pumpswap_direct_trading` | [examples/pumpswap_direct_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/pumpswap_direct_trading/main.go) |
| Raydium CPMM 交易操作 | `go run ./examples/raydium_cpmm_trading` | [examples/raydium_cpmm_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/raydium_cpmm_trading/main.go) |
| Raydium AMM V4 交易操作 | `go run ./examples/raydium_amm_v4_trading` | [examples/raydium_amm_v4_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/raydium_amm_v4_trading/main.go) |
| Meteora DAMM V2 交易操作 | `go run ./examples/meteora_damm_v2_trading` | [examples/meteora_damm_v2_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/meteora_damm_v2_trading/main.go) |
| Bonk 代币狙击交易 | `go run ./examples/bonk_sniper_trading` | [examples/bonk_sniper_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/bonk_sniper_trading/main.go) |
| Bonk 代币跟单交易 | `go run ./examples/bonk_copy_trading` | [examples/bonk_copy_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/bonk_copy_trading/main.go) |
| 自定义指令中间件示例 | `go run ./examples/middleware_system` | [examples/middleware_system](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/middleware_system/main.go) |
| 地址查找表示例 | `go run ./examples/address_lookup` | [examples/address_lookup](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/address_lookup/main.go) |
| Nonce 缓存（持久 Nonce）示例 | `go run ./examples/nonce_cache` | [examples/nonce_cache](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/nonce_cache/main.go) |
| SOL 与 WSOL 互转示例 | `go run ./examples/wsol_wrapper` | [examples/wsol_wrapper](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/wsol_wrapper/main.go) |
| Seed 交易示例 | `go run ./examples/seed_trading` | [examples/seed_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/seed_trading/main.go) |
| Gas 费策略示例 | `go run ./examples/gas_fee_strategy` | [examples/gas_fee_strategy](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/gas_fee_strategy/main.go) |
| 热路径交易（零 RPC） | `go run ./examples/hot_path_trading` | [examples/hot_path_trading](https://github.com/0xfnzero/sol-trade-sdk-golang/blob/main/examples/hot_path_trading/main.go) |

### ⚙️ SWQoS 服务配置说明

配置 SWQoS 服务时，通过 `APIKey` 传入 provider 凭据：

- **Jito**: `APIKey` 为 Jito UUID（不需要时传 `""`）
- **其他 MEV 服务**: `APIKey` 为对应 provider 的 API token

#### 自定义 URL 支持

每个 SWQoS 服务都支持可选的自定义 URL 参数：

```go
// 使用自定义 URL
jitoConfig := soltradesdk.SwqosConfig{
    Type:      soltradesdk.SwqosTypeJito,
    APIKey:    "your_uuid",
    Region:    soltradesdk.SwqosRegionFrankfurt,
    CustomURL: "https://custom-jito-endpoint.com",
}

// 使用默认区域端点
bloxrouteConfig := soltradesdk.SwqosConfig{
    Type:   soltradesdk.SwqosTypeBloxroute,
    APIKey: "your_api_token",
    Region: soltradesdk.SwqosRegionNewYork,
}
```

**URL 优先级逻辑**:
- 如果提供了自定义 URL，将使用该 URL 而非区域端点
- 如果未提供自定义 URL，系统将使用指定区域的默认端点
- 这在保持向后兼容性的同时提供了最大的灵活性

使用多个 MEV 服务时，您需要使用 `Durable Nonce`。您需要使用 `FetchNonceInfo` 函数获取最新的 `nonce` 值，并在交易时将其作为 `DurableNonce` 使用。

---

### 🔧 中间件系统说明

SDK 提供了强大的中间件系统，允许您在交易执行前修改、添加或移除指令。中间件按添加顺序执行：

```go
import "github.com/0xfnzero/sol-trade-sdk-golang/pkg/middleware"

manager := middleware.NewManager()
manager.Add(FirstMiddleware{})   // 最先执行
manager.Add(SecondMiddleware{})  // 其次执行
manager.Add(ThirdMiddleware{})   // 最后执行
```

### 🔍 地址查找表

地址查找表（ALT）允许您通过以紧凑的表格格式存储常用地址来优化交易大小并降低费用。

```go
import "github.com/0xfnzero/sol-trade-sdk-golang/pkg/addresslookup"

// 从链上获取 ALT
alt, err := addresslookup.FetchAddressLookupTableAccount(ctx, rpcClient, altAddress)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("ALT 包含 %d 个地址\n", len(alt.Addresses))
```

### 🔍 Nonce 缓存

使用持久 Nonce 实现交易重放保护并优化交易处理。

```go
import "github.com/0xfnzero/sol-trade-sdk-golang/pkg/nonce"

// 获取 nonce 信息
nonceInfo, err := nonce.FetchNonceInfo(ctx, rpcClient, nonceAccount)
if err != nil {
    log.Fatal(err)
}
```

## 💰 Cashback 支持（PumpFun / PumpSwap）

PumpFun 和 PumpSwap 为符合条件的代币支持 **cashback**：部分交易费用可以返还给用户。SDK **必须知道**代币是否启用了 cashback，以便买/卖指令包含正确的账户。

- **当参数来自 RPC 时**: 如果您使用 `PumpFunParams.FromMintByRPC` 或 `PumpSwapParams.FromPoolAddressByRPC`，SDK 会从链上读取 `IsCashbackCoin`——无需额外步骤。
- **当参数来自已解码事件时**: 如果您从已解码的交易事件构建参数（例如外部解析服务输出），您**必须**将 cashback 标志传递给 SDK：
  - **PumpFun**: 从解析的事件构建参数时设置 `IsCashbackCoin`。
  - **PumpSwap**: 手动构建参数时设置 `IsCashbackCoin` 字段。

## 🛡️ MEV 保护服务

您可以通过官网申请密钥：[社区网站](https://fnzero.dev/swqos)

- **Jito**: 高性能区块空间
- **ZeroSlot**: 零延迟交易
- **Temporal**: 时间敏感交易
- **Bloxroute**: 区块链网络加速
- **FlashBlock**: 高速交易执行（API 密钥认证）
- **BlockRazor**: 高速交易执行（API 密钥认证）
- **Node1**: 高速交易执行（API 密钥认证）
- **Astralane**: 区块链网络加速

## 📁 项目结构

```
pkg/
├── addresslookup/    # 地址查找表支持
├── cache/            # LRU、TTL 和分片缓存
├── calc/             # 带溢出检测的 AMM 计算
├── common/           # 核心类型、Gas 策略、错误
├── execution/        # 分支优化、预取
├── hotpath/          # 零-RPC 热路径执行
├── instruction/      # 所有 DEX 的指令构建器
├── middleware/       # 指令中间件系统
├── perf/             # 性能优化
├── pool/             # 连接池和工作池
├── rpc/              # 高性能 RPC 客户端
├── seed/             # 所有协议的 PDA 派生
├── security/         # 安全密钥存储、验证器
├── swqos/            # MEV 提供商客户端
└── trading/          # 高性能交易执行器
```

## 📄 许可证

MIT License

## 💬 联系方式

- 官方网站: https://fnzero.dev/
- 项目仓库: https://github.com/0xfnzero/sol-trade-sdk-golang
- Telegram 群组: https://t.me/fnzero_group
- Discord: https://discord.gg/vuazbGkqQE

## ⚠️ 重要注意事项

1. 在主网使用前请充分测试
2. 正确配置私钥和 API Token
3. 注意滑点设置以避免交易失败
4. 监控余额和交易费用
5. 遵守相关法律法规

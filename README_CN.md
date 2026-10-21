<div align="center">
    <h1>Sol Trade SDK — Go</h1>
    <p>面向 Solana DEX 的 Go 交易 SDK</p>
    <a href="https://pkg.go.dev/github.com/0xfnzero/sol-trade-sdk-golang/v9"><img src="https://pkg.go.dev/badge/github.com/0xfnzero/sol-trade-sdk-golang/v9.svg" alt="sol-trade-sdk-golang"></a>
    <a href="#许可证"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
</div>

<p align="center">
    <a href="README.md">English</a> · <a href="README_CN.md">中文</a> ·
    <a href="examples/README.md">示例</a> ·
    <a href="docs/USAGE.md">文档</a> ·
    <a href="RELEASE_NOTES.md">发布与迁移</a> ·
    <a href="#sdk-版本与相关项目">相关 SDK</a>
</p>

提供交易构建、签名、多通道提交与缓存路由，支持 PumpFun、PumpSwap、Raydium 和 Meteora 等 DEX。各语言接口与支持边界见使用指南。

当前版本：**9.0.1**。升级前请阅读 [迁移说明](RELEASE_NOTES.md)；历史变更见 [版本记录](CHANGELOG_CN.md)。

## 目录

Pump.fun 全局默认 V3，可按单笔交易切换 V1/V2/V3；[版本配置、多跳示例和模拟限制](docs/PUMPFUN_VERSIONS.md)。

- [SDK 版本与相关项目](#sdk-版本与相关项目)
- [安装](#安装)
- [这个 SDK 适合什么场景](#这个-sdk-适合什么场景)
- [项目特性](#项目特性)
- [使用示例](#使用示例)
- [文档导航](#文档导航)
- [PumpSwap 缓存路由](#pumpswap-自定义-quote-token-与缓存路由)

<a id="sdk-版本与相关项目"></a>

## 📦 SDK 版本与相关项目

各仓库版本独立，依赖兼容要求以对应 SDK 的安装说明为准。

| 语言 | 仓库 | 用途 | 版本 |
|----------|------------|-------------|---------|
| Rust | [sol-parser-sdk](https://github.com/0xfnzero/sol-parser-sdk) | DEX 交易与账户事件解析 | `v0.9.1` |
| Node.js | [sol-parser-sdk-nodejs](https://github.com/0xfnzero/sol-parser-sdk-nodejs) | DEX 交易与账户事件解析 | `v0.9.1` |
| Python | [sol-parser-sdk-python](https://github.com/0xfnzero/sol-parser-sdk-python) | DEX 交易与账户事件解析 | `v0.9.1` |
| Go | [sol-parser-sdk-golang](https://github.com/0xfnzero/sol-parser-sdk-golang) | DEX 交易与账户事件解析 | `v0.9.1` |
| Rust | [sol-trade-sdk](https://github.com/0xfnzero/sol-trade-sdk) | 交易构建、签名与多通道提交 | `v9.0.1` |
| Node.js | [sol-trade-sdk-nodejs](https://github.com/0xfnzero/sol-trade-sdk-nodejs) | 交易构建、签名与多通道提交 | `v9.0.1` |
| Python | [sol-trade-sdk-python](https://github.com/0xfnzero/sol-trade-sdk-python) | 交易构建、签名与多通道提交 | `v9.0.1` |
| Go | [sol-trade-sdk-golang](https://github.com/0xfnzero/sol-trade-sdk-golang) | 交易构建、签名与多通道提交 | `v9.0.1` |
| Rust | [sol-shred-sdk](https://github.com/0xfnzero/sol-shred-sdk) | 原始 Shred 解码与 ShredStream 指令事件 | `v6.0.1` |
| Rust | [solana-streamer](https://github.com/0xfnzero/solana-streamer) | gRPC / ShredStream 订阅与事件流封装 | `v5.0.0` |
| Rust | [sol-trade-router-sdk](https://github.com/0xfnzero/sol-trade-router-sdk) | 链上多跳 Router 与客户端 SDK | `v0.3.0` |

<a id="安装"></a>

## 安装

```sh
go get github.com/0xfnzero/sol-trade-sdk-golang/v9@v9.0.1
```

运行仓库示例或参与开发时，再克隆源码：

```sh
git clone https://github.com/0xfnzero/sol-trade-sdk-golang
cd sol-trade-sdk-golang
go mod download
```

示例配置、离线回放与在线交易的区别见 [示例索引](examples/README.md)。

<a id="这个-sdk-适合什么场景"></a>

## 这个 SDK 适合什么场景

`sol-trade-sdk-golang` 将 FnZero Solana 交易 SDK 带到 Go 生态，适合 Go 交易服务、跟单系统、狙击机器人、后端 worker 和 DEX 集成场景，提供强类型构建器、可复用执行组件，并保持 Rust SDK 行为对齐。

| 方向 | 覆盖范围 |
|------|----------|
| DEX 协议 | PumpFun、PumpSwap、Bonk、Meteora DAMM v2、Raydium AMM v4、Raydium CPMM |
| 提交通道 | 默认 Solana RPC，以及 Jito、ZeroSlot、Temporal、Bloxroute、FlashBlock、BlockRazor、Node1、Astralane、Stellium、Lightspeed、Soyas、Speedlanding、Helius、Solami；NextBlock 默认仍按 Rust 黑名单跳过 |
| 交易流程 | `BuySimple` / `SellSimple`、旧版 buy/sell 参数、跟单交易、狙击交易、地址查找表、durable nonce、中间件、预构建交易执行 |
| 运行环境 | Go 1.25+、后端服务、worker 和低延迟 Bot 基础设施 |

<a id="项目特性"></a>

## ✨ 项目特性

1. **PumpFun 构建器**: 提供 legacy/V2 PumpFun SOL 与 USDC quote 池流程的指令构建器和参数
2. **PumpSwap 构建器**: 提供 PumpSwap 池操作的指令构建器和参数
3. **Bonk 构建器**: 提供 Bonk 路由的指令构建器和参数
4. **Raydium CPMM 构建器**: 提供 Raydium CPMM fixed-input 和 fixed-output swap 的指令构建器和参数
5. **Raydium AMM V4 构建器**: 提供 Raydium AMM V4 swap 的指令构建器和参数
6. **Meteora DAMM V2 构建器**: 提供 Meteora DAMM V2 操作的指令构建器和参数
7. **多种 MEV 保护**: 支持 Rust v5.0.2 SWQoS 集合，包括 Jito、ZeroSlot、Temporal、Bloxroute、FlashBlock、BlockRazor、Node1、Astralane、Stellium、Lightspeed、Soyas、Speedlanding、Helius、Solami、LunarLander、Glaive 和默认 RPC
8. **并发交易**: 所有已配置的 SWQoS 通道和默认 RPC 通道都会发出提交；首个成功只影响返回，较慢通道会继续提交
9. **统一交易接口**: 使用统一的交易协议类型进行交易操作，并支持 Rust 对齐的 `BuySimple` / `SellSimple` intent 参数
10. **中间件系统**: 在低层指令/执行器路径支持自定义指令中间件
11. **可复用执行组件**: 通过 `pkg/trading`、`pkg/trading/core` 或 `pkg/hotpath` 构建并提交预构建交易
12. **热路径 RPC 边界**: 交易执行使用调用方传入的 blockhash 或 durable nonce，不在热路径查询 blockhash、账户或余额

<a id="使用示例"></a>

## 🛠️ 使用示例

### 📋 交易流程

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

    soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg"
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

### 示例索引

完整示例、运行命令与配置要求见 [示例索引](examples/README.md)。

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
import "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/middleware"

manager := middleware.NewManager()
manager.Add(FirstMiddleware{})   // 最先执行
manager.Add(SecondMiddleware{})  // 其次执行
manager.Add(ThirdMiddleware{})   // 最后执行
```

### 🔍 地址查找表

地址查找表（ALT）允许您通过以紧凑的表格格式存储常用地址来优化交易大小并降低费用。

```go
import "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/addresslookup"

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
import "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/nonce"

// 获取 nonce 信息
nonceInfo, err := nonce.FetchNonceInfo(ctx, rpcClient, nonceAccount)
if err != nil {
    log.Fatal(err)
}
```

<a id="pumpswap-自定义-quote-token-与缓存路由"></a>

## PumpSwap 自定义 quote token 与缓存路由

直接构建按明确输入、输出 mint 判断方向，支持普通 Token-2022 自定义 quote。
新增缓存路由准备、最高保护输出选路、报价有效期校验及 V0/V1 离线签名，支持明确的
nonce、小费和 ALT 配置。每跳仅使用前一跳保护金额，准备及签名热路径无 RPC。

执行 `go run ./examples/pumpswap_cached_prepare` 和
`go test ./examples/pumpswap_cached_prepare -count=2` 可重复回放 TTWO 历史缓存。
完整 API、限制及原生测试见[文档](docs/PUMPSWAP_QUOTE_ROUTING.md)。

在仓库根目录运行 `go run ./examples/hotpath_latency`，可测量缓存三跳买卖路由
搜索与准备、V0+ALT 构建签名和 V1 构建签名。通过
`examples/hotpath_latency/config.json` 配置预热次数、样本数和批次数量，
也可传入其它配置路径。输出 p50/p95/p99 纳秒耗时和内存分配统计。
示例使用公开的固定测试私钥与冻结账户，不调用 RPC、不发送交易；
请勿向该测试密钥转账。结果仅表示本地 CPU 工作量，不代表网络上链延迟。

<a id="cashback-支持pumpfun-pumpswap"></a>

## 💰 Cashback 支持（PumpFun / PumpSwap）

PumpFun 和 PumpSwap 为符合条件的代币支持 **cashback**：部分交易费用可以返还给用户。SDK **必须知道**代币是否启用了 cashback，以便买/卖指令包含正确的账户。

- **当参数来自 RPC 时**: 如果您使用 `PumpFunParams.FromMintByRPC` 或 `PumpSwapParams.FromPoolAddressByRPC`，SDK 会从链上读取 `IsCashbackCoin`——无需额外步骤。
- **当参数来自已解码事件时**: 如果您从已解码的交易事件构建参数（例如外部解析服务输出），您**必须**将 cashback 标志传递给 SDK：
  - **PumpFun**: 从解析的事件构建参数时设置 `IsCashbackCoin`。
  - **PumpSwap**: 手动构建参数时设置 `IsCashbackCoin` 字段。

<a id="mev-保护服务"></a>

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

<a id="项目结构"></a>

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

<a id="重要注意事项"></a>

## ⚠️ 重要注意事项

1. 在主网使用前请充分测试
2. 正确配置私钥和 API Token
3. 注意滑点设置以避免交易失败
4. 监控余额和交易费用
5. 遵守相关法律法规

<a id="api-兼容性"></a>

## API 兼容性

完整跨语言 API 对齐仍在进行；公开行为和支持边界见 [API 迁移说明](docs/USAGE.md#api-compatibility)，使用方法见 examples 目录。

### 离线链上程序模拟

[原生 Go PumpSwap bank 示例](examples/pumpswap_offline_bank/README.md) 使用真实签名和冻结的公开 SBF 程序，覆盖直接买卖、三跳正反向路由、ALT、最终一跳失败回滚等 12 类场景，每类在新 bank 重复两次。
在仓库根目录执行 `go test ./examples/pumpswap_offline_bank -count=1 -v`。
本地构建基准：`go test ./examples/pumpswap_cached_prepare -run '^$' -bench BenchmarkPumpSwapCached -benchmem`。
回放和构建无 RPC；样本为不同时间的历史捕获，测试资金为本地合成余额，不代表生产延迟。

<a id="文档导航"></a>

## 文档导航

- [发布与迁移](RELEASE_NOTES.md)
- [使用指南](docs/USAGE.md)
- [示例索引](examples/README.md)
- [跨语言功能与边界](docs/cross-language-sync.md)


<a id="许可证"></a>

## 📄 许可证

MIT License

<a id="联系方式"></a>

## 💬 联系方式

- 官方网站: https://fnzero.dev/
- 项目仓库: https://github.com/0xfnzero/sol-trade-sdk-golang
- Telegram 群组: https://t.me/fnzero_group
- Discord: https://discord.gg/vuazbGkqQE


股票池同步：33 种官方股票/ETF、58 个股票池；配置扩展、USDC 起点、缓存选路与使用边界见 [11 SDK 同步说明](docs/STOCK_BRIDGE_SYNC.md)。

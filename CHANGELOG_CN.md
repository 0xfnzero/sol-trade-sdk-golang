# 版本记录

## v9.0.0 — 2026-10-22

同步 SDK、examples、版本导航及 AI 发布规则。请阅读 [发布与迁移说明](RELEASE_NOTES.md)，尤其注意事件精简、V3 默认行为和 Go /v9 模块迁移。

以下保留旧版 README 的发布与集成记录，其中接口、依赖和验证结果描述当时版本，不代表当前行为。当前用法见 [README](README_CN.md) 和 [发布与迁移说明](RELEASE_NOTES.md)。

## v0.2.0

详见 [发布与迁移说明](RELEASE_NOTES.md)。

## v0.1.11 — PumpSwap 多报价池与 creator 账户修复

支持按实际 quote mint/token program 构建 PumpSwap 买卖，补齐 creator vault 校验，修复显式手续费收款方与 mayhem 模式行为，并移除不必要的全局成交量写锁。新增缓存报价、转换路线选择和完整离线签名准备，包含 TTWO 池及 SOL→USDC→TTWO→目标币的买卖回归。

四语言共享真实 3007 失败样本，并逐字段核对业务指令。原生本地 bank 回归使用历史公共程序／账户与合成钱包状态。说明见 [PumpSwap 修复与验证](docs/PUMPSWAP_REPORTED_FAILURE.md)。

## v0.1.10 — Pump depth-2 and SpeedLanding regional updates

Adds Pump depth-2 create_v2 account chains, including curve and migrated-pool combinations. Preserves stored-depth chains when an ancestor is later whitelisted. Synchronizes SpeedLanding regional endpoints, including London and the New York domain, and compatible client routing. Account construction adds no RPC calls.

Validation includes official Pump SDK 4.1.0 instruction comparisons, signed local LiteSVM simulations and SDK regressions. Local simulation state/config is synthetic; no funded transactions were broadcast and production latency was not measured.

## v0.1.9 — Signed transaction and hot-path hardening

Fixes SPL CloseAccount opcode and Token/Token-2022/WSOL execution boundaries. Hardens cached preparation, nonce lifecycle, bounded executor queues and transfer-fee rounding. Adds signed transaction parsing and offline bank regressions. Caller-signed SWQoS submission does not automatically filter provider-owned minimum-tip lanes.

Validation includes local CPU benchmarks and offline signed-bank scenarios. Measured hot paths use cached inputs without RPC. Benchmarks do not establish production network or transaction-landing latency. No funded mainnet transactions were broadcast.

## v0.1.8 — Token program 与 RPC 修复

PumpFun legacy/V2 买卖构建器使用解析事件或 mint-owner RPC 查询提供的显式 `TokenProgram`，不再根据 mint 后缀判断。ATA 创建与交易账户派生统一使用该 program。未传入时保留 Token-2022 默认值；SPL Token mint 请显式传入 mint owner。

`TransferSOL` 与 `CloseTokenAccount` 改用 `getLatestBlockhash`。回归测试覆盖 mint 后缀、两种 token program、legacy/V2 买卖、ATA 派生，以及本地模拟 RPC 下的交易签名。`go test ./...` 与 `go vet ./...` 已通过。

## v0.1.7 — CPMM creator-fee 对齐

对齐 Rust 5.0.7 的 CPMM creator-fee API：creator 签名领取（15 账户）、permissionless 领取（16 账户）、必传 share PDA、236 字节 AmmConfig/145 字节 CreatorFeeShare 解码、同一银行快照的冷路径 RPC 比例读取、整数收益拆分，以及无 RPC 的缓存准备与版本/时效/连续性重验。共享主网模拟样本逐项比较账户、指令字节和收益，包含 Token-2022 与零比例 override。PoolState、swap/LP 行为不变。估算不含 Token-2022 转账税；链上比例在领取执行时读取。

## v0.1.6

新增缓存交易/路由准备、CLMM/Whirlpool/DLMM 整数报价、PumpFun 原生结算及多跳、StonkFun 路由、V1 序列化、token mint 校验和模拟示例，包含原生对齐证据及回归样本。此语言版本尚未包含 Rust 新增的 CPMM creator-fee 领取 API，不代表完整的 Rust 5.0.7 API 对齐。

## Rust v5.0.2 对齐

本 SDK 现在按 Rust SDK `v5.0.2` 对齐高层交易 intent API 和 SWQoS provider 覆盖。同步包含：`TradeRiskGate`（仅买入路径）、Meteora DAMM v2 的 `swap_mode` / rate-limiter sysvar / referral，以及 Bonk 滑点 clamp（9999 bps）。新代码可以使用 `BuySimple` / `SellSimple`，并通过 `AccountPolicy`、`BuyAmount`、`SellAmount` 描述意图；内部会转换到现有 `Buy` / `Sell` 参数，不移除旧 API。根 `TradingClient` 仍明确其执行边界，不宣称自己会构建或提交交易。SWQoS 已包含 LunarLander、Glaive、Solami 等与 Rust 对齐的类型。配置显式 SWQoS 时仍会自动追加默认 RPC 通道。NextBlock 仍按 Rust parity 黑名单过滤。`Triton`、`QuickNode`、`Syndica`、`Figment`、`Alchemy` 等历史 extended provider 类仅为源码兼容保留，不属于 Rust `v5.0.2` 交易 provider 对齐范围。

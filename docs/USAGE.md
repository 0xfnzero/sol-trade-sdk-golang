# Sol Trade SDK golang usage

[Examples](../examples/README.md) · [English](../README.md) · [中文](../README_CN.md)

- [API compatibility](#api-compatibility)
- [Low-latency bot](#low-latency-bot)
- [gRPC account cache](#grpc-cache)
- [AMM v4 cached trading](#amm-v4-cache)
- [LaunchLab cached routes](#launchlab-cache)
- [Signed wire submission](#cached-swqos)
- [Simulation and offline verification](#simulation-matrix)
- [CPMM creator-fee collection](#cpmm-creator-fee)

## API compatibility

Use complete cached execution requests: Node `{request, signers, submit}`, Python a dict with those keys, or Go `*TradeExecutionRequest{Request, Signers, Submit}`. Provide wallet, amounts, asset mints, current account snapshots and blockhash explicitly. The submit callback receives signed raw transaction bytes; `submitted` does not imply `confirmed`.

Nonempty explicit hints take priority over caller-supplied candidate pools. Candidate search is bounded to five hops without repeated pools or asset cycles; it does not discover pools through RPC or an aggregator. Keep SOL endpoint intent (`native_input` / `native_output`) separate from WSOL mint identity.

Use guarded ready snapshots for live subscriptions. Interrupt readiness on disconnects, dropped updates, account-version conflicts or unverified continuity. Recovery requires a selected fork and revalidated dependencies; old guarded snapshots stay invalid. Same-version conflicts require a new cache. Automatic recovery, new-array discovery and complete commitment/fork association remain application integration responsibilities.

PumpSwap cached exact-in preparation is available; cashback and nonzero transfer fees remain unsupported in that path. PumpFun native-quote settlement requires complete prepared routes with matching swap instructions, payer and protected output. DAMM v2 uses an explicit positive `fixedOutputAmount` / `fixed_output_amount` / `FixedOutputAmount` lower bound; estimated output may be null. Check Go pointer outputs for nil. These capabilities do not imply full protocol or asset-matrix parity.

Use exact integers for reserves and amounts. Node Number helpers reject inexact values; use bigint. Go wide PumpFun helpers accept `*big.Int` reserves. Python calculation errors raise rather than returning fabricated zero quotes. PumpSwap FeeConfig decoding requires its actual discriminator.

Python instruction-only helpers are `build_buy_instructions` / `build_sell_instructions`; execution uses the cached core. Prefer keyword construction for quote-aware PumpFun parameters and pool structs. Default bonding-curve reserves are zero; initialize only from verified state. Legacy ambiguous PDA helpers reject incomplete seeds.

CPMM creator-fee collection is implemented. Observe the share PDA explicitly, including an empty-data tombstone for validated absence, then revalidate guarded account versions before use. See [creator-fee collection](#cpmm-creator-fee).


<a id="low-latency-bot"></a>

## Low-latency bot

`low_latency_bot` defines interfaces between `sol-parser-sdk`, `solana-streamer`, and an application execution adapter. Implement that adapter with protocol instruction builders and a configured prebuilt-transaction executor. The root `pkg.TradingClient` and protocol factory deliberately do not build or submit protocol trades.

该模板定义 parser、streamer 和应用交易适配器之间的接口。适配器应组合协议 instruction builder 与已配置的预构建交易执行器。根包 `pkg.TradingClient` 和协议 factory 都不会构建或发送协议交易。

### Required flow / 必须流程

Filter stale/wrong-target events, query the correct SPL Token or Token-2022 ATA balance, buy with current decoded state and a fresh blockhash, require confirmation, compute the checked balance delta, then refresh state and blockhash before selling. Never sell a fixed assumed amount and never reuse the buy blockhash for the sell.

先过滤过期事件和错误 mint/pool，按真实 Token Program 查询 ATA 余额；使用最新状态和 blockhash 买入；确认后计算余额增量；卖出前重新读取状态和 blockhash。不能用固定假定数量卖出，也不能复用买入 blockhash。

### `min_base_amount_out` / Custom(6040)

6040 (`BuySlippageBelowMinBaseAmountOut`) means actual output was below the explicit protected minimum, commonly because the quote was stale at execution time.

- Prefer `BuyWithMaxInput(...)` or slippage-derived protection for ordinary exact-input buys where supported.
- Populate `FixedOutputTokenAmount` only from a current protocol quote, never from an example constant.
- Removing the minimum suppresses the error by accepting worse execution; keep it for meaningful-size protected trades.
- On 6040, refresh pool state and blockhash, requote, and retry only within a bounded count, event-age limit, and strategy price limit.
- Slippage must be less than 10,000 basis points.

6040 是保护条件不满足。常规 exact-input 优先使用动态滑点保护；显式最低输出必须来自实时 quote。遇到错误后有限次刷新并重新报价，不能无限重试或直接取消保护。

Pre-create token accounts, keep event filters in memory, cap SWQoS concurrency, and request buy confirmation when an automatic sell depends on it. Waiting for all submit providers improves diagnostics at the cost of tail latency.

<a id="grpc-cache"></a>

## gRPC account cache

实时交易数据以 Rust sol-parser-sdk 的 Yellowstone gRPC 接口为准。三个原生 parser 使用相同来源：交易事件提供 pool/mint/方向线索，原始账户事件提供当前字节、owner、slot、write_version，Clock 提供 epoch/时间，BlockMeta 或显式 gRPC unary 提供 blockhash。

此流程不使用 WebSocket。parser 没有 WebSocket 订阅接口；parser 的 SubscriptionHandle/Subscription 管理 gRPC 流。trade-sdk 中另有通用 WebSocket 兼容模块，其实现与测试不是 parser gRPC 能力的证明。

| 语言 | parser 实时入口 | 原始账户写入 trade cache |
| --- | --- | --- |
| Node.js | YellowstoneGrpc.subscribeDexEvents | SubscriptionAccountCache.updateFromParser(raw) |
| Python | YellowstoneGrpc.subscribe_dex_events | SubscriptionAccountCache.update_from_parser(raw) |
| Go | YellowstoneGrpc.SubscribeDexEvents | SubscriptionAccountCache.UpdateRaw(pubkey, owner, data, lamports, slot, writeVersion) |

Node/Python 传原始账户事件的 payload，而非最外层 DexEvent wrapper。Go 将事件公钥解析为 solana.PublicKey；解析失败必须返回错误。账户关闭（lamports=0）保留为 tombstone，不复用旧字节。不要从 swap 事件伪造账户完整状态、当前手续费或 slot。

推荐接入顺序：

1. 在 parser 订阅交易与 AccountRawSnapshot；raw snapshot 需要显式开启事件过滤。按已知 pool/config/vault/mint/tick/bin/Clock 地址订阅。路由候选是线索，用户可选择 SOL、WSOL、USDC 或股票作为支付/收款资产。
2. 原始账户更新写入选定 fork 的本地 cache，保留真实版本。缺失配置/新 array 必须在交易前通过订阅或冷启动补齐；不会用 WebSocket 或热路径 RPC 自动兜底。
3. 冻结 cache.snapshot()，替换为用户的钱包、金额与资产，使用 cached prepare/route。本地校验 owner、pool/mint 连通性、状态、费率、Token-2022 费用及 freshness 后报价和构建。本次买入或卖出是独立交易。
4. 模拟可显式调用 simulateTransaction；这是验证步骤。读取模拟结果用 parser 的 simulation_routes 示例，不等于真实发送。

gRPC 流不保证首次提供所有静态账户。pool 已更新不等于 mint/config 已新鲜；不得把其它账户 slot 改成 Clock slot。重连后须确认缺失更新和 fork，缓存就绪前暂停构建。缺失数据或过期状态明确失败。

### 完整示例

两个 SDK 安装在各自环境中即可运行，也可分别在对应仓库运行。快照文件是两个示例间的明确接口，安装和运行不需要 Rust。

| 语言 | parser 仓库：gRPC 刷新 | trade 仓库：本地构建 |
| --- | --- | --- |
| Node.js | npx tsx examples/stonkfun_snapshot_refresh.ts /absolute/snapshot.json --require-pool-update | npx tsx examples/cached_trade.ts /absolute/snapshot.json |
| Python | python examples/stonkfun_snapshot_refresh.py /absolute/snapshot.json --require-pool-update | python examples/cached_trade.py /absolute/snapshot.json |
| Go | go run ./examples/stonkfun_snapshot_refresh /absolute/snapshot.json --require-pool-update | go run ./examples/cached_trade /absolute/snapshot.json |

使用与 cached_trade 匹配的 accounts[]、legs[] 完整快照，例如现有 cached_tip_route_* 样本；更换自己的钱包、金额和本次方向。CPMM 的 named snapshot 应交给 cached_cpmm 示例。不要直接用历史快照实时交易。

parser 刷新读取 GRPC_URL / GRPC_TOKEN，保留没更新账户的原 slot，可能因 provider 没有发送 pool 更新而超时；trade 默认不联网。需要模拟时为 trade 示例增加 --simulate，并配置 RPC_URL。冷启动与显式模拟允许 RPC，交易报价和构建热路径禁止 RPC。

本文件描述已实现的接入接口；完整自动重连恢复、fork 一致性策略、新 array 发现和全部协议缓存仍须逐项验证。之前十轮报告中的 WebSocket 修复仅属于 trade-sdk 通用模块，不计入这些 gRPC 对齐能力。


### PumpSwap 独立买卖示例

`fixtures/pumpswap_buy_mainnet_20261004.json` / `pumpswap_sell_mainnet_20261004.json` 可交给本目录 cached_trade 示例离线重建；分别是 SOL→meme 和 meme→SOL。三语言使用同一 JSON，参数包含完整 pool/mint/vault/GlobalConfig/FeeConfig 状态、实际 owner/slot/write_version、独立方向、钱包、金额、保护比例和 SOL 结算设置。

这些是历史验证快照。实时运行须先冷准备缺失账户，再使用 parser 的 stonkfun_snapshot_refresh gRPC 示例（名称不限制协议）刷新，读取 GRPC_URL/GRPC_TOKEN，不打印凭据。未收到静态更新不能伪造 slot；断线/fork 不明确时重新验证，禁止热路径 RPC 补齐。Pool 更新不等于全部依赖已就绪。

显式 `--simulate` 使用 RPC_URL 调用模拟，不发送交易。历史卖出使用已核实余额的公开账户且不持有私钥；更换钱包时必须确认真实 token ATA、余额及租金，不能把余额不足模拟记为成功。完整 parser 输出可用各语言 parser 的 simulation_routes 示例读取 `fixtures/pumpswap_mainnet_simulations_20261004.json`，无需网络。

cashback、非零转账费明确拒绝准备；本例未验收既有 WSOL 输入、USDC/股票端点或自动重组恢复。


### DAMM v2 显式保护参数示例

新增 damm_v2_snapshot 示例（各语言对应扩展名或 Go 目录）。它使用完整 accounts 冻结快照校验池、mint/vault、激活与 continuity 后构建 native swap2；需要明确 fixed_output_amount，不做自动报价。历史 fixtures/damm_v2_buy_mainnet_20261004.json 可离线重放，--simulate 才访问 RPC_URL；不提供发送路径。pool 来自实际 Yellowstone 发现，依赖账户明确冷启动，不将冷 RPC 伪称为 gRPC 全量账户初始化。历史 1-atom 下限用于 unsigned 验证，实时交易须替换为自己的当前保护下限；新 cache 不以默认费率或 RPC 补齐缺失信息。

<a id="amm-v4-cache"></a>

## AMM v4 cached trading

使用完整的 `cached_trade` 示例。普通 WSOL 使用现有 WSOL ATA；只有显式 `native_input` 或 `native_output` 才创建并关闭本次临时 WSOL 账户。买入与卖出独立执行，没有 router 合约。

```sh
go run ./examples/cached_trade examples/fixtures/amm_v4_buy_mainnet_20261002.json
go run ./examples/cached_trade examples/fixtures/amm_v4_sell_mainnet_20261002.json
go run ./examples/cached_trade examples/fixtures/amm_v4_route_buy_mainnet_20261002.json
go run ./examples/cached_trade examples/fixtures/amm_v4_route_sell_mainnet_20261002.json
```

这些保存的主网快照用于离线复现，不能当作当前池状态。单池 fixture 支付／收款是 SOL 与 USDC，多跳是 SOL ↔ USDC ↔ 股票 ↔ meme。单池 `dex_type=RaydiumAmmV4`，多跳目标内盘 `dex_type=StonkFun`；交易方向显式 `Buy` 或 `Sell`。

实盘初始化时在冷路径取得 AMM pool（752 bytes）、两个 vault（各165 bytes）、两个经典 SPL Token mint（各82 bytes）、Clock、blockhash以及当前rent。其后使用 parser 的 `stonkfun_snapshot_refresh` 示例订阅 JSON 中的 `accounts[]`；该示例不限于 StonkFun，AMM v4也适用。raw pool/vault与Clock更新需经过缓存slot/write_version检查。静态mint没有更新时保留其真实旧slot；账户过期直接拒绝。需要重新冷启动时应在交易热路径外执行，不能伪造slot或隐式RPC补齐。

使用自己的payer、amount、按本次方向排序的 `legs`、新snapshot/read context、blockhash，并为每笔SOL交易指定唯一seed。`--simulate`仅调用RPC模拟且不发送；用该选项之前必须提供当前状态及有相关资产的模拟payer。现成fixture包含测试钱包地址和历史租金，不应直接用于自己的交易。

缓存入口校验实际池费率、开盘状态、authority nonce、mint与vault身份；报价扣除 `need_take_pnl_coin/pc`，使用ceil swapfee和floor滑点。构建官方8账户V2 exact-in（tag16），不需要OpenBook账户。Token-2022池不适用于AMM v4。本语言API：snapshot.`AmmV4`、`QuoteCachedAmmV4ExactIn`、snapshot.`PrepareAmmV4`。

证据：`fixtures/amm_v4_mainnet_simulations_20261002.json`包含三语言12次独立模拟；每个交易方向三语言wire字节一致。Rust数学对照及执行前余额重算见语言测试fixture，运行时不需要Rust。

### Legacy buy/sell V2 simulation

旧AMM入口也已迁移至V2。单池验证可运行：

```sh
# Node
npx tsx examples/legacy_amm_v2.ts examples/fixtures/legacy_amm_v2_buy_20261002.json --simulate
# Python
PYTHONPATH=. python examples/legacy_amm_v2.py examples/fixtures/legacy_amm_v2_buy_20261002.json --simulate
# Go
go run ./examples/legacy_amm_v2 examples/fixtures/legacy_amm_v2_buy_20261002.json --simulate
```

使用sell快照可独立验证卖出；追加 `--exact-output=50000`（buy）或 `--exact-output=30000`（sell）验证V2 exact-out。输入amount此时为最大输入；exact-out构建器不会替你计算预算可行性。历史快照用于离线复现，当前执行需要当前状态。此示例Buy按native_input标志选择SOL充值WSOL ATA／已有WSOL；Sell保留WSOL，不关闭已有ATA。SOL收款和多跳请使用cached_trade。证明在 `legacy_amm_v2_simulations_20261002.json`，全部只模拟。

<a id="launchlab-cache"></a>

## LaunchLab cached routes

Generic LaunchLab/Bonk cached routes validate the pool's actual global/platform configuration instead of applying the StonkFun platform whitelist. The StonkFun executor and stonkfunCurve/stonkfun_curve/StonkFunCurve remain strict. The current curve implementation supports constant-product curve type 0, exact-in; this change does not add other curve types.

Use the existing cached_trade example with dex_type=LaunchLab (or Bonk), independent Buy/Sell, current subscribed account snapshots and explicit leg pool/input_mint/output_mint. Each leg must connect to the next mint. Choose SOL with native_input/native_output, or WSOL/USDC with explicit token mints and corresponding existing liquidity. Quote/build do not query RPC.

Node exports decodeLaunchLabCurve/buildLaunchLabCurveExactIn; Python exports decode_launchlab_curve/build_launchlab_curve_exact_in; Go instruction exports DecodeLaunchLabCurve/BuildLaunchLabCurveExactIn. Snapshot launchlabCurve/launchlab_curve/LaunchLabCurve decodes and validates subscribed state; configuration and mint/vault accounts must be present in the snapshot.

Offline fixture: examples/fixtures/review10_launchlab_generic.json. This is a synthetic platform-identity regression, NOT mainnet evidence. Run the cached_trade example without --simulate. The three native examples produce identical wire. Never submit this synthetic transaction. A real non-Stonk platform pool still requires live simulation before claiming platform-specific mainnet validation.

<a id="cached-swqos"></a>

## Signed wire submission

准备好当前账户缓存、明确的池路径、自己的钱包及交易金额后，可以把 low-level SWQOS client 适配为缓存执行器的 submit 回调。构建／报价不查询 RPC，适配器不请求确认轮询；receipt 的 `confirmed` 为 false。适配器校验单签名 V1 布局并原样传递字节，执行器校验返回签名。不要交给旧 `VersionedTransaction` 包装器重新序列化。

以下片段是实际发送的接入方式，需要调用方已有 `request`、自己的 signer、已配置的 low-level `client` 和服务端 V1 支持。本仓示例验证只使用模拟或本地 mock，没有发送真实交易。请求现在支持显式 tip 账户与 lamports；构建器会在签名前加入 System Program tip。服务端要求的金额由调用方选择，发送适配器不会自动添加或重复支付。tip 账户可从已配置 low-level client 的本地 tip 列表选择，不查询 RPC。

```ts
import { TradeExecutorFactory } from "sol-trade-sdk/trading";
import { createCachedWireSubmit } from "sol-trade-sdk/swqos";
import { PublicKey } from "@solana/web3.js";
const tippedRequest = {...request, tipAccount:new PublicKey(client.getTipAccount()), tipLamports:5000n};
const executor = TradeExecutorFactory.createCachedExecutor(request.dexType);
const receipt = await executor.execute(tippedRequest, [payer], createCachedWireSubmit(client));
```

```python
from sol_trade_sdk.trading.factory import TradeExecutorFactory
from sol_trade_sdk.swqos import create_cached_wire_submit
from dataclasses import replace
from solders.pubkey import Pubkey
tipped_request = replace(request, tip_account=Pubkey.from_string(client.get_tip_account()), tip_lamports=5000)
executor = TradeExecutorFactory.create_cached_executor(request.dex_type)
receipt = await executor.execute(tipped_request, [payer], create_cached_wire_submit(client))
```

```go
// executor is the existing *trading.CachedTradeExecutor.
request.TipAccount = solana.MustPublicKeyFromBase58(client.GetTipAccount())
request.TipLamports = 5000
receipt, err := executor.Execute(ctx, request, []solana.PrivateKey{payer},
    swqos.NewCachedWireSubmit(client))
```

`request` 的完整创建与gRPC快照输入见 `cached_trade` 和 [AMM v4 cached trading](#amm-v4-cache)。V1单签名的消息／signature边界、本地HTTP原始字节发送和Buy/Sell方向均有回归测试；不能将本地测试视为所有服务的线上验收。

金额是整数 lamports（示例5000不代表所有服务的最低要求）。tip必须预留SOL，即使支付／收款资产是USDC或WSOL。账户／金额需成对提供，零额、默认账户、自付和预算u64溢出均拒绝；未配置tip保持原交易字节。`requiredNativeLamports`／`required_native_lamports` 包含临时WSOL账户资金和tip，不含交易基础费或其它ATA租金。保护输出金额仍为交易到账，不能当作扣除SOL tip的净利润。

三语言完整构建与模拟可使用 cached_trade 和 examples/fixtures/cached_tip_route_buy_20261002.json（或sell）；仅 --simulate 调用RPC，默认无发送。模拟请求启用innerInstructions，执行证据在 cached_tip_simulations_20261002.json。


2026-10-03 boundary checks: The raw-wire adapter now verifies the transport's returned signature against the supplied transaction. V1 framing rejects a readonly payer, duplicate account keys and invalid heap configuration. Go checks cancellation before invoking the transport. These checks run locally; provider tests are mocked and do not constitute live service acceptance.

<a id="simulation-matrix"></a>

## Simulation and offline verification

这些示例只调用 `simulateTransaction`，不签名、不广播。支付者必须是银行中真实存在、SOL/输入 SPL 余额足够的账户；无需私钥，`sigVerify=false` 不能绕过余额与账户约束。买入和卖出分别执行。

### 原生示例

在各自 trade 仓库安装开发依赖后运行，SNAPSHOT 是完整的当前账户快照。`RPC_URL` 可覆盖默认 mainnet RPC，仅显式模拟调用 RPC；不带 `--simulate` 时，报价与构建只读取快照。

```sh
# Python
python examples/cached_trade.py SNAPSHOT.json --simulate --simulation-out simulation.json
# Node.js（开发环境需要 tsx）
npx tsx examples/cached_trade.ts SNAPSHOT.json --simulate --simulation-out simulation.json
# Go
go run ./examples/cached_trade SNAPSHOT.json --simulate --simulation-out simulation.json
```

DAMM v2 可把上述入口替换为 `cached_damm_v2.py`、`cached_damm_v2.ts` 或 `./examples/cached_damm_v2`。它要求调用方明确提供 `fixed_output_amount`，不将兼容恒定乘积公式当作 DAMM 协议报价。

`simulation.json` 保存原始 wire 与完整响应，可复制给对应 parser 仓库，离线检查成交：

```sh
python examples/simulation_routes.py simulation.json
npx tsx examples/simulation_routes.ts simulation.json
go run ./examples/simulation_routes simulation.json
```

只有响应明确包含 `result.value.err: null` 才算执行成功；RPC 错误、缺失结果、余额不足、滑点错误都不能报告成功。请求有 30 秒超时，指定 `minContextSlot`，启用 `innerInstructions`，保留原始 V1 wire，不经 Legacy/V0 重新序列化。

### 六仓开发工作区矩阵

工作区工具 `tools/native-parity/simulate_cached_examples.py` 接受一个或多个完整模板：冷启动加载账户与 Clock → parser Yellowstone gRPC 验证每个池的实际更新 → 冻结快照 → 三语言独立构建并比较 wire → 三次显式模拟 → 三语言 parser 检查每一跳的池、mint、成交状态及已知净到账保护值。

```sh
# 从 Solana-SDK-Projects 工作区根目录运行。
# Python 环境需要安装本地 parser/trade SDK 和 requests，Node 环境需要 tsx。
# 预先配置 GRPC_URL / GRPC_TOKEN，工具不会打印凭据。
python tools/native-parity/simulate_cached_examples.py \
  --output-dir /tmp/sdk-simulation --grpc-timeout 15 \
  sol-trade-sdk-python/examples/fixtures/cached_trade_buy_mainnet_20261002.json \
  sol-trade-sdk-python/examples/fixtures/cached_trade_sell_mainnet_20261002.json
```

模板仅提供账户地址、路径和请求，历史数据与预期结果会被当前银行状态替换。冷加载限于 100 个账户；迁移池、新数组或缺失账户需显式重新准备。静态 mint/config 每次冷启动重新加载，gRPC 更新后仍接受陈旧预算校验。此工具是有限时长的开发验证器，不能替代生产连接连续性和 fork 监控。

`--cold-only` 明确跳过 gRPC，仅验证当前银行模拟与跨语言结果，不算实时流验收。无池更新、RPC 错误或准备失败均写入 summary，不能算成交成功。账户、wire、模拟响应及解析结果保存到输出目录；重复运行前删除旧的对应模拟证据，避免读取过期成功记录。

退出码 0 表示所有形态都执行成功且各跳净到账已验证。退出码 1 也可能对应 `simulated_net_credit_unknown`：交易执行成功、parser 成功，但 Token-2022 普通 TransferChecked 没有证明扣费后净到账。此时保留 `null`，不能填零、用报价代替成交或声称净到账已验收。不同时间的模拟银行可以导致实际成交差异。

2026-10-05 实际验证覆盖 PumpSwap SOL 买入/卖回 SOL、StonkFun 股票/USDC/SOL/WSOL 独立买卖、DAMM v2 SOL 与已充值 WSOL 买入。WSOL→股票使用真实可连通路径 WSOL→USDC→股票；不假设存在直接池。工作区报告 `tools/native-parity/live_simulation_review_20261005.md` 记录每项证据及限制。

### 失败交易验证

验证余额不足或无法满足的输出保护时，可使用矩阵 `--expect-execution-failure`。它要求三语言模拟均失败、示例均返回非零，并且三语言 parser 均表示失败且所有实际成交金额为未知；传输/RPC/准备失败不算通过。退出码 0 与 `passed_expected_execution_failure` 表示“失败处理验收通过”，不表示交易成功。模板输入账户的当前余额变化可能改变预期结果，需检查银行日志。

```sh
python tools/native-parity/simulate_cached_examples.py \
  --cold-only --expect-execution-failure \
  --output-dir /tmp/sdk-negative-simulation \
  tools/native-parity/live_negative_simulations_20261005/impossible_protection_template.json
```

矩阵还检查保存证据的 wire 与独立准备的 wire 一致，并拒绝报价腿数与请求/解析腿数不一致的结果。Node.js cached 示例现在校验 u64/i64 范围，拒绝不安全 Number、布尔值、分数与非十进制整数字符串；大金额、slot、版本等必须传十进制字符串。

公开示例钱包余额会随真实交易变化；历史成功不能证明现在仍有资金。第二轮证据见 `tools/native-parity/live_negative_simulations_20261005/REVIEW.md`。Python parser 刷新示例的订阅、事件等待与 blockhash 请求共用截止时间，超时返回 `not_ready`，不保存部分快照。

<a id="cpmm-creator-fee"></a>

## CPMM creator-fee collection

This API matches the Rust 5.0.7 collection surface. Decoders take **full Anchor account bytes**, including discriminator: PoolState 637 bytes, AmmConfig 236, CreatorFeeShare 145. Existing swap/LP reserve accounting continues subtracting gross creator-fee counters.

| Operation | Node.js (root export) | Python (root export) | Go |
|---|---|---|---|
| Creator-signed instruction | collectCreatorFee | collect_creator_fee | instruction.CollectCreatorFee |
| Permissionless instruction | collectCreatorFeePermissionless | collect_creator_fee_permissionless | instruction.CollectCreatorFeePermissionless |
| Share PDA | getCreatorFeeSharePda | get_creator_fee_share_pda | instruction.GetCreatorFeeSharePDA |
| Exact split | splitCreatorFee | split_creator_fee | instruction.SplitCreatorFee |
| Cold RPC snapshot | fetchCreatorFeeShareRate | fetch_creator_fee_share_rate | instruction.FetchCreatorFeeShareRate |
| Cached preparation | prepareCpmmCreatorFeeCollection | prepare_cpmm_creator_fee_collection | snapshot.PrepareCpmmCreatorFeeCollection |
| Before-use validation | validateCpmmCreatorFeeCollection | validate_cpmm_creator_fee_collection | snapshot.ValidateCpmmCreatorFeeCollection |

Pass the actual pool creator, config, vaults, mints and token programs from the decoded pool. Canonical recipient ATAs always belong to the creator, including permissionless calls and Token-2022. The share PDA is always included even if absent on-chain. Creator-signed calls have 15 accounts; permissionless calls have 16, retaining original accounts 1–14.

Rates use millionths. A valid share PDA overrides config, including zero; missing, empty, closed or foreign-owned share accounts use config. Malformed CPMM-owned share accounts, mismatched creator/config, invalid rates and RPC errors are rejected. Protocol share rounds **down**, creator receives remaining dust. Estimates exclude Token-2022 transfer tax and the program reads the rate at execution time.

Preparation takes an immutable subscription snapshot and read context, with **no RPC**. Explicitly observe the share PDA: missing cache entries are unknown, not proof of absence. A validated absence/closed update is an empty-data tombstone. Use the existing readiness/continuity guard for live subscriptions. Validate against a fresh guarded snapshot before using prepared data; account-version changes, stale/future observations, protocol-counter overflow and tampered fields are rejected. Independent streamed accounts are not guaranteed to be one atomic bank snapshot.

Offline replay commands from each repository:

```sh
# Node.js
npx tsx examples/cpmm_creator_fee_replay.ts
# Python
uv run python examples/cpmm_creator_fee_replay.py
# Go
go run ./examples/cpmm_creator_fee_replay
```

The six cases reuse Rust mainnet unsigned simulation captures (default rate, zero override, Token-2022; signed/permissionless). Tests compare full account identities/flags, discriminators and instruction bytes, share PDA and exact payout/protocol split. These examples never sign or submit. Captured expected payouts before transfer taxes do not promise future execution results.


## Pump compact trades and Pump coin quotes (October 2026)

Aligned with [pump-public-docs](https://github.com/pump-fun/pump-public-docs/tree/8cda1fa30ea658b20909d8aedf002047119388d2).

- Compact trades: `instruction.BuildPumpBuyV3Instruction`, `instruction.BuildPumpAmmBuyV2Instruction` (plus exact-quote-in and sell variants). Pump v3 / PumpSwap v2 each use 17 accounts. Existing v2 / v1 APIs remain available for cashback coins.
- Account derivation: `instruction.DerivePumpV3Accounts`, `instruction.DerivePumpSwapV2Accounts`, `instruction.DerivePumpMultiHopAccounts`. Multi-hop validates continuity, one direction, canonical migration pools, Mayhem restrictions and cashback at the currency endpoint. Four or more hops require v0 + ALT.
- Pump coin creation: `instruction.BuildPumpCreateV2Instruction`; the coin-quote creation account helper supplies 5 roles while Q is on its curve, or 8 after migration. Supply decoded venue state, Q's depth, Global.max_curve_depth and listed quote mints. Complete Q without a migrated pool is rejected. QuoteControl decoding includes its new reserves-admin header; the initial-quote-reserves helper validates depth and graduation raise against Q's supply.
- Synthetic completing buy: `calc.QuotePumpBuyV3ExactIn`, `calc.QuotePumpBuyV3ExactOut`. Supply the **actual curve base vault balance**, current resolved protocol/creator fee rates and SOL migration fee (zero for token quotes). Normal non-Mayhem v3 fees use the fixed 1e15 curve market-cap supply. Completed curves reject further trades. Mayhem uses the old capped/partial-fill behavior.
- Pool effective quote reserve is `quote_vault_amount + signed virtual_quote_reserves`; retained protocol/creator fees are not subtracted again. Fee selection distinguishes SOL, USDC and exotic/Pump coin quotes.

Account derivation accepts the native SOL zero key from decoded curve state and the SDK internal SOL selector, normalizing both to WSOL for quote accounts and route endpoints. Caller-owned parameters and hop state are preserved.

These are bare instruction builders: fetch current state and create required token accounts first. On a SOL **curve**, a single compact trade uses native SOL; on an AMM **pool**, it uses WSOL. Multi-hop requires the user's WSOL ATA even for a native SOL curve endpoint. Its buyback recipient is always the currency quote ATA; a single SOL curve trade instead takes the recipient wallet. Set a compute budget for the route and simulate the full multi-hop instruction to determine final output and slippage; do not quote each hop with all fees enabled, since protocol fees apply at the currency endpoint and creator/LP fees at the coin endpoint.

Synthetic execution emits TradeEvent, CompleteEvent and PostCompleteBuyEvent. Sum the curve and post-completion execution legs within the same invocation; neither instruction limits nor the curve TradeEvent alone represent the whole completing buy. Multi-hop emits the existing per-hop trade events, not a new aggregate log event.

### Mainnet validation on 2026-10-08

All six compact buy/sell variants executed successfully with `simulateTransaction` (`sigVerify=false`, no broadcasts). Their instruction bytes and account flags are retained as cross-language regression fixtures. Standard `create_v2` also simulated successfully.

The announced features were not all available in the tested mainnet deployment: `multi_hop_swap` returned `InstructionFallbackNotFound` (101); creating with an unlisted Pump coin quote returned `UnsupportedQuoteMint` (6063), both for curve and migrated-pool quotes; buys above the remaining curve supply returned `NotEnoughTokensToBuy` (6021). The live Global account was 1087 bytes and did not yet include `max_curve_depth`. These outcomes are recorded as unresolved deployment validation, not successful feature tests. The announced-interface builders and synthetic quote math remain opt-in; validate program availability with a fresh simulation before using them.

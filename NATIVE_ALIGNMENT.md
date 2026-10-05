# 原生 SDK 对齐进度（2026-10-02）

对齐目标：Rust sol-parser-sdk **0.7.7**、sol-trade-sdk **5.0.6**。
新增协议逻辑和 V1 wire 编译分别用 Go、TypeScript、Python 实现，没有调用 Rust SDK、Rust 子进程或新增 Rust FFI。安装不要求 Rust 工具链。Python 保留原有 solders 通用 Solana 类型／签名依赖；报价、解析与 wire 编译逻辑不通过 solders 调用 Rust trade/parser SDK。

**整体对齐仍在进行。下面的底层能力已实现，不能据此宣称六个 SDK 已完整对齐或全部多跳路径已可生产交易。**

| 能力 | Go | Node.js | Python |
| --- | --- | --- | --- |
| Legacy/V0/V1 原生 wire 解码；接入 ShredStream | 已实现 | 已实现 | 已实现 |
| compiled/base64 RPC 交易路由、转账与 WSOL 生命周期证据 | Rust 0.7.7 样本通过 | Rust 0.7.7 样本通过 | Rust 0.7.7 样本通过 |
| StonkFun 内盘 Standard/Reward 识别、调用方迁移池记录 | 已实现 | 已实现 | 已实现 |
| 原始账户快照 opt-in（slot/write version/关闭账户） | 已实现 | 已实现 | 已实现 |
| LaunchLab/DLMM/Whirlpool/CLMM 扩展流动性账户快照 | 已实现 | 已实现 | 已实现 |
| 原生 V1 编译、Ed25519 签名 | Rust 字节对照通过 | Rust 字节对照通过 | Rust 字节对照通过 |
| 当前 LaunchLab 常数乘积报价、实际费率、毕业边界、两侧转账费 | Rust 5.0.6 对照通过 | Rust 5.0.6 对照通过 | Rust 5.0.6 对照通过 |
| 股票 quote ↔ meme 内盘单次 exact-in 指令 | 已实现并主网模拟 | 已实现并主网模拟 | 已实现并主网模拟 |
| CPMM 当前费率、累计费用、创建者收费方向、转账费与指令 | Rust 36 组对照＋双向主网模拟 | Rust 36 组对照＋双向主网模拟 | Rust 36 组对照＋双向主网模拟 |
| CLMM swap_v2、Whirlpool swap_v2、DLMM swap2 底层指令 | Rust 账户/字节对照通过 | Rust 账户/字节对照通过 | Rust 账户/字节对照通过 |
| 当前 epoch mint 费率；冻结／暂停／启用 hook 校验 | 已实现 | 已实现 | 已实现 |
| CLMM 原生跨 tick、动态费率、限价单报价及缓存准备 | Rust 807 组对照＋双向模拟 | Rust 807 组对照＋双向模拟 | Rust 807 组对照＋双向模拟 |
| Whirlpool 完整原生报价与缓存准备 | Rust 800 组＋模拟 | Rust 800 组＋模拟 | Rust 800 组＋模拟 |
| DLMM 完整原生报价与缓存准备 | Rust 420 组＋模拟 | Rust 420 组＋模拟 | Rust 420 组＋模拟 |
| 订阅缓存版本排序、冲突原子拒绝、快照与过期检查 | 已实现 | 已实现 | 已实现 |
| parser 身份 → 内盘/CPMM/CLMM/Whirlpool/DLMM 当前参数、报价与独立交易准备 | 已实现 | 已实现 | 已实现 |
| 内盘/CPMM/CLMM/Whirlpool/DLMM 明确池路径的缓存多跳准备 | 已实现 | 已实现 | 已实现 |
| USDC ↔ 股票 ↔ meme（CLMM＋内盘）独立买入／卖出 | 双向主网模拟通过 | 双向主网模拟通过 | 双向主网模拟通过 |
| 全部兑换协议的路线准备 | 待完成 | 待完成 | 待完成 |
| 高层 DexType/factory/executor 接入新增协议和 V1 | 待完成 | 待完成 | 待完成 |
| SOL ↔ USDC ↔ 股票 ↔ meme；WSOL 收款 | 主网模拟通过 | 主网模拟通过 | 主网模拟通过 |
| WSOL 输入多跳 | 本地构造通过；实测钱包余额不足 | 本地构造通过；实测钱包余额不足 | 本地构造通过；实测钱包余额不足 |
| 全部归一化事件新增字段、StonkFun 协议过滤、BlockMeta 对齐 | 待继续审计 | 待继续审计 | 待继续审计 |

## API 和样本

Parser 新接口：Go `AnalyzeRPCTransactionRoutes`、Node.js `analyzeRpcTransactionRoutes`、Python `analyze_rpc_transaction_routes`。接受 compiled JSON 或 base64 getTransaction 响应；不支持将 jsonParsed 指令当作原始指令使用。
金额采用 Go uint64、Node.js bigint、Python int；Go/Node.js 导出的 JSON 金额使用十进制字符串。失败交易保留指令线索，但实际成交额留空。Token-2022 未提供明确转账手续费的指令不会被推断成零手续费。

`StonkFunPoolRegistry` 由调用方持有。仅在成功迁移、有已知 StonkFun platform config 和 CPMM 目标时记录；重复事件幂等，同版本冲突整批拒绝。外盘池不能仅凭一个 CPMM swap 推断为 StonkFun。

五笔主网交易的路由对照样本在 parser 测试目录 `stonkfun_routes_0_7_7.json`：CLMM、复杂多跳、失败交易、毕业外盘和 Reward 内盘。
Trade 测试目录的 `v1_rust_4_4_1.json`、`launchlab_rust_5_0_6.json`、`hops_rust_5_0_6.json` 分别验证 V1 消息与签名、当前报价，以及 DEX 指令字节／账户标记。

交易新接口是独立底层 API；不要假定旧 Bonk 参数、旧高层工厂或旧 executor 已自动获得这些能力。内盘通过 `decodeStonkFunCurve`／`DecodeStonkFunCurve`／`decode_stonkfun_curve` 验证池、配置、quote 和实际费率，然后报价并构造一次买入或卖出。
CLMM、Whirlpool、DLMM 已接入当前订阅缓存的原生报价；调用方仍需提供明确池路径及当前 tick/bin/bitmap 账户，详情见新增章节。

## 已验证的主网内盘模拟

2026-10-02 使用主网当前池/config/mint 字节冷启动，并通过原生 Python parser 的 PublicNode gRPC 获取 Clock（epoch **1047**、slot **452495732**）和 blockhash（slot **452495730**）。三种语言随后分别在本地报价和构建 V1，只用 RPC 做模拟，**没有发送交易**。

| 独立交易 | 输入最小单位 | 最小到账 | 模拟实际到账 | Go/Node/Python 结果 |
| --- | ---: | ---: | ---: | --- |
| 股票 → meme | 10,000 | 6,817,301,666 | 6,886,163,298 | err=null，73,258 CU |
| meme → 股票 | 1,000,000,000 | 1,374 | 1,387 | err=null，73,004 CU |

使用 meme `DQsYFPcRKaKZWTjY4TiqvmxjN87vjumWMECJ7s4U1HbN`、股票 quote `Xsc9qvGR1efVDFGLrVsmkzv3qi45LTBjeUKSPmx9qEh`，池 `BmQj9pBopxouHecN5rYvVLqN7a48CVndfTkhESEZzWgN`。每笔 V1 为 **766 bytes**、零 ALT。

模拟发现并修正：示例必须显式设置 V1 loaded-accounts-data-size limit（64 MiB）；Python parser 查询必须使用生成 stub 的 `GetLatestBlockhash` 等方法名；Go 公共 SPL Token 工具必须按 solana-go 的 `(writable, signer)` 顺序构造账户标记；Node.js 旧 Bonk 构造器中的 authority 公钥大小写错误也已修正。

本节记录股票内盘单次交易；新完成的 USDC 双向多跳模拟见后续章节。保存的样本用于离线复现；实时交易需冷启动当前状态并订阅更新。

## 示例

Parser：`examples/stonkfun_routes`（Go）或 `examples/stonkfun_routes.ts/.py` 接受保存的 compiled/base64 RPC 交易 JSON，输出原生路由证据。三种语言另有 `examples/stonkfun_snapshot_refresh`（Go）和 `.ts/.py`，使用 `GRPC_URL`、`GRPC_TOKEN` 更新快照文件；`--require-pool-update` 要求实际收到池账户更新才完成。

Trade：`examples/stonkfun_native_curve`（Go）或 `examples/stonkfun_native_curve.ts/.py` 接受相同快照 JSON，完成 mint 校验、当前报价、ATA 创建和 V1 构建；加 `--simulate` 才调用 simulateTransaction。示例不包含 sendTransaction。`RPC_URL` 可覆盖模拟端点。

输入字段：`payer`、`recent_blockhash`、`epoch`（十进制字符串）、`buy`（布尔值）、`amount`（十进制字符串）、`slippage_bps`；以及 `pool/global/platform/base_mint/quote_mint` 五个对象，各含 `pubkey/owner/data/slot/write_version`，data 为完整账户字节的 base64。实时调用另提供订阅 Clock 的 `read_slot` 和调用方选择的 `maximum_slot_age`（示例默认 32）；旧快照离线回放默认使用保存的账户 slot。

保存的实测样本：trade 仓库 `examples/fixtures/stonkfun_curve_mainnet_20261002.json`。build/quote 路径无 RPC。该示例需要支付资产已在用户账户中；新增 cached_route 示例已验证 USDC ↔ 股票 ↔ meme；SOL 临时 WSOL 账户生命周期及 Whirlpool/DLMM 原生路线仍待完善。

后续发布前还需补齐上述缺口，完成集中流动性、多跳、资产生命周期和各语言 gRPC 全流程示例的主网模拟，再做包与版本发布。

回归验证：parser Node.js **134 passed / 8 skipped**、Python **111 passed**，Go 全仓通过。Trade 全量回归 Node.js **197 passed**、Python **242 passed**，另新增主网模拟报价复现测试各 1 项通过（Node.js 缓存专项 7 项、Python 缓存/CPMM 专项 7 项）。Go 全仓和新缓存包 race 检查通过。Node.js 包、类型与 trade 示例检查通过。

## 新增订阅缓存与 CPMM 准备

`SubscriptionAccountCache` 只存调用方交给它的账户状态；不会查询 RPC。按 `(slot, write_version)` 排序，旧更新与一致重放被忽略；同版本不同 owner/bytes 拒绝整个更新批次。关闭账户保留空字节 tombstone，旧更新不能复活它。调用方选择 fork 并创建对应缓存；缓存不自行处理重组。`snapshot()`／Go `Snapshot()` 固定一份账户集合，避免一次报价跨多次缓存更新。slot 年龄检查不代表来自原子 bank snapshot。

`PoolTradeHint.fromRouteLeg`／Go `PoolTradeHintFromRouteLeg`／Python `from_route_leg` 只采用 parser 的 pool/input mint/output mint 身份，并检查协议/程序对应关系；不采用他人的钱包、金额、阈值或历史储备。`prepareStonkFunCurve`／`PrepareStonkFunCurve`／`prepare_stonkfun_curve` 接受当前读取上下文、自己的 payer、amount 和 slippage，完成本地参数校验、当前报价和一次交换指令。ATA 设置仍由示例明确完成。

CPMM 使用 `prepareCpmm`／`PrepareCpmm`／`prepare_cpmm`，额外接受订阅 Clock 的 unix timestamp 校验开池时间。读取 pool/config/mints/vaults，保留 token0/token1 顺序，扣除 protocol/fund/creator 累计费用，按实际 creator_fee_on 和 enable_creator_fee 报价，并应用当前 epoch 两侧转账费。新接口不使旧默认费率构造器自动成为可靠的跟单适配器。

新增示例 `examples/cached_cpmm`（Go）、`examples/cached_cpmm.ts/.py`，与内盘示例同样只在 `--simulate` 时调用 RPC。输入另包含 `config/base_vault/quote_vault`、`base_in` 和 `unix_timestamp`。离线样本 `examples/fixtures/cpmm_mainnet_20261002.json`；模拟证据 `cpmm_mainnet_simulations_20261002.json`；Rust 对照用例 `cpmm_rust_5_0_6.json`。三种语言同输入输出相同 689-byte V1（无 ALT）。股票 → meme 与 meme → 股票各有三次独立主网模拟，六次 err=null，约 44k CU。储备会在冷启动与模拟之间变化：保存报价和模拟到账可能不同；用模拟事件中的执行前储备重算，三种语言的净到账与主网输出精确相同。保存样本不能充当实时交易状态。

原生 PublicNode gRPC 刷新已实测三种语言成功：Python/Node Clock slot 452518930、blockhash slot 452518928；Go Clock slot 452518937、blockhash slot 452518935，epoch 1047。本次短时刷新只取得 Clock，未取得池更新，保留冷启动配置原 slot；随后三种 trade 示例均在本地拒绝过期账户，没有通过 RPC 隐式补齐。Go parser 同时修复 https GRPC_URL 的端点解析与 TLS SNI。

Go/Node LaunchLab 归一化交易日志补齐执行前后储备、全部费用腿、pool status；严格验证事件长度与枚举。初始化指令补齐 quote/config/token program。迁移指令提供新旧池、base/quote mint、platform config、destination program，明确 liquidity_amount_known=false。三种 parser 账户填充按事件池匹配 LaunchLab 调用；同池多个不同用户/账户上下文存在歧义时保留缺失，不选择别人的钱包。

## 新增原生 CLMM 和 USDC 多跳（2026-10-02）

Go `calc.ClmmSwapExactIn`、Node.js `clmmSwapExactIn`、Python `clmm_swap_exact_in` 使用原生整数计算，覆盖 tick ↔ Q64 价格、exact-in 步长、跨 tick 流动性、零流动性区间、输入／输出侧收费、动态波动费及限价单／部分单余额。Rust 5.0.6 及 solana-clmm-raydium 0.3.0 开发对照的 **807** 个向量保存为 `clmm_rust_5_0_6.json`（157 个 tick、250 个 swap step、400 个完整交换，包括拒绝案例）。测试和安装不执行 Rust。该覆盖不等同于所有主网池均已实测。

冻结缓存接口 `PrepareClmm`／`prepareClmm`／`prepare_clmm` 从 pool/config/mints、bitmap、tick arrays 补齐当前参数；校验 owner/discriminator/池身份/方向/开池时间/current epoch 费率及 freshness。tick array PDA 使用有符号大端索引，扩展 bitmap 仅在越过基础范围时读取；缺少、关闭、过期、错误归属的账户和超过 1..32 array 预算均明确拒绝，不补查 RPC。CLMM 的阈值是扣除输出 Token-2022 转账费后的净到账。

`PrepareRoute`／`prepareRoute`／`prepare_route` 接受 1..5 个明确且连通的池身份；当前报价支持 LaunchLab、CPMM、CLMM、Whirlpool、DLMM。每跳按上一跳的保护净到账重新计算自己的金额，不借用用户已有股票余额。返回单次交易的 ATA 设置、交换指令、各跳金额和预计中间币余额。固定 exact-in 金额配合滑点保护可能留少量股票余额；接口明确报告该余额。重复使用同一个池、资产循环、断开的路径、零保护输出、未知报价协议均拒绝。买入和卖出需要调用方分别提供路径；接口不会组合套利或自动反转历史买入。

三语言示例 `examples/cached_clmm` 和 `examples/cached_route`（Go 目录，Node.js `.ts`，Python `.py`）只在 `--simulate` 时调用模拟 RPC；没有发送交易的路径。统一输入为 `accounts[{pubkey,owner,data,slot,write_version}]`、`payer/amount/read_slot/epoch/unix_timestamp/recent_blockhash`。单池另有 `pool/input_mint/output_mint`，多跳使用 `legs[{pool,input_mint,output_mint}]`；金额/slot/epoch/time 为十进制字符串。

三种 parser 的 `stonkfun_snapshot_refresh` gRPC 示例已支持上述完整 `accounts[]`，包含 tick/bitmap/mint/config。`--require-pool-update` 在多跳时要求每个池都收到新版本；Clock-only 更新会保留未变账户原来的 slot，trade 的 freshness 检查仍会拒绝过期状态。新增 initialized array 的发现和订阅扩展仍由调用方管理，不能将初始化时的账户列表永久视为完整流动性集合。

主网实测池 `49iMatQtoyabsYAQc8GafVq6aeBFVDxSRH44oiatyyw6`（股票/USDC CLMM）及 `BmQj9pBopxouHecN5rYvVLqN7a48CVndfTkhESEZzWgN`（股票/meme 内盘）：

| 路径 | 输入最小单位 | 保护净到账 | 模拟实际净到账 | 三语言结果 |
| --- | ---: | ---: | ---: | --- |
| 股票 → USDC | 10,000 | 22,964 | 23,196 | 全部 err=null，739-byte V1 |
| USDC → 股票 | 10,000 | 4,258 | 4,302 | 全部 err=null，739-byte V1 |
| USDC → 股票 → meme | 10,000 | 2,902,282,840 | 2,931,598,828 | 全部 err=null，1,188-byte V1 |
| meme → 股票 → USDC | 1,000,000,000 | 3,153 | 3,185 | 全部 err=null，1,188-byte V1 |

以上 12 次模拟均无 ALT、未发送，四组交易 wire 在三种语言中逐字节一致。买入多跳预计剩 44 股票最小单位，卖出预计剩 13，已输出在结果中。实测约 57k..62k CU（CLMM）和 126k..130k CU（多跳）。fixture 在各 trade 仓库 `examples/fixtures/clmm_mainnet_20261002.json`、`route_buy_mainnet_20261002.json`、`route_sell_mainnet_20261002.json`，对应 simulations 文件保存执行日志及实际输出。样本用于离线复现，不是实时可交易状态。

本轮 PublicNode gRPC：三语言均通过配置的 GRPC_URL/GRPC_TOKEN 获得 epoch 1047 的 Clock 与 blockhash（约 slot 452534392..452534396）。短时观察没有收到所选池更新，因此没有宣称完成了整条路线的实时账户刷新；Clock-only 刷新后过期准备必须失败。

本轮回归：Trade Node.js 1,023 项、Python 1,068 项通过；Go 全仓和 calc/subscription race 通过。Parser Node.js 134 项通过／8 项跳过，Python 112 项（含新增多跳刷新测试）通过，Go 全仓通过。Node.js 构建、类型和 trade 示例检查通过。语言包未改版本、未发布。后续仍需高层 factory/executor 集成、剩余 parser 对齐审计，以及下文列出的未覆盖协议与场景。


## 新增原生 Whirlpool、DLMM 与 SOL 结算

Whirlpool 原生整数报价覆盖 Orca 专用 tick 因子、固定／动态 88-tick array、跨 tick、adaptive fee 参考值衰减／分组波动／上限。**800** 组 Rust orca_whirlpools_core 2.1.1 对照通过。冻结缓存 `prepareWhirlpool`／`PrepareWhirlpool`／`prepare_whirlpool` 验证 pool/mint/oracle、tick array owner／PDA／池归属／tag／bitmap／freshness。支持最多 6 个 array；只用 1 或 2 个时重复最后一个已存在地址填满 swap_v2 的三个固定位置，不构造不存在的账户。

DLMM 原生 `dlmmSwapExactIn`／`calc.DlmmSwapExactIn`／`dlmm_swap_exact_in` 按 plain Q64.64 价格遍历 bin，支持动态波动费用、filter/decay、两种收费模式及相关方向的 processed/open limit orders。**420** 组 Rust meteora-dlmm 0.2.0 对照通过，含边界、负 bin、缺失 array、完整与部分填充。冻结缓存 `prepareDlmm`／`PrepareDlmm`／`prepare_dlmm` 验证 status、activation slot/time、function type、实际费率、当前 epoch mint、bin PDA／pool identity、基础及扩展 bitmap。只有 bitmap 证明为空的区间才按空流动性遍历；缺失、关闭、过期账户直接拒绝。报价必须完整消耗输入且有正保护输出。Whirlpool/DLMM 两侧转账费使用当前 mint 配置，路线下一跳只花上一跳保护净到账。

单池示例：`examples/cached_whirlpool`、`examples/cached_dlmm`（Go 目录，Node `.ts`，Python `.py`）。多跳统一使用 `examples/cached_route`。普通 WSOL 输入／输出使用用户 WSOL ATA；不自动用 SOL 充值，也不关闭已有 ATA。

SOL 使用显式 `settleCachedRouteWithNativeSol`／`SettleCachedRouteWithNativeSol`／`settle_cached_route_with_native_sol`，只接受恰好一个 SOL endpoint。输入路线的该端 mint 必须 WSOL。调用方冷启动提供当前 `rent_lamports` 和每笔唯一的 `temporary_wsol_seed`（1..32 UTF-8 bytes）；函数本地创建临时 Token 账户、InitializeAccount3、SyncNative，将相关 WSOL 交换账户替换为临时账户，最后仅关闭该临时账户并返还租金。SOL 买入充值 rent＋第一跳输入，SOL 收款仅准备 rent；原始路线不被修改。主网本次 165-byte Token 账户租金为 **1,488,440 lamports**，不要沿用旧常量 2,039,280。`cached_route` JSON 可选 `native_input`／`native_output`、`temporary_wsol_seed`、`rent_lamports`；普通 WSOL 则两个 native flag 都为 false 或省略。

2026-10-02 主网模拟证据（所有交易仅模拟、没有发送、V1 无 ALT）：

| 路径 | 最小输出 | 模拟实际输出 | 三语言结果 |
| --- | ---: | ---: | --- |
| Whirlpool USDC 10,000 → WSOL | 保存快照计算 | 82,096 / 82,063 / 82,063 | 全部 err=null，678 bytes |
| Whirlpool SOL 1,000,000 → USDC → 股票 → meme | 34,817,361,217 | 35,169,051,734 | 全部 err=null，1,685 bytes |
| meme 1,000,000,000 → 股票 → USDC → Whirlpool SOL | 25,615 | 25,866 lamports | 全部 err=null，1,686 bytes |
| DLMM USDC 10,000 → WSOL | 保存快照计算 | 81,749 | 全部 err=null，697 bytes |
| DLMM SOL 1,000,000 → USDC → 股票 → meme | 34,992,011,735 | 35,345,466,398 | 全部 err=null，1,702 bytes |
| meme 1,000,000,000 → 股票 → USDC → DLMM SOL | 25,619 | 25,878 lamports | 全部 err=null，1,703 bytes |

DLMM 六次三跳模拟最终报价和实际输出精确相同；Whirlpool 保存报价与部分实际输出有价格变化差异，使用 parser 事件中的执行前 sqrt price 重算，九个执行结果在三语言回归中全部精确匹配。事件价格重算使用保存的相同 liquidity，不能假定事件提供完整池状态。三种语言构建单池及 SOL 双向路线的 wire 字节完全相同。普通 `meme → 股票 → USDC → WSOL` 也通过三语言主网模拟，保留 WSOL ATA；普通 WSOL 买入在实测钱包余额不足时被链上正确拒绝，不能将其报告为成功实测。

证据保存在各 trade 仓库 `examples/fixtures/{whirlpool,dlmm}_mainnet_20261002.json`、`*_mainnet_simulations_20261002.json`、`whirlpool_execution_replays_20261002.json`、`sol_route_{buy,sell}_mainnet_20261002.json`、`dlmm_sol_route_{buy,sell}_mainnet_20261002.json`。冷启动与显式模拟可使用 RPC；所有报价、账户选择、指令构建、SOL 结算和 V1 编译均没有 RPC。

三种 parser PublicNode gRPC 本次真实收到 DLMM 池及活动 bin array 更新：Clock/pool slot **452558748**、epoch **1047**、blockhash slot **452558748**，均 `pool_updated=true`。未更新的 mint 和远端 array 保留冷启动 slot；32-slot 严格读取仍拒绝旧 mint。此示例证明增量订阅与过期保护，不宣称账户集合来自同一原子 bank，也不通过伪造 slot 掩盖未更新账户。订阅重连完整性、静态账户 freshness 策略和新 array 发现仍须调用方管理及进一步完善。

剩余对齐项：AMM v4 等未接入缓存路线的兑换协议、高层 DexType/factory/executor/V1＋SWQOS、剩余 parser normalized fields/filter/BlockMeta，以及完整安装包检查和 native 发布。以上能力不等同于全部 Rust 功能已对齐；本次未变更语言版本、未发布 native 包。

验证汇总：trade Node.js 2,300 项、Python 2,344 项通过；Go 全仓、calc/subscription race 通过。Parser Node.js 134 项／8 skipped、Python 112 项、Go 全仓通过。Node 构建／类型／trade 示例检查通过，六仓 diff --check 无空白错误。

包验证：Python wheel 已构建并从独立 /tmp 安装目录成功导入新增原生报价／缓存／SOL API；Node npm pack dry-run 包清单包含新增模块，构建产物的根导出也通过检查。安装检查没有调用 Rust 编译器或 Rust trade/parser SDK。


## 2026-10-02：缓存高层入口与真实 BlockMeta

本节更新之前记录中的高层入口／BlockMeta待办。

三种 trade 语言新增 `CachedTradeRequest`、`PreparedCachedTrade`、`CachedTradeExecutor`，以及 factory 的 `createCachedExecutor`（Node）、`create_cached_executor`（Python）、`CreateCachedExecutor`（Go）。`TradingClient` 也提供显式的 prepare／execute cached 方法；Go 使用 `pkg/trading.TradingClient`。支持 LaunchLab、Bonk、StonkFun 内盘、CPMM、CLMM、Whirlpool、DLMM。买入以最后一跳、卖出以第一跳为交易池；验证协议和当前缓存，StonkFun 还验证真实 platform，不能只凭共享 LaunchLab program 归属。

`CachedTradeRequest` 需要调用方自己的 payer、amount、独立 Buy/Sell 方向、显式有序 hints、冻结 snapshot、read context（slot/epoch/maximum age）、unix timestamp、recent blockhash。支付／收款 mint 来自路径两端；SOL 另需显式 native flag、每笔唯一 seed 和冷启动取得的 rent。默认 V1 compute limit 300,000、loaded accounts data 64 MiB，可覆盖；签名器和 raw-wire transport 由调用方提供。transport 返回签名必须匹配已签交易，返回 `submitted=true, confirmed=false`，不会自动 RPC 查询或伪造确认。旧通用 buy/sell 参数入口没有自动接入新增协议；应使用 cached 方法。Node 旧 factory 中空签名假成功已改为明确报错。现成 SWQOS provider 的 V1 raw-wire适配仍待补齐，不能直接把 V1 wire 解码成 legacy/v0 transaction。

三种 trade 完整示例 `examples/cached_trade`（Node `.ts`、Python `.py`、Go目录），输入显式 `dex_type` 和 `trade_type`，离线构建；仅 `--simulate` 调用模拟 RPC，没有发送路径。本次真实主网模拟六次（全为 err=null）：SOL 1,000,000 → DLMM USDC → CLMM 股票 → StonkFun meme，保护输出 **34,822,193,446**；独立 meme 1,000,000,000 → 股票 → USDC → DLMM SOL，保护输出 **25,725 lamports**。六次消耗约160k／163k CU。三语言相同快照生成的交易 wire 完全相同。冷启动账户slot452586354；模拟仅说明保存状态和该时刻执行有效，不能把历史快照用于当前热路径。输入和模拟证据：`examples/fixtures/cached_trade_{buy,sell}_mainnet_20261002.json`、`cached_trade_mainnet_simulations_20261002.json`。

三种 parser 的 BlockMeta 已从类型定义接通真实 blocks_meta订阅、事件输出和订阅更新／重连所用请求。仅显式 event filter 包含 BlockMeta 时订阅；银行 block_time转换到微秒并按i64饱和，缺失时使用transport时间。事件提供slot、时间、recent blockhash；不会RPC查区块。Node和Go补上LaunchLab／StonkFun协议别名（Python已有），共享program别名只控制订阅，不证明平台归属。

`examples/block_meta_stream` 三语言通过已有 GRPC_URL／GRPC_TOKEN 从 PublicNode 免费gRPC实测成功：Python slot452587500、Node452587503、Go452587528。证据在各parser的 `examples/fixtures/block_meta_grpc_20261002.json`，没有凭据。BlockMeta不包含epoch或完整池状态，仍需Clock及账户订阅；多账户更新并不保证同一bank原子快照。

回归：Trade Node **2,312**、Python **2,355**；Parser Node **138 passed / 8 skipped**、Python **114**；Go两仓全量及trading／solparser race通过。Node build/typecheck/trade examples、parser migration检查通过。新增Python wheel在独立目录安装并导入高层API与BlockMeta，Node构建根／trading导出和npm pack dry-run通过；未使用RustSDK或编译器。

仍未完成全部Rust功能对齐：AMM v4等其它协议的缓存路线、完整旧buy/sell参数与V1 SWQOS provider集成、剩余parser normalized字段／平台过滤审计，以及静态账户freshness策略／订阅重连状态完整性／新tick与bin array发现。StonkFun高层标签目前是内盘LaunchLab，迁移后的外盘应显式选实际DEX并使用已知迁移池；不能宣称该标签自动覆盖所有外盘。语言版本未变更、未发布。


## 2026-10-02：AMM v4 V2 缓存路线与 parser 对齐

之前的AMM v4缓存待办已完成。新增三语言 `CachedAmmV4State`、`AmmV4Quote`、冻结snapshot的 `ammV4`／`amm_v4`／`AmmV4` 和 `prepareAmmV4`／`prepare_amm_v4`／`PrepareAmmV4`。根导出包含native准备和报价API，Node trading子路径也导出。统一cached route支持AMM v4；cached factory现支持 **8** 个DEX类型，含 `RaydiumAmmV4`。Go完整示例已更新协议分支。

报价和构建使用官方V2 exact-in（tag16，8账户）；不需要OpenBook账户，不需要router。池752bytes、经典SPL mint82bytes和vault165bytes严格检查；验证pool owner、hint两侧mint、vault身份／authority／状态、authority nonce、decimals、池status及WaitingTrade开盘时间、实际swapfee分子／分母。准备使用vault余额扣除待提取PnL的储备，ceil输入fee、floor输出和保护值，不把输入单位fee从输出代币单位再减一次。Token-2022不适用于该协议。全部热路径无RPC。

数学验证 **480** 组向量，直接调用Rust5.0.6的 `compute_swap_amount_for_pool` 源码作为开发对照，涵盖两方向、实际fee、u64极值和滑点；运行和安装不依赖RustSDK。另有 **12** 条主网模拟 `ray_log` 执行前coin／pc余额重算，三语言全部精确匹配实际模拟输出。模拟前后池价格会变化，不能把保存快照预测与所有后续模拟输出强行视为相同。

三语言 **12** 次主网模拟全为err=null，单池SOL↔USDC和独立SOL↔USDC↔股票↔meme买入／卖出；三语言同方向wire完全一致。单池buy保护USDC **120,516**，sell保护SOL **80,917 lamports**；三跳buy保护meme **34,593,698,018**，sell保护SOL **25,804 lamports**。单池约23k CU，三跳约141k／144k CU。仅模拟，没有发送真实交易。冷启动slot452593963。

完整用法：各trade `examples/AMM_V4_CACHE.md` 与 `examples/cached_trade`；新增 `examples/fixtures/amm_v4_{buy,sell,route_buy,route_sell}_mainnet_20261002.json` 和 `amm_v4_mainnet_simulations_20261002.json`。历史snapshot只用于离线复现；当前模拟／交易必须使用真实当前状态和自己的钱包／金额／支付收款路径，SOLseed每笔唯一。

修复三语言旧AMM752-byte解码器：四个swap计数是u128，不是u64，原来会让后续vault/mint/market地址错位32bytes。Node bigint、Python int完整保留128bits；Go四个公开counter字段改为 `*big.Int`，调用方需调整旧uint64读取代码。这是本轮明确的Go源接口变化，不能将其误称为完全无破坏更新；本轮仍未改语言版本或发布。

Parser普通指令事件与正常instruction candidate筛选补上V2 tag16／17及8账户映射，保留旧tag9／11的17／18账户路径。exact-in是输入／最小输出，exact-out是最大输入／精确输出；V2缺失的OpenBook字段保持默认，不能拿用户账户冒充market。Ray log账户填充补齐V2token program、authority、vault、source／destination及user，按已知事件pool锚定；无pool时仅接受唯一账户上下文，多调用冲突留空，不按最长账户列表猜测。原有事务route识别已经支持V2，本轮补的是普通事件和填充链路。

三语言PublicNode gRPC都实测收到AMM pool更新：Python slot452597661，Node／Go452597664，epoch1047，pool_updated=true。pool+Clock更新不会把静态mint旧slot刷新成新slot；三语言离线trade随后正确拒绝过期mint，无RPC补齐。各parser证据 `examples/fixtures/amm_v4_grpc_refresh_20261002.json`。

本轮完整回归：Trade Node **2,820**、Python **2,863**；Parser Node **143 passed / 8 skipped**、Python **119**；Go两仓全量、subscription/trading/solparser race通过。Node类型／trade示例／build、parser migration检查通过。Node新AMM根／trading构建导出与npm pack dry-run、Python独立安装wheel原生AMM／parserV2导出检查通过。

仍未全部对齐Rust：其它未接入缓存路线的协议、旧通用AMM buy/sell builder到V2与实际fee配置的迁移、V1 SWQOS provider raw-wire适配、剩余parser normalized字段及完整重连／静态账户freshness／新array发现策略。旧通用AMM builder仍使用旧账户布局；本轮验证路径是explicit cached入口，不能混用或宣称旧入口也已完成迁移。缓存route本轮支持exact-in；parser能识别exact-out不代表cached交易已支持exact-out。未改语言版本、未发布native包。

## 2026-10-02：旧 AMM 入口 V2 与 V1 SWQOS 适配

三语言旧 AMM v4 buy/sell builder 已迁移为 V2 tag16／17、8账户，不再要求 OpenBook 市场账户。旧 tag9／11 常量保留用于历史解码。方向由请求的输出 mint（buy）／输入 mint（sell）决定；支持任意股票/token 池和两侧均为 WSOL/USDC 的池，不再用“是否含 WSOL/USDC”猜方向。高层 Python 参数补齐实际 fee 和 input mint 传递；Node 根入口传递支付 mint。传入储备必须已经扣除待提取PnL，实际 fee 默认25/10000仅用于兼容旧调用；推荐从经过验证的当前 cache state 补齐。exact-in fee向上取整、输出及滑点保护向下取整。exact-out tag17使用最大输入／精确输出；本轮只补齐旧单池构建器，缓存route仍仅支持exact-in。

Node 主网模拟发现普通 ATA 创建在已有钱包账户时以 `IllegalOwner` 失败；已改为幂等创建，并增加回归测试。三语言新增 `examples/legacy_amm_v2`（Go）／`.ts`／`.py`，从冻结cache校验池、mint、vault、实际fee及PnL，再调用旧买卖入口并本地编译V1。仅显式 `--simulate` 调用RPC；没有send路径。示例Buy的native_input=true会将SOL转入钱包WSOL ATA；Sell收到WSOL，不关闭已有ATA。需临时账户SOL结算或多跳时使用原来的cached_trade示例，不能把这个旧ATA生命周期视为完全等同于缓存SOL生命周期。

冷启动slot **452630067**，三语言独立buy／sell的exact-in、exact-out共 **12** 次主网模拟均err=null，同一形态wire完全一致：buy605bytes、sell538bytes。exact-out测试buy目标50,000 USDC最小单位、最大输入1,000,000 lamports；sell目标30,000 lamports、最大输入10,000 USDC最小单位。仅模拟，没有真实发送。账户快照 `examples/fixtures/legacy_amm_v2_{buy,sell}_20261002.json`，执行证据 `legacy_amm_v2_simulations_20261002.json`，包含发现并修复的ATA问题。

SWQOS low-level客户端原来只按legacy/v0头部取签名；三语言已识别V1 message-first、signature-last布局，校验配置mask、账户／指令边界和完整长度，保持原始wire字节。新增Node `createCachedWireSubmit`、Python `create_cached_wire_submit`（swqos模块）、Go `swqos.NewCachedWireSubmit`，直接接入缓存执行器submit回调，保留Buy/Sell、复制字节、固定wait_confirmation=false。签名布局在发送之前校验；目前适配器支持单签名。缓存执行器仍核对返回签名并返回submitted=true／confirmed=false，不做RPC确认轮询。

模拟HTTP测试证明原始V1字节被正确base64转发、返回尾部签名、没有额外RPC请求。并未真实向每个SWQOS服务发送V1，服务端版本／tip策略仍需实际服务支持；旧只接受VersionedTransaction对象的provider包装器不能用来重新序列化V1。使用示例见 `examples/CACHED_SWQOS.md`。

本轮trade回归Node **2,830**、Python **2,871**，Go全量及instruction／swqos race；Node类型、示例、build及npm pack dry-run，Python wheel独立安装及原生导出检查通过。全量回归发现旧age测试的100ms真实定时器偶发提前1ms，已改为受控Date.now断言，不降低判断标准。本轮parser代码未改，保留上一轮Node143／8skipped、Python119、Go全量验证记录，不把它记成本轮重跑。

剩余完整Rust对齐工作：其它未接入cache的协议、parser剩余normalized字段及平台过滤审计、订阅重连／静态账户freshness／新tick-bin array发现策略，以及各SWQOS服务实际V1兼容性。此次没有改版本、发布native包或创建GitHub Actions。

## 2026-10-02：缓存交易显式 tip 与模拟执行证据

三语言 CachedTradeRequest 新增 Node `tipAccount`／`tipLamports`，Python `tip_account`／`tip_lamports`，Go `TipAccount`／`TipLamports`。tip账户来自调用方已配置的low-level SWQOS client本地列表，金额以整数lamports明确提供，不调用RPC查询。构建器在业务指令前加入System Program支付，与Rust V1构建顺序一致；临时WSOL生命周期及路径不变。不配置tip保持原交易；账户／金额必须成对，拒绝零额、默认账户、自付和u64预算溢出。requiredNativeLamports／required_native_lamports包含临时账户资金与tip，仍不含基础交易费和其它ATA租金。支付资产为USDC或WSOL时，也需预留tip所用SOL；保护到账金额不做套利或利润计算。

三语言 cached_trade完整示例均读取可选tip字段，模拟显式启用innerInstructions。CACHED_SWQOS.md已补齐签名前设置tip并接入发送适配器的代码；构建器与发送适配器不会重复添加tip。此次仅模拟，没有向SWQOS服务实际发送，不据此宣称所有服务已线上接受V1。

冷启动slot **452634063**，5000 lamports tip，三语言SOL↔USDC单池以及独立SOL↔USDC↔股票↔meme三跳买卖，共 **12** 次主网模拟均err=null。同一形态wire完全一致，模拟tip收款账户余额增加5000，tip不进入WSOL充值记录。快照 `examples/fixtures/cached_tip_{buy,sell,route_buy,route_sell}_20261002.json`；执行证据 `cached_tip_simulations_20261002.json`。历史样本只用于复现，当前执行须使用当前冻结缓存与独立临时账户seed。

模拟发现parser接入缺口：模拟RPC内部转账返回jsonParsed；原compiled/base64 route API有意拒绝jsonParsed。Node新增 `analyzeSimulationRoutes`，Python新增 `analyze_simulation_routes`，接受原始wire和simulateTransaction响应，无RPC也不重新序列化交易。专用适配器支持可还原的SPL普通／checked／明确fee转账，保留raw和compiled CPI、位置／stack height，按当前wire账户集合严格校验。未知parsed指令明确报错，不静默跳过。V0含ALT时不猜lookup地址，需使用原compiled输入与显式loadedAddresses。调用方负责配对同一笔wire／模拟响应；此入口处理受信任模拟证据，不验证服务器提供证据的真实性。

专用入口保留失败交易意图，失败时实际成交字段为空；Token-2022 checked转账未给手续费时，净到账仍未知，只有明确fee才从毛额扣除。单池实际买入输入1,000,000 lamports、输出121,561 USDC最小单位；卖出输入10,000 USDC最小单位、输出81,852 lamports。不同模拟银行价格会变化，不能把保存快照估算当作后续执行结果。三跳各leg实际输入与其本次指定金额一致，未知净到账不会伪填。示例 `examples/simulation_routes.ts/.py` 可离线读取保存证据；新增16项回归覆盖主网样本、tip、fee、失败意图、raw/compiled CPI及错误元数据。

**上一轮状态（现已补齐，见后续章节）：Go parser的专用模拟结果适配入口尚未完成**：PreToolUse安全钩子连续把普通Go源码补丁误识别为仓库转移（误报目标先是依赖包，后是users/wangwei），拒绝写入。没有真实仓库转移，没有修改安全政策或审批记录。Go trade的tip及主网模拟已完成；Go parser仍提供原有compiled/base64 route API。不能把新增Node/Python入口写成三语言全部完成。

本轮版本未修改，native包未发布，未创建GitHub Actions。其它未接入cache的协议、完整订阅重连／静态账户freshness／新array发现、parser其它字段审计及真实SWQOS服务验收仍在对齐清单。

本节最终验证：trade Node **2,840**、Python **2,881**；parser Node **159 passed / 8 skipped**、Python **135**；Go两仓全量及trade trading／swqos race通过。Node trade类型／示例／build，parser migration／build及两仓npm pack dry-run通过；Python两仓wheel在独立/tmp目录安装并导入tip／SWQOS／simulation原生API通过。Node/Python四个离线模拟解析示例输出一致（u64按十进制字符串比较）。六仓diff --check无空白错误。Go parser原有全量测试通过不代表受阻的新模拟适配器已实现。

## 2026-10-02：Go 模拟适配补齐与当前银行回归

Go parser 新增 `AnalyzeSimulationRoutes(wire, simulationResponseJSON, graduatedPools)`，补齐 Node/Python 已有专用入口。只解析调用方提供的原始 wire 和完整 simulateTransaction 响应，无 RPC、Rust 依赖或签名交易重新构建。支持 parsed SPL transfer／transferChecked／Token-2022 transferCheckedWithFee，保留 raw／compiled CPI、指令位置和 stackHeight；校验 wire 账户归属、整数范围、重复组、失败元数据及 JSON 尾随内容。未知 parsed 指令显式拒绝。含 ALT 的 V0 仍需原有 compiled API 与显式 loadedAddresses；此入口不查询 ALT。Token-2022 未明确手续费时净到账保持未知，失败交易保留意图但清空实际成交字段。

新增完整离线 Go 示例 `examples/simulation_routes/main.go`；三语言 `examples/SIMULATION_ROUTES.md` 同步说明。之前的安全钩子误报此次未重现，普通补丁成功写入；没有更改安全政策或审批记录，上一节 Go 未实现的描述仅为历史状态。

当前主网用保存 wire、`sigVerify=false`、`replaceRecentBlockhash=true`、`innerInstructions=true` 重新模拟四笔独立交易，没有真实发送。slot **452643845..452643853**：buy 与 route-buy 因当前价格触发原 wire 的滑点保护（AMM custom 30）；sell 与 route-sell 成功。这里验证的是原 wire 在当前银行执行后的 parser 证据，不是重新报价交易。三语言解析输出完全一致，失败交易实际 input/output 均 null，成功交易保留真实转账与未知手续费。旧报价不应直接作为实时交易报价；账户冷启动本轮两次遭遇 RPC 响应截断，未将旧缓存伪造为新 slot，也未放宽滑点。

新增三语言 `simulation_routes_live_20261002.json` 执行证据与回归，各 parser 的测试及 examples/fixtures 均有副本。原 cached_tip_routes 四种样本也全部与 Go 输出一致。全量验证：parser Node **163 passed / 8 skipped**，Python **139 passed**；Go全仓、`go vet ./...` 和 parser race通过。Go离线示例及三语言当前银行示例输出一致。此次未改版本或发布包；完整 Rust 功能对齐仍有前文列出的协议缓存、订阅一致性和其它字段审计缺口。

## 2026-10-02：首次 ATA CPI 与执行状态审查修复

本轮重点审查模拟适配、核心route成交归属和相关离线示例，使用code-review技能／OCR delegate读取文件范围与规则，由当前Codex审查。三语言模拟适配此前拒绝首次建ATA的parsed CPI，导致成功交易的模拟证据无法解析。现支持 System createAccount、SPL getAccountDataSize、initializeImmutableOwner、initializeAccount3 的原生指令布局重建，保留原指令索引和stackHeight；初始化mint参与后续unchecked transfer识别，setup不计入swap成交转账。getAccountDataSize只接受空列表或immutableOwner扩展，其它扩展明确拒绝；仍未宣称支持全部parsed指令。输入owner必须是32字节公钥，lamports/space校验u64范围，所有指令账户必须存在于原wire。

审查修复另一项正确性问题：三语言 compiled route API 以前将缺失meta.err当作成功，现要求执行状态显式存在。完整RPC／模拟输入不受影响，不完整自构meta须补齐真实状态，不能为了通过校验伪填null。Node/Python同时补齐非法parsed/group/tokenAmount结构的显式错误，避免偶发属性访问异常。

真实主网四项模拟均成功且三语言parser输出一致：classic SPL ATA（slot452648322），附带首次建ATA的单池卖出（452648959），三跳卖出（452648963），Token-2022股票ATA（452650145）。首次ATA由现有payer支付、新owner接收；卖出仍为独立交易，没有构造套利闭环。这些模拟只用于验证setup CPI与route解析，没有真实sendTransaction；没有热路径RPC或Rust安装／运行依赖。证据见各parser `simulation_ata_20261002.json`（测试fixture与examples/fixtures都有副本）。额外单测去除token balances，验证initAccount3仍能恢复mint与后续transfer的位置。

全量验证：parser Node **173 passed / 8 skipped**，Python **151 passed**，Go全仓／vet／parser race通过；Node migration/build通过。最终新增fixture的离线三语言输出一致。此次未修改版本或发布native包。

审查覆盖清单（root `NATIVE_REVIEW_20261002.json/.md`）：OCR三个parser仓库preview共72个reviewable文件，本轮审查9个、未审63个，coverage_rate **12.5%**；另检查3个默认排除的测试文件。未审文件逐项记录为本轮模拟／route审查范围之外，不将本轮结论扩大为全SDK无缺陷。完整Rust对齐剩余协议缓存、订阅一致性和其它字段审计仍在前文清单。


## 2026-10-03：十轮审查与修复

完成六个原生SDK的十轮限定审查：wire/compiled索引和CPI元数据、预充值ATA模拟CPI、冻结cache构造、关闭账户识别、LaunchLab重复解码、SOL/WSOL生命周期、通用LaunchLab平台、V1签名元数据和raw-wire提交边界。第7轮无新确证缺陷。StonkFun入口仍验证平台归属；通用LaunchLab/Bonk route使用实际pool配置。Node/Python根入口已导出通用LaunchLab解码／指令构建API。Go提交取消时不调用transport，三语言核对返回签名。没有热路径RPC或真实交易发送。

全量验证：trade Node2850、Python2894；parser Node190 passed/8 skipped、Python168；Go两仓test/vet和相关race通过。通用平台合成快照和真实历史三跳卖出完整示例三语言wire一致。预充值ATA主网模拟slot452657875、三跳卖出重放slot452663400成功；parser三语言输出按十进制整数规范化后一致。通用非Stonk平台尚仅离线验证，历史快照重放不等于当前快照重新报价。SWQOS只mock验证，不宣称服务商已接受。

审查覆盖：最终preview reviewable190，完整审查31，跳过159，16.32%；partial客户端片段未计整文件审查。每轮与逐文件原因见工作区根 NATIVE_REVIEW_10_ROUNDS_20261003.md/.json，模拟证据见同名前缀_EVIDENCE.json。完整Rust功能对齐的其它协议cache、订阅一致性和其它parser字段审计仍待继续。此次未修改版本、提交或发布。


## 2026-10-03：第二批十轮审查修复

完成第二批十轮限定code-review：CPMM authority与零保护金额、AMM v4、CLMM/Whirlpool/DLMM缓存、Token-2022 epoch、迁移registry、BlockMeta、原生hop、WebSocket生命周期。AMM v4与Whirlpool未发现新确证缺陷。其余确证问题已修复：CLMM/DLMM覆盖合法稀疏范围，bitmap证明空区间且缺失已初始化array明确拒绝；CPMM验证vault authority、prepare拒绝零min-out；Node精确整数/boolean参数验证；Node/Python迁移先走统一执行状态和索引校验；CLMM错误bitmap参数拒绝。

Node WebSocket同key替换串行、unsubscribe合并、旧callback忽略；Go补齐服务端ack/ID/标准通知与真实unsubscribe、并发写锁及幂等断线重连；Python用既有aiohttp补齐实际WebSocket连接、订阅、通知、重连与资源清理。通过本地WebSocket集成及Go race验证，不等于线上provider验收或完整gRPC/cache分叉一致性验证。

全量：trade Node **2859**、Python **2900**；parser Node **192 passed / 8 skipped**、Python **174**；Go两仓test/vet与相关race通过，Node类型/示例/build通过。新CPMM快照slot452680745，独立买/卖主网模拟slot452680763/452680767成功，三语言wire689bytes完全一致，parser输出一致。卖出净到账证据不足保留null；不将快照报价当真实净到账。之前延迟快照买入及历史三跳重放触发滑点保护也保留在证据中，没有放宽保护或真实发送。

完整快照：trade examples/fixtures/batch2_cpmm_{buy,sell}_20261003.json；模拟输入：六仓examples/fixtures/batch2_cpmm_simulations_20261003.json。历史快照供离线复现，当前执行须使用当前冻结状态与调用方资产/金额。热路径没有RPC或Rust依赖。

覆盖：OCR reviewable196、完整审查30、跳过166、15.31%。根目录NATIVE_REVIEW_BATCH2_10_ROUNDS_20261003.md/.json记录每轮范围及逐文件原因，_EVIDENCE.json保存主网成功/失败证据。确证问题均修复，未审文件不宣称无缺陷；其它协议cache、静态freshness、fork一致性、新array发现与其它parser字段对齐仍是范围外工作。未改版本、提交或发布。


## gRPC 接入范围纠正（2026-10-03）

与 Rust sol-parser-sdk 对齐：实时事件/账户/Clock/BlockMeta 来自 Yellowstone gRPC；parser 的订阅句柄管理 gRPC 流，没有 WebSocket 订阅接口。trade cache 通过已有原始账户适配API写入 gRPC 数据，冻结快照后本地报价/构建。第二批第10轮的 WebSocket 修复只属于 trade 通用兼容模块，不能作为 parser 或 gRPC 生命周期已对齐的证据。具体接口、完整示例和就绪条件见 examples/GRPC_CACHE.md。


## 2026-10-03：对齐计划实施中

固定 parser 0.7.7 / trade 5.0.6 的 tag commit；新增声明候选索引，名称匹配没有标记验收。三语言补事件别名、精确元数据、单 DEX gRPC 生命周期和缓存 generation/fork 就绪门控；候选池支持确定性的最少跳数逐个验证，显式路径优先；factory 完整请求进入 cached 签名/提交核心。Node parser gRPC 改为纯 JavaScript grpc-js，移除 Yellowstone Rust N-API 依赖，通过本地实际 TCP gRPC、PublicNode BlockMeta 和 npm 独立安装检查。PumpFun 去除旧小额固定输出、修复宽整数中间值，与固定 Rust 源码生成的 26 组 buy/sell 结果一致；PumpSwap FeeConfig 验证 discriminator。

本轮全量：parser Node199 passed/8 skipped、Python185；trade Node2898、Python2937；Go 两仓 test/vet、parser 全包与 trade 相关 race 通过。新增 wire 验证使用历史快照和本地提交适配器，没有真实广播。两 Python wheel 独立安装及公开 API 验证通过。

整体仍未完成：完整公开 API/字段审核、静态状态/fork恢复/新array自动订阅、PumpFun/DAMM v2统一缓存，以及 PumpSwap cashback/非零转账费、全部资产模拟示例、各 SWQOS 服务协议验收及发布准备。实时缓存必须将订阅中断/丢弃/冲突接入 readiness.interrupt；markValidated 需基于真实状态验证，不能仅因缓存含该账户就调用。本轮 PumpSwap 实际状态见 NATIVE_PUMPSWAP_ALIGNMENT_20261004.md。未改版本、提交或发布。


## 2026-10-04：PumpSwap 报价与缓存状态

修复宽整数/费用错误传播/滑点边界与 exact-output 输入预算；Python 字典接口共用报价核心；新增三语言冻结缓存读取当前 pool/vault/mint/global/fee config 状态与实际费率，校验 PDA/bump、账户身份、转账能力和 freshness。Node2931/Python2971/Go test+vet+相关race通过；64个报价分支对照固定 Rust 源码。缓存账户fixture为 synthetic，没有新增主网模拟。缓存交易指令及统一route/factory接入尚未完成；整体对齐未完成。详细见 NATIVE_PUMPSWAP_ALIGNMENT_20261004.md 及对应 evidence JSON。


## PumpSwap 缓存交易与模拟更新（2026-10-04）

三语言已接入真实 cached factory/route；当前费用与收款人由冻结快照提供，报价/构建不联网。独立 SOL 买入、meme 卖出 SOL 的 V1 字节完全一致，六次主网模拟成功；对应快照、模拟响应和 parser 重放见 `NATIVE_PUMPSWAP_ALIGNMENT_20261004.md`。旧状态入口“尚未接入准备”的记录已被本次覆盖。cashback、非零转账费和其它资产矩阵仍未完成，不能据此宣称全量对齐。


## PumpFun / DAMM v2 更新（2026-10-04）

PumpFun 现在从 pool quote 与本次 SOL/WSOL 支付或收款选择 legacy/V2；非 native 资产不能误走消费 SOL 的布局。Node fromTrade 保留 quote/fee recipient/observed creator。DAMM v2 修复 248→160 费用结构偏移、referral None 固定槽位、payer 权限、Node idempotent ATA 与 bigint funding，以及 Go 非法 mode 默认成功。三语言完整账户布局通过固定 Rust Borsh oracle；新增冻结 cache 状态与显式下限准备示例，三次真实买入模拟成功、完整 V1 字节相同。此入口不提供自动报价或完整 cached route/factory。Python Pool 结构新增保留区及奖励字段，建议使用关键词构造；Go u128 原始 16 字节表示不变。详见 NATIVE_PUMPFUN_DAMM_ALIGNMENT_20261004.md。


## 公共曲线、参数、PDA 和独立安装更新（2026-10-04）

修复曲线账户公式与 Rust 的差异、Go 宽整数溢出、Node 精确储备、完整 V2 quote 解码、quote 切换和重复公共类；统一错误 program ID / discriminator / Mayhem 默认列表。修复 Node/Python 简化 PDA 派生，PumpSwap 通用池要求完整 seeds，无法确认身份的旧 helper 明确拒绝。Go 曲线计算返回 `(uint64,error)`；Python 已完成曲线抛错，运行最低 3.10，移除未使用的 anchorpy。详细 API 迁移、9 组固定源码 oracle 与验收边界见 NATIVE_CURVE_PUBLIC_API_ALIGNMENT_20261004.md。没有把这些入口认定为已完成当前费用缓存准备，整体对齐仍未完成。


## DAMM cached factory / parser route 更新（2026-10-04）

DAMM 单跳 cached factory 与 route mode/失败意图已接入；此更新覆盖历史未接入记录。两笔实际买入模拟成功、一笔余额不足失败，真实卖出未验收。整体对齐仍未完成，详见 [本阶段记录](NATIVE_DAMM_FACTORY_ALIGNMENT_20261004.md)。


## PumpFun 当前配置缓存（2026-10-04）

已新增严格 Global/SharingConfig 冻结缓存读取，Node 冷加载补当前收款人与激活分成 vault；当前是配置能力，非完整费用报价/factory。缺 SharingConfig 明确拒绝，不默认未激活。详见 [本阶段记录](NATIVE_PUMPFUN_CONFIG_ALIGNMENT_20261004.md)。


当前 PumpFun 费用、SOL/WSOL 结算与验证边界：[2026-10-04 对齐记录](NATIVE_PUMPFUN_SETTLEMENT_ALIGNMENT_20261004.md)。整体对齐仍未全部验收。


持币卖出已补真实模拟；WSOL 仍为 minimum + SOL 残余：[最新卖出验收](NATIVE_PUMPFUN_FUNDED_SELL_ALIGNMENT_20261004.md)。


Native quote 多跳现已接入，USDC/WSOL 路径通过双向真实模拟：[最新多跳验收](NATIVE_PUMPFUN_MULTIHOP_ALIGNMENT_20261004.md)。旧的全部多跳拒绝状态已被替代，完整组合矩阵仍未全部验收。

## 2026-10-05：PumpFun + Whirlpool／DLMM／CLMM 多跳实测

新增六笔 USDC ↔ WSOL ↔ PumpFun meme 的独立买卖模拟；三语言 wire、保护金额和残余资产类型一致。实际扣款、到账和原 WSOL 账户保留均已验证，缺失实际使用数组时准备失败。范围、证据及剩余事项见 [本轮验收报告](NATIVE_PUMPFUN_CONCENTRATED_ALIGNMENT_20261005.md)。这不等于全部股票路线或完整 Rust 功能已经对齐。


## 2026-10-05：连续性及冲突失效修复

新增依赖不会解除连续性中断。同版本账户冲突会使来源缓存及此前冻结快照失效，必须明确选择 fork、建立新缓存并重新验证；仅重置 readiness 无法恢复旧缓存。详见 [缓存修复与迁移说明](NATIVE_CACHE_CONTINUITY_ALIGNMENT_20261005.md)。本项是本地状态机验收，完整实时重连／重组及新数组发现仍待验收。

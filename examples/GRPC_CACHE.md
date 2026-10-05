# sol-parser-sdk gRPC → trade cache 接入

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

## 完整示例

两个 SDK 安装在各自环境中即可运行，也可分别在对应仓库运行。快照文件是两个示例间的明确接口，安装和运行不需要 Rust。

| 语言 | parser 仓库：gRPC 刷新 | trade 仓库：本地构建 |
| --- | --- | --- |
| Node.js | npx tsx examples/stonkfun_snapshot_refresh.ts /absolute/snapshot.json --require-pool-update | npx tsx examples/cached_trade.ts /absolute/snapshot.json |
| Python | python examples/stonkfun_snapshot_refresh.py /absolute/snapshot.json --require-pool-update | python examples/cached_trade.py /absolute/snapshot.json |
| Go | go run ./examples/stonkfun_snapshot_refresh /absolute/snapshot.json --require-pool-update | go run ./examples/cached_trade /absolute/snapshot.json |

使用与 cached_trade 匹配的 accounts[]、legs[] 完整快照，例如现有 cached_tip_route_* 样本；更换自己的钱包、金额和本次方向。CPMM 的 named snapshot 应交给 cached_cpmm 示例。不要直接用历史快照实时交易。

parser 刷新读取 GRPC_URL / GRPC_TOKEN，保留没更新账户的原 slot，可能因 provider 没有发送 pool 更新而超时；trade 默认不联网。需要模拟时为 trade 示例增加 --simulate，并配置 RPC_URL。冷启动与显式模拟允许 RPC，交易报价和构建热路径禁止 RPC。

本文件描述已实现的接入接口；完整自动重连恢复、fork 一致性策略、新 array 发现和全部协议缓存仍须逐项验证。之前十轮报告中的 WebSocket 修复仅属于 trade-sdk 通用模块，不计入这些 gRPC 对齐能力。


## PumpSwap 独立买卖示例

`fixtures/pumpswap_buy_mainnet_20261004.json` / `pumpswap_sell_mainnet_20261004.json` 可交给本目录 cached_trade 示例离线重建；分别是 SOL→meme 和 meme→SOL。三语言使用同一 JSON，参数包含完整 pool/mint/vault/GlobalConfig/FeeConfig 状态、实际 owner/slot/write_version、独立方向、钱包、金额、保护比例和 SOL 结算设置。

这些是历史验证快照。实时运行须先冷准备缺失账户，再使用 parser 的 stonkfun_snapshot_refresh gRPC 示例（名称不限制协议）刷新，读取 GRPC_URL/GRPC_TOKEN，不打印凭据。未收到静态更新不能伪造 slot；断线/fork 不明确时重新验证，禁止热路径 RPC 补齐。Pool 更新不等于全部依赖已就绪。

显式 `--simulate` 使用 RPC_URL 调用模拟，不发送交易。历史卖出使用已核实余额的公开账户且不持有私钥；更换钱包时必须确认真实 token ATA、余额及租金，不能把余额不足模拟记为成功。完整 parser 输出可用各语言 parser 的 simulation_routes 示例读取 `fixtures/pumpswap_mainnet_simulations_20261004.json`，无需网络。

cashback、非零转账费明确拒绝准备；本例未验收既有 WSOL 输入、USDC/股票端点或自动重组恢复。详细证据见仓库 NATIVE_PUMPSWAP_ALIGNMENT_20261004.md。


## DAMM v2 显式保护参数示例

新增 damm_v2_snapshot 示例（各语言对应扩展名或 Go 目录）。它使用完整 accounts 冻结快照校验池、mint/vault、激活与 continuity 后构建 native swap2；需要明确 fixed_output_amount，不做自动报价。历史 fixtures/damm_v2_buy_mainnet_20261004.json 可离线重放，--simulate 才访问 RPC_URL；不提供发送路径。pool 来自实际 Yellowstone 发现，依赖账户明确冷启动，不将冷 RPC 伪称为 gRPC 全量账户初始化。历史 1-atom 下限用于 unsigned 验证，实时交易须替换为自己的当前保护下限；新 cache 不以默认费率或 RPC 补齐缺失信息。

# 公共曲线、参数和 PDA 对齐修复（2026-10-04）

固定基线：sol-trade-sdk 5.0.6 / dc9741b6074df21418f16b6175f497fbebecb723。本文只验收所列入口，不代表完整六仓功能对齐。

## 已修复

| 能力 | Go | Node.js | Python | 证据 |
| --- | --- | --- | --- | --- |
| 公共曲线买价、卖价、市场市值、买尽价格、最终市值 | math/big 中间值；错误显式返回 | bigint 储备和结果 | int 储备和结果；合并重复公共类 | 固定 Rust 源码直接提取 5 个方法，9 组输入，45 个结果逐语言相同 |
| 曲线完整 V2 布局 | 读取 quote mint | 精确 bigint 解码并读取 quote mint | 精确 int 解码并读取 quote mint | 115 字节账户；u64 最大值及 >2^53 储备；扩展数据、legacy 兼容、无效布尔及截断测试 |
| parser → PumpFun 参数 | 保留 curve quote；补派生地址 | 移除 Number 储备转换；保留 curve quote | 拒绝浮点、bool、非十进制及超范围输入；保留 curve quote | 参数及整数边界测试 |
| quote 切换 | 同步 curve/layout；仅调整重建初始值 | 同左 | 同左 | SOL → USDC → SOL 及实际储备保持测试 |
| 公共 program ID / 校验表 / fee recipient 默认值 | 修正与构造器不一致的地址及 discriminator | 修正 seed/校验表和完整 Mayhem 列表 | 修正根入口/seed/校验表 | 公共入口、构造器一致性测试及已知主网 Global / event authority 地址 |
| 公共 PDA 工具 | 正确 event seed 和 PumpSwap 完整 seeds | 标准 Solana 派生，正确 domain/curve/bump；base58 前导零；缓存返回副本 | 标准 Solana 派生，正确 domain/curve/bump | 与已有 Solana 库对照及实际固定地址 |
| Python 独立安装 | 不适用 | 不适用 | 移除未使用 anchorpy，最低版本改为 3.10 | 干净 Python 3.12 环境全部依赖只安装 wheel，无 Rust 编译；网络禁止后公共 API 和历史 DAMM V1 重建通过 |

曲线账户 `get_buy_price` 是未扣费用的原始曲线价格，与 `calc` 的支付预算报价含义不同；二者不能互换。`get_market_cap_sol` 按 Rust 返回 quote 最小单位整数，不再返回除以 1e9 的展示浮点值。token price 仍是展示用浮点值。

历史 legacy 曲线允许无 quote 字段的 75 字节 Borsh body / 83 字节带 discriminator 账户；完整 V2 是 107 / 115 字节。缺失 legacy quote 以各入口 native 约定表示；不是证明 USDC 或其它 quote。带已知 discriminator 的截断 V2 不作为 raw body 接受。所有实际账户准备仍须校验 owner、mint、vault、epoch、当前状态和连续性。

## API 迁移

- Node 根入口提供可实例化的 `BondingCurveAccount`；储备、价格、金额使用 bigint。安全 Number 只允许在 parser 兼容输入处显式转换；不安全 Number 会拒绝。公共曲线和原生 calc 的含义分别如上。
- Go `GetBuyPrice`、`GetSellPrice`、`GetBuyOutPrice`、`GetFinalMarketCapSol` 返回 `(uint64, error)`；调用方必须处理错误。新增 Checked 别名委托相同核心，不再吞错误为零。
- Python 根入口、common.types、common.bonding_curve 使用同一个曲线类；已完成曲线买卖抛出 ValueError。最低 Python 版本是 3.10，运行不需要 anchorpy。
- 通用 PumpSwap 池派生增加 index(u16) 与 creator；不能沿用原来的两 mint seeds。交易路径仍优先使用 parser/订阅已观测、经验证的池地址。
- 旧 fee-recipient PDA 帮助函数明确拒绝：收款人应来自当前 Global 配置。旧 DAMM 两 mint 帮助函数也明确拒绝：缺少实际池/配置身份不能生成交易池地址。
- Node 移除并不存在的 PumpSwap `SWAP` discriminator，使用 `BUY`、`BUY_EXACT_QUOTE_IN`、`SELL`；deposit/withdraw 取固定基线 IDL。

## 验证范围与未完成项

这是源码 oracle、离线账户/指令回归和独立安装验证。本轮未新增主网模拟；已有成功/失败的 PumpSwap / DAMM 模拟回归仍执行，未把离线回归计作新模拟。

PumpFun 当前 FeeConfig/Global/fee-sharing 的完整缓存准备与 factory 仍待接入；不能把这里的原始曲线方法或固定默认费率当成经过当前费用校验的热路径报价。DAMM 自动缓存路线、全部资产买卖矩阵、完整公开事件/API 对照、自动缓存恢复和服务商真实提交验收也未据此完成。所有 RPC 报价/构建热路径约束保持，例外仅是明确冷启动和显式模拟。

工具：工作区 tools/native-parity/generate_curve_account_vectors.py。Rust 只用于开发生成 oracle；发布包执行这些功能不调用 Rust SDK/子进程。fixture：curve_account_rust_5_0_6.json。未改版本、提交、发布、tag/Release 或真实发送。

## 后续审查修复（同日）

- 默认曲线与 Rust derived Default 对齐：储备和总供应量为零。Node quote selector 默认公钥为零；有效 quote 仍归一为 WSOL。未知状态不能伪装为新发行曲线。dev-trade/from-trade 的固定初始值和供应量保持显式重建。
- Python trading.params.PumpFunParams 直接复用根入口实现，删除错误的 field() 函数默认值和丢字段的重复类。新增根入口 from_dev_trade，支持 quote-aware 初始重建、实际 fee recipient 和 creator，严格拒绝非法整数及储备溢出。
- Python factory 保留 curve_quote_mint，缺少显式支付／收款 mint 时使用有效曲线 quote；不再把 USDC 曲线误默认为 SOL。独立买、卖与原生 builder 的完整指令账户/data 对照通过；随机 buyback recipient 在测试中固定，生产保留基线策略。
- Python 和 Go 补齐公共曲线的 quote normalization、effective quote、储备访问、quote-aware 重建、creator vault PDA 入口和饱和切换；Python 接受 Pubkey／bytes 两种公钥表示。Python 原有非 quote 构造器委托同一核心。
- 三语言按固定 Rust utils/pumpfun.rs 校正 Mayhem 标记与已知收款池冲突；普通主收款人覆盖错误 true，reserved Mayhem 收款人覆盖错误 false。未知轮换收款人保留显式事件标记；不能由 AMM 收款人推断 Mayhem。
- Node 普通 bonding-curve fallback 不再随机选历史 AMM fee recipient。三语言拒绝与当前模式冲突的已观测 fee recipient，再使用基线 fallback；允许未知非默认轮换地址。这里仍不是当前 Global 账户验证。

最终本阶段回归：trade Node 3044 项、Python 3122 项；Go 全量 test/vet/race 通过；Node typecheck、examples typecheck、CJS/ESM/declarations 构建通过。parser 无本阶段新修改，沿用 203 passed/8 skipped（Node）、189 passed（Python）、Go 全量通过的既有结果。六仓 git diff --check 通过。

最新独立安装：npm tarball CJS/ESM 核心入口；干净 Python 3.12 venv 安装 wheel、统一参数类与原生 USDC 重建；Go 独立 module consumer 对当前本地源码副本 test。npm --ignore-scripts 不验收 QUIC provider 安装，Go replace 不证明已发布远端模块。详见同名 JSON evidence。

此阶段没有新主网模拟、广播、发布或提交。完整 PumpFun FeeConfig/Global/SharingConfig 冻结缓存准备、DAMM 自动报价路线、全部协议/事件字段对照与完整实链模拟矩阵仍未完成，不能把上述测试数作为整体对齐结论。

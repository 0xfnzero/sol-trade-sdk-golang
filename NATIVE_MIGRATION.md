# v0.1.6 API 迁移（2026-10-06 发布）

基线：Rust sol-trade-sdk 5.0.6；Rust 5.0.7 新增的 CPMM creator-fee 领取 API 尚未移植。以下仅覆盖本轮修改的 cached/factory 入口，完整 API 对齐仍在进行。

## 完整交易请求

Node `TradeExecutorFactory.createExecutor(...).executeBuy/executeSell` 接收 `{request, signers, submit}`，其中 request 是 `CachedTradeRequest`，submit 接收已签名原始 wire。Go 的 `pkg/trading` 对应入口传入 `*TradeExecutionRequest{Request, Signers, Submit}`。Python `trading.factory` 入口传入同样三个键的 dict。钱包、金额、资产、冻结状态、报价上下文、blockhash 必须由调用方提供。

```ts
const executor = TradeExecutorFactory.createExecutor(DexType.StonkFun);
const receipt = await executor.executeBuy({request, signers: [wallet], submit});
```

买入和卖出独立调用。提交结果包含 submitted/confirmed；发送适配器返回成功只能报告 submitted，链上确认需另行观察 gRPC 证据。Node factory 热路径拒绝 waitConfirmation。quote/prepare/sign 不补 RPC。

原先仅传入 `{payer, amount, pool...}` 的 execute 调用会报缺少完整请求；通过 instruction 模块直接构建指令的能力仍提供。Python 旧 PumpFun/PumpSwap/CPMM 类的指令准备辅助改为 `build_buy_instructions` / `build_sell_instructions`，返回 prepared/submitted/confirmed 状态，execute 则进入真实 cached 核心。

统一 cached 核心目前覆盖 LaunchLab/Bonk/StonkFun、CPMM、AMM v4、CLMM、Whirlpool、DLMM。PumpFun/PumpSwap/DAMM v2 完整缓存尚未接入；factory 创建这些协议的执行器不代表可以完成 cached 准备，缺少实现/状态会明确拒绝。

## 候选池与显式路径

非空 hints（Go Hints）优先。没有 hints 时提供 candidates（Go Candidates）、inputMint/outputMint（Python input_mint/output_mint；Go InputMint/OutputMint）。仅使用调用方候选池，不访问聚合器或私有池服务；按最少跳数逐个检查实际缓存状态并报价，最多 5 跳，无池重复、无资产环路。选中有效路径后停止搜索；失败原因和搜索预算可观察。

三个 `examples/cached_trade` 示例的 JSON 支持：

```json
{"legs":[],"input_mint":"支付mint","output_mint":"收款mint","candidates":[{"pool":"池地址","input_mint":"mintA","output_mint":"mintB"}]}
```

候选身份可来自 parser；当前配置、费用、vault/mint 和 array 状态来自预先验证的缓存。parser 事件本身不证明当前状态。native_input/native_output 决定 SOL 包装/结算；WSOL/USDC/股票币以对应 mint 指定。SOL 与 WSOL 使用相同流动性路径，账户生命周期由 endpoint 标志区别。

## 实时缓存就绪

三语言的 SubscriptionReadiness 记录初始化、就绪、连续性中断、恢复中及 generation/fork。实时使用 cache.readySnapshot/readiness（Python ready_snapshot；Go ReadySnapshot）。普通 snapshot 供离线或由调用方另行维护连续性。

订阅断线、队列丢弃、版本冲突或无法确认连续性时先 interrupt。beginRecovery 选择 fork；实际验证所需账户后才 markValidated。新增 tick/bin array 加入 requireAccounts，并重新验证。旧 snapshot 的 guard 在中断后永久失效，即使新缓存恢复，旧准备状态仍不能使用。SDK 在读取状态、完成准备和提交前复核 guard。

完整自动订阅/恢复接线仍未完成。markValidated 不能仅根据“cache 含有这个 key”调用。冷准备允许显式 RPC；热路径禁止 RPC。

## PumpFun 报价

旧小额固定输出已移除。买入根据 net input - 1 计算并受真实储备限制。计算保持整数，Go 新增 `GetBuyTokenAmountFromSolAmountU128` / `GetSellSolAmountFromTokenAmountU128`，接受 *big.Int 储备，覆盖 Rust u128 输入域；原 u64 入口委托宽整数实现。Node 的旧 Number 辅助入口拒绝不精确输入或输出，应使用 bigint calc 入口。

26 组固定 Rust 源码 oracle 结果已核对；这不替代当前状态实际费用及主网模拟。PumpSwap FeeConfig 必须包含实际 discriminator，全零头部不再接受。

此前验证未真实发送交易；v0.1.6 发布包含本文所述已实现入口。完整端到端资产矩阵、其它协议缓存及发送服务验收仍待完成。


## 2026-10-04：PumpSwap 金额与错误行为

- exact-output buy（以及 reverse exact-output sell）的输入金额是明确预算，不再隐式追加滑点；与 sell ABI 不兼容的 exact-output 方向明确拒绝。
- 合法大额计算使用宽整数中间值；最终报价仍需满足 u64，溢出/缺失储备/费用不足返回错误。Python 不再以零值结果表示这些错误，字典接口使用同一个原生核心。
- 公共卖出滑点零输入返回零，滑点钳制到9999；买入预算饱和到u64上限。
- 可显式传入 protocol fee recipient override，调用方应从经过验证的当前 GlobalConfig 获取收款人。
- 新增 PumpSwap 冻结缓存状态及统一 factory/route 独立 exact-in 准备入口；cashback 和非零转账费仍明确拒绝。


## PumpSwap 缓存交易与模拟更新（2026-10-04）

三语言已接入真实 cached factory/route；当前费用与收款人由冻结快照提供，报价/构建不联网。独立 SOL 买入、meme 卖出 SOL 的 V1 字节完全一致，六次主网模拟成功；旧状态入口“尚未接入准备”的记录已被本次覆盖。cashback、非零转账费和其它资产矩阵仍未完成，不能据此宣称全量对齐。


## PumpFun / DAMM v2 更新（2026-10-04）

PumpFun 现在从 pool quote 与本次 SOL/WSOL 支付或收款选择 legacy/V2；非 native 资产不能误走消费 SOL 的布局。Node fromTrade 保留 quote/fee recipient/observed creator。DAMM v2 修复 248→160 费用结构偏移、referral None 固定槽位、payer 权限、Node idempotent ATA 与 bigint funding，以及 Go 非法 mode 默认成功。三语言完整账户布局通过固定 Rust Borsh oracle；新增冻结 cache 状态与显式下限准备示例，三次真实买入模拟成功、完整 V1 字节相同。此入口不提供自动报价或完整 cached route/factory。Python Pool 结构新增保留区及奖励字段，建议使用关键词构造；Go u128 原始 16 字节表示不变。


## 公共曲线、参数、PDA 和独立安装更新（2026-10-04）

修复曲线账户公式与 Rust 的差异、Go 宽整数溢出、Node 精确储备、完整 V2 quote 解码、quote 切换和重复公共类；统一错误 program ID / discriminator / Mayhem 默认列表。修复 Node/Python 简化 PDA 派生，PumpSwap 通用池要求完整 seeds，无法确认身份的旧 helper 明确拒绝。Go 曲线计算返回 `(uint64,error)`；Python 已完成曲线抛错，运行最低 3.10，移除未使用的 anchorpy。没有把这些入口认定为已完成当前费用缓存准备，整体对齐仍未完成。

### 公共参数与曲线默认值补充

默认 BondingCurveAccount 储备/供应量改为 Rust 零状态；初始化曲线必须显式调用 dev-trade 重建或填写已验证状态。Python trading.params.PumpFunParams 与根入口使用同一个类；from_trade 参数采用根入口完整 quote-aware 签名，调用时请使用明确字段名（virtual_quote_reserves / real_quote_reserves），不再依赖旧的缺 quote 位置参数。from_dev_trade 支持可选 quote_mint，默认参数不再使用 dataclass field()。factory 保留 curve quote 并默认使用其支付／收款 mint。Node 普通 PumpFun fee fallback 固定主收款人；事件标记与已知收款池冲突时按固定 Rust 纠正。当前 Global 验证仍须准备流程补齐。


## DAMM 下限与可空估值（2026-10-04）

单跳 DAMM 请求新增 Node fixedOutputAmount / Python fixed_output_amount / Go FixedOutputAmount *uint64，必须为正 u64 显式下限；其它协议拒绝使用。预计输出未知：estimatedNetAmountOut / estimated_net_amount_out / EstimatedNetAmountOut 可为空。Go 由 uint64 改为 *uint64，使用前检查 nil 再解引用，JSON 未知值输出 null。新 examples/cached_damm_v2 区分 SOL 和已有 WSOL；历史 damm_v2_snapshot 保留旧 SOL funding wire。


## PumpFun 配置读取（2026-10-04）

新增 Node cachedPumpFunConfiguration / Python cached_pumpfun_configuration / Go AccountCacheSnapshot.PumpFunConfiguration，要求当前 Global 与有效 SharingConfig 账户均在冻结缓存中。缺失配置表示未确认，不是未激活；返回 fee recipient 与可空的激活分成 vault。这不是费用报价或 cached factory。Node fromMintByRpc 现在还读取 Global 与可选分成配置，调用方 mock 需要提供 Global，仍仅可冷初始化使用。


整体对齐仍未全部验收。


持币卖出已补真实模拟；WSOL 仍为 minimum + SOL 残余。


Native quote 多跳现已接入，USDC/WSOL 路径通过双向真实模拟。旧的全部多跳拒绝状态已被替代，完整组合矩阵仍未全部验收。

## 2026-10-05: PumpFun settlement consistency

Public native-quote settlement helpers accept complete prepared cached routes, not instruction-free route stubs. Supply one swap per quoted leg, with matching bytes/accounts, payer, protected output and a PumpFun exact-in V2 native anchor. Legacy/exact-out or manually edited amounts/pools without matching instructions return errors. Method signatures and valid factory output wires are unchanged; the factory already supplies the required complete routes.

## 2026-10-05：连续性及冲突失效修复

新增依赖不会解除连续性中断。同版本账户冲突会使来源缓存及此前冻结快照失效，必须明确选择 fork、建立新缓存并重新验证；仅重置 readiness 无法恢复旧缓存。本项是本地状态机验收，完整实时重连／重组及新数组发现仍待验收。

## v0.1.7：CPMM creator-fee（2026-10-06）

现已对齐 Rust 5.0.7 的 creator 签名/permissionless 领取、share PDA、config/share 解码、收益拆分、同快照冷 RPC 读取与缓存准备/重验。缓存必须显式观察 share PDA（不存在/关闭用空数据 tombstone 表示），不能把未收到 gRPC 更新当作不存在。提交前重验账户版本和连续性；估算不含 Token-2022 转账税。

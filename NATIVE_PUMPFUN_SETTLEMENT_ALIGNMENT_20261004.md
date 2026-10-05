# PumpFun 当前费用与资产结算对齐（2026-10-04）

固定 Rust 对齐基线仍为 sol-trade-sdk 5.0.6。当前链的 PumpFun 行为需要单独记录，不能用旧固定费用 helper 代替当前缓存报价。

## 实现

Go、Node.js、Python 原生缓存报价读取 Global、FeeConfig、curve、mint 和 SharingConfig。买入、卖出保持独立，不依赖 router。当前费用算术使用官方 npm @pump-fun/pump-sdk 2.0.0 作为开发 oracle：240 个相同向量验证三个原生实现；安装和运行不依赖该包或 Rust。保留旧固定 Rust helper 的兼容行为。当前报价拒绝非零转账费；holder reward 曲线构建仍明确拒绝。

Global recipient、buyback recipient 和 active SharingConfig creator vault 从冻结缓存读取。SharingConfig 的已观察关闭记录与未观察缺失分开：前者可表示 inactive，后者要求准备数据。模拟实际发现 buyback recipient 必须 writable，cached V2 builder 已修复；旧低层兼容 builder 未纳入本次当前链验收。

## SOL 与 WSOL

PumpFun 的 WSOL-sentinel V2 实际从钱包 SOL 结算，提供 WSOL ATA 不代表它会扣 WSOL。

- SOL 买入直接执行 V2，不再预先锁进 WSOL，避免暂时占用两份输入资金。当前 mainnet 显式模拟成功，wire 为 1044 字节。
- WSOL 买入先创建租金账户，从原 WSOL ATA 转出指定数量至临时 WSOL，然后只关闭临时账户，把 SOL 用于 V2。真实 funded ATA 模拟成功，输入余额减少 10000，原 ATA 保留。V1 wire 为 1238 字节，模拟端接受，发送服务兼容性未验证。
- SOL 卖出直接接收钱包 SOL。
- WSOL 卖出只把保护 minimum 包装到 ATA，超出部分留为 SOL；返回 estimatedNativeResidualLamports / EstimatedNativeResidualLamports / estimated_native_residual_lamports。这是估计值，不是已确认的实际残余。此路径有离线结构测试，尚无余额充足持币钱包的当前链卖出模拟证据，不能宣称完整 WSOL 收款验收。

通用 cached route 默认拒绝原生 quote PumpFun。cached factory 仅开放单跳专用结算；涉及这种曲线的多跳明确拒绝，直到支持并验证每一跳的资产衔接。非原生 quote 的 USDC/股票曲线未纳入本次真实模拟验收。准备、报价、编译不发 RPC；冷账户初始化与显式模拟可用 RPC。无广播。

## 证据与迁移

examples/fixtures/pumpfun_direct_sol_simulations_20261004.json 保存本次 SOL 模拟响应和原始 wire。
examples/fixtures/pumpfun_settled_funded_wsol_evidence_20261004.json 保存原始 wire、冷账户验证、模拟后的 ATA 余额、实际 WSOL debit 及响应。
pumpfun_settled_0/1_sol_buy 文件是历史快照上的新结算离线回归，带 historical_wire_sha256；不可把它们配历史模拟响应称作重新模拟。
旧 pumpfun_current_funded_wsol_evidence_20261004.json 仅证明执行成功，实质钱包 SOL 扣款，不能作为 WSOL 支付验收。

新增 PreparedCachedTrade 原生残余字段；Go 改用具名结构初始化。WSOL 买入要求独立唯一 seed 和当前 165 字节账户 rent；交易费用、其它 ATA rent 仍需钱包余额，RequiredNativeLamports 不是全部交易预算。禁止复用链上已有的 seeded 临时账户。

## 尚未完成

完整 Rust 公开 API/字段矩阵、gRPC fork/恢复/新增数组验收、全部资产矩阵和 SWQOS 服务兼容证据仍按现有计划继续；本报告不表示整体对齐完成。真实广播验证和发布未执行。

验证摘要见 [机器可读记录](NATIVE_PUMPFUN_SETTLEMENT_EVIDENCE_20261004.json)：六仓全量测试通过；最终新增回归单独通过；Go 两仓全量 race/vet、Node 类型/构建/示例类型、独立 npm/wheel 安装后的公共入口 wire 对照通过。跳过的在线测试不计为验收。

parser 后续已补齐 V2 买入钱包账户及实际 SOL debit 10000 的 System CPI 归属，外层 WSOL 转换不重复计入。卖出账户冷核验记录见 examples/fixtures/pumpfun_sell_discovery_unverified_20261004.json，未获得可用余额，仍未验收。


持币卖出已补真实模拟；WSOL 仍为 minimum + SOL 残余：[最新卖出验收](NATIVE_PUMPFUN_FUNDED_SELL_ALIGNMENT_20261004.md)。


Native quote 多跳现已接入，USDC/WSOL 路径通过双向真实模拟：[最新多跳验收](NATIVE_PUMPFUN_MULTIHOP_ALIGNMENT_20261004.md)。旧的全部多跳拒绝状态已被替代，完整组合矩阵仍未全部验收。

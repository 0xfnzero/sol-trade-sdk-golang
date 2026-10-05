# PumpFun / DAMM v2 原生对齐记录（2026-10-04）

固定基线：Rust parser 0.7.7 (`78a686a012ef3d685624d1b1bdba39766cb000bd`) / trade 5.0.6 (`dc9741b6074df21418f16b6175f497fbebecb723`)。本记录是本轮代码和验证证据，不代表六仓全量对齐完成。

## 已修复

1. PumpFun 三语言根据本次支付/收款 mint 选择布局：native SOL 使用 legacy，显式 WSOL 使用 V2，USDC 等非 native quote 使用 V2。quote 参数未指定时读取曲线 quote；显式 quote 优先。共同 fixture 覆盖 20 个 quote/curve/endpoint 组合，买卖分别验证。另增加严格资产匹配：native pool 不允许用户请求 USDC 却默默消费 SOL；这是明确记录的较 Rust 基线更严格的输入拒绝行为。
2. Node 导出的 params 类 `fromTrade` 保留 quote、观察到的 creator、fee recipient 和 mayhem 标记，避免丢失后选择错误布局或随机静态收款人。native builder 的 curve quote 字段已补齐；Python flat builder 使用 `curve_quote_mint` 表示对应状态。
3. DAMM v2 Go/Node 原 decoder 错误跳过 248 字节费用结构，修正为固定 Rust Borsh 的 160 字节；三语言 decoder 现在保留全部 Pool/BaseFee/DynamicFee/PoolFees/Metrics/RewardInfo 字段，包括价格范围、激活状态、费用增长、奖励时间和保留区。Node u64/u128 使用 bigint，Go 原 u128 `[16]byte` 表示保留兼容；Python 使用精确整数。开发工具 `tools/native-parity/generate_damm_v2_layout.py` 从固定 Rust 源码生成 1104 字节 Borsh 序列化/反序列化 oracle，三语言核对每一个字段并测试尾部扩展、截断。安装运行及 native 回归不依赖 Rust。
4. DAMM v2 无 referral 仍保留第 11 索引的 program ID 只读占位；event authority/program 不再移位。signer account 在指令中为只读，与 Rust/IDL 相同。Node ATA 创建改为 idempotent，SOL funding 保留 bigint，不再通过 Number 损失精度；Go 非法显式 swap mode 返回错误，不再转成看似成功的 PartialFill。

## 缓存状态与示例

新增 Node `snapshot.dammV2` / `cachedDammV2`、Python `snapshot.damm_v2` / `cached_damm_v2`、Go `snapshot.DammV2`。从冻结状态校验 owner/discriminator、mint 连通性及 token program flag、mint 转账能力及当前 epoch fee、vault authority/mint/initialized、激活 slot/time、池状态、流动性、价格边界、freshness 和 continuity guard。原 parser route identity 适配表允许 DAMM v2 线索；未虚构自动报价或注册为自动 cached route 协议。

`examples/damm_v2_snapshot.ts`、`examples/damm_v2_snapshot.py`、`examples/damm_v2_snapshot/main.go` 从相同冻结快照构建 swap2 ExactIn + unsigned V1。调用方必须明确给出 `fixed_output_amount`，与 Rust baseline 相同；这个值是保护下限，不是 SDK 报价结果。示例显式拒绝非零转账费的未验证阈值语义。默认不联网、不发送；只有 --simulate 调用 RPC。

本轮 PublicNode Yellowstone gRPC 实际发现池 `4kuAXoXBgRq79PMJ1bArHF1UfPoKnXZo7nYysUrZEzpj`，slot 453008882，payload 1104 字节。随后通过明确冷 RPC 在 confirmed bank slot 453009793 补齐 pool/mint/vault；不是静态账户已由 gRPC 全量初始化的证明，也没有重写账户观测 slot。

## 实际模拟证据

第一次真实模拟返回 `InstructionError [4, Custom 3005]`（AccountNotEnoughKeys），暴露了漏掉 referral None 槽位的问题；此失败保留在 fixture，未记作成功。修复后三语言使用同一冻结状态分别模拟买入成功：

| 语言 | 模拟 slot | err |
| --- | --- | --- |
| Node | 453010856 | null |
| Go | 453010862 | null |
| Python | 453010482 | null |

原始 V1 均为 683 字节，SHA256 `1ecb3d7d397b18ac6e499f62352de4773009460150489c670dc9b17c8a2faf6e`。使用公开、有实际 SOL 余额的钱包观察地址、10000 lamports 通过 ATA wrap，未签名、未真实广播。输出下限 1 atom 仅用于该 unsigned 指令验证，不是可直接复用的实时策略报价或安全滑点建议。本例不证明已有 WSOL 余额输入、独立卖出或 USDC/股票全部形态。

对应 `examples/fixtures/damm_v2_buy_mainnet_20261004.json` 和 `damm_v2_mainnet_simulations_20261004.json` 已保存。三语言离线重建核对完整 wire 哈希；Node/Python 拦截网络，Go 准备不使用 RPC。三个 parser 对成功、失败模拟全部字段经精确整数表示归一后一致；失败意图保留，实际金额仍未知时不填零。parser regression 保存这两种模拟。

## 仍需完成

- PumpFun 完整当前手续费/fee-sharing 状态准备及统一 cached factory、实际独立买卖模拟。
- DAMM v2 自动报价的原生费用调度、动态费、价格/时间/rate limiter/转账费语义，以及统一 cached route/factory；当前新入口是已验证状态 + 显式保护参数，不能称为自动报价完成。
- DAMM v2 独立卖出、全部支付/收款资产矩阵和自动 gRPC readiness/fork 恢复仍待验证。
- 完整公开 API/事件字段审计、其它剩余缺口、SWQOS 服务验证、干净安装与最终发布准备仍待完成。

最新六仓测试结果与日志哈希见 `NATIVE_PUMPFUN_DAMM_EVIDENCE_20261004.json`；本轮没有提交、广播或发布。

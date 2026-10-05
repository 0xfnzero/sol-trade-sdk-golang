# PumpSwap 原生缓存交易对齐（2026-10-04）

固定基线：Rust sol-trade-sdk 5.0.6，commit `dc9741b6074df21418f16b6175f497fbebecb723`。本记录仅覆盖 PumpSwap 本轮新增能力，全量 SDK 对齐尚未完成。

## 本轮实现

三语言统一 factory、cached trade、route 与 parser 交易线索协议表接入 PumpSwap。状态入口为 Node `snapshot.pumpSwap`、Python `snapshot.pumpswap`、Go `snapshot.PumpSwap`；独立准备入口为 `preparePumpSwap`、`prepare_pumpswap`、`PreparePumpSwap`。均读取冻结快照，无 RPC、聚合器或 router 合约。

校验 pool/config owner、discriminator、PDA/bump、mint/vault 身份与状态、epoch、freshness、signed virtual quote reserves 和 continuity guard。canonical fee tier 使用 mint supply 和有效 quote 储备（真实储备加 signed virtual），非 canonical 使用当前 flat fees；拒绝默认费率补齐。当前 GlobalConfig 决定 protocol/reserved 和 buyback 收款人，默认 coin creator 不收 creator fee。禁用方向拒绝准备。

ABI 根据实际支付 mint 选择：quote 输入使用 buy_exact_quote_in，base 输入使用 sell。quote-in 包含 volume accounts；非默认 creator 附带 pool-v2；最后附带当前 buyback recipient/ATA。route 负责 ATA 初始化、SOL 临时 WSOL 和关闭收款账户，买卖各自独立构建。

三语言四个原生金额入口采用精确宽整数中间值、最终 u64 校验；Go 不忽略费用错误，Python 不以默认零表示非法状态。exact-output 预算不隐式追加滑点，显式 protocol fee recipient override 已对齐。共同 oracle 为 18 个输入 × 四个入口，共 72 个成功/错误结果，来自固定 Rust 源码；运行 SDK 和测试不依赖 Rust。

## 实际模拟与可重放证据

`examples/fixtures/pumpswap_buy_mainnet_20261004.json` 和 `pumpswap_sell_mainnet_20261004.json` 为相同池的独立交易快照；不是可直接用于当前交易的实时状态。冷启动明确使用 RPC 补齐静态账户，随后 Yellowstone gRPC 实际更新 pool：slot 452990569。未更新 mint/config 保留原观测 slot，不伪造新鲜度；账户 write_version 保留来源语义。自动 fork 恢复尚未完成。

三语言各自模拟买入与卖出成功（共六次 simulateTransaction，全部 err=null），没有真实发送、签名私钥或广播。买入 10000 lamports，使用临时 WSOL，保护输出 154335；卖出 100000000 meme atoms，使用只读查询核实了公开持币账户的合法 ATA、实际余额和 SOL 余额，保护输出 5788727。不是在同一笔交易买入再卖出。

| 交易 | Node / Go / Python 模拟 slot | 原始 V1 长度 | 三语言相同 wire SHA256 |
| --- | --- | --- | --- |
| SOL→meme | 452992833 / 452992835 / 452991476 | 1177 | `7cf1c2af38ae59dcb9a1bc0131efaf27a40db9f3817e23ad4590681e787bf80a` |
| meme→SOL | 452993201 / 452993199 / 452993167 | 1111 | `4b2764e4445ff59e9b4dbd2706e787fb252956df3b86dd2c970e911a1e91f6b2` |

`examples/fixtures/pumpswap_mainnet_simulations_20261004.json` 保存 unsigned wire、真实模拟响应、三语言验证 slot 和共同 parser 期望。三个 parser 的全部输出经精确整数 JSON 表示归一后相同。买入 Token-2022 输出仅有 gross transfer，未知 withheld fee / net output 保留 null；卖出 WSOL 实际输出为 6096773。模拟银行已变化，实际成交不强求等于历史缓存报价。

三语言离线回归通过真实 factory 重建上述 wire，并核对保护金额及完整字节哈希。Node/Python 测试阻断网络；Go factory 使用 nil RPC 客户端。模拟发现并修复 Node factory 未注册 PumpSwap、Go 示例不接受 maximum_slot_age 精确整数字符串的问题。Go 示例同时接受合法 u64 JSON 整数，拒绝小数、负数、溢出及 null。

## 明确未完成

- cashback 池所需当前费率上下文、非零 Token-2022 转账费语义尚未验证，缓存准备明确拒绝；不能归入已支持。
- mayhem、禁用位、动态收款人有 synthetic 拒绝/账户测试，未完成每种主网模拟。
- 已持有 WSOL 的输入、USDC/股票端点及其它多跳形态未由本次 PumpSwap 两个模拟证明。
- PumpFun / DAMM v2 统一缓存准备、完整 parser 字段审计、自动 readiness/fork/静态账户重验和新增 array 发现仍待完成。
- SWQOS 供应商真实广播验证、干净安装全矩阵和最终发布准备仍待完成。本轮不发布。

最新测试及日志哈希见 `NATIVE_PUMPSWAP_EVIDENCE_20261004.json`。

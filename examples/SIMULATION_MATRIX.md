# 当前银行模拟与 parser 成交检查

这些示例只调用 `simulateTransaction`，不签名、不广播。支付者必须是银行中真实存在、SOL/输入 SPL 余额足够的账户；无需私钥，`sigVerify=false` 不能绕过余额与账户约束。买入和卖出分别执行。

## 原生示例

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

## 六仓开发工作区矩阵

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

## 失败交易验证

验证余额不足或无法满足的输出保护时，可使用矩阵 `--expect-execution-failure`。它要求三语言模拟均失败、示例均返回非零，并且三语言 parser 均表示失败且所有实际成交金额为未知；传输/RPC/准备失败不算通过。退出码 0 与 `passed_expected_execution_failure` 表示“失败处理验收通过”，不表示交易成功。模板输入账户的当前余额变化可能改变预期结果，需检查银行日志。

```sh
python tools/native-parity/simulate_cached_examples.py \
  --cold-only --expect-execution-failure \
  --output-dir /tmp/sdk-negative-simulation \
  tools/native-parity/live_negative_simulations_20261005/impossible_protection_template.json
```

矩阵还检查保存证据的 wire 与独立准备的 wire 一致，并拒绝报价腿数与请求/解析腿数不一致的结果。Node.js cached 示例现在校验 u64/i64 范围，拒绝不安全 Number、布尔值、分数与非十进制整数字符串；大金额、slot、版本等必须传十进制字符串。

公开示例钱包余额会随真实交易变化；历史成功不能证明现在仍有资金。第二轮证据见 `tools/native-parity/live_negative_simulations_20261005/REVIEW.md`。Python parser 刷新示例的订阅、事件等待与 blockhash 请求共用截止时间，超时返回 `not_ready`，不保存部分快照。

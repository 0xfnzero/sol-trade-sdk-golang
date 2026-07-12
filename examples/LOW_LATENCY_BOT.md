# Low-latency bot guide / 低延迟机器人指南

`low_latency_bot` defines interfaces between `sol-parser-sdk`, `solana-streamer`, and an application execution adapter. Implement that adapter with protocol instruction builders and a configured prebuilt-transaction executor. The root `pkg.TradingClient` and protocol factory deliberately do not build or submit protocol trades.

该模板定义 parser、streamer 和应用交易适配器之间的接口。适配器应组合协议 instruction builder 与已配置的预构建交易执行器。根包 `pkg.TradingClient` 和协议 factory 都不会构建或发送协议交易。

## Required flow / 必须流程

Filter stale/wrong-target events, query the correct SPL Token or Token-2022 ATA balance, buy with current decoded state and a fresh blockhash, require confirmation, compute the checked balance delta, then refresh state and blockhash before selling. Never sell a fixed assumed amount and never reuse the buy blockhash for the sell.

先过滤过期事件和错误 mint/pool，按真实 Token Program 查询 ATA 余额；使用最新状态和 blockhash 买入；确认后计算余额增量；卖出前重新读取状态和 blockhash。不能用固定假定数量卖出，也不能复用买入 blockhash。

## `min_base_amount_out` / Custom(6040)

6040 (`BuySlippageBelowMinBaseAmountOut`) means actual output was below the explicit protected minimum, commonly because the quote was stale at execution time.

- Prefer `BuyWithMaxInput(...)` or slippage-derived protection for ordinary exact-input buys where supported.
- Populate `FixedOutputTokenAmount` only from a current protocol quote, never from an example constant.
- Removing the minimum suppresses the error by accepting worse execution; keep it for meaningful-size protected trades.
- On 6040, refresh pool state and blockhash, requote, and retry only within a bounded count, event-age limit, and strategy price limit.
- Slippage must be less than 10,000 basis points.

6040 是保护条件不满足。常规 exact-input 优先使用动态滑点保护；显式最低输出必须来自实时 quote。遇到错误后有限次刷新并重新报价，不能无限重试或直接取消保护。

Pre-create token accounts, keep event filters in memory, cap SWQoS concurrency, and request buy confirmation when an automatic sell depends on it. Waiting for all submit providers improves diagnostics at the cost of tail latency.

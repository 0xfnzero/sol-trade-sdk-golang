# Sol Trade SDK Go Examples

Examples are updated for the current Go SDK API. Basic protocol demonstrations use synthetic accounts and do not submit transactions. Cached simulation examples use explicit bank simulation; see SIMULATION_MATRIX.md.

## Run

```bash
go run ./examples/trading_client
```

Start bot integration from [low_latency_bot](low_latency_bot/main.go) and read [LOW_LATENCY_BOT.md](LOW_LATENCY_BOT.md). `PRIVATE_KEY` is a base58-encoded 64-byte secret key.

Important: the root `pkg.TradingClient` is a facade and intentionally returns `ErrTradingExecutionUnavailable`. Implement the template's `TradeExecutor` adapter with protocol instruction builders plus a configured prebuilt-transaction executor. The examples do not claim that the root facade or protocol factory submits trades.

## Coverage

| Area | Example |
| --- | --- |
| Trading client and low-latency config | [trading_client](trading_client/main.go) |
| Parser + streamer guarded bot workflow | [low_latency_bot](low_latency_bot/main.go) |
| Shared config across wallets | [shared_infrastructure](shared_infrastructure/main.go) |
| PumpFun v2 fee recipient and cashback | [pumpfun_sniper_trading](pumpfun_sniper_trading/main.go), [pumpfun_copy_trading](pumpfun_copy_trading/main.go) |
| PumpSwap cashback-aware params | [pumpswap_trading](pumpswap_trading/main.go), [pumpswap_direct_trading](pumpswap_direct_trading/main.go) |
| Bonk / USD1 routing | [bonk_sniper_trading](bonk_sniper_trading/main.go), [bonk_copy_trading](bonk_copy_trading/main.go) |
| Raydium CPMM / AMM v4 | [raydium_cpmm_trading](raydium_cpmm_trading/main.go), [raydium_amm_v4_trading](raydium_amm_v4_trading/main.go) |
| Meteora DAMM v2 | [meteora_damm_v2_trading](meteora_damm_v2_trading/main.go) |
| Durable nonce | [nonce_cache](nonce_cache/main.go) |
| Hot path / zero-RPC preparation | [hot_path_trading](hot_path_trading/main.go) |
| Address lookup tables | [address_lookup](address_lookup/main.go) |
| Middleware | [middleware_system](middleware_system/main.go) |
| WSOL helpers | [wsol_wrapper](wsol_wrapper/main.go) |


### 第二批审查的 CPMM 样本（2026-10-03）

`fixtures/batch2_cpmm_buy_20261003.json` 和 `batch2_cpmm_sell_20261003.json` 是完整冷启动快照，可交给本仓 `cached_cpmm` 示例离线构建独立买入/卖出；三语言 wire 一致。对应 `batch2_cpmm_simulations_20261003.json` 保存两笔成功主网模拟，可供 parser 的 `simulation_routes` 示例读取。没有真实发送。保存快照仅供复现，执行新交易前须由冷启动/订阅提供当前状态；构建与报价不调用 RPC。

缓存 CPMM 现检查 vault 的 token authority，prepare 拒绝零最小到账。CLMM/DLMM 可扫描完整合法稀疏范围；bitmap 证明空区间不需要虚构账户，已初始化 array 缺少订阅数据仍明确拒绝。实时账户输入通过 sol-parser-sdk 的 gRPC 接入；见 [GRPC_CACHE.md](GRPC_CACHE.md)。新 array 发现、静态账户 freshness 和分叉一致性仍由 gRPC 订阅集成策略处理。

当前银行模拟与跨语言成交检查见 [SIMULATION_MATRIX.md](SIMULATION_MATRIX.md)，包含 `--simulation-out` 保存证据和 parser 离线验收流程。

# Sol Trade SDK Go Examples

Examples are updated for the current Go SDK API. Protocol examples use synthetic accounts and do not submit transactions.

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

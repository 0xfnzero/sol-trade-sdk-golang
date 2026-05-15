# Sol Trade SDK Go Examples

Examples are updated for the current Go SDK API. They run in dry-run mode by default so they do not send mainnet transactions accidentally.

## Run

```bash
go run ./examples/trading_client
```

Set `RUN_LIVE_EXAMPLES=1` only after replacing placeholder params with real RPC/parser data and funding the signer.

## Coverage

| Area | Example |
| --- | --- |
| Trading client and low-latency config | [trading_client](trading_client/main.go) |
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

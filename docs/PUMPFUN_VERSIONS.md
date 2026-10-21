# Pump.fun instruction versions

Client TradeConfig defaults to V3 (PumpFunVersion: soltradesdk.PumpFunV3). An explicit per-request version wins over the global setting. Low-level unversioned legacy builders retain compatibility; pass a version explicitly when calling them directly. This is independent of the Solana transaction-message version.

- V1 (Legacy): native SOL curves and wallet SOL settlement only. Direct WSOL, USDC, and other token endpoints are rejected; use cached trade preparation for explicit conversion and native settlement.
- V2: full account layout, selectable for compatibility.
- V3: compact 17-account layout; supports native SOL and token quotes using the actual mint owner programs. Active non-cashback curves only in the current builder; unsupported states fail explicitly.

Token-quoted V3 direct trades require an explicit minimum output and verified zero base/quote transfer fees. Unknown or nonzero transfer fees are rejected. Native automatic quotes reject graduation-boundary buys rather than silently using partial fill. Cached multi-hop routes use coherent account snapshots and protect every leg; no RPC runs during quoting or building.

## Version precedence for cached routes

`PrepareCachedTrade`, `PreparePumpSwapTrade`, `PrepareBestPumpSwapTrade`, and
`PumpSwapRoutingConfig.Prepare` apply the same PumpFun version precedence:
explicit request version → effective TradeConfig → snapshot → V3 default.
`TradingClient` and factory-created executors inherit the base executor's
TradeConfig, including cached preparation/submission and PumpFun bridge hops in
other DEX routes. A non-nil request `TradeConfig` replaces the client's config
for that call. Standalone preparation functions use the request's `TradeConfig`;
they do not have a client configuration. Preparation leaves the caller's request
and snapshot unchanged and performs no RPC.

中文：版本优先级为“单笔显式版本 → 生效的 TradeConfig → 快照配置 → 默认 V3”。
客户端和工厂创建的执行器自动继承全局配置，缓存准备、签名提交和其它 DEX 路由中的
PumpFun 跳均适用。请求中非 nil 的 `TradeConfig` 会覆盖客户端配置；独立准备函数仅使用
请求提供的配置。调用过程不修改原请求或快照，不访问 RPC。

Use the factory to inherit client configuration. When constructing
`CachedTradeExecutor` or `UnifiedTradeExecutor` directly, use named fields
(for example, `CachedTradeExecutor{DexType: soltradesdk.DexTypePumpFun}`);
positional struct literals must be updated because these types now retain configuration.

中文：需要继承客户端配置时使用工厂创建执行器。手动构造以上执行器时请使用命名字段；
由于结构体新增配置存储，旧的位置式字面量需要调整。

## Replay and optional simulation

Direct native V1/V3 buys and sells use wallet SOL. An explicit WSOL endpoint is
rejected; use cached trade preparation for WSOL settlement and multi-hop routes,
which include the required wrapping/unwrapping instructions.

A per-call fixed output overrides the protocol minimum: on sells it is the
minimum received; on buys it selects exact output with the supplied input as the
maximum budget. Native `WithMaxInput` buys estimate the token target from the
supplied curve and apply slippage to the input budget. Token-quoted maximum-input
buys require an explicit target; the SDK does not infer it from SOL units.

中文：原生 V1/V3 直接买卖使用钱包 SOL；V1 不会把显式指定的 WSOL、USDC 或其它代币改成 SOL，需走缓存路由的显式转换流程。
单笔固定输出优先于协议最小输出；买入表示目标数量，卖出表示到账下限。
`WithMaxInput` 保留最大输入语义，非 SOL 报价必须明确提供目标数量。

Run from this repository, with development dependencies installed:

```sh
go run ./examples/cached_trade examples/fixtures/pumpfun_whirlpool_usdc_buy_20261005.json --pumpfun-version v3
go run ./examples/cached_trade examples/fixtures/pumpfun_whirlpool_usdc_sell_20261005.json --pumpfun-version v3
```

Replace v3 with v1 or v2 to compare versions. These fixtures route USDC through SOL into a native Pump.fun curve (and the reverse); they are not token-quoted stock curves. The direct V3 unit tests separately exercise token-quoted curves and Token-2022 account roles. Snapshot files explicitly retain their original V2 version so saved simulation hashes stay reproducible. V1 fee recipient selection may vary, so compare instruction semantics rather than whole-wire hashes.

Add `--simulate` with `SOLANA_RPC_URL` to request simulateTransaction without broadcasting. Historical account snapshots may no longer match current chain state; refresh snapshots before treating simulation as current evidence. Offline construction alone is not successful mainnet execution.

中文：客户端全局默认 V3，单笔显式版本优先；V1 仅支持原生 SOL 内盘。示例支持三个版本的多跳买卖，默认离线构建，只有 --simulate 才访问 RPC，不广播。历史 V2 快照保持原样，不能作为 V3 当前链上模拟成功的证据。股票代币作为报价币时，V3 必须提供最小输出及已验证为零的转账费，并使用实际 Token Program。

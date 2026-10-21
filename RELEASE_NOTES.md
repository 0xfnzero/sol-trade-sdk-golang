# sol-trade-sdk-golang v9.0.0

Released 2026-10-22. Rust, Node.js, Python and Go trade packages share family version 9.0.0; parser packages share 0.9.0.

## Changes / 更新

- Default client Pump.fun trades to V3 with global configuration and explicit per-request V1/V2/V3 overrides. Keep the chosen version through cached preparation, factories and multi-hop bridge legs.
- Support stock-quote Pump.fun routes and instance-owned PumpSwap routing configuration. Validate settlement endpoints, coherent snapshots, fees and minimum output for each hop.
- Reject unsupported V1 direct token settlement rather than silently changing the requested currency. Unsupported V3 states fail explicitly.
- Synchronize native examples, public exports, documentation and shared stock catalog integration. Quotes/builders using prepared inputs perform no RPC.

## Migration / 升级

The default Pump.fun instruction version changes to V3. Configure V1/Legacy or V2 explicitly when required. This is separate from the Solana transaction-message version. Low-level legacy builders retain their documented behavior; shared family versions do not imply identical language APIs or identical defaults on every low-level function.

Update public struct/configuration literals for new fields and prefer constructors/defaults. V3 token quotes require explicit minimum output and supported fee/state checks. Historical catalog data is not current liquidity or a guaranteed executable route.

中文：客户端默认使用 V3，单笔显式版本优先；缓存与多跳准备保留版本选择。V1 不再把不支持的代币结算端点静默改为 SOL。升级时检查配置和公开结构体字段，示例回放不代表实时成交或生产延迟。

### Go module migration

Run `go get github.com/0xfnzero/sol-trade-sdk-golang/v9@v9.0.0` and change every SDK import from `github.com/0xfnzero/sol-trade-sdk-golang/...` to `github.com/0xfnzero/sol-trade-sdk-golang/v9/...`. The repository URL is unchanged. v0 users can keep their previously pinned versions.

CachedTradeExecutor and UnifiedTradeExecutor now carry private configuration. Replace external positional struct literals with named fields or factory constructors. See docs/PUMPFUN_VERSIONS.md.

中文：Go Modules 主版本升级要求导入路径增加 `/v9`，SDK、examples 和用户代码都须同步；旧 v0 版本仍可按原路径使用。

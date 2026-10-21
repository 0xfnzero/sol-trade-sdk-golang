# PumpSwap custom quotes and cached route preparation

See [reported 3007 failure and synchronized fixes](PUMPSWAP_REPORTED_FAILURE.md)
for creator validation, explicit fee recipients and readonly volume accounts.

PumpSwap accepts arbitrary pool quote mints, including ordinary Token-2022
mints such as TTWO. Amounts are raw mint units. Direct instruction builders
accept `InputMint` and `OutputMint`: quote→base uses `buy_exact_quote_in` (or
`buy` for fixed output); base→quote uses `sell`, independently of the public
buy/sell method name. SOL aliases WSOL only for pool matching. A different
funding mint requires an explicit conversion route.

High-level builders forward both endpoints. Low-level callers omitting **both**
retain the legacy WSOL/USDC default direction; supply both for unambiguous custom
pools. Builders validate required identities, supported token programs and the
creator vault PDA/ATA against the creator and actual quote mint/program. Enrich
incomplete parser/log identities from verified subscription state first.

## Prepare and select

`trading.PreparePumpSwapTrade(CachedTradeRequest)` prepares an ordered exact-input
path. `trading.PrepareBestPumpSwapTrade(search, request)` discovers connected paths
only within the caller's conversion pool allowlist. Construct search with
`trading.NewPumpSwapRouteSearch(pool, token, asset, hints)` for defaults of two
conversion hops and 512 search nodes. Buy places the selected PumpSwap last;
sell places it first and requotes independently.

Search permits at most 128 supplied hints, 0–3 conversion hops and 1–4096 nodes.
Reversed duplicates collapse; conflicting pairs or selected-pool reuse fail.
Mint cycles, reused pools and conversion through the traded token are excluded.
Dead ends count against the node budget; exhaustion rejects the whole search.
Every complete path is quoted, invalid/stale paths are recorded in
`RejectedRoutes`, and the highest protected final output wins. Ties prefer fewer
hops, then raw pool public-key bytes. Ranking excludes network discovery, gas,
rent, wallet balances, transfer success and packet/ALT optimization.

Each hop spends the preceding protected output; slippage applies per hop.
`Steps()` returns the selected pools, directions and exact quoted inputs.
`MinimumOutput()` exposes final protected output. Use a coherent frozen
`subscription.AccountCacheSnapshot`: each protocol validates current pools,
mints, vaults, fees and required auxiliary accounts without RPC. PumpSwap cached
preparation rejects unsupported transfer fees/extensions and cashback pools
without a verifiable fee context. This does not ban ordinary Token-2022 mints.

## Offline signing

The prepared request freezes amount, slippage, endpoint mints and snapshot.
`ValidateFreshness(current)` requires the same epoch, no slot regression, and age
within both the original and current maximum ages. It bounds quote age and does
not prove fork consistency or refresh accounts; reprepare on updates.

`BuildTransaction(current, signers)` signs V1 with the supplied blockhash,
`V1Config`, tip and native settlement policy. `BuildV1Transaction` additionally
accepts caller-cached `PumpSwapNonce`. V1 uses its existing 4096-byte limit and
does not use ALTs. Submission requires a compatible V1 transport/cluster.

`BuildV0Transaction(current, signers, PumpSwapV0Options)` accepts cached ALTs,
explicit compute-unit gas settings and an optional cached nonce. It rejects
oversized 1232-byte packets rather than switching versions. Prepare without
`V1Config` when selecting V0; cross-version priority settings are never silently
discarded. Set `ComputeUnitPrice` for V0 priority fees and request `TipLamports`
with a valid `TipAccount` for tips. Nonce advance remains the first instruction;
all required signers must be supplied and unrelated signers are rejected.

Native SOL endpoints select wrap/unwrap; explicit WSOL spends existing WSOL.
All route token accounts use standard idempotent ATAs; intermediate residuals
stay open. Native settlement uses the supplied temporary WSOL seed and rent.
Preparation/signing never calls RPC, simulates, confirms or submits a trade.
The prepared API currently creates route ATAs idempotently and does not expose
arbitrary middleware or seeded token account policies.

## Repeatable native example and tests

From the repository root:

```sh
go run ./examples/pumpswap_cached_prepare
go run ./examples/pumpswap_cached_prepare examples/fixtures/pumpswap_ttwo_route_accounts.json sell
go test ./examples/pumpswap_cached_prepare -count=2
go test ./pkg/instruction ./pkg/subscription ./pkg/calc ./pkg/trading
```

The example loads decompressed `data_base64` account captures, quotes
SOL→USDC→TTWO→Dacity or the independently quoted reverse path, generates a local
wallet and signs without sending. Its placeholder hash is for offline replay.
The same fixture at 1,000,000 raw lamports and 100 bps per hop protects
102110→491→47932572; selling 1,000,000,000 raw target units protects
9924→2017142→19264527. Node/Python tests pin these same values.

Native tests repeat buy/sell, direct and three-hop paths, independently decode
signed V1 bytes, verify Ed25519 signatures and amounts, and parse signed V0 with
ALT, nonce authority and tip. Synthetic competing fee schedules exercise ranking,
ties, stale-route rejection and incomplete-search failure. These are historical
cache/quote/signing simulations. Additional [native Go SBF bank tests](../examples/pumpswap_offline_bank/README.md)
now execute direct and complete three-hop buys/sells, verify protected balances,
and roll back earlier swaps when the final swap fails. The bank checks repeat
12 scenarios twice using real signatures and cached ALTs. They use mixed
historical public program/account captures with synthetic local wallet funding;
they do not establish current mainnet acceptance or production latency. Existing
`examples/cached_trade --simulate` remains an explicitly requested cold RPC check.

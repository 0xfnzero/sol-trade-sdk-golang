# Changelog

## v9.0.0 — 2026-10-22

Synchronize SDK code, examples, version navigation and AI release rules. Read the [release and migration notes](RELEASE_NOTES.md) for callback removals, V3 defaults and the Go /v9 migration.

Release and integration notes preserved from the previous README. APIs, dependencies and validation results describe those releases, not the current contract. See the [README](README.md) and [release and migration notes](RELEASE_NOTES.md) for current usage.

## v0.2.0

See [release and migration notes](RELEASE_NOTES.md).

## v0.1.11 — PumpSwap custom quotes and creator account fixes

Builds PumpSwap buys and sells with the actual quote mint/token program, validates creator vault identities, honors explicit fee recipients in mayhem mode, and removes an unnecessary global volume write lock. Adds cached quotes, conversion-route selection and complete offline signing preparation, with TTWO pool and SOL→USDC→TTWO→target buy/sell regressions.

Four language implementations share a real 3007 failure fixture and compare instruction data and account privileges. Native local bank regressions use captured public programs/accounts and synthetic wallets. See [PumpSwap fixes and validation](docs/PUMPSWAP_REPORTED_FAILURE.md).

## v0.1.10 — Pump depth-2 and SpeedLanding regional updates

Adds Pump depth-2 create_v2 account chains, including curve and migrated-pool combinations. Preserves stored-depth chains when an ancestor is later whitelisted. Synchronizes SpeedLanding regional endpoints, including London and the New York domain, and compatible client routing. Account construction adds no RPC calls.

Validation includes official Pump SDK 4.1.0 instruction comparisons, signed local LiteSVM simulations and SDK regressions. Local simulation state/config is synthetic; no funded transactions were broadcast and production latency was not measured.

## v0.1.9 — Signed transaction and hot-path hardening

Fixes SPL CloseAccount opcode and Token/Token-2022/WSOL execution boundaries. Hardens cached preparation, nonce lifecycle, bounded executor queues and transfer-fee rounding. Adds signed transaction parsing and offline bank regressions. Caller-signed SWQoS submission does not automatically filter provider-owned minimum-tip lanes.

Validation includes local CPU benchmarks and offline signed-bank scenarios. Measured hot paths use cached inputs without RPC. Benchmarks do not establish production network or transaction-landing latency. No funded mainnet transactions were broadcast.

## v0.1.8 — Token-program and RPC fixes

PumpFun legacy/V2 buy and sell builders now honor the explicit `TokenProgram` from a parser event or mint-owner RPC lookup, regardless of the mint suffix. ATA creation and swap account derivation use that program consistently. Omitting it retains the Token-2022 default; supply the mint owner explicitly for SPL Token mints.

`TransferSOL` and `CloseTokenAccount` now use `getLatestBlockhash`. Regression tests cover mint suffixes, both token programs, legacy/V2 buys and sells, ATA derivation, and signed transactions against a local mock RPC. `go test ./...` and `go vet ./...` passed.

## v0.1.7 — CPMM creator-fee parity

Aligns the CPMM creator-fee collection surface with Rust 5.0.7: creator-signed (15 accounts) and permissionless (16 accounts) builders, mandatory share PDA, 236-byte AmmConfig and 145-byte CreatorFeeShare decoders, same-bank cold RPC rate reads, exact integer payout splitting, and RPC-free cached preparation with version/freshness/continuity revalidation. Uses shared mainnet simulation fixtures for direct account/byte/payout comparisons, including Token-2022 and a zero-rate override. PoolState and swap/LP behavior are unchanged. Estimates exclude Token-2022 transfer taxes; rates are read by the program at collection time.

See [CPMM example](examples/cpmm_creator_fee_replay/main.go) and [migration guide](docs/USAGE.md#api-compatibility).

## v0.1.6

Adds cache-backed trade/route preparation, CLMM/Whirlpool/DLMM integer quotes, PumpFun native settlement and multihop support, StonkFun routes, V1 serialization, token mint validation and simulation examples. Includes native alignment evidence and regression fixtures. The new Rust CPMM creator-fee collection API is not yet included in this language release; do not assume full Rust 5.0.7 API parity.

## Rust v5.0.2 Parity

This SDK now tracks the Rust SDK `v5.0.2` public behavior for high-level trade intent APIs and SWQoS provider coverage. Also aligned: `TradeRiskGate` / `WithRiskGate` (buy paths only), Meteora DAMM v2 `swapMode` / rate-limiter sysvar / referral, and Bonk slippage clamp at 9999 bps. New code can use `BuySimple` / `SellSimple` with `AccountPolicy`, `BuyAmount`, and `SellAmount`; these convert to the existing `Buy` / `Sell` params without removing the legacy API. The root `TradingClient` still documents its execution boundary and does not claim to build or submit trades by itself. SWQoS coverage includes LunarLander, Glaive, and Solami alongside the existing provider set. Explicit SWQoS routes still keep the default RPC lane appended. NextBlock is still filtered by the Rust parity blacklist unless Rust changes that behavior. Legacy extended provider classes such as `Triton`, `QuickNode`, `Syndica`, `Figment`, and `Alchemy` are kept only for source compatibility and are not part of Rust `v5.0.2` trading provider parity.

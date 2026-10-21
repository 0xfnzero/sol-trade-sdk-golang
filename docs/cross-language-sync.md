# Cross-language synchronization (v9.0.1)

The SDK and examples use their own language. All replays below run offline; no parser, price tracker, or cached quote hot path requests RPC.

## Trade evidence and wrappers

Use confirmed execution data and coherent current pool snapshots for trading.
Unresolved Shred candidates are not executable quotes: third_program preserves
opaque wrapper input, not inferred direction or amounts. RawInstruction and
PumpRouteIntent callbacks and wrapper-rule decoders have been removed.
See the matching [parser callback contract](https://github.com/0xfnzero/sol-parser-sdk-golang/blob/main/docs/wrapped-pumpfun.md)
for filtering, provenance and metadata details.

## Stock catalog and prices

Both parser and trade SDKs ship the same historical stock-token/pool catalog as Rust. Opt in through the screened catalog API and extend your owned copy. The catalog records fee/reserve observations; it is not live liquidity, a guaranteed executable depth, or a current tax quote. Pool preference reflects historical fees. Revalidate current state before trading.

The parser's `StockPriceTracker` consumes existing transaction events and produces notifications containing mint, symbol, pool, DEX, quote mint, reported execution price, optional SOL/USDC conversion, and optional supplied liquidity. It requires confirmed execution evidence and validates protocol-specific fields. Unresolved Shred placeholders never produce prices.

The SOL/USDC reference must precede the stock transaction, be fresh, and match the configured reference pool/protocol. Same-transaction conversion is withheld. A stale or missing reference leaves the unavailable denomination null. Supply coherent liquidity separately; unknown liquidity stays null. Reset the tracker on disconnect, dropped events, or a selected-fork change. Offline capture timestamps are replay inputs, not current market time.

Conversion-leg matching checks successful execution, protocol, program ID, pool and both mints. It returns independent matching legs, not a guaranteed connected path. StonkFun naming is retained. Pump.fun/StonkFun/PumpSwap launch origin does not restrict the conversion venue to Raydium or PumpSwap.

## Cached trade preparation

Trade SDKs expose a reusable `CandidateRouteGraph`: construct once when pool identities change and reuse for multiple searches. It owns topology, yields independent paths, enforces budgets and preserves deterministic breadth-first ordering. The compatibility route functions still build a graph per call.

Observed snapshots provide a separate observation watermark without changing actual account slot/write version. Only attest observation after a coherent baseline and continuous provider coverage of every dependency. Account replay, local time and unrelated slot heartbeats do not establish this. Readiness loss invalidates snapshots. Ordinary snapshots retain their conservative age checks.

## Native examples

Run from the repository root after installing its development dependencies:

```sh
go run ./examples/stock_conversion_replay
```

Stock examples accept `--fixture FILE`. Parser captures are dated 2026-10-20; trade examples replay normalized legs from those same signatures. Historical wrapper captures remain regression evidence and are not labeled live transactions.

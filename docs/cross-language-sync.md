# Cross-language synchronization (v0.2.0)

The SDK and examples use their own language. All replays below run offline; no parser, price tracker, or cached quote hot path requests RPC.

## Trade evidence and wrappers

RPC/gRPC uses supplied execution metadata and CPI instructions. Supported trade events expose `parse_status` and optional `third_program`. `ExecutionConfirmed` means a successful transaction supplied execution context; it does not guarantee every amount is known. Instruction budgets and minimum outputs are not fills. `InstructionDecoded` denotes decoded instructions without execution confirmation.

For unknown Shred wrappers whose own account list references Pump.fun, the default callback is `PumpFunTrade` with `Unresolved`. Only a candidate is known: direction, method, mint, pool and amounts remain defaults. A Pump.fun account reference does not prove a CPI occurred. Count candidates separately from confirmed trades. A missing ALT address remains null and cannot establish a Pump.fun reference.

`third_program` contains the outer `program_id`, `instruction_index`, ordered `accounts` (including duplicates/nulls), original `account_indices`, and byte-array `instruction_data`. Event metadata retains transaction position and receive time. Entries positions are local to a message, not slot-wide execution ordering.

Direction-only subscriptions do not receive unresolved candidates. Excluding `RawInstruction` does not exclude Pump.fun callbacks. Explicit `RawInstruction` subscription remains a compatibility API for all outer instructions. An unknown non-Pump router is not turned into a confirmed swap solely from account references. RPC/gRPC can decode supported native CPI swaps under those routers normally.

Optional wrapper rules remain explicit, offline-reviewed helpers. Their restored input fields carry `InstructionDecoded`; they do not establish fills. Rules are never installed automatically and are never applied to gRPC execution events.

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

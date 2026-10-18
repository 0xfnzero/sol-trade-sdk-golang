# PumpSwap deployed-program replay

From the Go repository root:

```sh
go run ./examples/pumpswap_offline_bank
go test ./examples/pumpswap_offline_bank -count=1 -v
go test ./examples/pumpswap_cached_prepare -run '^$' -bench BenchmarkPumpSwapCached -benchmem
```

The SDK process prepares and signs actual V0 transactions. A separate **Go**
LiteSVM engine executes the unmodified deployed SBF programs. The process boundary
avoids conflicting Yellowstone protobuf registrations in the two Go dependency
sets; no Python or Rust runner is used. The first build may resolve Go modules.
The replay itself has no RPC client or transaction broadcaster.

The direct TTWO/Dacity matrix covers independently quoted buys and sells, static
accounts and cached ALTs, successful execution and signed impossible-slippage
transactions. Four more ALT scenarios execute the complete SOL→USDC→TTWO→target
buy and reverse sell, including final-swap failures after earlier swaps execute.
Each of the 12 scenarios repeats in a fresh bank and compares outcomes. It verifies
Ed25519 signatures and blockhashes, rejects nonexistent ALTs before execution,
checks simulation leaves state unchanged, checks committed state against simulated
post-accounts, checks exact input debit and protected output credit, and checks
failed transactions roll back token state while charging only the transaction fee.
V0 gas configuration with zero compute-unit price is exercised by every case.
Existing cached preparation tests cover V1 wire decoding, signatures, nonce/tip,
route selection, freshness and concurrent V0/V1 construction.

The program bundle in `../pumpswap_bank` contains public deployed programs from
October 8/9 captures. Pool/mint/vault accounts come from the October 18 frozen
route snapshot, and the global volume accumulator, DLMM reserves and oracle from
separate public reads.
These are mixed historical captures, **not an atomic mainnet fork**. New
synthetic account lamports use local rent-exempt balances; captured vaults retain
their original lamports so native WSOL remains backed. Wallets, token balances, fee ATAs
and ALTs are synthetic local state; Token2022 account extensions are preserved
from captured vault layouts. The public deterministic test key must never be
funded on chain. The tests neither spend mainnet funds nor establish current
mainnet acceptance.

Benchmarks measure local cache preparation/search and transaction signing only.
Fixture loading, bank startup and compilation are outside timed loops. Results
are hardware/runtime dependent and do not measure submission or landing latency.

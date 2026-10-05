# PumpFun native-quote concentrated multi-hop verification — 2026-10-05

Baseline: Rust trade 5.0.6 (dc9741b6074df21418f16b6175f497fbebecb723), parser 0.7.7 (78a686a012ef3d685624d1b1bdba39766cb000bd). Native Go, Node.js and Python; no Rust dependency, router or broadcast.

Six independent current-mainnet simulations verify USDC → WSOL → PumpFun meme and the reverse sell. The existing cached factory/settlement code passes these combinations; this increment adds shared frozen fixtures, missing-array regressions, parser execution corpora and a reproducible verifier. It does not change the pricing formulas or expand the supported protocol set.

| Conversion | Direction | Paid units | Protected output | Actual output | Original WSOL delta | V1 bytes |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Whirlpool | Buy | 1,000 USDC | 92,415,771 meme | 97,279,759 meme | +410 | 1,610 |
| Whirlpool | Sell | 100,000,000 meme | 853 USDC | 898 USDC | 0 | 1,407 |
| DLMM | Buy | 1,000 USDC | 92,668,371 meme | 97,532,993 meme | +411 | 1,629 |
| DLMM | Sell | 100,000,000 meme | 855 USDC | 900 USDC | 0 | 1,426 |
| CLMM | Buy | 1,000 USDC | 92,728,466 meme | 97,596,251 meme | +411 | 1,639 |
| CLMM | Sell | 100,000,000 meme | 855 USDC | 900 USDC | 0 | 1,436 |

All amounts are raw integer token units. All six responses have err=null; pre/post token-account balances prove the paid debit and protected credit. Existing returned token accounts preserve token-program owner, mint, authority and initialized state. The original WSOL account remains open. Each sell leaves 390 lamports in native SOL after converting the protected minimum; that is not a WSOL residual.

Pools: Whirlpool HJPjoWUrhoZzkNfRpHuieeFk9WcZWjwy6PBjZ81ngndJ; DLMM 5rCf1DM8LjKTw4YqhnoLcngyZYeNnQqztScTogYHAS6; CLMM 3ucNos4NbumPLZNWztqGHNFFgkHeRMBQAVemeeomsUxv. The CLMM pool was observed by the native Python Yellowstone gRPC client using the configured GRPC_URL/GRPC_TOKEN at slots 453329502–453329503. This run does not claim renewed gRPC discovery of the other two pools.

Cold preparation re-read current pool/mint/config/clock accounts and derived current directional tick/bin arrays. Account reads and latest blockhash/rent queries occurred only before prepare. Quotes and V1 preparation made zero RPC calls; simulateTransaction was explicit. The snapshot and simulation bank can differ: do not require a later simulation's actual execution to equal this historical estimate. Buy additionally creates PumpFun tracking state, so returned wallet/ATA balances alone do not reconstruct all rent costs.

Go/Node.js/Python examples independently recreate the same wire bytes, minimums and residual classification for all six snapshots. Node/Python tests prohibit network access and remove an actually referenced array to require preparation failure. Parser corpora match all normalized route fields across the three languages: conversion-leg execution input/output are known, PumpFun buy wallet-SOL debit is known, and its output remains unknown without Token-2022 transfer-fee evidence. PumpFun sell wallet-SOL output remains unknown rather than being replaced by a quote.

Evidence: examples/fixtures/pumpfun_concentrated_multihop_simulations_20261005.json. Snapshots: examples/fixtures/pumpfun_{whirlpool,dlmm,clmm}_usdc_{buy,sell}_20261005.json. Parser shared corpus: pumpfun_concentrated_multihop_mainnet_simulations_20261005.json in each parser's fixture directory.

From the workspace root run a Python interpreter with SDK dependencies:
`python tools/native-parity/verify_pumpfun_concentrated.py`
It executes all three offline examples and recomputes actual token debits/credits, validates account identity, checks sell native surplus and compares every wire to the saved simulation. It requires Go, installed Node dependencies and tsx (also available from the parser Node development dependencies). No simulation or send is invoked by this replay command.

Validation: trade Node 3,347 tests; trade Python 3,429 tests; parser Node 234 passed / 8 network cases skipped; parser Python 220 tests; both Go full suites pass. Skipped cases are not acceptance evidence.

Remaining work: comprehensive Rust public API/event-field matrix; all stock/DEX and alternate settlement combinations; gRPC continuity/fork/new-array recovery acceptance; provider-specific SWQOS non-broadcast verification and separately authorized live acceptance. These six PumpFun routes do not certify all StonkFun routes or full SDK parity.


## Follow-up settlement review and fixes

The three public PumpFun settlement helpers now require matching quoted-leg/swap instructions (program, data, ordered accounts and signer/writable flags), the same payer, positive u64 input/protection, and final protection consistent with the last leg. A native anchor additionally requires exact-in V2, matching serialized input/minimum amounts, wallet at index 13, native quote mint, base mint and curve pool. This fixes single-hop wrong-wallet/foreign-protocol acceptance and same-wallet instruction replacement in multi-hop. Invalid endpoint flag types are rejected in Node/Python; Go uses typed booleans. Go rejects nil setup instructions/accounts instead of panicking. Regression tests cover these failures; all six valid cross-language wires remain byte-identical.

This is structural consistency validation, not a replacement for validating current pool state, account balances, token fees and subscription continuity before preparation.

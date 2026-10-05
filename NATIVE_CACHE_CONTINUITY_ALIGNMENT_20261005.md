# Native cache continuity fixes — 2026-10-05

Scope: the Go, Node.js and Python trade subscription caches/readiness gates. Fixed baseline remains Rust trade 5.0.6 and parser 0.7.7. No RPC was added to quoting, preparation or signing.

Two defects are fixed:

1. After interrupt(), adding a tick/bin dependency formerly changed continuity-broken to recovering. markValidated() could then make the cache ready without beginRecovery(). Dependency additions now retain continuity-broken and its reason. Explicit fork selection and complete validation are still required.
2. Equal slot/write-version updates with conflicting owner or bytes formerly raised an error but left frozen snapshots usable. A conflict now permanently invalidates the source cache and every snapshot obtained from it, including readiness-bound snapshots and optional-account reads. Subsequent updates cannot silently choose a branch or revive the cache. The conflicting batch remains uncommitted.

The invalidation deliberately covers the whole selected-fork cache: accounts streamed independently cannot establish which other accounts came from the conflicted branch. Recovery requires a new SubscriptionAccountCache for the explicitly selected fork, current verified account data, and a readiness generation validated for every required account. Resetting readiness on the old conflicted cache is insufficient. Healthy snapshots still preserve frozen bytes when ordinary newer updates arrive.

Migration: update/updateMany retain their signatures and conflict errors. snapshot() can still return a snapshot object, but all its reads/preparation checks reject a conflicted cache; readySnapshot() rejects it immediately. Callers must treat an update conflict as a continuity failure, stop preparation, select/revalidate the fork and replace their cache. Never treat the update error as a recoverable per-message warning and continue preparing from an old snapshot.

Regression coverage in all three languages:
- ready → interrupt → require new array remains interrupted;
- late generation or wrong-fork validation fails;
- all dependencies must validate after explicit recovery;
- data, owner and closure conflicts invalidate plain and live-bound snapshots;
- optional reads cannot bypass invalidation;
- readiness recovery alone cannot revive conflicted account state;
- a new explicitly validated cache works, while the old guard remains invalid.

Verification: Python trade 3,433 tests; Node trade 3,351 tests; Go full suite/vet and subscription/trading race tests pass. Node typecheck, example types and production build pass. The six previously simulated Whirlpool/CLMM/DLMM PumpFun routes recreate identical wires and amounts in all three offline examples after this change.

These are deterministic state-machine/cache tests. They do not certify live fork recovery, dynamic array discovery or full gRPC reconnect acceptance. Those remain separate items in the overall alignment plan. No transaction was broadcast or package published.


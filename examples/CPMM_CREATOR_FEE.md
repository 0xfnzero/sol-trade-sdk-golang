# CPMM creator-fee collection (v0.1.7)

This API matches the Rust 5.0.7 collection surface. Decoders take **full Anchor account bytes**, including discriminator: PoolState 637 bytes, AmmConfig 236, CreatorFeeShare 145. Existing swap/LP reserve accounting continues subtracting gross creator-fee counters.

| Operation | Node.js (root export) | Python (root export) | Go |
|---|---|---|---|
| Creator-signed instruction | collectCreatorFee | collect_creator_fee | instruction.CollectCreatorFee |
| Permissionless instruction | collectCreatorFeePermissionless | collect_creator_fee_permissionless | instruction.CollectCreatorFeePermissionless |
| Share PDA | getCreatorFeeSharePda | get_creator_fee_share_pda | instruction.GetCreatorFeeSharePDA |
| Exact split | splitCreatorFee | split_creator_fee | instruction.SplitCreatorFee |
| Cold RPC snapshot | fetchCreatorFeeShareRate | fetch_creator_fee_share_rate | instruction.FetchCreatorFeeShareRate |
| Cached preparation | prepareCpmmCreatorFeeCollection | prepare_cpmm_creator_fee_collection | snapshot.PrepareCpmmCreatorFeeCollection |
| Before-use validation | validateCpmmCreatorFeeCollection | validate_cpmm_creator_fee_collection | snapshot.ValidateCpmmCreatorFeeCollection |

Pass the actual pool creator, config, vaults, mints and token programs from the decoded pool. Canonical recipient ATAs always belong to the creator, including permissionless calls and Token-2022. The share PDA is always included even if absent on-chain. Creator-signed calls have 15 accounts; permissionless calls have 16, retaining original accounts 1–14.

Rates use millionths. A valid share PDA overrides config, including zero; missing, empty, closed or foreign-owned share accounts use config. Malformed CPMM-owned share accounts, mismatched creator/config, invalid rates and RPC errors are rejected. Protocol share rounds **down**, creator receives remaining dust. Estimates exclude Token-2022 transfer tax and the program reads the rate at execution time.

Preparation takes an immutable subscription snapshot and read context, with **no RPC**. Explicitly observe the share PDA: missing cache entries are unknown, not proof of absence. A validated absence/closed update is an empty-data tombstone. Use the existing readiness/continuity guard for live subscriptions. Validate against a fresh guarded snapshot before using prepared data; account-version changes, stale/future observations, protocol-counter overflow and tampered fields are rejected. Independent streamed accounts are not guaranteed to be one atomic bank snapshot.

Offline replay commands from each repository:

```sh
# Node.js
npx tsx examples/cpmm_creator_fee_replay.ts
# Python
uv run python examples/cpmm_creator_fee_replay.py
# Go
go run ./examples/cpmm_creator_fee_replay
```

The six cases reuse Rust mainnet unsigned simulation captures (default rate, zero override, Token-2022; signed/permissionless). Tests compare full account identities/flags, discriminators and instruction bytes, share PDA and exact payout/protocol split. These examples never sign or submit. Captured expected payouts before transfer taxes do not promise future execution results.

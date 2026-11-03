# sol-trade-sdk-golang v11.0.1

Released November 3, 2026.

## Changes

Complete the Go v11 module migration in the protobuf source declaration and keep the trade SDK family aligned at 11.0.1. The earlier 11.0.0 releases remain available.

Coordinate the trade SDK family with sol-parser-sdk 0.11.0 and solana-streamer-sdk 6.0.0. Rust parser adapters and examples use the published dependencies. Refresh all SDK navigation and installation versions. Existing V1/V2/V3 instruction selection and Auto / Official / Sequential routing behavior remain unchanged. Node.js and Python have no new trading behavior in this coordinated release.

## Migration

Rust consumers using parser-adapter must upgrade their direct sol-parser-sdk dependency to 0.11.0; types from 0.10 and 0.11 are distinct. The public dependency change is released as a major family version. Go imports and go.mod must change from github.com/0xfnzero/sol-trade-sdk-golang/v10 to github.com/0xfnzero/sol-trade-sdk-golang/v11; run go get github.com/0xfnzero/sol-trade-sdk-golang/v11@v11.0.1 and go mod tidy. Node.js and Python package names are unchanged.

## Validation

Native trading regressions and official multi-hop examples were validated. Go validation includes an external consumer of the new module path. No live trade was broadcast.

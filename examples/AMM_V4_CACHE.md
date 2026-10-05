# AMM v4：冻结缓存到 V1 交易

使用完整的 `cached_trade` 示例。普通 WSOL 使用现有 WSOL ATA；只有显式 `native_input` 或 `native_output` 才创建并关闭本次临时 WSOL 账户。买入与卖出独立执行，没有 router 合约。

```sh
go run ./examples/cached_trade examples/fixtures/amm_v4_buy_mainnet_20261002.json
go run ./examples/cached_trade examples/fixtures/amm_v4_sell_mainnet_20261002.json
go run ./examples/cached_trade examples/fixtures/amm_v4_route_buy_mainnet_20261002.json
go run ./examples/cached_trade examples/fixtures/amm_v4_route_sell_mainnet_20261002.json
```

这些保存的主网快照用于离线复现，不能当作当前池状态。单池 fixture 支付／收款是 SOL 与 USDC，多跳是 SOL ↔ USDC ↔ 股票 ↔ meme。单池 `dex_type=RaydiumAmmV4`，多跳目标内盘 `dex_type=StonkFun`；交易方向显式 `Buy` 或 `Sell`。

实盘初始化时在冷路径取得 AMM pool（752 bytes）、两个 vault（各165 bytes）、两个经典 SPL Token mint（各82 bytes）、Clock、blockhash以及当前rent。其后使用 parser 的 `stonkfun_snapshot_refresh` 示例订阅 JSON 中的 `accounts[]`；该示例不限于 StonkFun，AMM v4也适用。raw pool/vault与Clock更新需经过缓存slot/write_version检查。静态mint没有更新时保留其真实旧slot；账户过期直接拒绝。需要重新冷启动时应在交易热路径外执行，不能伪造slot或隐式RPC补齐。

使用自己的payer、amount、按本次方向排序的 `legs`、新snapshot/read context、blockhash，并为每笔SOL交易指定唯一seed。`--simulate`仅调用RPC模拟且不发送；用该选项之前必须提供当前状态及有相关资产的模拟payer。现成fixture包含测试钱包地址和历史租金，不应直接用于自己的交易。

缓存入口校验实际池费率、开盘状态、authority nonce、mint与vault身份；报价扣除 `need_take_pnl_coin/pc`，使用ceil swapfee和floor滑点。构建官方8账户V2 exact-in（tag16），不需要OpenBook账户。Token-2022池不适用于AMM v4。本语言API：snapshot.`AmmV4`、`QuoteCachedAmmV4ExactIn`、snapshot.`PrepareAmmV4`。

证据：`fixtures/amm_v4_mainnet_simulations_20261002.json`包含三语言12次独立模拟；每个交易方向三语言wire字节一致。Rust数学对照及执行前余额重算见语言测试fixture，运行时不需要Rust。

## Legacy buy/sell V2 simulation

旧AMM入口也已迁移至V2。单池验证可运行：

```sh
# Node
npx tsx examples/legacy_amm_v2.ts examples/fixtures/legacy_amm_v2_buy_20261002.json --simulate
# Python
PYTHONPATH=. python examples/legacy_amm_v2.py examples/fixtures/legacy_amm_v2_buy_20261002.json --simulate
# Go
go run ./examples/legacy_amm_v2 examples/fixtures/legacy_amm_v2_buy_20261002.json --simulate
```

使用sell快照可独立验证卖出；追加 `--exact-output=50000`（buy）或 `--exact-output=30000`（sell）验证V2 exact-out。输入amount此时为最大输入；exact-out构建器不会替你计算预算可行性。历史快照用于离线复现，当前执行需要当前状态。此示例Buy按native_input标志选择SOL充值WSOL ATA／已有WSOL；Sell保留WSOL，不关闭已有ATA。SOL收款和多跳请使用cached_trade。证明在 `legacy_amm_v2_simulations_20261002.json`，全部只模拟。

# Cached LaunchLab routes

Generic LaunchLab/Bonk cached routes validate the pool's actual global/platform configuration instead of applying the StonkFun platform whitelist. The StonkFun executor and stonkfunCurve/stonkfun_curve/StonkFunCurve remain strict. The current curve implementation supports constant-product curve type 0, exact-in; this change does not add other curve types.

Use the existing cached_trade example with dex_type=LaunchLab (or Bonk), independent Buy/Sell, current subscribed account snapshots and explicit leg pool/input_mint/output_mint. Each leg must connect to the next mint. Choose SOL with native_input/native_output, or WSOL/USDC with explicit token mints and corresponding existing liquidity. Quote/build do not query RPC.

Node exports decodeLaunchLabCurve/buildLaunchLabCurveExactIn; Python exports decode_launchlab_curve/build_launchlab_curve_exact_in; Go instruction exports DecodeLaunchLabCurve/BuildLaunchLabCurveExactIn. Snapshot launchlabCurve/launchlab_curve/LaunchLabCurve decodes and validates subscribed state; configuration and mint/vault accounts must be present in the snapshot.

Offline fixture: examples/fixtures/review10_launchlab_generic.json. This is a synthetic platform-identity regression, NOT mainnet evidence. Run the cached_trade example without --simulate. The three native examples produce identical wire. Never submit this synthetic transaction. A real non-Stonk platform pool still requires live simulation before claiming platform-specific mainnet validation.

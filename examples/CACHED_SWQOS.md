# Cached V1 → SWQOS

准备好当前账户缓存、明确的池路径、自己的钱包及交易金额后，可以把 low-level SWQOS client 适配为缓存执行器的 submit 回调。构建／报价不查询 RPC，适配器不请求确认轮询；receipt 的 `confirmed` 为 false。适配器校验单签名 V1 布局并原样传递字节，执行器校验返回签名。不要交给旧 `VersionedTransaction` 包装器重新序列化。

以下片段是实际发送的接入方式，需要调用方已有 `request`、自己的 signer、已配置的 low-level `client` 和服务端 V1 支持。本仓示例验证只使用模拟或本地 mock，没有发送真实交易。请求现在支持显式 tip 账户与 lamports；构建器会在签名前加入 System Program tip。服务端要求的金额由调用方选择，发送适配器不会自动添加或重复支付。tip 账户可从已配置 low-level client 的本地 tip 列表选择，不查询 RPC。

```ts
import { TradeExecutorFactory } from "sol-trade-sdk/trading";
import { createCachedWireSubmit } from "sol-trade-sdk/swqos";
import { PublicKey } from "@solana/web3.js";
const tippedRequest = {...request, tipAccount:new PublicKey(client.getTipAccount()), tipLamports:5000n};
const executor = TradeExecutorFactory.createCachedExecutor(request.dexType);
const receipt = await executor.execute(tippedRequest, [payer], createCachedWireSubmit(client));
```

```python
from sol_trade_sdk.trading.factory import TradeExecutorFactory
from sol_trade_sdk.swqos import create_cached_wire_submit
from dataclasses import replace
from solders.pubkey import Pubkey
tipped_request = replace(request, tip_account=Pubkey.from_string(client.get_tip_account()), tip_lamports=5000)
executor = TradeExecutorFactory.create_cached_executor(request.dex_type)
receipt = await executor.execute(tipped_request, [payer], create_cached_wire_submit(client))
```

```go
// executor is the existing *trading.CachedTradeExecutor.
request.TipAccount = solana.MustPublicKeyFromBase58(client.GetTipAccount())
request.TipLamports = 5000
receipt, err := executor.Execute(ctx, request, []solana.PrivateKey{payer},
    swqos.NewCachedWireSubmit(client))
```

`request` 的完整创建与gRPC快照输入见 `cached_trade` 和 `AMM_V4_CACHE.md`。V1单签名的消息／signature边界、本地HTTP原始字节发送和Buy/Sell方向均有回归测试；不能将本地测试视为所有服务的线上验收。

金额是整数 lamports（示例5000不代表所有服务的最低要求）。tip必须预留SOL，即使支付／收款资产是USDC或WSOL。账户／金额需成对提供，零额、默认账户、自付和预算u64溢出均拒绝；未配置tip保持原交易字节。`requiredNativeLamports`／`required_native_lamports` 包含临时WSOL账户资金和tip，不含交易基础费或其它ATA租金。保护输出金额仍为交易到账，不能当作扣除SOL tip的净利润。

三语言完整构建与模拟可使用 cached_trade 和 examples/fixtures/cached_tip_route_buy_20261002.json（或sell）；仅 --simulate 调用RPC，默认无发送。模拟请求启用innerInstructions，执行证据在 cached_tip_simulations_20261002.json。


2026-10-03 boundary checks: The raw-wire adapter now verifies the transport's returned signature against the supplied transaction. V1 framing rejects a readonly payer, duplicate account keys and invalid heap configuration. Go checks cancellation before invoking the transport. These checks run locally; provider tests are mocked and do not constitute live service acceptance.

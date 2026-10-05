# PumpFun 原生 quote 多跳结算（2026-10-04）

Go、Node.js、Python 的 cached factory 已支持一个 native-quote PumpFun anchor 的多跳交易。允许买入最后一跳、卖出第一跳，沿用现有 exact-in、最多 5 跳、无资产环和无重复池限制。买卖仍是独立请求，没有 router 合约或套利闭环。

## 资金衔接

买入 USDC/股票等资产先经已验证候选池兑换到 WSOL，按照上一跳保护 minimum 构建下一跳输入。只把 PumpFun 要花的固定 WSOL 数量转入新临时账户并关闭该临时账户，回到钱包 SOL 后执行 PumpFun；原 WSOL ATA 保留。兑换剩余是 WSOL，不消费用户原先持有的 WSOL。

卖出先由 PumpFun 接收钱包 SOL，仅包装该曲线的保护 minimum 到 WSOL ATA，再执行其它 DEX 指令。超出 minimum 的部分仍是 SOL，单独返回 estimatedNativeResidualLamports / EstimatedNativeResidualLamports / estimated_native_residual_lamports；WSOL 中间残余只计算实际包装量减下一跳输入。不能把原生 SOL 剩余报告成 WSOL 剩余。

拒绝超出保护 credit 的下一跳、错钱包、多曲线、非 anchor native 曲线和不匹配的 native endpoint flags。非 native quote 的 PumpFun anchor 不会给其它 native 曲线开放 raw 构建权限。通用 prepareRoute 默认仍拒绝未经 factory 结算的 native quote，不把原始指令当成已支付 WSOL。

## 实际证据

调用方提供已验证的 Raydium AMM v4 SOL/USDC 池与 PumpFun 池，显式冷更新状态后，各语言准备/报价/编译不调用 RPC。独立模拟：

- USDC -> WSOL -> meme：实际扣除 USDC 1000，实际到账 meme 96924209（保护 92089955）；WSOL 原余额增加 411。V1 wire 1513 字节。
- meme -> WSOL -> USDC：实际扣除 meme 100000000，实际到账 USDC 902（保护 856）；WSOL 原余额不变，实际 SOL 残余 393。V1 wire 1310 字节。

三语言 wire 相同。实际买入数量和历史报价略有差异，验收按实时模拟到账满足保护金额，并不要求模拟银行变化后恒等于历史估值。V1 不依赖 ALT。仅确认模拟服务接受这些 wire，不声称所有发送商支持。

## 示例

使用已有完整 examples/cached_trade 示例读取 examples/fixtures/pumpfun_usdc_buy_20261004.json 或 pumpfun_usdc_sell_20261004.json；--simulate 为显式 RPC 模拟，不发送。Python、Node、Go 读取同一 schema。真实响应及账户余额见 examples/fixtures/pumpfun_usdc_multihop_simulations_20261004.json。

实时仍通过 parser Yellowstone gRPC 得到线索和账户更新，GRPC_URL / GRPC_TOKEN 不打印。冷引导 snapshot 可经 parser 的 stonkfun_snapshot_refresh 示例更新，状态失去连续性时必须重新确认准备；这些 fixture 是可复放证据，不能直接拿历史 snapshot 用作新的热路径交易。

## 边界

本轮只真实验收了 AMM v4 + native PumpFun 的 USDC 双向路径。代码衔接可复用已有其它 DEX 的缓存指令，所有 CLMM/Whirlpool/DLMM/股票路径组合尚未逐项实际模拟。全收益 WSOL 化、非 native USDC/股票 quote 当前链模拟、完整公开能力矩阵及其它 gRPC/SWQOS 验证仍未全部完成。先前“native quote 多跳全部拒绝”的历史状态由本记录替代。

独立 npm 安装和 Python wheel 安装后的公共 API，均复现两方向的实际模拟 wire；六仓 diff-check 通过。

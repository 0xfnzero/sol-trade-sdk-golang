# PumpFun 持币卖出真实模拟验收（2026-10-04）

本轮从 Yellowstone gRPC 的 token account owner + mint memcmp 订阅发现 9 个有余额的账户，再显式冷验证 mint、authority、owner、初始化状态、余额和钱包 SOL 预算。8 个钱包完成独立 SOL / WSOL 卖出模拟，共 16 次成功；没有广播。选择 canonical ATA 钱包的两笔原始 factory wire 保存为三语言相同 fixture，均无需额外资金前置指令。

SOL 卖出：实际源账户扣除 100000000 meme 最小单位，钱包收到 7939 lamports（扣除交易费前）；模拟钱包净变化 +2939，交易费 5000。

WSOL 卖出：同样实际扣除 100000000；合法、已初始化的 WSOL ATA（mint、token program owner、authority 验证通过）新增 7542，恰等于保护 minimum。钱包保留 397 lamports 的实际 SOL 残余，和 estimatedNativeResidualLamports 对应。新增 ATA rent 1493440、交易费 5000 分开核对。该实现仍不是保证全部卖出收益都变成 WSOL：超过保护金额的部分留作 SOL。此限制不能因为 err=null 而隐藏。

所有报价、准备与编译零 RPC；账户初始化和 simulateTransaction 是显式冷操作。每次买卖独立，不构造套利闭环或 router 合约。

## 验证

examples/fixtures/pumpfun_funded_sell_simulations_20261004.json 保留源账户冷状态、wire、响应、实际扣款/收款、租金/fee 与残余证明。Go、Node、Python 的 SOL wire 1011 字节、WSOL wire 1046 字节完全相同。

parser 三语言保存 pumpfun_funded_sell_mainnet_simulations_20261004.json：成功卖出输入准确为 100000000，native 收款账户为钱包。直接 lamport 修改的实际输出仍为未知，不能据净钱包变化在通用解析中猜测成交量（新 ATA / volume account 的 rent 会干扰）。失败交易保留意图，实际输入/输出未知。

实验脚本最初重编译 V1 时漏带 loaded accounts 预算，产生 MaxLoadedAccountsDataSizeExceeded；保留失败 wire 和响应，并加入 parser 回归。这是实验脚本遗漏，不是 SDK factory 错误。补回 SDK 的 64MiB 预算后成功，最终所选 wire 与 factory 原始输出一致。

先前未持币的交易钱包核验记录仍保留；其“未验收”是当时状态，现由本次持币账户实际模拟补齐。原生 quote PumpFun 多跳、全部收益 WSOL 化、非原生 USDC/股票 quote 模拟、完整公开 API/字段矩阵和其余 gRPC / SWQOS 验收仍未全部完成。


Native quote 多跳现已接入，USDC/WSOL 路径通过双向真实模拟：[最新多跳验收](NATIVE_PUMPFUN_MULTIHOP_ALIGNMENT_20261004.md)。旧的全部多跳拒绝状态已被替代，完整组合矩阵仍未全部验收。

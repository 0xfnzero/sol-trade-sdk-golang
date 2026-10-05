# DAMM cached factory / parser route 验收（2026-10-04）

固定基线：trade 5.0.6 `dc9741b6074df21418f16b6175f497fbebecb723`，parser 0.7.7 `78a686a012ef3d685624d1b1bdba39766cb000bd`。整体对齐仍未完成。

三语言 cached factory 已接入 DAMM 单跳 exact-in，Buy/Sell 分别使用对应 builder；冻结缓存校验账户、激活、epoch、freshness 和连续性，热路径不补 RPC。Node fixedOutputAmount / Python fixed_output_amount / Go FixedOutputAmount 必须为正 u64 显式下限，其它协议拒绝该字段。预计净输出未知，返回 null/None/nil。非零转账费下限与 DAMM 多跳明确拒绝。

新 examples/cached_damm_v2 默认离线准备，仅 --simulate 显式联网。SOL 使用临时 WSOL；已有 WSOL 不充值、不 SyncNative、不关闭。旧 damm_v2_snapshot 为历史 wire 复现保留 SOL funding，不能作为已有 WSOL 输入的示范。

三语言 parser route 新增 DAMM swap/swap2：mode 0/1 exact-in，2 exact-out，其它 unknown；mode 2 第一个金额为期望输出、第二个为最大输入。方向来自用户账户、余额和 transfer 证据；失败保留意图，成交量未知；Token-2022 缺净到账证据保持空值。历史 expected 已补 DAMM leg，原始响应/wire 保留。

实际主网模拟：SOL 买入成功（821 bytes），余额充足的合法 WSOL ATA 买入成功（659 bytes），原钱包 WSOL 不足失败 InstructionError [2, Custom 1]，不计成功。广播次数为零。trade 原始证据见 examples/fixtures/cached_damm_v2_simulations_20261004.json，parser corpus 见 examples/fixtures/cached_damm_v2_mainnet_simulations_20261004.json。卖出只有离线回归：所检查 fixture 钱包没有足够 meme 余额，实际卖出未验收。USDC/股票矩阵、自动报价与多跳也未验收。

全量验证：trade Node 3054、Python 3135；parser Node 210 passed / 8 skipped、Python 196；Go 两仓 test/vet/race 通过，Node 类型/示例/build 通过。独立 npm CJS/ESM trading 安装发现 protobufjs/minimal 路径错误，修复为 minimal.js 默认导入后通过；两 wheel 独立安装和离线重放通过。Go 为本地 module consumer，非远端安装验收。

未改版本、提交或发布。完整公开 API/字段矩阵、gRPC fork 恢复及新数组发现、PumpFun 当前配置缓存准备和 SWQOS 服务商证据仍需完成。


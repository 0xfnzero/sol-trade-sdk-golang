# PumpFun 当前配置缓存（2026-10-04，部分对齐）

固定 Rust trade 5.0.6 `dc9741b6074df21418f16b6175f497fbebecb723`，对照 `instruction/utils/pumpfun.rs` Global/SharingConfig 布局和 `trading/core/params/pumpfun.rs::from_mint_by_rpc`。

三语言新增配置读取：Node trading.cachedPumpFunConfiguration(snapshot,mint,context)，Python trading.cached_pumpfun_configuration，Go snapshot.PumpFunConfiguration。Global owner/program/address/discriminator 校验后读取 offset 41..73 当前 fee recipient；SharingConfig 必须位于 fee program 的 sharing-config,mint PDA，size >=43，验证 discriminator 和 mint。status=1 返回 creator-vault(SharingConfig PDA)，其它状态返回未激活空值。仅配置读取，不提供费用报价或 cached factory。

所有读取通过现有冻结快照：缺失/过期/未来/错误 owner/关闭账户或连续性中断拒绝；Global 零收款人拒绝。缺少 SharingConfig 不等于未激活，必须在冷准备层明确确认。当前 API 要求缓存已有有效配置账户，暂不提供可验证的不存在证明；无此账户的币不能仅靠此入口完成准备。此严格缓存行为与 Rust 显式冷 RPC helper 的可选配置行为有意区分。

Node PumpFunParams.fromMintByRpc 补读取 Global 和可选 SharingConfig，保存当前收款人和激活的分成 vault，不再默认 feeRecipient=zero 或固定 creator vault。冷 RPC 可选配置为空/错误布局/错误 owner 时按 Rust helper 返回未激活；RPC 网络异常继续向外传播。此入口只能用于冷初始化，不能用于交易热路径。

验收：Node 全量 3068、Python 全量 3144；Go test/vet/race，Node 类型/示例/build。新缓存测试覆盖活动/非活动/缺账户、过期、owner/mint/discriminator、零收款人、截断和断线 guard；Node 冷加载额外验证实际配置选取及 Global 缺失/owner/discriminator/size。配置为合成账户，未新增主网模拟或真实广播，不能据此宣称 PumpFun 当前费用报价/factory 已验收。

仍需 FeeConfig/Global 全字段当前费率、Token-2022、实际买卖模拟、配置不存在证明与 gRPC 验证恢复接线。整体对齐未完成。


独立安装验证已通过：npm CJS/ESM trading 入口（禁止 fetch）、Python wheel（禁止 socket connect）以及本地 Go module consumer 均通过完整配置缓存读取。包 SHA256 见 NATIVE_PUMPFUN_CONFIG_EVIDENCE_20261004.json。Go consumer 使用本地复制 module，未验证远端发布包。

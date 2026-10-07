// Compact Pump instructions; official IDL commit 8cda1fa.
package instruction

import (
	"encoding/binary"
	"fmt"
	"github.com/gagliardetto/solana-go"
)

type pumpUpgradeAccount struct {
	name             string
	writable, signer bool
}
type pumpUpgradeSpec struct {
	program  string
	disc     [8]byte
	accounts []pumpUpgradeAccount
	args     int
	partial  bool
}

var pumpUpgradeSpecs = map[string]pumpUpgradeSpec{
	"pump_buy_v3":                    {"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P", [8]byte{7, 5, 29, 196, 245, 23, 101, 80}, []pumpUpgradeAccount{{"global", false, false}, {"base_mint", false, false}, {"quote_mint", false, false}, {"base_token_program", false, false}, {"quote_token_program", false, false}, {"bonding_curve", true, false}, {"associated_base_bonding_curve", true, false}, {"associated_quote_bonding_curve", true, false}, {"user", true, true}, {"associated_base_user", true, false}, {"associated_quote_user", true, false}, {"user_volume_accumulator", true, false}, {"fee_config", false, false}, {"buyback_fee_recipient", true, false}, {"system_program", false, false}, {"event_authority", false, false}, {"program", false, false}}, 2, true},
	"pump_buy_exact_quote_in_v3":     {"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P", [8]byte{225, 247, 80, 30, 213, 179, 132, 136}, []pumpUpgradeAccount{{"global", false, false}, {"base_mint", false, false}, {"quote_mint", false, false}, {"base_token_program", false, false}, {"quote_token_program", false, false}, {"bonding_curve", true, false}, {"associated_base_bonding_curve", true, false}, {"associated_quote_bonding_curve", true, false}, {"user", true, true}, {"associated_base_user", true, false}, {"associated_quote_user", true, false}, {"user_volume_accumulator", true, false}, {"fee_config", false, false}, {"buyback_fee_recipient", true, false}, {"system_program", false, false}, {"event_authority", false, false}, {"program", false, false}}, 2, true},
	"pump_sell_v3":                   {"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P", [8]byte{28, 146, 222, 119, 38, 196, 105, 213}, []pumpUpgradeAccount{{"global", false, false}, {"base_mint", false, false}, {"quote_mint", false, false}, {"base_token_program", false, false}, {"quote_token_program", false, false}, {"bonding_curve", true, false}, {"associated_base_bonding_curve", true, false}, {"associated_quote_bonding_curve", true, false}, {"user", true, true}, {"associated_base_user", true, false}, {"associated_quote_user", true, false}, {"user_volume_accumulator", true, false}, {"fee_config", false, false}, {"buyback_fee_recipient", true, false}, {"system_program", false, false}, {"event_authority", false, false}, {"program", false, false}}, 2, false},
	"pump_sweep_creator_fee":         {"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P", [8]byte{32, 246, 191, 52, 8, 201, 73, 186}, []pumpUpgradeAccount{{"payer", true, true}, {"global", false, false}, {"base_mint", false, false}, {"quote_mint", false, false}, {"quote_token_program", false, false}, {"associated_token_program", false, false}, {"system_program", false, false}, {"bonding_curve", true, false}, {"associated_quote_bonding_curve", true, false}, {"recipient", true, false}, {"associated_quote_recipient", true, false}, {"event_authority", false, false}, {"program", false, false}}, 0, false},
	"pump_sweep_protocol_fee":        {"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P", [8]byte{8, 48, 190, 7, 182, 68, 183, 229}, []pumpUpgradeAccount{{"payer", true, true}, {"global", false, false}, {"base_mint", false, false}, {"quote_mint", false, false}, {"quote_token_program", false, false}, {"associated_token_program", false, false}, {"system_program", false, false}, {"bonding_curve", true, false}, {"associated_quote_bonding_curve", true, false}, {"recipient", true, false}, {"associated_quote_recipient", true, false}, {"event_authority", false, false}, {"program", false, false}}, 0, false},
	"pump_amm_buy_v2":                {"pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA", [8]byte{184, 23, 238, 97, 103, 197, 211, 61}, []pumpUpgradeAccount{{"pool", true, false}, {"user", true, true}, {"global_config", false, false}, {"base_mint", false, false}, {"quote_mint", false, false}, {"user_base_token_account", true, false}, {"user_quote_token_account", true, false}, {"pool_base_token_account", true, false}, {"pool_quote_token_account", true, false}, {"base_token_program", false, false}, {"quote_token_program", false, false}, {"system_program", false, false}, {"user_volume_accumulator", true, false}, {"fee_config", false, false}, {"buyback_fee_recipient", true, false}, {"event_authority", false, false}, {"program", false, false}}, 2, false},
	"pump_amm_buy_exact_quote_in_v2": {"pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA", [8]byte{194, 171, 28, 70, 104, 77, 91, 47}, []pumpUpgradeAccount{{"pool", true, false}, {"user", true, true}, {"global_config", false, false}, {"base_mint", false, false}, {"quote_mint", false, false}, {"user_base_token_account", true, false}, {"user_quote_token_account", true, false}, {"pool_base_token_account", true, false}, {"pool_quote_token_account", true, false}, {"base_token_program", false, false}, {"quote_token_program", false, false}, {"system_program", false, false}, {"user_volume_accumulator", true, false}, {"fee_config", false, false}, {"buyback_fee_recipient", true, false}, {"event_authority", false, false}, {"program", false, false}}, 2, false},
	"pump_amm_sell_v2":               {"pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA", [8]byte{93, 246, 130, 60, 231, 233, 64, 178}, []pumpUpgradeAccount{{"pool", true, false}, {"user", true, true}, {"global_config", false, false}, {"base_mint", false, false}, {"quote_mint", false, false}, {"user_base_token_account", true, false}, {"user_quote_token_account", true, false}, {"pool_base_token_account", true, false}, {"pool_quote_token_account", true, false}, {"base_token_program", false, false}, {"quote_token_program", false, false}, {"system_program", false, false}, {"user_volume_accumulator", true, false}, {"fee_config", false, false}, {"buyback_fee_recipient", true, false}, {"event_authority", false, false}, {"program", false, false}}, 2, false},
	"pump_amm_multi_hop_swap":        {"pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA", [8]byte{43, 100, 73, 19, 233, 246, 111, 148}, []pumpUpgradeAccount{{"user", true, true}, {"user_in_token_account", true, false}, {"user_out_token_account", true, false}, {"global_config", false, false}, {"fee_config", false, false}, {"user_volume_accumulator", true, false}, {"buyback_fee_recipient", true, false}, {"token_program", false, false}, {"token_2022_program", false, false}, {"system_program", false, false}, {"event_authority", false, false}, {"program", false, false}, {"pump_program", false, false}, {"pump_global", false, false}, {"pump_fee_config", false, false}, {"pump_event_authority", false, false}}, 2, false},
	"pump_amm_sweep_creator_fee":     {"pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA", [8]byte{32, 246, 191, 52, 8, 201, 73, 186}, []pumpUpgradeAccount{{"payer", true, true}, {"global_config", false, false}, {"pool", true, false}, {"quote_mint", false, false}, {"quote_token_program", false, false}, {"pool_quote_token_account", true, false}, {"recipient", false, false}, {"recipient_token_account", true, false}, {"system_program", false, false}, {"associated_token_program", false, false}, {"event_authority", false, false}, {"program", false, false}}, 0, false},
	"pump_amm_sweep_protocol_fee":    {"pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA", [8]byte{8, 48, 190, 7, 182, 68, 183, 229}, []pumpUpgradeAccount{{"payer", true, true}, {"global_config", false, false}, {"pool", true, false}, {"quote_mint", false, false}, {"quote_token_program", false, false}, {"pool_quote_token_account", true, false}, {"recipient", false, false}, {"recipient_token_account", true, false}, {"system_program", false, false}, {"associated_token_program", false, false}, {"event_authority", false, false}, {"program", false, false}}, 0, false},
}

func BuildPumpUpgradeInstruction(name string, accounts map[string]solana.PublicKey, amounts []uint64, partialFill *bool, remaining []*solana.AccountMeta) (solana.Instruction, error) {
	s, ok := pumpUpgradeSpecs[name]
	if !ok {
		return nil, fmt.Errorf("unsupported Pump upgrade instruction")
	}
	if len(amounts) != s.args {
		return nil, fmt.Errorf("invalid u64 argument count")
	}
	if len(amounts) > 0 && amounts[0] == 0 {
		return nil, fmt.Errorf("input amount must be positive")
	}
	if name == "pump_amm_multi_hop_swap" {
		if amounts[1] == 0 || len(remaining) < 5 || len(remaining)%5 != 0 {
			return nil, fmt.Errorf("invalid multi-hop route or minimum output")
		}
	} else if len(remaining) > 0 {
		return nil, fmt.Errorf("compact trades and sweeps take no remaining accounts")
	}
	if partialFill != nil && !s.partial {
		return nil, fmt.Errorf("invalid partial fill")
	}
	keys := make([]*solana.AccountMeta, 0, len(s.accounts)+len(remaining))
	for _, a := range s.accounts {
		k, ok := accounts[a.name]
		if !ok {
			return nil, fmt.Errorf("missing account: %s", a.name)
		}
		keys = append(keys, &solana.AccountMeta{PublicKey: k, IsWritable: a.writable, IsSigner: a.signer})
	}
	for i, a := range remaining {
		if a == nil || a.IsSigner || a.IsWritable != (i%5 >= 2) {
			return nil, fmt.Errorf("invalid hop account flags")
		}
	}
	data := append([]byte{}, s.disc[:]...)
	for _, n := range amounts {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], n)
		data = append(data, b[:]...)
	}
	if partialFill != nil {
		if *partialFill {
			data = append(data, 1)
		} else {
			data = append(data, 0)
		}
	}
	return solana.NewInstruction(solana.MustPublicKeyFromBase58(s.program), append(keys, remaining...), data), nil
}
func BuildPumpBuyV3Instruction(accounts map[string]solana.PublicKey, amount, limit uint64, partialFill *bool) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_buy_v3", accounts, []uint64{amount, limit}, partialFill, nil)
}
func BuildPumpBuyExactQuoteInV3Instruction(accounts map[string]solana.PublicKey, amount, limit uint64, partialFill *bool) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_buy_exact_quote_in_v3", accounts, []uint64{amount, limit}, partialFill, nil)
}
func BuildPumpSellV3Instruction(accounts map[string]solana.PublicKey, amount, limit uint64) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_sell_v3", accounts, []uint64{amount, limit}, nil, nil)
}
func BuildPumpSweepCreatorFeeInstruction(accounts map[string]solana.PublicKey) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_sweep_creator_fee", accounts, nil, nil, nil)
}
func BuildPumpSweepProtocolFeeInstruction(accounts map[string]solana.PublicKey) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_sweep_protocol_fee", accounts, nil, nil, nil)
}
func BuildPumpAmmBuyV2Instruction(accounts map[string]solana.PublicKey, amount, limit uint64) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_amm_buy_v2", accounts, []uint64{amount, limit}, nil, nil)
}
func BuildPumpAmmBuyExactQuoteInV2Instruction(accounts map[string]solana.PublicKey, amount, limit uint64) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_amm_buy_exact_quote_in_v2", accounts, []uint64{amount, limit}, nil, nil)
}
func BuildPumpAmmSellV2Instruction(accounts map[string]solana.PublicKey, amount, limit uint64) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_amm_sell_v2", accounts, []uint64{amount, limit}, nil, nil)
}
func BuildPumpAmmMultiHopSwapInstruction(accounts map[string]solana.PublicKey, amount, limit uint64, remaining []*solana.AccountMeta) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_amm_multi_hop_swap", accounts, []uint64{amount, limit}, nil, remaining)
}
func BuildPumpAmmSweepCreatorFeeInstruction(accounts map[string]solana.PublicKey) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_amm_sweep_creator_fee", accounts, nil, nil, nil)
}
func BuildPumpAmmSweepProtocolFeeInstruction(accounts map[string]solana.PublicKey) (solana.Instruction, error) {
	return BuildPumpUpgradeInstruction("pump_amm_sweep_protocol_fee", accounts, nil, nil, nil)
}

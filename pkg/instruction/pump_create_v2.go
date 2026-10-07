package instruction

import (
	"encoding/binary"
	"fmt"
	"github.com/gagliardetto/solana-go"
)

type PumpCreateV2Params struct {
	Name, Symbol, URI    string
	Creator              solana.PublicKey
	Mayhem, HolderReward bool
	CreatorFeeBps        uint64
}

// BuildPumpCreateV2Instruction accepts validated quote roles after the fixed 16 accounts.
func BuildPumpCreateV2Instruction(accounts map[string]solana.PublicKey, p PumpCreateV2Params, remaining []*solana.AccountMeta) (solana.Instruction, error) {
	n := len(remaining)
	if n != 0 && n != 3 && n != 4 && n != 5 && n != 8 {
		return nil, fmt.Errorf("invalid quote creation accounts")
	}
	for i, a := range remaining {
		if a == nil || a.IsSigner || a.IsWritable != (i == 1) {
			return nil, fmt.Errorf("invalid quote account flags")
		}
	}
	if n >= 5 && p.Mayhem {
		return nil, fmt.Errorf("mayhem pump coin quote not allowed")
	}
	if p.CreatorFeeBps > 10000 {
		return nil, fmt.Errorf("invalid creator fee")
	}
	data := []byte{214, 144, 76, 236, 95, 139, 49, 180}
	for _, s := range []string{p.Name, p.Symbol, p.URI} {
		if uint64(len(s)) > 0xffffffff {
			return nil, fmt.Errorf("string too long")
		}
		data = binary.LittleEndian.AppendUint32(data, uint32(len(s)))
		data = append(data, []byte(s)...)
	}
	data = append(data, p.Creator[:]...)
	mayhem, holder := byte(0), byte(0)
	if p.Mayhem {
		mayhem = 1
	}
	if p.HolderReward {
		holder = 1
	}
	data = append(data, mayhem, 0)
	data = binary.LittleEndian.AppendUint64(data, p.CreatorFeeBps)
	data = append(data, holder)
	keys := []*solana.AccountMeta{}
	for _, a := range []struct {
		name             string
		writable, signer bool
	}{{"mint", true, true}, {"mint_authority", false, false}, {"bonding_curve", true, false}, {"associated_bonding_curve", true, false}, {"global", false, false}, {"user", true, true}, {"system_program", false, false}, {"token_program", false, false}, {"associated_token_program", false, false}, {"mayhem_program_id", true, false}, {"global_params", false, false}, {"sol_vault", true, false}, {"mayhem_state", true, false}, {"mayhem_token_vault", true, false}, {"event_authority", false, false}, {"program", false, false}} {
		key, ok := accounts[a.name]
		if !ok {
			return nil, fmt.Errorf("missing account: %s", a.name)
		}
		keys = append(keys, &solana.AccountMeta{PublicKey: key, IsWritable: a.writable, IsSigner: a.signer})
	}
	keys = append(keys, remaining...)
	return solana.NewInstruction(compactPump, keys, data), nil
}

type PumpQuoteControlMint struct {
	Mint                        solana.PublicKey
	InitialVirtualQuoteReserves uint64
}
type PumpQuoteControl struct {
	Admin, ReservesAdmin solana.PublicKey
	Mints                []PumpQuoteControlMint
}

// DecodePumpQuoteControl includes reserves_admin and reserved[32] in the header.
func DecodePumpQuoteControl(data []byte) (PumpQuoteControl, error) {
	var result PumpQuoteControl
	disc := []byte{56, 244, 35, 238, 193, 213, 162, 201}
	if len(data) < 108 {
		return result, fmt.Errorf("invalid QuoteControl")
	}
	for i, b := range disc {
		if data[i] != b {
			return result, fmt.Errorf("invalid QuoteControl")
		}
	}
	count := uint64(binary.LittleEndian.Uint32(data[104:108]))
	if count > uint64((len(data)-108)/40) {
		return result, fmt.Errorf("truncated QuoteControl")
	}
	copy(result.Admin[:], data[8:40])
	copy(result.ReservesAdmin[:], data[40:72])
	for i := 0; i < int(count); i++ {
		o := 108 + i*40
		var mint solana.PublicKey
		copy(mint[:], data[o:o+32])
		result.Mints = append(result.Mints, PumpQuoteControlMint{mint, binary.LittleEndian.Uint64(data[o+32 : o+40])})
	}
	return result, nil
}

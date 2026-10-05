package subscription

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"unicode/utf8"
)

func validatePumpFunSettlementInstructions(route PreparedCachedRoute, payer solana.PublicKey) error {
	if len(route.SwapInstructions) != len(route.Legs) {
		return errors.New("invalid PumpFun settlement instructions")
	}
	for i, leg := range route.Legs {
		ix, quoted := route.SwapInstructions[i], leg.Instruction
		if ix == nil || quoted == nil || ix.ProgramID() != quoted.ProgramID() {
			return errors.New("PumpFun settlement instruction differs from quoted leg")
		}
		data, err := ix.Data()
		if err != nil {
			return err
		}
		gold, err := quoted.Data()
		if err != nil {
			return err
		}
		keys, expected := ix.Accounts(), quoted.Accounts()
		if !bytes.Equal(data, gold) || len(keys) != len(expected) {
			return errors.New("PumpFun settlement instruction differs from quoted leg")
		}
		matched := false
		for j, a := range keys {
			b := expected[j]
			if a == nil || b == nil || a.PublicKey != b.PublicKey || a.IsSigner != b.IsSigner || a.IsWritable != b.IsWritable {
				return errors.New("PumpFun settlement instruction differs from quoted leg")
			}
			matched = matched || (a.PublicKey == payer && a.IsSigner)
		}
		if !matched {
			return errors.New("PumpFun settlement belongs to a different wallet")
		}
		if leg.AmountIn == 0 || leg.MinimumNetAmountOut == 0 {
			return errors.New("invalid PumpFun settlement amount or protection")
		}
	}
	if route.MinimumNetAmountOut != route.Legs[len(route.Legs)-1].MinimumNetAmountOut {
		return errors.New("PumpFun settlement protection differs from quoted leg")
	}
	return nil
}

func validatePumpFunNativeAnchor(leg CachedRouteLeg, payer solana.PublicKey, buy bool) error {
	data, err := leg.Instruction.Data()
	if err != nil {
		return err
	}
	keys := leg.Instruction.Accounts()
	disc, count, mint := []byte{93, 246, 130, 60, 231, 233, 64, 178}, 26, leg.Hint.InputMint
	if buy {
		disc, count, mint = []byte{194, 171, 28, 70, 104, 77, 91, 47}, 27, leg.Hint.OutputMint
	}
	if len(data) != 24 || !bytes.Equal(data[:8], disc) || len(keys) != count || binary.LittleEndian.Uint64(data[8:]) != leg.AmountIn || binary.LittleEndian.Uint64(data[16:]) != leg.MinimumNetAmountOut || keys[13].PublicKey != payer || !keys[13].IsSigner || !keys[13].IsWritable || keys[10].PublicKey != leg.Hint.Pool || keys[2].PublicKey != pumpWSOL || keys[1].PublicKey != mint {
		return errors.New("PumpFun native settlement requires matching exact-in V2 quote and accounts")
	}
	return nil
}

type PumpFunNativeSettlement struct {
	Instructions                                      []solana.Instruction
	RequiredLamports, EstimatedNativeResidualLamports uint64
}

func SettlePumpFunNativeQuote(route PreparedCachedRoute, payer solana.PublicKey, nativeInput, nativeOutput bool, seed string, rent uint64) (PumpFunNativeSettlement, error) {
	empty := PumpFunNativeSettlement{}
	if len(route.Legs) != 1 {
		if len(route.Legs) < 2 || len(route.Legs) > 5 || nativeInput || nativeOutput {
			return empty, errors.New("invalid PumpFun native-quote multi-hop endpoints")
		}
		index, count := -1, 0
		for i, leg := range route.Legs {
			if leg.Instruction != nil && leg.Instruction.ProgramID() == instruction.PUMPFUN_PROGRAM {
				index = i
				count++
			}
		}
		if count != 1 || (index != 0 && index != len(route.Legs)-1) {
			return empty, errors.New("PumpFun multi-hop requires one curve at the buy or sell anchor")
		}
		anchor := route.Legs[index]
		buy := index == len(route.Legs)-1
		if (buy && anchor.Hint.InputMint != pumpWSOL) || (!buy && anchor.Hint.OutputMint != pumpWSOL) {
			return empty, errors.New("PumpFun multi-hop anchor must use native quote")
		}
		for i := 1; i < len(route.Legs); i++ {
			previous, current := route.Legs[i-1], route.Legs[i]
			if previous.Hint.OutputMint != current.Hint.InputMint || current.AmountIn == 0 || current.AmountIn > previous.MinimumNetAmountOut {
				return empty, errors.New("PumpFun multi-hop exceeds protected intermediate credit")
			}
		}
		if err := validatePumpFunSettlementInstructions(route, payer); err != nil {
			return empty, err
		}
		one := PreparedCachedRoute{Legs: []CachedRouteLeg{anchor}, SwapInstructions: []solana.Instruction{anchor.Instruction}, MinimumNetAmountOut: anchor.MinimumNetAmountOut}
		converted, err := SettlePumpFunNativeQuote(one, payer, false, false, seed, rent)
		if err != nil {
			return empty, err
		}
		out := append([]solana.Instruction{}, route.SetupInstructions...)
		if buy {
			out = append(out, route.SwapInstructions[:len(route.Legs)-1]...)
			out = append(out, converted.Instructions...)
		} else {
			out = append(out, converted.Instructions...)
			out = append(out, route.SwapInstructions[1:]...)
		}
		converted.Instructions = out
		return converted, nil
	}
	leg := route.Legs[0]
	buy := leg.Hint.InputMint == pumpWSOL
	if leg.Instruction == nil || leg.Instruction.ProgramID() != instruction.PUMPFUN_PROGRAM || (leg.Hint.InputMint == pumpWSOL) == (leg.Hint.OutputMint == pumpWSOL) {
		return empty, errors.New("PumpFun settlement requires one native-quote curve")
	}
	if err := validatePumpFunSettlementInstructions(route, payer); err != nil {
		return empty, err
	}
	if err := validatePumpFunNativeAnchor(leg, payer, buy); err != nil {
		return empty, err
	}
	if buy && nativeOutput || !buy && nativeInput {
		return empty, errors.New("invalid PumpFun native endpoint")
	}
	token := solana.TokenProgramID
	ata, _, e := solana.FindProgramAddress([][]byte{payer[:], token[:], pumpWSOL[:]}, solana.SPLAssociatedTokenAccountProgramID)
	if e != nil {
		return empty, e
	}
	var keep []solana.Instruction
	for _, ix := range route.SetupInstructions {
		if ix == nil {
			return empty, errors.New("invalid PumpFun setup instruction")
		}
		keys := ix.Accounts()
		for _, a := range keys {
			if a == nil {
				return empty, errors.New("invalid PumpFun setup account")
			}
		}
		if ix.ProgramID() != solana.SPLAssociatedTokenAccountProgramID || len(keys) < 2 || keys[1].PublicKey != ata {
			keep = append(keep, ix)
		}
	}
	appendSwaps := func(setup []solana.Instruction) []solana.Instruction {
		return append(append([]solana.Instruction{}, setup...), route.SwapInstructions...)
	}
	if buy && nativeInput {
		return PumpFunNativeSettlement{Instructions: appendSwaps(keep), RequiredLamports: leg.AmountIn}, nil
	}
	if !buy && nativeOutput {
		return PumpFunNativeSettlement{Instructions: appendSwaps(keep)}, nil
	}
	meta := func(key solana.PublicKey, signer, writable bool) *solana.AccountMeta {
		return &solana.AccountMeta{PublicKey: key, IsSigner: signer, IsWritable: writable}
	}
	if !buy {
		minimum := route.MinimumNetAmountOut
		data := make([]byte, 12)
		binary.LittleEndian.PutUint32(data, 2)
		binary.LittleEndian.PutUint64(data[4:], minimum)
		fund := solana.NewInstruction(solana.SystemProgramID, []*solana.AccountMeta{meta(payer, true, true), meta(ata, false, true)}, data)
		sync := solana.NewInstruction(token, []*solana.AccountMeta{meta(ata, false, true)}, []byte{17})
		if leg.EstimatedNetAmountOut == nil || *leg.EstimatedNetAmountOut < minimum {
			return empty, errors.New("missing PumpFun output estimate")
		}
		return PumpFunNativeSettlement{Instructions: append(appendSwaps(route.SetupInstructions), fund, sync), EstimatedNativeResidualLamports: *leg.EstimatedNetAmountOut - minimum}, nil
	}
	if rent == 0 || len(seed) < 1 || len(seed) > 32 || !utf8.ValidString(seed) {
		return empty, errors.New("WSOL PumpFun input requires unique seed and current account rent")
	}
	bytes := append(append(append([]byte{}, payer[:]...), []byte(seed)...), token[:]...)
	temporary := solana.PublicKey(sha256.Sum256(bytes))
	if temporary == ata {
		return empty, errors.New("temporary WSOL account collides with input ATA")
	}
	data := make([]byte, 4+32+8+len(seed)+8+8+32)
	binary.LittleEndian.PutUint32(data, 3)
	copy(data[4:], payer[:])
	binary.LittleEndian.PutUint64(data[36:], uint64(len(seed)))
	copy(data[44:], seed)
	o := 44 + len(seed)
	binary.LittleEndian.PutUint64(data[o:], rent)
	binary.LittleEndian.PutUint64(data[o+8:], 165)
	copy(data[o+16:], token[:])
	create := solana.NewInstruction(solana.SystemProgramID, []*solana.AccountMeta{meta(payer, true, true), meta(temporary, false, true)}, data)
	init := solana.NewInstruction(token, []*solana.AccountMeta{meta(temporary, false, true), meta(pumpWSOL, false, false)}, append([]byte{18}, payer[:]...))
	transferData := make([]byte, 9)
	transferData[0] = 3
	binary.LittleEndian.PutUint64(transferData[1:], leg.AmountIn)
	transfer := solana.NewInstruction(token, []*solana.AccountMeta{meta(ata, false, true), meta(temporary, false, true), meta(payer, true, false)}, transferData)
	close := solana.NewInstruction(token, []*solana.AccountMeta{meta(temporary, false, true), meta(payer, false, true), meta(payer, true, false)}, []byte{9})
	return PumpFunNativeSettlement{Instructions: append([]solana.Instruction{create, init, transfer, close}, appendSwaps(keep)...), RequiredLamports: rent}, nil
}

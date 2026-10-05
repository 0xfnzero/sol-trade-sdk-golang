package subscription

import (
	"encoding/binary"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func pumpSettlementLeg(payer, meme solana.PublicKey, buy bool, amount, minimum uint64, estimate *uint64) CachedRouteLeg {
	pool := solana.NewWallet().PublicKey()
	count, disc := 26, []byte{93, 246, 130, 60, 231, 233, 64, 178}
	hint := PoolTradeHint{Pool: pool, InputMint: meme, OutputMint: pumpWSOL}
	if buy {
		count, disc, hint.InputMint, hint.OutputMint = 27, []byte{194, 171, 28, 70, 104, 77, 91, 47}, pumpWSOL, meme
	}
	keys := make([]*solana.AccountMeta, count)
	for i := range keys {
		keys[i] = &solana.AccountMeta{PublicKey: solana.NewWallet().PublicKey()}
	}
	keys[1].PublicKey, keys[2].PublicKey, keys[10].PublicKey, keys[13].PublicKey = meme, pumpWSOL, pool, payer
	keys[10].IsWritable, keys[13].IsWritable, keys[13].IsSigner = true, true, true
	data := make([]byte, 24)
	copy(data, disc)
	binary.LittleEndian.PutUint64(data[8:], amount)
	binary.LittleEndian.PutUint64(data[16:], minimum)
	return CachedRouteLeg{Hint: hint, AmountIn: amount, MinimumNetAmountOut: minimum, EstimatedNetAmountOut: estimate, Instruction: solana.NewInstruction(instruction.PUMPFUN_PROGRAM, keys, data)}
}

func TestPumpFunNativeSettlement(t *testing.T) {
	payer := solana.NewWallet().PublicKey()
	estimate := uint64(11000)
	leg := pumpSettlementLeg(payer, solana.NewWallet().PublicKey(), true, 10000, 10000, &estimate)
	route := PreparedCachedRoute{Legs: []CachedRouteLeg{leg}, SwapInstructions: []solana.Instruction{leg.Instruction}, MinimumNetAmountOut: 10000}
	direct, e := SettlePumpFunNativeQuote(route, payer, true, false, "", 0)
	if e != nil || len(direct.Instructions) != 1 || direct.RequiredLamports != 10000 {
		t.Fatal(direct, e)
	}
	wrapped, e := SettlePumpFunNativeQuote(route, payer, false, false, "p", 2039280)
	if e != nil || len(wrapped.Instructions) != 5 || wrapped.RequiredLamports != 2039280 {
		t.Fatal(wrapped, e)
	}
	source := wrapped.Instructions[2].Accounts()[0].PublicKey
	if wrapped.Instructions[3].Accounts()[0].PublicKey == source {
		t.Fatal("must preserve original WSOL account")
	}
	data, _ := wrapped.Instructions[2].Data()
	if binary.LittleEndian.Uint64(data[1:]) != 10000 {
		t.Fatal("incorrect debit")
	}
	route.Legs[0] = pumpSettlementLeg(payer, solana.NewWallet().PublicKey(), false, 10000, 10000, &estimate)
	route.SwapInstructions = []solana.Instruction{route.Legs[0].Instruction}
	sold, e := SettlePumpFunNativeQuote(route, payer, false, false, "", 0)
	if e != nil || sold.EstimatedNativeResidualLamports != 1000 || len(sold.Instructions) != 3 {
		t.Fatal(sold, e)
	}
	route.Legs = append(route.Legs, route.Legs[0])
	if _, e = SettlePumpFunNativeQuote(route, payer, false, true, "", 0); e == nil {
		t.Fatal("native multi-hop accepted")
	}
}

func TestPumpFunMultiHopSettlementCredit(t *testing.T) {
	payer := solana.NewWallet().PublicKey()
	asset, meme := solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey()
	estimate := uint64(11000)
	ix := func(program solana.PublicKey) solana.Instruction {
		return solana.NewInstruction(program, []*solana.AccountMeta{{PublicKey: payer, IsSigner: true}}, []byte{42})
	}
	anchor, other := pumpSettlementLeg(payer, meme, false, 100, 10000, &estimate), ix(solana.TokenProgramID)
	pump := anchor.Instruction
	sell := PreparedCachedRoute{Legs: []CachedRouteLeg{anchor, {Hint: PoolTradeHint{InputMint: pumpWSOL, OutputMint: asset}, AmountIn: 9800, MinimumNetAmountOut: 980, Instruction: other}}, SwapInstructions: []solana.Instruction{pump, other}, MinimumNetAmountOut: 980}
	settled, e := SettlePumpFunNativeQuote(sell, payer, false, false, "", 0)
	if e != nil {
		t.Fatal(e)
	}
	data, _ := settled.Instructions[1].Data()
	if binary.LittleEndian.Uint64(data[4:]) != 10000 || settled.EstimatedNativeResidualLamports != 1000 {
		t.Fatal("must wrap curve minimum, not final route minimum")
	}
	altered := sell
	altered.SwapInstructions = []solana.Instruction{pump, solana.NewInstruction(other.ProgramID(), other.Accounts(), []byte("changed"))}
	if _, e = SettlePumpFunNativeQuote(altered, payer, false, false, "", 0); e == nil {
		t.Fatal("same-wallet instruction replacement accepted")
	}
	sell.Legs[1].AmountIn = 10001
	if _, e = SettlePumpFunNativeQuote(sell, payer, false, false, "", 0); e == nil {
		t.Fatal("intermediate overconsumption accepted")
	}
	sell.Legs[1].AmountIn = 9800
	if _, e = SettlePumpFunNativeQuote(sell, solana.NewWallet().PublicKey(), false, false, "", 0); e == nil {
		t.Fatal("wrong wallet accepted")
	}
	anchor = pumpSettlementLeg(payer, meme, true, 10000, 100, nil)
	pump = anchor.Instruction
	buy := PreparedCachedRoute{Legs: []CachedRouteLeg{{Hint: PoolTradeHint{InputMint: asset, OutputMint: pumpWSOL}, AmountIn: 100, MinimumNetAmountOut: 10000, Instruction: other}, anchor}, SwapInstructions: []solana.Instruction{other, pump}, MinimumNetAmountOut: 100}
	settled, e = SettlePumpFunNativeQuote(buy, payer, false, false, "p", 1493440)
	if e != nil {
		t.Fatal(e)
	}
	if settled.Instructions[0].ProgramID() != other.ProgramID() || settled.Instructions[len(settled.Instructions)-1].ProgramID() != pump.ProgramID() {
		t.Fatal("incorrect hop ordering")
	}
	data, _ = settled.Instructions[3].Data()
	if len(data) != 9 || binary.LittleEndian.Uint64(data[1:]) != 10000 {
		t.Fatal("incorrect protected WSOL debit")
	}
}

func TestPumpFunSettlementRejectsInconsistentRequest(t *testing.T) {
	for _, condition := range []string{"wallet", "protocol", "quote", "instruction", "protection", "zero", "encoded_amount", "pool", "nil_setup"} {
		t.Run(condition, func(t *testing.T) {
			payer := solana.NewWallet().PublicKey()
			leg := pumpSettlementLeg(payer, solana.NewWallet().PublicKey(), true, 10000, 10000, nil)
			r := PreparedCachedRoute{Legs: []CachedRouteLeg{leg}, SwapInstructions: []solana.Instruction{leg.Instruction}, MinimumNetAmountOut: 10000}
			switch condition {
			case "wallet":
				payer = solana.NewWallet().PublicKey()
			case "protocol":
				data, _ := leg.Instruction.Data()
				ix := solana.NewInstruction(solana.TokenProgramID, leg.Instruction.Accounts(), data)
				r.Legs[0].Instruction = ix
				r.SwapInstructions = []solana.Instruction{ix}
			case "quote":
				r.Legs[0].Hint.OutputMint = pumpWSOL
			case "instruction":
				r.SwapInstructions = []solana.Instruction{solana.NewInstruction(instruction.PUMPFUN_PROGRAM, leg.Instruction.Accounts(), []byte("changed"))}
			case "protection":
				r.MinimumNetAmountOut = 1
			case "zero":
				r.Legs[0].AmountIn = 0
			case "encoded_amount":
				r.Legs[0].AmountIn = 10001
			case "pool":
				r.Legs[0].Hint.Pool = solana.NewWallet().PublicKey()
			case "nil_setup":
				r.SetupInstructions = []solana.Instruction{nil}
			}
			if _, err := SettlePumpFunNativeQuote(r, payer, true, false, "", 0); err == nil {
				t.Fatal("inconsistent settlement accepted")
			}
		})
	}
}

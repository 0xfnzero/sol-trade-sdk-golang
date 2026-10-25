package instruction

import (
	"encoding/binary"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/constants"
	"github.com/gagliardetto/solana-go"
	"strings"
	"testing"
)

func TestV1NativeSettlementEndpoints(t *testing.T) {
	mint, payer := solana.PublicKey{1}, solana.PublicKey{3}
	p := PumpFunParams{Version: constants.PumpFunLegacy, BondingCurve: &BondingCurve{
		Account: GetBondingCurvePDA(mint), VirtualTokenReserves: 1000000,
		VirtualSolReserves: 1000000, RealTokenReserves: 800000}, TokenProgram: solana.Token2022ProgramID}
	minimum := uint64(7)
	for _, endpoint := range []solana.PublicKey{{}, constants.SOL_TOKEN_ACCOUNT,
		constants.WSOL_TOKEN_ACCOUNT, constants.USDC_TOKEN_ACCOUNT, {4}} {
		for _, buy := range []bool{true, false} {
			var ixs []solana.Instruction
			var err error
			if buy {
				ixs, err = PumpFunBuildBuyInstructions(&PumpFunBuildBuyParams{Payer: payer,
					InputMint: endpoint, OutputMint: mint, InputAmount: 10000, ProtocolParams: &p, UseExactSolAmount: true})
			} else {
				ixs, err = PumpFunBuildSellInstructions(&PumpFunBuildSellParams{Payer: payer,
					InputMint: mint, OutputMint: endpoint, InputAmount: 10000, ProtocolParams: &p, FixedOutputAmount: &minimum})
			}
			if !endpoint.IsZero() && endpoint != constants.SOL_TOKEN_ACCOUNT {
				if err == nil || !strings.Contains(err.Error(), "wallet SOL") {
					t.Fatalf("token settlement accepted: buy=%v mint=%s err=%v", buy, endpoint, err)
				}
				continue
			}
			if err != nil || len(ixs) != 1 {
				t.Fatalf("native settlement: %v", err)
			}
			data, err := ixs[0].Data()
			if err != nil {
				t.Fatal(err)
			}
			disc := []byte{51, 230, 133, 164, 1, 127, 131, 173}
			if buy {
				disc = []byte{56, 252, 116, 8, 158, 223, 205, 95}
			}
			if string(data[:8]) != string(disc) || binary.LittleEndian.Uint64(data[8:]) != 10000 ||
				(!buy && binary.LittleEndian.Uint64(data[16:]) != minimum) {
				t.Fatalf("native instruction or limits changed: %x", data)
			}
		}
	}
	if p.Version != constants.PumpFunLegacy {
		t.Fatal("caller params mutated")
	}
}

func TestV3StockQuoteProgramsAndBounds(t *testing.T) {
	key := func(n byte) (p solana.PublicKey) {
		for i := range p {
			p[i] = n
		}
		return
	}
	mint, quote, payer := key(1), key(2), key(3)
	minimum := uint64(7)
	fee := uint16(0)
	p := PumpFunParams{Version: constants.PumpFunV3, BondingCurve: &BondingCurve{Account: GetBondingCurvePDA(mint), VirtualTokenReserves: 1000000, VirtualSolReserves: 1000000, RealTokenReserves: 800000}, TokenProgram: solana.Token2022ProgramID, QuoteMint: quote, QuoteTokenProgram: solana.Token2022ProgramID, MinimumOutputAmount: &minimum, BaseTransferFeeBps: &fee, QuoteTransferFeeBps: &fee}
	buy := func(pp *PumpFunParams) ([]solana.Instruction, error) {
		return PumpFunBuildBuyInstructions(&PumpFunBuildBuyParams{Payer: payer, InputMint: quote, OutputMint: mint, InputAmount: 100, ProtocolParams: pp, UseExactSolAmount: true})
	}
	a, e := buy(&p)
	if e != nil {
		t.Fatal(e)
	}
	b, e := PumpFunBuildSellInstructions(&PumpFunBuildSellParams{Payer: payer, InputMint: mint, OutputMint: quote, InputAmount: 100, ProtocolParams: &p})
	if e != nil {
		t.Fatal(e)
	}
	for _, ix := range []solana.Instruction{a[0], b[0]} {
		d, e := ix.Data()
		if e != nil {
			t.Fatal(e)
		}
		k := ix.Accounts()
		if len(k) != 17 || k[2].PublicKey != quote || k[4].PublicKey != solana.Token2022ProgramID || binary.LittleEndian.Uint64(d[8:]) != 100 || binary.LittleEndian.Uint64(d[16:]) != 7 {
			t.Fatal("V3 stock quote layout or bounds")
		}
	}
	for _, mutate := range []func(*PumpFunParams){func(p *PumpFunParams) { p.Version = constants.PumpFunLegacy }, func(p *PumpFunParams) { p.QuoteTransferFeeBps = nil }, func(p *PumpFunParams) { n := uint16(1); p.QuoteTransferFeeBps = &n }, func(p *PumpFunParams) { c := *p.BondingCurve; c.Complete = true; p.BondingCurve = &c }} {
		q := p
		mutate(&q)
		if _, err := buy(&q); err == nil {
			t.Fatal("invalid state accepted")
		}
	}
	r := PumpFunBuildBuyParams{Payer: payer, InputMint: quote, OutputMint: mint, InputAmount: 100, ProtocolParams: &p}
	if _, err := PumpFunBuildBuyInstructions(&r); err == nil {
		t.Fatal("token-quoted maximum-input buy needs explicit output")
	}
	fixed := uint64(55)
	r.FixedOutputAmount = &fixed
	ixs, err := PumpFunBuildBuyInstructions(&r)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := ixs[0].Data()
	if binary.LittleEndian.Uint64(data[8:]) != 55 || binary.LittleEndian.Uint64(data[16:]) != 100 {
		t.Fatal("explicit output/max input lost")
	}
}

func TestV3NativeMaximumInputAndSettlement(t *testing.T) {
	mint, payer := solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey()
	p := PumpFunParams{Version: constants.PumpFunV3, BondingCurve: &BondingCurve{Account: GetBondingCurvePDA(mint), VirtualTokenReserves: 1000000, VirtualSolReserves: 1000000, RealTokenReserves: 800000}, TokenProgram: solana.Token2022ProgramID}
	r := PumpFunBuildBuyParams{Payer: payer, InputMint: constants.SOL_TOKEN_ACCOUNT, OutputMint: mint, InputAmount: 10000, SlippageBasisPoints: 1000, ProtocolParams: &p}
	ixs, err := PumpFunBuildBuyInstructions(&r)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := ixs[0].Data()
	if string(data[:8]) != string([]byte{7, 5, 29, 196, 245, 23, 101, 80}) || binary.LittleEndian.Uint64(data[8:]) != 9806 || binary.LittleEndian.Uint64(data[16:]) != 11000 {
		t.Fatalf("maximum-input mode lost: %x", data)
	}
	r.UseExactSolAmount = true
	ixs, err = PumpFunBuildBuyInstructions(&r)
	if err != nil {
		t.Fatal(err)
	}
	data, _ = ixs[0].Data()
	if string(data[:8]) != string([]byte{225, 247, 80, 30, 213, 179, 132, 136}) {
		t.Fatal("exact-input mode lost")
	}
	r.InputMint = constants.WSOL_TOKEN_ACCOUNT
	if _, err := PumpFunBuildBuyInstructions(&r); err == nil {
		t.Fatal("direct WSOL buy accepted")
	}
	if _, err := PumpFunBuildSellInstructions(&PumpFunBuildSellParams{Payer: payer, InputMint: mint, OutputMint: constants.WSOL_TOKEN_ACCOUNT, InputAmount: 10000, ProtocolParams: &p}); err == nil {
		t.Fatal("direct WSOL sell accepted")
	}
}

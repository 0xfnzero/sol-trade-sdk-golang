package instruction

import (
	"encoding/binary"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestPumpSwapExactOutputBudget(t *testing.T) {
	output := uint64(123)
	for _, amount := range []uint64{1, 1000000} {
		ixs, e := BuildBuyInstructions(&BuildBuyParams{Payer: testPK(9), InputAmount: amount, FixedOutputAmount: &output, SlippageBasisPoints: 300, ProtocolParams: testPumpSwapParams(nil)})
		if e != nil {
			t.Fatal(e)
		}
		d, e := ixs[len(ixs)-1].Data()
		if e != nil {
			t.Fatal(e)
		}
		if binary.LittleEndian.Uint64(d[8:16]) != output || binary.LittleEndian.Uint64(d[16:24]) != amount {
			t.Fatal(d)
		}
	}
	reverse := testPumpSwapParams(func(p *PumpSwapParams) { p.BaseMint = constants.WSOL_TOKEN_ACCOUNT; p.QuoteMint = testPK(22) })
	ixs, e := BuildSellInstructions(&BuildSellParams{Payer: testPK(9), InputAmount: 1, FixedOutputAmount: &output, SlippageBasisPoints: 300, ProtocolParams: reverse})
	if e != nil {
		t.Fatal(e)
	}
	d, e := ixs[len(ixs)-1].Data()
	if e != nil {
		t.Fatal(e)
	}
	if binary.LittleEndian.Uint64(d[8:16]) != 123 || binary.LittleEndian.Uint64(d[16:24]) != 1 {
		t.Fatal(d)
	}
	if _, e = BuildBuyInstructions(&BuildBuyParams{Payer: testPK(9), InputAmount: 1, FixedOutputAmount: &output, ProtocolParams: reverse}); e == nil {
		t.Fatal("accepted exact-out buy requiring sell ABI")
	}
	if _, e = BuildSellInstructions(&BuildSellParams{Payer: testPK(9), InputAmount: 1, FixedOutputAmount: &output, ProtocolParams: testPumpSwapParams(nil)}); e == nil {
		t.Fatal("accepted exact-out sell requiring sell ABI")
	}
	output = testPumpSwapParams(nil).PoolBaseTokenReserves
	if _, e = BuildBuyInstructions(&BuildBuyParams{Payer: testPK(9), InputAmount: 1, FixedOutputAmount: &output, ProtocolParams: testPumpSwapParams(nil)}); e == nil {
		t.Fatal("accepted depleted exact-out")
	}
}

func TestPumpSwapExplicitConfigRecipient(t *testing.T) {
	recipient := testPK(20)
	p := testPumpSwapParams(nil)
	p.ProtocolFeeRecipientOverride = &recipient
	buy, e := BuildBuyInstructions(&BuildBuyParams{Payer: testPK(9), InputAmount: 10000, SlippageBasisPoints: 100, ProtocolParams: p})
	if e != nil {
		t.Fatal(e)
	}
	sell, e := BuildSellInstructions(&BuildSellParams{Payer: testPK(9), InputAmount: 10000, SlippageBasisPoints: 100, ProtocolParams: p})
	if e != nil {
		t.Fatal(e)
	}
	for _, ixs := range [][]solana.Instruction{buy, sell} {
		if ixs[len(ixs)-1].Accounts()[9].PublicKey != recipient {
			t.Fatal("ignored config recipient")
		}
	}
}

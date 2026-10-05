package instruction

import (
	"encoding/binary"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestAmmV2LegacyStockPairs(t *testing.T) {
	pk := func(n byte) solana.PublicKey {
		var k solana.PublicKey
		for i := range k {
			k[i] = n
		}
		return k
	}
	num, den := uint64(1), uint64(3)
	p := &RaydiumAmmV4Params{Amm: pk(1), CoinMint: pk(2), PcMint: pk(3), TokenCoin: pk(4), TokenPc: pk(5), CoinReserve: 10000, PcReserve: 20000, SwapFeeNumerator: &num, SwapFeeDenominator: &den}
	for _, coinIn := range []bool{true, false} {
		im, om, i, o := p.CoinMint, p.PcMint, uint64(10000), uint64(20000)
		if !coinIn {
			im, om, i, o = om, im, o, i
		}
		b, e := RaydiumAmmV4BuildBuyInstructions(&RaydiumAmmV4BuildBuyParams{Payer: pk(99), InputMint: im, OutputMint: om, InputAmount: 1001, SlippageBasisPoints: 100, ProtocolParams: p})
		if e != nil {
			t.Fatal(e)
		}
		s, e := RaydiumAmmV4BuildSellInstructions(&RaydiumAmmV4BuildSellParams{Payer: pk(99), InputMint: im, OutputMint: om, InputAmount: 1001, SlippageBasisPoints: 100, ProtocolParams: p})
		if e != nil {
			t.Fatal(e)
		}
		bd, _ := b[0].Data()
		sd, _ := s[0].Data()
		want := o * 667 / (i + 667) * 9900 / 10000
		if string(bd) != string(sd) || bd[0] != 16 || binary.LittleEndian.Uint64(bd[9:]) != want || len(b[0].Accounts()) != 8 || b[0].Accounts()[7].IsWritable {
			t.Fatal("V2 direction or actual fee mismatch")
		}
		fixed := uint64(42)
		b, e = RaydiumAmmV4BuildBuyInstructions(&RaydiumAmmV4BuildBuyParams{Payer: pk(99), OutputMint: om, InputAmount: 1001, FixedOutputAmount: &fixed, ProtocolParams: p})
		if e != nil {
			t.Fatal(e)
		}
		bd, _ = b[0].Data()
		if bd[0] != 17 || binary.LittleEndian.Uint64(bd[9:]) != 42 {
			t.Fatal("exact out layout")
		}
		_, e = RaydiumAmmV4BuildBuyInstructions(&RaydiumAmmV4BuildBuyParams{Payer: pk(99), InputMint: om, OutputMint: om, InputAmount: 1001, ProtocolParams: p})
		if e == nil {
			t.Fatal("accepted wrong input")
		}
	}
	den = 0
	_, e := RaydiumAmmV4BuildSellInstructions(&RaydiumAmmV4BuildSellParams{Payer: pk(99), InputMint: p.CoinMint, InputAmount: 1, ProtocolParams: p})
	if e == nil {
		t.Fatal("invalid fee accepted")
	}
}

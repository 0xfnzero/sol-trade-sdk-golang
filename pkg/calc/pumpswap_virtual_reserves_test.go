package calc

import (
	"math"
	"math/big"
	"strings"
	"testing"
)

func TestEffectiveQuoteReservesSignedRange(t *testing.T) {
	tests := []struct {
		name    string
		raw     uint64
		virtual *big.Int
		want    uint64
		wantErr bool
	}{
		{name: "positive", raw: 1_000, virtual: big.NewInt(250), want: 1_250},
		{name: "negative", raw: 1_000, virtual: big.NewInt(-250), want: 750},
		{name: "zero", raw: 1_000, virtual: big.NewInt(-1_000), wantErr: true},
		{name: "negative result", raw: 100, virtual: big.NewInt(-101), wantErr: true},
		{name: "u64 overflow", raw: math.MaxUint64, virtual: big.NewInt(1), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := EffectiveQuoteReserves(test.raw, test.virtual)
			if (err != nil) != test.wantErr {
				t.Fatalf("EffectiveQuoteReserves() error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("EffectiveQuoteReserves() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestEffectiveQuoteReservesRejectsValuesOutsideI128(t *testing.T) {
	twoTo127 := new(big.Int).Lsh(big.NewInt(1), 127)
	belowI128 := new(big.Int).Neg(new(big.Int).Add(new(big.Int).Set(twoTo127), big.NewInt(1)))

	for _, virtual := range []*big.Int{twoTo127, belowI128} {
		_, err := EffectiveQuoteReserves(1, virtual)
		if err == nil || !strings.Contains(err.Error(), "signed i128") {
			t.Fatalf("EffectiveQuoteReserves(%s) error = %v", virtual, err)
		}
	}
}

func TestPumpSwapQuoteModesUseEffectiveQuoteReserves(t *testing.T) {
	fees := PumpSwapFeeBasisPoints{
		LPFeeBasisPoints:          20,
		ProtocolFeeBasisPoints:    5,
		CoinCreatorFeeBasisPoints: 30,
	}
	baseReserve := uint64(800_000_000_000_000)
	quoteReserve := uint64(100_000_000_000)
	virtual := big.NewInt(5_000_000_000)
	slippage := uint64(125)

	buyBase, err := BuyBaseInputInternalWithFees(
		123_456_789_000, slippage, baseReserve, quoteReserve, virtual, fees,
	)
	if err != nil {
		t.Fatal(err)
	}
	if buyBase.InternalQuoteAmount != 16_206_205 || buyBase.UIQuote != 16_295_341 || buyBase.MaxQuote != 16_499_032 {
		t.Fatalf("buy base = %+v", buyBase)
	}

	buyQuote, err := BuyQuoteInputInternalWithFees(
		1_500_000_000, slippage, baseReserve, quoteReserve, virtual, fees,
	)
	if err != nil {
		t.Fatal(err)
	}
	if buyQuote.InternalQuoteWithoutFees != 1_491_795_125 || buyQuote.Base != 11_206_836_149_304 || buyQuote.MaxQuote != 1_518_750_000 {
		t.Fatalf("buy quote = %+v", buyQuote)
	}

	sellBase, err := SellBaseInputInternalWithFees(
		123_456_789_000, slippage, baseReserve, quoteReserve, virtual, fees,
	)
	if err != nil {
		t.Fatal(err)
	}
	if sellBase.InternalQuoteAmountOut != 16_201_203 || sellBase.UIQuote != 16_112_095 || sellBase.MinQuote != 15_910_694 {
		t.Fatalf("sell base = %+v", sellBase)
	}

	sellQuote, err := SellQuoteInputInternalWithFees(
		500_000_000, slippage, baseReserve, quoteReserve, virtual, fees,
	)
	if err != nil {
		t.Fatal(err)
	}
	if sellQuote.InternalRawQuote != 502_765_209 || sellQuote.Base != 3_849_022_110_532 || sellQuote.MinQuote != 493_750_000 {
		t.Fatalf("sell quote = %+v", sellQuote)
	}
}

func TestPumpSwapSellRejectsVirtualLiquidityBeyondVaultBalance(t *testing.T) {
	fees := PumpSwapFeeBasisPoints{}
	_, err := SellBaseInputInternalWithFees(
		1_000_000, 0, 1, 1, big.NewInt(1_000_000), fees,
	)
	if err == nil {
		t.Fatal("expected real quote reserve error")
	}
}

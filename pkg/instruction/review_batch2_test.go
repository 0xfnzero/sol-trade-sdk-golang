package instruction

import (
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestBatch2ClmmBitmapMismatch(t *testing.T) {
	wrong := solana.PublicKey{}
	a := RaydiumClmmSwapV2Accounts{TickArrayBitmapExtension: &wrong, TickArrays: []solana.PublicKey{wrong}}
	if _, err := BuildRaydiumClmmSwapV2(a, SwapV2Args{Amount: 1, OtherAmountThreshold: 1, AmountSpecifiedIsInput: true}); err == nil {
		t.Fatal("explicit wrong bitmap silently ignored")
	}
	a.TickArrayBitmapExtension = nil
	if _, err := BuildRaydiumClmmSwapV2(a, SwapV2Args{Amount: 1, OtherAmountThreshold: 1, AmountSpecifiedIsInput: true}); err != nil {
		t.Fatal(err)
	}
}

package calc

import "testing"

const (
	bonkTestVirtualBase  uint64 = 1_073_025_605_596_382
	bonkTestVirtualQuote uint64 = 30_000_852_951
)

func TestBonkBuySlippageAt10000BpsDoesNotZeroMinOut(t *testing.T) {
	withMax := GetBonkBuyTokenAmountFromSolAmount(
		1_000_000, bonkTestVirtualBase, bonkTestVirtualQuote, 0, 0, 10_000,
	)
	withClampCap := GetBonkBuyTokenAmountFromSolAmount(
		1_000_000, bonkTestVirtualBase, bonkTestVirtualQuote, 0, 0, MaxSlippageBasisPoints,
	)
	if withMax != withClampCap {
		t.Fatalf("expected clamp parity, got %d vs %d", withMax, withClampCap)
	}
	if withMax == 0 {
		t.Fatal("expected non-zero min out at 10000 bps")
	}
}

func TestBonkSellSlippageAbove10000BpsDoesNotUnderflow(t *testing.T) {
	withOverflow := GetBonkSellSolAmountFromTokenAmount(
		1_000_000_000, bonkTestVirtualBase, bonkTestVirtualQuote, 0, 0, 50_000,
	)
	withClampCap := GetBonkSellSolAmountFromTokenAmount(
		1_000_000_000, bonkTestVirtualBase, bonkTestVirtualQuote, 0, 0, MaxSlippageBasisPoints,
	)
	if withOverflow != withClampCap {
		t.Fatalf("expected clamp parity, got %d vs %d", withOverflow, withClampCap)
	}
	if withOverflow == 0 {
		t.Fatal("expected non-zero min out for oversized slippage")
	}
}

func TestClampSlippageBasisPoints(t *testing.T) {
	if ClampSlippageBasisPoints(0) != 0 {
		t.Fatal("0")
	}
	if ClampSlippageBasisPoints(100) != 100 {
		t.Fatal("100")
	}
	if ClampSlippageBasisPoints(10_000) != MaxSlippageBasisPoints {
		t.Fatal("10000")
	}
}

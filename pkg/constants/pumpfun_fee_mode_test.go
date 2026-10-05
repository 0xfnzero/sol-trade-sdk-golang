package constants

import "testing"
import "github.com/gagliardetto/solana-go"

func TestPumpFunFeeModeReconciliation(t *testing.T) {
	no, yes := false, true
	unknown := solana.PublicKey{42}
	cases := []struct {
		key  solana.PublicKey
		want [3]bool
	}{
		{solana.PublicKey{}, [3]bool{false, false, true}},
		{FEE_RECIPIENT, [3]bool{false, false, false}},
		{MAYHEM_FEE_RECIPIENTS[0], [3]bool{true, true, true}},
		{PUMPFUN_NORMAL_FEE_RECIPIENTS[1], [3]bool{false, false, true}},
		{unknown, [3]bool{false, false, true}},
	}
	for _, c := range cases {
		for i, flag := range []*bool{nil, &no, &yes} {
			if got := ReconcileMayhemModeForTrade(flag, c.key); got != c.want[i] {
				t.Fatalf("%s flag %d got %v", c.key, i, got)
			}
		}
	}
	if FeeRecipientOKForBondingCurveMode(MAYHEM_FEE_RECIPIENTS[0], false) || FeeRecipientOKForBondingCurveMode(PUMPFUN_NORMAL_FEE_RECIPIENTS[1], true) {
		t.Fatal("cross-mode recipient accepted")
	}
	if !FeeRecipientOKForBondingCurveMode(unknown, true) || !FeeRecipientOKForBondingCurveMode(unknown, false) {
		t.Fatal("rotated recipient rejected")
	}
}

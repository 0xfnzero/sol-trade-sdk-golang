package calc

import (
	"fmt"
	"math"
	"math/big"
)

func pumpFunReserve128(value *big.Int) error {
	if value == nil || value.Sign() < 0 || value.BitLen() > 128 {
		return fmt.Errorf("%w: reserve outside u128", ErrInvalidInput)
	}
	return nil
}

// GetBuyTokenAmountFromSolAmountU128 supports the full Rust reserve domain.
// Input integers are never mutated. Checked u128 overflow follows the Rust quote result.
func GetBuyTokenAmountFromSolAmountU128(virtualToken, virtualQuote, realToken *big.Int, hasCreator bool, amount uint64) (uint64, error) {
	for _, v := range []*big.Int{virtualToken, virtualQuote, realToken} {
		if err := pumpFunReserve128(v); err != nil {
			return 0, err
		}
	}
	if amount == 0 || virtualToken.Sign() == 0 {
		return 0, nil
	}
	fee := PumpFunFeeBasisPoints
	if hasCreator {
		fee += PumpFunCreatorFee
	}
	net := new(big.Int).Mul(new(big.Int).SetUint64(amount), big.NewInt(10000))
	net.Quo(net, new(big.Int).SetUint64(fee+10000))
	if net.Sign() == 0 {
		return 0, nil
	}
	net.Sub(net, big.NewInt(1))
	if net.Sign() == 0 {
		return 0, nil
	}
	denominator := new(big.Int).Add(virtualQuote, net)
	if denominator.BitLen() > 128 || denominator.Sign() == 0 {
		return 0, nil
	}
	tokens := new(big.Int).Mul(net, virtualToken)
	if tokens.BitLen() > 128 {
		return 0, nil
	}
	tokens.Quo(tokens, denominator)
	if tokens.Cmp(realToken) > 0 {
		tokens.Set(realToken)
	}
	if !tokens.IsUint64() {
		return math.MaxUint64, nil
	}
	return tokens.Uint64(), nil
}

// GetSellSolAmountFromTokenAmountU128 mirrors Rust's wide, saturating sell calculation.
func GetSellSolAmountFromTokenAmountU128(virtualToken, virtualQuote *big.Int, hasCreator bool, amount uint64) (uint64, error) {
	for _, v := range []*big.Int{virtualToken, virtualQuote} {
		if err := pumpFunReserve128(v); err != nil {
			return 0, err
		}
	}
	if amount == 0 || virtualToken.Sign() == 0 {
		return 0, nil
	}
	numerator := new(big.Int).Mul(new(big.Int).SetUint64(amount), virtualQuote)
	if numerator.BitLen() > 128 {
		return math.MaxUint64, nil
	}
	denominator := new(big.Int).Add(virtualToken, new(big.Int).SetUint64(amount))
	if denominator.BitLen() > 128 {
		denominator.SetUint64(1)
	}
	gross := numerator.Quo(numerator, denominator)
	bps := PumpFunFeeBasisPoints
	if hasCreator {
		bps += PumpFunCreatorFee
	}
	fee := new(big.Int).Mul(new(big.Int).Set(gross), new(big.Int).SetUint64(bps))
	fee.Add(fee, big.NewInt(9999))
	fee.Quo(fee, big.NewInt(10000))
	if gross.Cmp(fee) < 0 {
		return 0, nil
	}
	gross.Sub(gross, fee)
	if !gross.IsUint64() {
		return math.MaxUint64, nil
	}
	return gross.Uint64(), nil
}

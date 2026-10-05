package calc

// Current LaunchLab math with exact integers, epoch token fees and graduation cap.
import (
	"errors"
	"math/big"
)

type TokenTransferFee struct {
	BasisPoints uint16
	MaximumFee  uint64
}

func (f TokenTransferFee) Calculate(amount uint64, inverse bool) (uint64, error) {
	if f.BasisPoints > 10000 {
		return 0, errors.New("invalid token fee basis points")
	}
	if f.BasisPoints == 0 || amount == 0 {
		return 0, nil
	}
	if inverse && f.BasisPoints == 10000 {
		return f.MaximumFee, nil
	}
	den := uint64(10000)
	if inverse {
		den -= uint64(f.BasisPoints)
	}
	v := llCeil(new(big.Int).Mul(llN(amount), llN(uint64(f.BasisPoints))), llN(den))
	if v.Cmp(llN(f.MaximumFee)) > 0 {
		return f.MaximumFee, nil
	}
	return v.Uint64(), nil
}

type LaunchLabQuoteState struct {
	VirtualBase, VirtualQuote, RealBase, RealQuote, TotalBaseSell uint64
	CurveType                                                     uint8
	TradeFeeRate, PlatformFeeRate, CreatorFeeRate                 uint64
	BaseTransferFee, QuoteTransferFee                             TokenTransferFee
}
type LaunchLabQuote struct{ AmountIn, MinimumAmountOut uint64 }

func llN(v uint64) *big.Int { return new(big.Int).SetUint64(v) }
func llCeil(n, d *big.Int) *big.Int {
	return new(big.Int).Quo(new(big.Int).Add(n, new(big.Int).Sub(d, big.NewInt(1))), d)
}
func llU64(n *big.Int) (uint64, error) {
	if n.Sign() < 0 || !n.IsUint64() {
		return 0, errors.New("LaunchLab amount outside u64")
	}
	return n.Uint64(), nil
}
func llProduct(a, b *big.Int) (*big.Int, error) {
	n := new(big.Int).Mul(a, b)
	if n.Sign() < 0 || n.BitLen() > 128 {
		return nil, errors.New("LaunchLab u128 overflow")
	}
	return n, nil
}
func QuoteLaunchLabExactIn(p LaunchLabQuoteState, amount uint64, buy bool, slippageBps uint16, shareFeeRate uint64) (LaunchLabQuote, error) {
	fail := func(e error) (LaunchLabQuote, error) { return LaunchLabQuote{}, e }
	if amount == 0 {
		return fail(errors.New("amount cannot be zero"))
	}
	if p.CurveType != 0 {
		return fail(errors.New("unsupported LaunchLab curve type"))
	}
	if p.RealBase > p.VirtualBase {
		return fail(errors.New("LaunchLab reserve underflow"))
	}
	if slippageBps > 9999 {
		return fail(errors.New("slippage must be 0..9999"))
	}
	rate := new(big.Int)
	for _, v := range []uint64{p.TradeFeeRate, p.PlatformFeeRate, p.CreatorFeeRate, shareFeeRate} {
		rate.Add(rate, llN(v))
	}
	if rate.Cmp(llN(1000000)) > 0 {
		return fail(errors.New("LaunchLab fee exceeds denominator"))
	}
	base := llN(p.VirtualBase - p.RealBase)
	quote := new(big.Int).Add(llN(p.VirtualQuote), llN(p.RealQuote))
	actual := amount
	var received uint64
	if buy {
		fee, err := p.QuoteTransferFee.Calculate(amount, false)
		if err != nil {
			return fail(err)
		}
		vault := llN(amount - fee)
		net := new(big.Int).Sub(vault, llCeil(new(big.Int).Mul(vault, rate), llN(1000000)))
		den := new(big.Int).Add(quote, net)
		if den.Sign() == 0 {
			return fail(errors.New("empty curve reserves"))
		}
		product, err := llProduct(net, base)
		if err != nil {
			return fail(err)
		}
		out := new(big.Int).Quo(product, den)
		if p.TotalBaseSell != 0 {
			if p.RealBase > p.TotalBaseSell {
				return fail(errors.New("LaunchLab sold amount exceeds cap"))
			}
			remaining := llN(p.TotalBaseSell - p.RealBase)
			if out.Cmp(remaining) > 0 {
				after := new(big.Int).Sub(base, remaining)
				if after.Sign() <= 0 || rate.Cmp(llN(1000000)) == 0 {
					return fail(errors.New("LaunchLab graduation exhausts reserves"))
				}
				product, err = llProduct(quote, remaining)
				if err != nil {
					return fail(err)
				}
				required := llCeil(product, after)
				product, err = llProduct(required, llN(1000000))
				if err != nil {
					return fail(err)
				}
				requiredVault, err := llU64(llCeil(product, new(big.Int).Sub(llN(1000000), rate)))
				if err != nil {
					return fail(err)
				}
				fee, err = p.QuoteTransferFee.Calculate(requiredVault, true)
				if err != nil {
					return fail(err)
				}
				actual, err = llU64(new(big.Int).Add(llN(requiredVault), llN(fee)))
				if err != nil {
					return fail(err)
				}
				if actual > amount {
					actual = amount
				}
				out = remaining
			}
		}
		gross, err := llU64(out)
		if err != nil {
			return fail(err)
		}
		fee, err = p.BaseTransferFee.Calculate(gross, false)
		if err != nil {
			return fail(err)
		}
		received = gross - fee
	} else {
		fee, err := p.BaseTransferFee.Calculate(amount, false)
		if err != nil {
			return fail(err)
		}
		net := llN(amount - fee)
		den := new(big.Int).Add(base, net)
		if den.Sign() == 0 {
			return fail(errors.New("empty curve reserves"))
		}
		product, err := llProduct(net, quote)
		if err != nil {
			return fail(err)
		}
		gross := new(big.Int).Quo(product, den)
		vault, err := llU64(new(big.Int).Sub(gross, llCeil(new(big.Int).Mul(gross, rate), llN(1000000))))
		if err != nil {
			return fail(err)
		}
		fee, err = p.QuoteTransferFee.Calculate(vault, false)
		if err != nil {
			return fail(err)
		}
		received = vault - fee
	}
	deduction := new(big.Int).Quo(new(big.Int).Mul(llN(received), llN(uint64(slippageBps))), llN(10000)).Uint64()
	return LaunchLabQuote{actual, received - deduction}, nil
}

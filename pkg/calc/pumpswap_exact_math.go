package calc

import (
	"encoding/binary"
	"math/big"
	"math/bits"
)

// Quotient must fit u64; Div64 requires hi < d. Round only after division
// so an intermediate addition cannot overflow a full-width numerator.
func pumpSwapDiv128(hi, lo, d uint64, ceil bool) (uint64, error) {
	if d == 0 {
		return 0, ErrDivisionByZero
	}
	if hi >= d {
		return 0, ErrOverflow
	}
	q, r := bits.Div64(hi, lo, d)
	if ceil && r != 0 {
		if q == ^uint64(0) {
			return 0, ErrOverflow
		}
		q++
	}
	return q, nil
}

func pumpSwapInt(v uint64) *big.Int { return new(big.Int).SetUint64(v) }
func pumpSwapDiv(n, d *big.Int, ceil bool) (uint64, error) {
	if d.Sign() == 0 {
		return 0, ErrDivisionByZero
	}
	if n.Sign() >= 0 && n.BitLen() <= 128 && d.IsUint64() {
		var wide [16]byte
		n.FillBytes(wide[:])
		return pumpSwapDiv128(binary.BigEndian.Uint64(wide[:8]), binary.BigEndian.Uint64(wide[8:]), d.Uint64(), ceil)
	}
	q, r := new(big.Int), new(big.Int)
	if ceil {
		q.QuoRem(n, d, r)
		if r.Sign() != 0 {
			q.Add(q, big.NewInt(1))
		}
	} else {
		q.Quo(n, d)
	}
	if !q.IsUint64() {
		return 0, ErrOverflow
	}
	return q.Uint64(), nil
}
func pumpSwapReserves(base, quote uint64, virtual *big.Int) (uint64, error) {
	if base == 0 || quote == 0 {
		return 0, ErrInvalidReserves
	}
	effective, err := EffectiveQuoteReserves(quote, virtual)
	if err != nil {
		return 0, err
	}
	if effective == 0 {
		return 0, ErrInvalidReserves
	}
	return effective, nil
}
func pumpSwapFeeSum(f PumpSwapFeeBasisPoints) (uint64, error) {
	if err := checkAddOverflow(f.LPFeeBasisPoints, f.ProtocolFeeBasisPoints); err != nil {
		return 0, err
	}
	sum := f.LPFeeBasisPoints + f.ProtocolFeeBasisPoints
	if err := checkAddOverflow(sum, f.CoinCreatorFeeBasisPoints); err != nil {
		return 0, err
	}
	return sum + f.CoinCreatorFeeBasisPoints, nil
}
func pumpSwapFees(amount uint64, f PumpSwapFeeBasisPoints) (uint64, uint64, error) {
	fee := func(rate uint64) (uint64, error) {
		hi, lo := bits.Mul64(amount, rate)
		return pumpSwapDiv128(hi, lo, 10000, true)
	}
	lp, err := fee(f.LPFeeBasisPoints)
	if err != nil {
		return 0, 0, err
	}
	protocol, err := fee(f.ProtocolFeeBasisPoints)
	if err != nil {
		return 0, 0, err
	}
	creator, err := fee(f.CoinCreatorFeeBasisPoints)
	if err != nil {
		return 0, 0, err
	}
	if err := checkAddOverflow(lp, protocol); err != nil {
		return 0, 0, err
	}
	sum := lp + protocol
	if err := checkAddOverflow(sum, creator); err != nil {
		return 0, 0, err
	}
	return sum + creator, lp, nil
}

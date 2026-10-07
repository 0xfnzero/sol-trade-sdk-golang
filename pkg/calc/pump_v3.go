package calc

import (
	"fmt"
	"math/big"
)

// PumpV3QuoteState contains current resolved curve rates; MigrationFee is zero for token quotes.
type PumpV3QuoteState struct {
	VirtualBase, VirtualQuote, RemainingBase, RealQuote, CurveBaseBalance, MigrationFee, ProtocolBps, CreatorBps uint64
	Complete, Mayhem                                                                                             bool
}
type PumpV3Quote struct{ CurveBase, CurveQuote, PoolBase, PoolQuote, ProtocolFee, CreatorFee, BaseOut, QuoteIn uint64 }

func v3n(n uint64) *big.Int { return new(big.Int).SetUint64(n) }
func v3fee(n *big.Int, b uint64) *big.Int {
	return new(big.Int).Quo(new(big.Int).Add(new(big.Int).Mul(n, v3n(b)), v3n(9999)), v3n(10000))
}
func v3net(b uint64, s PumpV3QuoteState) *big.Int {
	n := new(big.Int).Quo(new(big.Int).Mul(v3n(b), v3n(10000)), v3n(10000+s.ProtocolBps+s.CreatorBps))
	cost := new(big.Int).Add(n, new(big.Int).Add(v3fee(n, s.ProtocolBps), v3fee(n, s.CreatorBps)))
	if cost.Cmp(v3n(b)) > 0 {
		n.Sub(n, new(big.Int).Sub(cost, v3n(b)))
	}
	return n
}
func v3validate(s PumpV3QuoteState) error {
	if s.Complete {
		return fmt.Errorf("BondingCurveComplete")
	}
	if s.VirtualBase <= s.RemainingBase || s.VirtualQuote == 0 || s.ProtocolBps > 10000 || s.CreatorBps > 10000-s.ProtocolBps {
		return fmt.Errorf("invalid curve reserves or fee rates")
	}
	return nil
}
func v3curve(n uint64, s PumpV3QuoteState) *big.Int {
	if n == 0 {
		return v3n(0)
	}
	return new(big.Int).Add(new(big.Int).Quo(new(big.Int).Mul(v3n(n), v3n(s.VirtualQuote)), v3n(s.VirtualBase-n)), v3n(1))
}
func v3pool(s PumpV3QuoteState, cq *big.Int) (*big.Int, *big.Int, error) {
	b := new(big.Int).Sub(v3n(s.CurveBaseBalance), v3n(s.RemainingBase))
	q := new(big.Int).Sub(new(big.Int).Add(v3n(s.RealQuote), cq), v3n(s.MigrationFee))
	if b.Sign() <= 0 || q.Sign() <= 0 {
		return nil, nil, fmt.Errorf("empty pool-to-be; fetch actual curve base vault balance")
	}
	return b, q, nil
}
func v3result(cb uint64, cq *big.Int, pb uint64, pq *big.Int, s PumpV3QuoteState) (PumpV3Quote, error) {
	pf := new(big.Int).Add(v3fee(cq, s.ProtocolBps), v3fee(pq, s.ProtocolBps))
	cf := new(big.Int).Add(v3fee(cq, s.CreatorBps), v3fee(pq, s.CreatorBps))
	bo := new(big.Int).Add(v3n(cb), v3n(pb))
	qi := new(big.Int).Add(new(big.Int).Add(cq, pq), new(big.Int).Add(pf, cf))
	for _, n := range []*big.Int{cq, pq, pf, cf, bo, qi} {
		if !n.IsUint64() {
			return PumpV3Quote{}, fmt.Errorf("quote exceeds u64")
		}
	}
	return PumpV3Quote{cb, cq.Uint64(), pb, pq.Uint64(), pf.Uint64(), cf.Uint64(), bo.Uint64(), qi.Uint64()}, nil
}
func QuotePumpBuyV3ExactOut(s PumpV3QuoteState, amount uint64, partialFill bool) (PumpV3Quote, error) {
	if err := v3validate(s); err != nil {
		return PumpV3Quote{}, err
	}
	cb := amount
	if cb > s.RemainingBase {
		cb = s.RemainingBase
	}
	cq := v3curve(cb, s)
	if amount <= s.RemainingBase {
		return v3result(cb, cq, 0, v3n(0), s)
	}
	if s.Mayhem {
		if !partialFill {
			return PumpV3Quote{}, fmt.Errorf("NotEnoughTokensToBuy")
		}
		return v3result(cb, cq, 0, v3n(0), s)
	}
	b, q, err := v3pool(s, cq)
	if err != nil {
		return PumpV3Quote{}, err
	}
	pb := amount - cb
	if v3n(pb).Cmp(b) >= 0 {
		return PumpV3Quote{}, fmt.Errorf("NotEnoughTokensToBuy")
	}
	den := new(big.Int).Sub(b, v3n(pb))
	pq := new(big.Int).Quo(new(big.Int).Add(new(big.Int).Mul(q, v3n(pb)), new(big.Int).Sub(den, v3n(1))), den)
	return v3result(cb, cq, pb, pq, s)
}
func QuotePumpBuyV3ExactIn(s PumpV3QuoteState, budget uint64) (PumpV3Quote, error) {
	if err := v3validate(s); err != nil {
		return PumpV3Quote{}, err
	}
	n := v3net(budget, s)
	if n.Cmp(v3n(1)) <= 0 {
		return PumpV3Quote{}, nil
	}
	n.Sub(n, v3n(1))
	tokens := new(big.Int).Quo(new(big.Int).Mul(n, v3n(s.VirtualBase)), new(big.Int).Add(v3n(s.VirtualQuote), n)).Uint64()
	cb := tokens
	if cb > s.RemainingBase {
		cb = s.RemainingBase
	}
	cq := v3curve(cb, s)
	if tokens <= s.RemainingBase || s.Mayhem {
		return v3result(cb, cq, 0, v3n(0), s)
	}
	spent := new(big.Int).Add(cq, new(big.Int).Add(v3fee(cq, s.ProtocolBps), v3fee(cq, s.CreatorBps)))
	left := new(big.Int).Sub(v3n(budget), spent)
	if left.Sign() <= 0 {
		return v3result(cb, cq, 0, v3n(0), s)
	}
	leg := v3net(left.Uint64(), s)
	if leg.Cmp(v3n(1)) <= 0 {
		return v3result(cb, cq, 0, v3n(0), s)
	}
	b, q, err := v3pool(s, cq)
	if err != nil {
		return PumpV3Quote{}, err
	}
	x := new(big.Int).Sub(leg, v3n(1))
	pb := new(big.Int).Quo(new(big.Int).Mul(x, b), new(big.Int).Add(q, x)).Uint64()
	if pb == 0 {
		return v3result(cb, cq, 0, v3n(0), s)
	}
	return v3result(cb, cq, pb, leg, s)
}

// PumpCoinInitialQuoteReserves uses actual migrated vault balances and signed virtual quote.
func PumpCoinInitialQuoteReserves(seed, quoteBase uint64, effectiveQuote *big.Int, initialBase, initialReal, quoteSupply uint64, depth, maxDepth uint8) (uint64, error) {
	if effectiveQuote == nil || effectiveQuote.Sign() <= 0 || initialBase <= initialReal || depth >= maxDepth {
		return 0, fmt.Errorf("invalid Pump quote state or CurveDepthExceeded")
	}
	reserves := new(big.Int).Quo(new(big.Int).Mul(new(big.Int).SetUint64(seed), new(big.Int).SetUint64(quoteBase)), effectiveQuote)
	if !reserves.IsUint64() || reserves.Sign() == 0 {
		return 0, fmt.Errorf("QuoteReservesOutOfRange")
	}
	raise := new(big.Int).Quo(new(big.Int).Mul(new(big.Int).Set(reserves), new(big.Int).SetUint64(initialReal)), new(big.Int).SetUint64(initialBase-initialReal))
	if raise.Cmp(new(big.Int).SetUint64(quoteSupply)) > 0 {
		return 0, fmt.Errorf("QuoteReservesOutOfRange")
	}
	return reserves.Uint64(), nil
}

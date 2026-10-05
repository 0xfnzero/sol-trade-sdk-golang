// Package subscription prepares trades from ordered account updates without RPC.
// Use a separate cache per selected fork. Snapshots bound account age but do not
// imply that independently streamed accounts belong to an atomic bank snapshot.
package subscription

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"sync"
)

type CachedAccount struct {
	Owner              solana.PublicKey
	Data               []byte
	Slot, WriteVersion uint64
}
type CacheReadContext struct{ Slot, Epoch, MaximumSlotAge uint64 }
type PoolTradeHint struct{ Pool, InputMint, OutputMint solana.PublicKey }
type ParserRouteIdentity struct {
	Protocol   string  `json:"protocol"`
	Program    string  `json:"program"`
	Pool       string  `json:"pool"`
	InputMint  *string `json:"input_mint"`
	OutputMint *string `json:"output_mint"`
}

var protocols = map[string]string{
	"PumpFun":       "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P",
	"MeteoraDammV2": "cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG",
	"PumpSwap":      "pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA",
	"LaunchLab":     instruction.BONK_PROGRAM.String(),
	"RaydiumCpmm":   "CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C",
	"RaydiumClmm":   "CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK",
	"OrcaWhirlpool": "whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc",
	"MeteoraDlmm":   "LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo",
	"RaydiumAmmV4":  "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8",
}

// PoolTradeHintFromRouteLeg accepts a native parser RouteSwapLeg (or compatible
// JSON value) without linking a specific parser version. Copies identities only.
func PoolTradeHintFromRouteLeg(leg any) (PoolTradeHint, error) {
	b, err := json.Marshal(leg)
	if err != nil {
		return PoolTradeHint{}, err
	}
	var identity ParserRouteIdentity
	if err = json.Unmarshal(b, &identity); err != nil {
		return PoolTradeHint{}, err
	}
	return PoolTradeHintFromIdentity(identity)
}

// PoolTradeHintFromIdentity avoids serialization when the caller already has identities.
func PoolTradeHintFromIdentity(leg ParserRouteIdentity) (PoolTradeHint, error) {
	fail := func(s string) (PoolTradeHint, error) { return PoolTradeHint{}, errors.New(s) }
	expected, ok := protocols[leg.Protocol]
	if !ok || expected != leg.Program {
		return fail("unsupported or mismatched route protocol")
	}
	if leg.InputMint == nil || leg.OutputMint == nil {
		return fail("route mints are unresolved")
	}
	keys := []string{leg.Pool, *leg.InputMint, *leg.OutputMint}
	var parsed [3]solana.PublicKey
	for i, s := range keys {
		k, err := solana.PublicKeyFromBase58(s)
		if err != nil {
			return PoolTradeHint{}, err
		}
		parsed[i] = k
	}
	h := PoolTradeHint{parsed[0], parsed[1], parsed[2]}
	if err := h.validate(); err != nil {
		return PoolTradeHint{}, err
	}
	return h, nil
}
func (h PoolTradeHint) validate() error {
	if h.Pool.IsZero() || h.InputMint.IsZero() || h.OutputMint.IsZero() || h.InputMint == h.OutputMint {
		return errors.New("missing or identical pool/mint identity")
	}
	return nil
}
func (h PoolTradeHint) matches(base, quote solana.PublicKey) error {
	if err := h.validate(); err != nil {
		return err
	}
	if base == quote || !(h.InputMint == base && h.OutputMint == quote || h.InputMint == quote && h.OutputMint == base) {
		return errors.New("route mint identity mismatch")
	}
	return nil
}
func owned(a CachedAccount) CachedAccount { a.Data = bytes.Clone(a.Data); return a }

type AccountUpdate struct {
	Pubkey  solana.PublicKey
	Account CachedAccount
}
type SubscriptionAccountCache struct {
	mu         sync.RWMutex
	accounts   map[solana.PublicKey]CachedAccount
	conflicted bool
}

func (c *SubscriptionAccountCache) assertNoConflict() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conflicted {
		return errors.New("conflicting cached account version; create a new cache for the explicitly selected fork")
	}
	return nil
}

func (c *SubscriptionAccountCache) Update(key solana.PublicKey, a CachedAccount) (bool, error) {
	n, e := c.UpdateMany([]AccountUpdate{{key, a}})
	return n > 0, e
}

// UpdateMany rejects the entire batch if any equal version has different bytes.
func (c *SubscriptionAccountCache) UpdateMany(updates []AccountUpdate) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conflicted {
		return 0, errors.New("conflicting cached account version; create a new cache for the explicitly selected fork")
	}
	// Stage only touched identities; no full subscription-map copy per update.
	pending := make(map[solana.PublicKey]CachedAccount, len(updates))
	changed := 0
	for _, u := range updates {
		a := u.Account
		old, ok := pending[u.Pubkey]
		if !ok {
			old, ok = c.accounts[u.Pubkey]
		}
		if ok {
			if a.Slot < old.Slot || a.Slot == old.Slot && a.WriteVersion < old.WriteVersion {
				continue
			}
			if a.Slot == old.Slot && a.WriteVersion == old.WriteVersion {
				if a.Owner != old.Owner || !bytes.Equal(a.Data, old.Data) {
					c.conflicted = true
					return 0, errors.New("conflicting cached account version; select a fork explicitly")
				}
				continue
			}
		}
		pending[u.Pubkey] = owned(a)
		changed++
	}
	if len(pending) != 0 && c.accounts == nil {
		c.accounts = make(map[solana.PublicKey]CachedAccount, len(pending))
	}
	for key, a := range pending {
		c.accounts[key] = a
	}
	return changed, nil
}

// UpdateRaw accepts raw parser fields, keeping closed-account tombstones.
func (c *SubscriptionAccountCache) UpdateRaw(key, owner solana.PublicKey, data []byte, lamports, slot, writeVersion uint64) (bool, error) {
	if lamports == 0 {
		data = nil
	}
	return c.Update(key, CachedAccount{owner, data, slot, writeVersion})
}

type AccountCacheSnapshot struct {
	continuityGuard func() error
	accounts        map[solana.PublicKey]CachedAccount
}

func (c *SubscriptionAccountCache) Snapshot() *AccountCacheSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s := &AccountCacheSnapshot{accounts: make(map[solana.PublicKey]CachedAccount, len(c.accounts)), continuityGuard: c.assertNoConflict}
	for k, a := range c.accounts {
		s.accounts[k] = owned(a)
	}
	return s
}
func (s *AccountCacheSnapshot) Get(key solana.PublicKey, ctx CacheReadContext, expectedOwner *solana.PublicKey) (CachedAccount, error) {
	if err := s.AssertUsable(); err != nil {
		return CachedAccount{}, err
	}
	a, ok := s.accounts[key]
	if !ok {
		return CachedAccount{}, errors.New("missing cached account: " + key.String())
	}
	if expectedOwner != nil && a.Owner != *expectedOwner {
		return CachedAccount{}, errors.New("cached account owner mismatch")
	}
	if a.Slot > ctx.Slot || ctx.Slot-a.Slot > ctx.MaximumSlotAge {
		return CachedAccount{}, errors.New("cached account is future or stale")
	}
	if len(a.Data) == 0 {
		return CachedAccount{}, errors.New("cached account is closed")
	}
	return owned(a), nil
}

// GetOptional accepts an observed zero-lamport tombstone, never an unobserved key.
func (s *AccountCacheSnapshot) GetOptional(key solana.PublicKey, ctx CacheReadContext, expectedOwner *solana.PublicKey) (*CachedAccount, error) {
	if err := s.AssertUsable(); err != nil {
		return nil, err
	}
	a, ok := s.accounts[key]
	if !ok {
		return nil, errors.New("missing cached account: " + key.String())
	}
	if a.Slot > ctx.Slot || ctx.Slot-a.Slot > ctx.MaximumSlotAge {
		return nil, errors.New("cached account is future or stale")
	}
	if len(a.Data) == 0 {
		return nil, nil
	}
	if expectedOwner != nil && a.Owner != *expectedOwner {
		return nil, errors.New("cached account owner mismatch")
	}
	copy := owned(a)
	return &copy, nil
}
func (s *AccountCacheSnapshot) StonkFunCurve(h PoolTradeHint, ctx CacheReadContext) (instruction.StonkFunCurveAccounts, calc.LaunchLabQuoteState, error) {
	return s.launchlabState(h, ctx, true)
}
func (s *AccountCacheSnapshot) LaunchLabCurve(h PoolTradeHint, ctx CacheReadContext) (instruction.StonkFunCurveAccounts, calc.LaunchLabQuoteState, error) {
	return s.launchlabState(h, ctx, false)
}
func (s *AccountCacheSnapshot) launchlabState(h PoolTradeHint, ctx CacheReadContext, strict bool) (instruction.StonkFunCurveAccounts, calc.LaunchLabQuoteState, error) {
	fail := func(err error) (instruction.StonkFunCurveAccounts, calc.LaunchLabQuoteState, error) {
		return instruction.StonkFunCurveAccounts{}, calc.LaunchLabQuoteState{}, err
	}
	pool, err := s.Get(h.Pool, ctx, &instruction.BONK_PROGRAM)
	if err != nil {
		return fail(err)
	}
	d := pool.Data
	if len(d) < 429 || !bytes.Equal(d[:8], []byte{247, 237, 227, 245, 215, 195, 222, 70}) {
		return fail(errors.New("invalid cached LaunchLab pool"))
	}
	key := func(o int) solana.PublicKey { var k solana.PublicKey; copy(k[:], d[o:o+32]); return k }
	if err = h.matches(key(205), key(237)); err != nil {
		return fail(err)
	}
	gk, pk := key(141), key(173)
	global, err := s.Get(gk, ctx, &instruction.BONK_PROGRAM)
	if err != nil {
		return fail(err)
	}
	platform, err := s.Get(pk, ctx, &instruction.BONK_PROGRAM)
	if err != nil {
		return fail(err)
	}
	base, err := s.Get(key(205), ctx, nil)
	if err != nil {
		return fail(err)
	}
	quote, err := s.Get(key(237), ctx, nil)
	if err != nil {
		return fail(err)
	}
	bf, err := instruction.TokenTransferFeeForEpoch(base.Data, base.Owner, ctx.Epoch)
	if err != nil {
		return fail(err)
	}
	qf, err := instruction.TokenTransferFeeForEpoch(quote.Data, quote.Owner, ctx.Epoch)
	if err != nil {
		return fail(err)
	}
	decode := instruction.DecodeLaunchLabCurve
	if strict {
		decode = instruction.DecodeStonkFunCurve
	}
	a, state, err := decode(instruction.LaunchLabAccountBytes{Pubkey: h.Pool, Owner: pool.Owner, Data: d}, instruction.LaunchLabAccountBytes{Pubkey: gk, Owner: global.Owner, Data: global.Data}, instruction.LaunchLabAccountBytes{Pubkey: pk, Owner: platform.Owner, Data: platform.Data}, base.Owner, quote.Owner, bf, qf)
	if err != nil {
		return fail(err)
	}
	// Avoid overflow before adding hostile fee values.
	if state.CurveType != 0 || state.TradeFeeRate >= 1000000 || state.PlatformFeeRate >= 1000000 || state.CreatorFeeRate >= 1000000 || state.TradeFeeRate+state.PlatformFeeRate+state.CreatorFeeRate >= 1000000 {
		return fail(errors.New("invalid cached LaunchLab curve or fee configuration"))
	}
	return a, state, nil
}

type PreparedStonkFunCurve struct {
	Accounts    instruction.StonkFunCurveAccounts
	Quote       calc.LaunchLabQuote
	Instruction solana.Instruction
}

func (s *AccountCacheSnapshot) PrepareStonkFunCurve(h PoolTradeHint, ctx CacheReadContext, payer solana.PublicKey, amount uint64, slippageBps uint16) (PreparedStonkFunCurve, error) {
	a, state, e := s.StonkFunCurve(h, ctx)
	if e != nil {
		return PreparedStonkFunCurve{}, e
	}
	buy := h.InputMint == a.QuoteMint
	q, e := calc.QuoteLaunchLabExactIn(state, amount, buy, slippageBps, 0)
	if e != nil {
		return PreparedStonkFunCurve{}, e
	}
	ix, e := instruction.BuildStonkFunCurveExactIn(a, payer, q.AmountIn, q.MinimumAmountOut, buy, 0)
	if e != nil {
		return PreparedStonkFunCurve{}, e
	}
	return PreparedStonkFunCurve{a, q, ix}, nil
}

func (s *AccountCacheSnapshot) AssertUsable() error {
	if s == nil {
		return errors.New("missing frozen account snapshot")
	}
	if s.continuityGuard != nil {
		return s.continuityGuard()
	}
	return nil
}

// ReadySnapshot binds a live frozen cache to the current validated gRPC generation.
func (c *SubscriptionAccountCache) ReadySnapshot(readiness *SubscriptionReadiness) (*AccountCacheSnapshot, error) {
	if readiness == nil {
		return nil, errors.New("provide live subscription readiness")
	}
	guard, err := readiness.Guard()
	if err != nil {
		return nil, err
	}
	s := c.Snapshot()
	cacheGuard := s.continuityGuard
	s.continuityGuard = func() error {
		if err := cacheGuard(); err != nil {
			return err
		}
		return guard()
	}
	if err = s.AssertUsable(); err != nil {
		return nil, err
	}
	return s, nil
}

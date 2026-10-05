package subscription

import (
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"math"
	"reflect"
)

type CpmmAccountVersion struct {
	Address            solana.PublicKey
	Slot, WriteVersion uint64
}
type PreparedCpmmCreatorFeeCollection struct {
	Instruction                                                                                   solana.Instruction
	SnapshotSlot                                                                                  uint64
	AccountVersions                                                                               [3]CpmmAccountVersion
	Pool, Creator, AmmConfig, CreatorFeeShare                                                     solana.PublicKey
	ShareRate, CreatorPayoutToken0, CreatorPayoutToken1, ProtocolShareToken0, ProtocolShareToken1 uint64
}

// GetObservation includes closed-account tombstones, rejects unknown/stale/future keys.
func (s *AccountCacheSnapshot) GetObservation(key solana.PublicKey, ctx CacheReadContext) (CachedAccount, error) {
	if err := s.AssertUsable(); err != nil {
		return CachedAccount{}, err
	}
	a, ok := s.accounts[key]
	if !ok {
		return CachedAccount{}, fmt.Errorf("missing cached account: %s", key)
	}
	if a.Slot > ctx.Slot || ctx.Slot-a.Slot > ctx.MaximumSlotAge {
		return CachedAccount{}, fmt.Errorf("cached account is future or stale")
	}
	return owned(a), nil
}

// Requires explicit PDA observation; no stream update does not prove absence.
func (s *AccountCacheSnapshot) PrepareCpmmCreatorFeeCollection(address solana.PublicKey, payer *solana.PublicKey, ctx CacheReadContext) (*PreparedCpmmCreatorFeeCollection, error) {
	program := instruction.RAYDIUM_CPMM_PROGRAM
	a, err := s.Get(address, ctx, &program)
	if err != nil {
		return nil, err
	}
	pool, err := instruction.DecodeCpmmCollectionPool(a.Data)
	if err != nil {
		return nil, err
	}
	for _, p := range []solana.PublicKey{pool.Token0Program, pool.Token1Program} {
		if p != constants.TOKEN_PROGRAM && p != constants.TOKEN_PROGRAM_2022 {
			return nil, fmt.Errorf("unsupported CPMM token program")
		}
	}
	if pool.Token0Mint.IsZero() || pool.Token1Mint.IsZero() || pool.Token0Mint == pool.Token1Mint {
		return nil, fmt.Errorf("invalid mint pair")
	}
	if pool.CreatorFeesToken0 == 0 && pool.CreatorFeesToken1 == 0 {
		return nil, fmt.Errorf("no accrued CPMM creator fees")
	}
	c, err := s.Get(pool.AmmConfig, ctx, &program)
	if err != nil {
		return nil, err
	}
	config, err := instruction.DecodeCpmmAmmConfig(c.Data)
	if err != nil {
		return nil, err
	}
	share, err := instruction.GetCreatorFeeSharePDA(pool.PoolCreator, pool.AmmConfig)
	if err != nil {
		return nil, err
	}
	observation, err := s.GetObservation(share, ctx)
	if err != nil {
		return nil, err
	}
	rate, err := instruction.ResolveCreatorFeeShareRate(config, pool.PoolCreator, pool.AmmConfig, &rpc.Account{Owner: observation.Owner, Data: rpc.DataBytesOrJSONFromBytes(observation.Data), Lamports: 1})
	if err != nil {
		return nil, err
	}
	payout0, protocol0, err := instruction.SplitCreatorFee(pool.CreatorFeesToken0, rate)
	if err != nil {
		return nil, err
	}
	payout1, protocol1, err := instruction.SplitCreatorFee(pool.CreatorFeesToken1, rate)
	if err != nil {
		return nil, err
	}
	if pool.ProtocolFeesToken0 > math.MaxUint64-protocol0 || pool.ProtocolFeesToken1 > math.MaxUint64-protocol1 {
		return nil, fmt.Errorf("CPMM protocol fee overflow")
	}
	var ix solana.Instruction
	if payer == nil {
		ix, err = instruction.CollectCreatorFee(address, pool)
	} else {
		ix, err = instruction.CollectCreatorFeePermissionless(*payer, address, pool)
	}
	if err != nil {
		return nil, err
	}
	if err = s.AssertUsable(); err != nil {
		return nil, err
	}
	return &PreparedCpmmCreatorFeeCollection{Instruction: ix, SnapshotSlot: ctx.Slot, AccountVersions: [3]CpmmAccountVersion{{address, a.Slot, a.WriteVersion}, {pool.AmmConfig, c.Slot, c.WriteVersion}, {share, observation.Slot, observation.WriteVersion}}, Pool: address, Creator: pool.PoolCreator, AmmConfig: pool.AmmConfig, CreatorFeeShare: share, ShareRate: rate, CreatorPayoutToken0: payout0, CreatorPayoutToken1: payout1, ProtocolShareToken0: protocol0, ProtocolShareToken1: protocol1}, nil
}
func (s *AccountCacheSnapshot) ValidateCpmmCreatorFeeCollection(p *PreparedCpmmCreatorFeeCollection, payer *solana.PublicKey, ctx CacheReadContext) error {
	if p == nil || ctx.Slot < p.SnapshotSlot {
		return fmt.Errorf("invalid/backwards collection context")
	}
	for _, v := range p.AccountVersions {
		if v.Slot > p.SnapshotSlot {
			return fmt.Errorf("prepared snapshot precedes observations")
		}
		a, err := s.GetObservation(v.Address, ctx)
		if err != nil {
			return err
		}
		if a.Slot != v.Slot || a.WriteVersion != v.WriteVersion {
			return fmt.Errorf("creator-fee preparation changed; reprepare")
		}
	}
	current, err := s.PrepareCpmmCreatorFeeCollection(p.Pool, payer, ctx)
	if err != nil {
		return err
	}
	current.SnapshotSlot = p.SnapshotSlot
	if !reflect.DeepEqual(current, p) {
		return fmt.Errorf("creator-fee preparation changed; reprepare")
	}
	return nil
}

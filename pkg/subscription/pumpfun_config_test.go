package subscription

import (
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"testing"
)

var configMint = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
var configRecipient = solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
var configContext = CacheReadContext{Slot: 100, Epoch: 1, MaximumSlotAge: 5}

func pumpConfigFixture() *AccountCacheSnapshot {
	g := make([]byte, 73)
	copy(g, []byte{167, 232, 232, 177, 200, 108, 114, 127})
	copy(g[41:], configRecipient[:])
	d := make([]byte, 43)
	copy(d, []byte{216, 74, 9, 0, 56, 140, 93, 75})
	d[10] = 1
	copy(d[11:], configMint[:])
	return &AccountCacheSnapshot{accounts: map[solana.PublicKey]CachedAccount{
		instruction.PUMPFUN_GLOBAL_ACCOUNT:                    {Owner: instruction.PUMPFUN_PROGRAM, Data: g, Slot: 100},
		instruction.GetPumpFunFeeSharingConfigPDA(configMint): {Owner: instruction.PUMPFUN_FEE_PROGRAM, Data: d, Slot: 100},
	}}
}

func TestPumpFunCurrentConfiguration(t *testing.T) {
	s := pumpConfigFixture()
	state, e := s.PumpFunConfiguration(configMint, configContext)
	if e != nil {
		t.Fatal(e)
	}
	vault := instruction.GetCreatorVaultPDA(state.SharingConfig)
	if state.FeeRecipient != configRecipient || state.FeeSharingCreatorVaultIfActive == nil || *state.FeeSharingCreatorVaultIfActive != vault {
		t.Fatal("current configuration mismatch")
	}
	s.accounts[state.SharingConfig].Data[10] = 0
	state, e = s.PumpFunConfiguration(configMint, configContext)
	if e != nil || state.FeeSharingCreatorVaultIfActive != nil {
		t.Fatal("inactive configuration mismatch", e)
	}
	delete(s.accounts, state.SharingConfig)
	if _, e = s.PumpFunConfiguration(configMint, configContext); e == nil {
		t.Fatal("missing configuration treated as inactive")
	}
}

func TestPumpFunInvalidConfiguration(t *testing.T) {
	for _, failure := range []string{"owner", "stale", "mint", "global-disc", "sharing-disc", "zero-recipient"} {
		t.Run(failure, func(t *testing.T) {
			s := pumpConfigFixture()
			g := instruction.PUMPFUN_GLOBAL_ACCOUNT
			c := instruction.GetPumpFunFeeSharingConfigPDA(configMint)
			switch failure {
			case "owner":
				a := s.accounts[g]
				a.Owner = solana.PublicKey{}
				s.accounts[g] = a
			case "stale":
				a := s.accounts[g]
				a.Slot = 94
				s.accounts[g] = a
			case "mint":
				s.accounts[c].Data[11] ^= 1
			case "global-disc":
				s.accounts[g].Data[0] ^= 1
			case "sharing-disc":
				s.accounts[c].Data[0] ^= 1
			case "zero-recipient":
				copy(s.accounts[g].Data[41:], make([]byte, 32))
			}
			if _, e := s.PumpFunConfiguration(configMint, configContext); e == nil {
				t.Fatal("invalid state accepted")
			}
		})
	}
	if _, e := DecodePumpFunGlobalFeeRecipient(make([]byte, 72)); e == nil {
		t.Fatal("truncated global accepted")
	}
	if _, e := DecodePumpFunSharingCreatorVault(make([]byte, 42), configMint); e == nil {
		t.Fatal("truncated config accepted")
	}
	s := pumpConfigFixture()
	s.continuityGuard = func() error { return errors.New("interrupted") }
	if _, e := s.PumpFunConfiguration(configMint, configContext); e == nil {
		t.Fatal("interrupted snapshot accepted")
	}
}

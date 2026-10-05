package subscription

import (
	"bytes"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
)

// CachedPumpFunConfiguration is account configuration, not a quote or factory.
type CachedPumpFunConfiguration struct {
	FeeRecipient                   solana.PublicKey
	SharingConfig                  solana.PublicKey
	FeeSharingCreatorVaultIfActive *solana.PublicKey
}

func DecodePumpFunGlobalFeeRecipient(data []byte) (solana.PublicKey, error) {
	if len(data) < 73 || !bytes.Equal(data[:8], []byte{167, 232, 232, 177, 200, 108, 114, 127}) {
		return solana.PublicKey{}, errors.New("invalid PumpFun Global discriminator or size")
	}
	return solana.PublicKeyFromBytes(data[41:73]), nil
}

func DecodePumpFunSharingCreatorVault(data []byte, mint solana.PublicKey) (*solana.PublicKey, error) {
	if len(data) < 43 || !bytes.Equal(data[:8], []byte{216, 74, 9, 0, 56, 140, 93, 75}) {
		return nil, errors.New("invalid PumpFun SharingConfig discriminator or size")
	}
	if !bytes.Equal(data[11:43], mint[:]) {
		return nil, errors.New("PumpFun SharingConfig mint mismatch")
	}
	if data[10] != 1 {
		return nil, nil
	}
	vault := instruction.GetCreatorVaultPDA(instruction.GetPumpFunFeeSharingConfigPDA(mint))
	return &vault, nil
}

// PumpFunConfiguration rejects missing configs rather than assuming inactivity.
func (s *AccountCacheSnapshot) PumpFunConfiguration(mint solana.PublicKey, ctx CacheReadContext) (CachedPumpFunConfiguration, error) {
	var result CachedPumpFunConfiguration
	if mint.IsZero() {
		return result, errors.New("missing PumpFun mint")
	}
	global, err := s.Get(instruction.PUMPFUN_GLOBAL_ACCOUNT, ctx, &instruction.PUMPFUN_PROGRAM)
	if err != nil {
		return result, err
	}
	recipient, err := DecodePumpFunGlobalFeeRecipient(global.Data)
	if err != nil {
		return result, err
	}
	if recipient.IsZero() {
		return result, errors.New("missing PumpFun Global fee recipient")
	}
	config := instruction.GetPumpFunFeeSharingConfigPDA(mint)
	sharing, err := s.Get(config, ctx, &instruction.PUMPFUN_FEE_PROGRAM)
	if err != nil {
		return result, err
	}
	vault, err := DecodePumpFunSharingCreatorVault(sharing.Data, mint)
	if err != nil {
		return result, err
	}
	if err = s.AssertUsable(); err != nil {
		return result, err
	}
	return CachedPumpFunConfiguration{recipient, config, vault}, nil
}

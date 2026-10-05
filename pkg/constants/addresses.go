package constants

import "github.com/gagliardetto/solana-go"

// Fee recipients
var (
	// Standard fee recipient for PumpFun
	FEE_RECIPIENT = MustPublicKeyFromBase58("62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV")

	// Mayhem fee recipients (random selection supported)
	MAYHEM_FEE_RECIPIENTS = []solana.PublicKey{
		MustPublicKeyFromBase58("GesfTA3X2arioaHp8bbKdjG9vJtskViWACZoYvxp4twS"),
		MustPublicKeyFromBase58("4budycTjhs9fD6xw62VBducVTNgMgJJ5BgtKq7mAZwn6"),
		MustPublicKeyFromBase58("8SBKzEQU4nLSzcwF4a74F2iaUDQyTfjGndn6qUWBnrpR"),
		MustPublicKeyFromBase58("4UQeTP1T39KZ9Sfxzo3WR5skgsaP6NZa87BAkuazLEKH"),
		MustPublicKeyFromBase58("8sNeir4QsLsJdYpc9RZacohhK1Y5FLU3nC5LXgYB4aa6"),
		MustPublicKeyFromBase58("Fh9HmeLNUMVCvejxCtCL2DbYaRyBFVJ5xrWkLnMH6fdk"),
		MustPublicKeyFromBase58("463MEnMeGyJekNZFQSTUABBEbLnvMTALbT6ZmsxAbAdq"),
		MustPublicKeyFromBase58("6AUH3WEHucYZyC61hqpqYUWVto5qA5hjHuNQ32GNnNxA"),
	}
)

// PumpSwap fee recipients
var (
	PUMPSWAP_FEE_RECIPIENT = MustPublicKeyFromBase58("62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV")
)

// MustPublicKeyFromBase58 parses a base58 public key or panics
func MustPublicKeyFromBase58(s string) solana.PublicKey {
	pk, err := solana.PublicKeyFromBase58(s)
	if err != nil {
		panic(err)
	}
	return pk
}

// Known normal fee recipients for bonding-curve mode validation.
var PUMPFUN_NORMAL_FEE_RECIPIENTS = []solana.PublicKey{FEE_RECIPIENT,
	MustPublicKeyFromBase58("7VtfL8fvgNfhz17qKRMjzQEXgbdpnHHHQRh54R9jP2RJ"),
	MustPublicKeyFromBase58("7hTckgnGnLQR6sdH7YkqFTAA7VwTfYFaZ6EhEsU3saCX"),
	MustPublicKeyFromBase58("9rPYyANsfQZw3DnDmKE3YCQF5E8oD89UXoHn9JFEhJUz"),
	MustPublicKeyFromBase58("AVmoTthdrX6tKt4nDjco2D775W2YK3sDhxPcMmzUAmTY"),
	MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
	MustPublicKeyFromBase58("FWsW1xNtWscwNmKv6wVsU1iTzRN6wmmk3MjxRP5tT7hz"),
	MustPublicKeyFromBase58("G5UZAVbAf46s7cKWoyKu8kYTip9DGTpbLZ2qa9Aq69dP"),
}

func IsMayhemFeeRecipient(key solana.PublicKey) bool {
	for _, k := range MAYHEM_FEE_RECIPIENTS {
		if key == k {
			return true
		}
	}
	return false
}
func ReconcileMayhemModeForTrade(flag *bool, key solana.PublicKey) bool {
	if key.IsZero() {
		return flag != nil && *flag
	}
	mayhem := IsMayhemFeeRecipient(key)
	if flag == nil {
		return mayhem
	}
	if mayhem && !*flag {
		return true
	}
	if key == FEE_RECIPIENT && *flag {
		return false
	}
	return *flag
}
func FeeRecipientOKForBondingCurveMode(key solana.PublicKey, mayhem bool) bool {
	reserved := IsMayhemFeeRecipient(key)
	normal := false
	for _, k := range PUMPFUN_NORMAL_FEE_RECIPIENTS {
		if key == k {
			normal = true
			break
		}
	}
	if mayhem {
		return reserved || (!normal && !key.IsZero())
	}
	return normal || (!reserved && !key.IsZero())
}

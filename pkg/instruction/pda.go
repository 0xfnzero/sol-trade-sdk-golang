package instruction

import (
	"crypto/sha256"

	"github.com/your-org/sol-trade-sdk-go/pkg/constants"
	"github.com/gagliardetto/solana-go"
)

// GetBondingCurvePDA derives the bonding curve PDA for a mint
func GetBondingCurvePDA(mint solana.PublicKey) solana.PublicKey {
	seeds := [][]byte{
		[]byte("bonding-curve"),
		mint[:],
	}
	pubkey, _, _ := solana.FindProgramAddress(seeds, constants.PUMPFUN_PROGRAM_ID)
	return pubkey
}

// GetBondingCurveV2PDA derives the bonding curve v2 PDA for a mint
func GetBondingCurveV2PDA(mint solana.PublicKey) solana.PublicKey {
	seeds := [][]byte{
		[]byte("bonding-curve-v2"),
		mint[:],
	}
	pubkey, _, _ := solana.FindProgramAddress(seeds, constants.PUMPFUN_PROGRAM_ID)
	return pubkey
}

// GetUserVolumeAccumulatorPDA derives the user volume accumulator PDA
func GetUserVolumeAccumulatorPDA(user solana.PublicKey) solana.PublicKey {
	seeds := [][]byte{
		[]byte("user-volume-accumulator"),
		user[:],
	}
	pubkey, _, _ := solana.FindProgramAddress(seeds, constants.PUMPFUN_PROGRAM_ID)
	return pubkey
}

// GetCreatorVaultPDA derives the creator vault PDA
func GetCreatorVaultPDA(creator solana.PublicKey) solana.PublicKey {
	seeds := [][]byte{
		[]byte("creator-vault"),
		creator[:],
	}
	pubkey, _, _ := solana.FindProgramAddress(seeds, constants.PUMPFUN_PROGRAM_ID)
	return pubkey
}

// GetGlobalAccount returns the global account public key
func GetGlobalAccount() solana.PublicKey {
	seeds := [][]byte{[]byte("global")}
	pubkey, _, _ := solana.FindProgramAddress(seeds, constants.PUMPFUN_PROGRAM_ID)
	return pubkey
}

// GetEventAuthority returns the event authority PDA
func GetEventAuthority() solana.PublicKey {
	seeds := [][]byte{[]byte("__event_authority")}
	pubkey, _, _ := solana.FindProgramAddress(seeds, constants.PUMPFUN_PROGRAM_ID)
	return pubkey
}

// GetFeeRecipient returns the fee recipient based on mayhem mode
func GetFeeRecipient(isMayhemMode bool) solana.PublicKey {
	if isMayhemMode {
		// Return one of the mayhem fee recipients
		return constants.MAYHEM_FEE_RECIPIENTS[0]
	}
	return constants.FEE_RECIPIENT
}

// GetAssociatedTokenAddress derives the associated token account address
func GetAssociatedTokenAddress(owner, mint solana.PublicKey, tokenProgram solana.PublicKey) solana.PublicKey {
	seeds := [][]byte{
		owner[:],
		tokenProgram[:],
		mint[:],
	}
	pubkey, _, _ := solana.FindProgramAddress(seeds, constants.ASSOCIATED_TOKEN_PROGRAM_ID)
	return pubkey
}

// GetPoolPDA derives the pool PDA for PumpSwap
func GetPoolPDA(baseMint, quoteMint solana.PublicKey) solana.PublicKey {
	seeds := [][]byte{
		[]byte("pool"),
		baseMint[:],
		quoteMint[:],
	}
	pubkey, _, _ := solana.FindProgramAddress(seeds, constants.PUMPSWAP_PROGRAM_ID)
	return pubkey
}

// CreateAssociatedTokenAccountInstruction creates an instruction to create an ATA
func CreateAssociatedTokenAccountInstruction(
	payer, owner, mint solana.PublicKey,
	tokenProgram solana.PublicKey,
) solana.Instruction {
	ata := GetAssociatedTokenAddress(owner, mint, tokenProgram)

	data := make([]byte, 0)

	accounts := []solana.AccountMeta{
		{PublicKey: payer, IsSigner: true, IsWritable: true},
		{PublicKey: ata, IsSigner: false, IsWritable: true},
		{PublicKey: owner, IsSigner: false, IsWritable: false},
		{PublicKey: mint, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: constants.ASSOCIATED_TOKEN_PROGRAM_ID, IsSigner: false, IsWritable: false},
		{PublicKey: constants.RENT, IsSigner: false, IsWritable: false},
	}

	return solana.NewInstruction(constants.ASSOCIATED_TOKEN_PROGRAM_ID, accounts, data)
}

// BuildCloseAccountInstruction builds a close token account instruction
func BuildCloseAccountInstruction(
	tokenProgram, account, owner, destination solana.PublicKey,
) solana.Instruction {
	// Close account instruction discriminator
	data := []byte{151, 9, 59, 186, 208, 190, 183, 75}

	accounts := []solana.AccountMeta{
		{PublicKey: account, IsSigner: false, IsWritable: true},
		{PublicKey: destination, IsSigner: false, IsWritable: true},
		{PublicKey: owner, IsSigner: true, IsWritable: false},
	}

	return solana.NewInstruction(tokenProgram, accounts, data)
}

// Hash computes SHA256 hash
func Hash(data []byte) [32]byte {
	return sha256.Sum256(data)
}

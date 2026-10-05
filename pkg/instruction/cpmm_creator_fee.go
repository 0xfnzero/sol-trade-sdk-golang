package instruction

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"math/bits"
)

var cpmmConfigDisc = []byte{218, 244, 33, 104, 203, 203, 43, 111}
var cpmmShareDisc = []byte{30, 235, 98, 252, 26, 197, 66, 86}
var cpmmPoolDisc = []byte{247, 237, 227, 245, 215, 195, 222, 70}

func cpmmBytes(data []byte, size int, disc []byte) error {
	if len(data) < size || !bytes.Equal(data[:8], disc) {
		return fmt.Errorf("invalid CPMM layout/discriminator")
	}
	return nil
}

type CpmmAmmConfig struct {
	Bump                                                      uint8
	DisableCreatePool                                         bool
	Index                                                     uint16
	TradeFeeRate, ProtocolFeeRate, FundFeeRate, CreatePoolFee uint64
	ProtocolOwner, FundOwner                                  solana.PublicKey
	CreatorFeeRate, CreatorFeeShareRate                       uint64
	Padding                                                   [14]uint64
}

func DecodeCpmmAmmConfig(b []byte) (*CpmmAmmConfig, error) {
	if err := cpmmBytes(b, 236, cpmmConfigDisc); err != nil {
		return nil, err
	}
	if b[9] > 1 {
		return nil, fmt.Errorf("invalid config bool")
	}
	u := func(o int) uint64 { return binary.LittleEndian.Uint64(b[o : o+8]) }
	pk := func(o int) solana.PublicKey { var k solana.PublicKey; copy(k[:], b[o:o+32]); return k }
	c := &CpmmAmmConfig{Bump: b[8], DisableCreatePool: b[9] == 1, Index: binary.LittleEndian.Uint16(b[10:12]), TradeFeeRate: u(12), ProtocolFeeRate: u(20), FundFeeRate: u(28), CreatePoolFee: u(36), ProtocolOwner: pk(44), FundOwner: pk(76), CreatorFeeRate: u(108), CreatorFeeShareRate: u(116)}
	for i := range c.Padding {
		c.Padding[i] = u(124 + i*8)
	}
	return c, nil
}

type CpmmCreatorFeeShare struct {
	Bump               uint8
	Creator, AmmConfig solana.PublicKey
	ShareRate          uint64
	Padding            [8]uint64
}

func DecodeCpmmCreatorFeeShare(b []byte) (*CpmmCreatorFeeShare, error) {
	if err := cpmmBytes(b, 145, cpmmShareDisc); err != nil {
		return nil, err
	}
	s := &CpmmCreatorFeeShare{Bump: b[8], ShareRate: binary.LittleEndian.Uint64(b[73:81])}
	copy(s.Creator[:], b[9:41])
	copy(s.AmmConfig[:], b[41:73])
	for i := range s.Padding {
		s.Padding[i] = binary.LittleEndian.Uint64(b[81+i*8 : 89+i*8])
	}
	return s, nil
}

type CpmmCollectionPool struct {
	AmmConfig, PoolCreator, Token0Vault, Token1Vault, Token0Mint, Token1Mint, Token0Program, Token1Program solana.PublicKey
	CreatorFeesToken0, CreatorFeesToken1, ProtocolFeesToken0, ProtocolFeesToken1                           uint64
}

func DecodeCpmmCollectionPool(b []byte) (*CpmmCollectionPool, error) {
	if err := cpmmBytes(b, 637, cpmmPoolDisc); err != nil {
		return nil, err
	}
	if b[390] > 1 {
		return nil, fmt.Errorf("invalid pool bool")
	}
	pk := func(i int) solana.PublicKey { var k solana.PublicKey; copy(k[:], b[8+i*32:40+i*32]); return k }
	return &CpmmCollectionPool{AmmConfig: pk(0), PoolCreator: pk(1), Token0Vault: pk(2), Token1Vault: pk(3), Token0Mint: pk(5), Token1Mint: pk(6), Token0Program: pk(7), Token1Program: pk(8), CreatorFeesToken0: binary.LittleEndian.Uint64(b[397:405]), CreatorFeesToken1: binary.LittleEndian.Uint64(b[405:413]), ProtocolFeesToken0: binary.LittleEndian.Uint64(b[341:349]), ProtocolFeesToken1: binary.LittleEndian.Uint64(b[349:357])}, nil
}
func GetCreatorFeeSharePDA(creator, config solana.PublicKey) (solana.PublicKey, error) {
	p, _, err := solana.FindProgramAddress([][]byte{[]byte("creator_fee_share"), creator[:], config[:]}, RAYDIUM_CPMM_PROGRAM)
	return p, err
}
func ResolveCreatorFeeShareRate(config *CpmmAmmConfig, creator, address solana.PublicKey, share *rpc.Account) (uint64, error) {
	if config == nil {
		return 0, fmt.Errorf("missing config")
	}
	n := config.CreatorFeeShareRate
	if share != nil && share.Lamports != 0 && share.Owner == RAYDIUM_CPMM_PROGRAM && share.Data != nil && len(share.Data.GetBinary()) != 0 {
		s, err := DecodeCpmmCreatorFeeShare(share.Data.GetBinary())
		if err != nil {
			return 0, err
		}
		if s.Creator != creator || s.AmmConfig != address {
			return 0, fmt.Errorf("CreatorFeeShare creator/config mismatch")
		}
		n = s.ShareRate
	}
	if n > 1000000 {
		return 0, fmt.Errorf("creator fee share rate exceeds 1,000,000")
	}
	return n, nil
}

// Cold-path RPC: one confirmed snapshot for config and override, never on the preparation path.
func FetchCreatorFeeShareRate(ctx context.Context, client *rpc.Client, creator, config solana.PublicKey) (uint64, error) {
	p, err := GetCreatorFeeSharePDA(creator, config)
	if err != nil {
		return 0, err
	}
	r, err := client.GetMultipleAccountsWithOpts(ctx, []solana.PublicKey{config, p}, &rpc.GetMultipleAccountsOpts{Commitment: rpc.CommitmentConfirmed, Encoding: solana.EncodingBase64})
	if err != nil {
		return 0, err
	}
	if r == nil || len(r.Value) != 2 {
		return 0, fmt.Errorf("incomplete fee-share snapshot")
	}
	a := r.Value[0]
	if a == nil || a.Owner != RAYDIUM_CPMM_PROGRAM || a.Lamports == 0 || a.Data == nil {
		return 0, fmt.Errorf("missing/invalid CPMM config")
	}
	c, err := DecodeCpmmAmmConfig(a.Data.GetBinary())
	if err != nil {
		return 0, err
	}
	return ResolveCreatorFeeShareRate(c, creator, config, r.Value[1])
}

// Returns creator payout and protocol share; protocol rounds down, creator receives dust.
func SplitCreatorFee(gross, rate uint64) (uint64, uint64, error) {
	if rate > 1000000 {
		return 0, 0, fmt.Errorf("creator fee share rate exceeds 1,000,000")
	}
	hi, lo := bits.Mul64(gross, rate)
	protocol, _ := bits.Div64(hi, lo, 1000000)
	return gross - protocol, protocol, nil
}
func EstimateCreatorFeePayout(pool *CpmmCollectionPool, rate uint64) (uint64, uint64, error) {
	if pool == nil {
		return 0, 0, fmt.Errorf("missing pool")
	}
	a, _, err := SplitCreatorFee(pool.CreatorFeesToken0, rate)
	if err != nil {
		return 0, 0, err
	}
	b, _, err := SplitCreatorFee(pool.CreatorFeesToken1, rate)
	return a, b, err
}
func collectCpmmCreatorFee(address solana.PublicKey, pool *CpmmCollectionPool, payer *solana.PublicKey) (solana.Instruction, error) {
	if pool == nil {
		return nil, fmt.Errorf("missing pool")
	}
	share, err := GetCreatorFeeSharePDA(pool.PoolCreator, pool.AmmConfig)
	if err != nil {
		return nil, err
	}
	m := func(k solana.PublicKey, w, s bool) *solana.AccountMeta {
		return &solana.AccountMeta{PublicKey: k, IsWritable: w, IsSigner: s}
	}
	c := pool.PoolCreator
	keys := solana.AccountMetaSlice{}
	if payer != nil {
		keys = append(keys, m(*payer, true, true), m(c, false, false), m(RAYDIUM_CPMM_AUTHORITY, false, false), m(address, true, false))
	} else {
		keys = append(keys, m(c, true, true), m(RAYDIUM_CPMM_AUTHORITY, false, false), m(address, true, false), m(pool.AmmConfig, false, false))
	}
	keys = append(keys, m(pool.Token0Vault, true, false), m(pool.Token1Vault, true, false), m(pool.Token0Mint, false, false), m(pool.Token1Mint, false, false), m(GetAssociatedTokenAddress(c, pool.Token0Mint, pool.Token0Program), true, false), m(GetAssociatedTokenAddress(c, pool.Token1Mint, pool.Token1Program), true, false), m(pool.Token0Program, false, false), m(pool.Token1Program, false, false), m(constants.ASSOCIATED_TOKEN_PROGRAM_ID, false, false), m(solana.SystemProgramID, false, false))
	data := []byte{20, 22, 86, 123, 198, 28, 219, 132}
	if payer != nil {
		keys = append(keys, m(pool.AmmConfig, false, false))
		data = []byte{202, 202, 34, 83, 226, 122, 145, 229}
	}
	keys = append(keys, m(share, false, false))
	return solana.NewInstruction(RAYDIUM_CPMM_PROGRAM, keys, data), nil
}
func CollectCreatorFee(address solana.PublicKey, pool *CpmmCollectionPool) (solana.Instruction, error) {
	return collectCpmmCreatorFee(address, pool, nil)
}
func CollectCreatorFeePermissionless(payer, address solana.PublicKey, pool *CpmmCollectionPool) (solana.Instruction, error) {
	return collectCpmmCreatorFee(address, pool, &payer)
}

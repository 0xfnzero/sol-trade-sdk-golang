package instruction

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	dexparams "github.com/0xfnzero/sol-trade-sdk-golang/pkg/params"
	"github.com/gagliardetto/solana-go"
)

func testPK(seed byte) solana.PublicKey {
	var out solana.PublicKey
	for i := range out {
		out[i] = seed
	}
	return out
}

func testPumpFunParams(quoteMint solana.PublicKey) *PumpFunParams {
	return &PumpFunParams{
		BondingCurve: &BondingCurve{
			VirtualTokenReserves: 1_073_000_000_000_000,
			VirtualSolReserves:   30_000_000_000,
			RealTokenReserves:    793_100_000_000_000,
			Creator:              testPK(7),
		},
		CreatorVault: testPK(8),
		TokenProgram: constants.TOKEN_PROGRAM,
		QuoteMint:    quoteMint,
	}
}

func testPumpSwapParams(overrides func(*PumpSwapParams)) *PumpSwapParams {
	params := &PumpSwapParams{
		Pool:                      testPK(21),
		BaseMint:                  testPK(22),
		QuoteMint:                 constants.WSOL_TOKEN_ACCOUNT,
		PoolBaseTokenAccount:      testPK(23),
		PoolQuoteTokenAccount:     testPK(24),
		PoolBaseTokenReserves:     1_000_000_000_000,
		PoolQuoteTokenReserves:    4_500_000_000,
		CoinCreatorVaultAta:       testPK(25),
		CoinCreatorVaultAuthority: testPK(26),
		BaseTokenProgram:          constants.TOKEN_PROGRAM,
		QuoteTokenProgram:         constants.TOKEN_PROGRAM,
		CoinCreator:               testPK(27),
		CoinCreatorKnown:          true,
		FeeBasisPoints: &calc.PumpSwapFeeBasisPoints{
			LPFeeBasisPoints:          20,
			ProtocolFeeBasisPoints:    5,
			CoinCreatorFeeBasisPoints: 75,
		},
	}
	if overrides != nil {
		overrides(params)
	}
	return params
}

func TestRaydiumAmmV4UsesMarketAccountOrder(t *testing.T) {
	fixedOutput := uint64(42)
	ixs, err := RaydiumAmmV4BuildBuyInstructions(&RaydiumAmmV4BuildBuyParams{
		Payer:               testPK(99),
		OutputMint:          testPK(2),
		InputAmount:         100_000,
		SlippageBasisPoints: 300,
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		FixedOutputAmount:   &fixedOutput,
		ProtocolParams: &RaydiumAmmV4Params{
			Amm:                   testPK(1),
			CoinMint:              constants.WSOL_TOKEN_ACCOUNT,
			PcMint:                testPK(2),
			TokenCoin:             testPK(3),
			TokenPc:               testPK(4),
			AmmOpenOrders:         testPK(5),
			AmmTargetOrders:       testPK(6),
			SerumProgram:          testPK(7),
			SerumMarket:           testPK(8),
			SerumBids:             testPK(9),
			SerumAsks:             testPK(10),
			SerumEventQueue:       testPK(11),
			SerumCoinVaultAccount: testPK(12),
			SerumPcVaultAccount:   testPK(13),
			SerumVaultSigner:      testPK(14),
			CoinReserve:           1_000_000_000,
			PcReserve:             2_000_000_000,
		},
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	ix := ixs[len(ixs)-1]
	accounts := ix.Accounts()
	data, err := ix.Data()
	if err != nil {
		t.Fatalf("data error: %v", err)
	}

	if len(accounts) != 18 {
		t.Fatalf("accounts len = %d", len(accounts))
	}
	if data[0] != RaydiumAmmV4SwapBaseOutDiscriminator[0] {
		t.Fatalf("discriminator = %d", data[0])
	}
	if !accounts[3].PublicKey.Equals(testPK(5)) || !accounts[4].PublicKey.Equals(testPK(6)) {
		t.Fatalf("market order accounts not mapped")
	}
	if !accounts[7].PublicKey.Equals(testPK(7)) || !accounts[14].PublicKey.Equals(testPK(14)) {
		t.Fatalf("serum accounts not mapped")
	}
}

func TestRaydiumAmmV4RejectsBuyOutputMintMismatch(t *testing.T) {
	fixedOutput := uint64(42)
	_, err := RaydiumAmmV4BuildBuyInstructions(&RaydiumAmmV4BuildBuyParams{
		Payer:               testPK(99),
		OutputMint:          testPK(3),
		InputAmount:         100_000,
		SlippageBasisPoints: 300,
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		FixedOutputAmount:   &fixedOutput,
		ProtocolParams: &RaydiumAmmV4Params{
			Amm:                   testPK(1),
			CoinMint:              constants.WSOL_TOKEN_ACCOUNT,
			PcMint:                testPK(2),
			TokenCoin:             testPK(3),
			TokenPc:               testPK(4),
			AmmOpenOrders:         testPK(5),
			AmmTargetOrders:       testPK(6),
			SerumProgram:          testPK(7),
			SerumMarket:           testPK(8),
			SerumBids:             testPK(9),
			SerumAsks:             testPK(10),
			SerumEventQueue:       testPK(11),
			SerumCoinVaultAccount: testPK(12),
			SerumPcVaultAccount:   testPK(13),
			SerumVaultSigner:      testPK(14),
			CoinReserve:           1_000_000_000,
			PcReserve:             2_000_000_000,
		},
	})
	if err == nil {
		t.Fatalf("expected output mint mismatch error")
	}
}

func TestRaydiumCPMMUsesSwapBaseOutForFixedOutputBuy(t *testing.T) {
	fixedOutput := uint64(42)
	ixs, err := RaydiumCPMMBuildBuyInstructions(&RaydiumCPMMBuildBuyParams{
		Payer:               testPK(99),
		OutputMint:          testPK(2),
		InputAmount:         100_000,
		SlippageBasisPoints: 300,
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		FixedOutputAmount:   &fixedOutput,
		ProtocolParams: &RaydiumCPMMParams{
			AmmConfig:         testPK(1),
			BaseMint:          constants.WSOL_TOKEN_ACCOUNT,
			QuoteMint:         testPK(2),
			BaseReserve:       1_000_000_000,
			QuoteReserve:      2_000_000_000,
			BaseTokenProgram:  constants.TOKEN_PROGRAM,
			QuoteTokenProgram: constants.TOKEN_PROGRAM,
		},
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	data, err := ixs[len(ixs)-1].Data()
	if err != nil {
		t.Fatalf("data error: %v", err)
	}
	if string(data[:8]) != string(RaydiumCPMMSwapBaseOutDiscriminator) {
		t.Fatalf("expected swap_base_out discriminator")
	}
	if got := binary.LittleEndian.Uint64(data[8:16]); got != 100_000 {
		t.Fatalf("max input = %d", got)
	}
	if got := binary.LittleEndian.Uint64(data[16:24]); got != fixedOutput {
		t.Fatalf("amount out = %d", got)
	}
}

func TestGenericBuilderWiresInputAtaControl(t *testing.T) {
	fixedOutput := uint64(42)
	builder := NewRaydiumAmmV4InstructionBuilder()
	ixs, err := builder.BuildBuyInstructions(&BuildParams{
		Payer:               testPK(99),
		OutputMint:          testPK(2),
		InputAmount:         100_000,
		SlippageBasisPoints: 300,
		ProtocolParams: &dexparams.RaydiumAmmV4Params{
			Amm:                   testPK(1),
			CoinMint:              constants.WSOL_TOKEN_ACCOUNT,
			PcMint:                testPK(2),
			TokenCoin:             testPK(3),
			TokenPc:               testPK(4),
			AmmOpenOrders:         testPK(5),
			AmmTargetOrders:       testPK(6),
			SerumProgram:          testPK(7),
			SerumMarket:           testPK(8),
			SerumBids:             testPK(9),
			SerumAsks:             testPK(10),
			SerumEventQueue:       testPK(11),
			SerumCoinVaultAccount: testPK(12),
			SerumPcVaultAccount:   testPK(13),
			SerumVaultSigner:      testPK(14),
			CoinReserve:           1_000_000_000,
			PcReserve:             2_000_000_000,
		},
		CreateInputATA:    true,
		CreateOutputATA:   false,
		FixedOutputAmount: &fixedOutput,
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if len(ixs) <= 1 {
		t.Fatalf("expected input ATA/WSOL setup instructions")
	}
}

func TestMeteoraDammV2UsesSwap2PartialFill(t *testing.T) {
	fixedOutput := uint64(42)
	ixs, err := MeteoraDammV2BuildBuyInstructions(&MeteoraDammV2BuildBuyParams{
		Payer:               testPK(99),
		InputMint:           constants.WSOL_TOKEN_ACCOUNT,
		OutputMint:          testPK(2),
		InputAmount:         100_000,
		SlippageBasisPoints: 300,
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		FixedOutputAmount:   &fixedOutput,
		ProtocolParams: &MeteoraDammV2Params{
			Pool:          testPK(1),
			TokenAMint:    constants.WSOL_TOKEN_ACCOUNT,
			TokenBMint:    testPK(2),
			TokenAVault:   testPK(3),
			TokenBVault:   testPK(4),
			TokenAProgram: constants.TOKEN_PROGRAM,
			TokenBProgram: constants.TOKEN_PROGRAM,
		},
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	ix := ixs[len(ixs)-1]
	accounts := ix.Accounts()
	data, err := ix.Data()
	if err != nil {
		t.Fatalf("data error: %v", err)
	}

	if len(accounts) != 13 {
		t.Fatalf("accounts len = %d", len(accounts))
	}
	if string(data[:8]) != string(MeteoraDammV2Swap2Discriminator) {
		t.Fatalf("unexpected swap2 discriminator")
	}
	if data[24] != MeteoraDammV2SwapModePartialFill {
		t.Fatalf("swap mode = %d", data[24])
	}
	if !accounts[12].PublicKey.Equals(METEORA_DAMM_V2_PROGRAM) {
		t.Fatalf("program account not last")
	}
}

func TestPumpSwapBuyUsesFeeBasisPointsFromParams(t *testing.T) {
	current, err := BuildBuyInstructions(&BuildBuyParams{
		Payer:               testPK(99),
		InputAmount:         1_000_000,
		SlippageBasisPoints: 300,
		ProtocolParams:      testPumpSwapParams(nil),
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		UseExactQuoteAmount: true,
	})
	if err != nil {
		t.Fatalf("build current error: %v", err)
	}
	legacy, err := BuildBuyInstructions(&BuildBuyParams{
		Payer:               testPK(99),
		InputAmount:         1_000_000,
		SlippageBasisPoints: 300,
		ProtocolParams: testPumpSwapParams(func(p *PumpSwapParams) {
			p.FeeBasisPoints = &calc.PumpSwapFeeBasisPoints{
				LPFeeBasisPoints:          25,
				ProtocolFeeBasisPoints:    5,
				CoinCreatorFeeBasisPoints: 5,
			}
		}),
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		UseExactQuoteAmount: true,
	})
	if err != nil {
		t.Fatalf("build legacy error: %v", err)
	}
	currentData, _ := current[len(current)-1].Data()
	legacyData, _ := legacy[len(legacy)-1].Data()
	if string(currentData[:8]) != string(PUMPSWAP_BUY_EXACT_QUOTE_IN_DISCRIMINATOR) {
		t.Fatalf("expected buy_exact_quote_in discriminator")
	}
	if len(currentData) != 25 {
		t.Fatalf("buy_exact_quote_in data len = %d", len(currentData))
	}
	if binary.LittleEndian.Uint64(currentData[16:24]) == binary.LittleEndian.Uint64(legacyData[16:24]) {
		t.Fatalf("fee bps did not affect min output")
	}
}

func TestPumpSwapFixedOutputBuyUsesBuyDiscriminator(t *testing.T) {
	fixedOutput := uint64(123)
	ixs, err := BuildBuyInstructions(&BuildBuyParams{
		Payer:               testPK(99),
		InputAmount:         1_000_000,
		FixedOutputAmount:   &fixedOutput,
		SlippageBasisPoints: 300,
		ProtocolParams:      testPumpSwapParams(nil),
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		UseExactQuoteAmount: true,
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	data, err := ixs[len(ixs)-1].Data()
	if err != nil {
		t.Fatalf("data error: %v", err)
	}
	if string(data[:8]) != string(PUMPSWAP_BUY_DISCRIMINATOR) {
		t.Fatalf("expected buy discriminator")
	}
	if len(data) != 25 {
		t.Fatalf("buy data len = %d", len(data))
	}
	if got := binary.LittleEndian.Uint64(data[8:16]); got != fixedOutput {
		t.Fatalf("base amount = %d", got)
	}
	if data[24] != 0 {
		t.Fatalf("track volume = %d", data[24])
	}
}

func TestPumpSwapReverseSellUsesBuyTwoArgData(t *testing.T) {
	payer := testPK(99)
	quoteMint := testPK(22)
	ixs, err := BuildSellInstructions(&BuildSellParams{
		Payer:               payer,
		InputAmount:         1_000_000,
		SlippageBasisPoints: 300,
		ProtocolParams: testPumpSwapParams(func(p *PumpSwapParams) {
			p.BaseMint = constants.WSOL_TOKEN_ACCOUNT
			p.QuoteMint = quoteMint
		}),
		CreateOutputMintAta: false,
		CloseInputMintAta:   true,
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	data, err := ixs[len(ixs)-2].Data()
	if err != nil {
		t.Fatalf("data error: %v", err)
	}
	if string(data[:8]) != string(PUMPSWAP_BUY_DISCRIMINATOR) {
		t.Fatalf("expected buy discriminator for reverse sell")
	}
	if len(data) != 24 {
		t.Fatalf("reverse sell data len = %d", len(data))
	}
	closeIx := ixs[len(ixs)-1]
	userQuoteAta := GetAssociatedTokenAddress(payer, quoteMint, constants.TOKEN_PROGRAM)
	if !closeIx.Accounts()[0].PublicKey.Equals(userQuoteAta) {
		t.Fatalf("close input account = %s, want %s", closeIx.Accounts()[0].PublicKey, userQuoteAta)
	}
}

func TestPumpSwapOmitsPoolV2WhenKnownCoinCreatorIsDefault(t *testing.T) {
	baseMint := testPK(22)
	ixs, err := BuildBuyInstructions(&BuildBuyParams{
		Payer:               testPK(99),
		InputAmount:         1_000_000,
		SlippageBasisPoints: 300,
		ProtocolParams: testPumpSwapParams(func(p *PumpSwapParams) {
			p.BaseMint = baseMint
			p.CoinCreator = solana.PublicKey{}
			p.CoinCreatorKnown = true
		}),
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		UseExactQuoteAmount: true,
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	poolV2 := GetPoolV2PDA(baseMint)
	for _, meta := range ixs[len(ixs)-1].Accounts() {
		if meta.PublicKey.Equals(poolV2) {
			t.Fatalf("pool-v2 should be omitted for known default coin_creator")
		}
	}
}

type fakePumpSwapFetcher struct {
	accounts map[solana.PublicKey][]byte
	balances map[solana.PublicKey]uint64
}

func (f fakePumpSwapFetcher) GetAccountInfo(pubkey solana.PublicKey) ([]byte, error) {
	data, ok := f.accounts[pubkey]
	if !ok {
		return nil, fmt.Errorf("missing account %s", pubkey)
	}
	return data, nil
}

func (f fakePumpSwapFetcher) GetTokenAccountBalance(pubkey solana.PublicKey) (uint64, error) {
	balance, ok := f.balances[pubkey]
	if !ok {
		return 0, fmt.Errorf("missing balance %s", pubkey)
	}
	return balance, nil
}

func appendU64(data []byte, value uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, value)
	return append(data, buf...)
}

func appendU128(data []byte, value uint64) []byte {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint64(buf, value)
	return append(data, buf...)
}

func testFeeConfigBytes() []byte {
	data := make([]byte, 0, 8+1+32+24+4+2*40+4)
	data = append(data, make([]byte, 8)...)    // discriminator
	data = append(data, byte(1))               // bump
	data = append(data, testPK(55).Bytes()...) // admin
	data = appendU64(data, 30)
	data = appendU64(data, 7)
	data = appendU64(data, 9)
	data = append(data, []byte{2, 0, 0, 0}...)
	data = appendU128(data, 0)
	data = appendU64(data, 25)
	data = appendU64(data, 5)
	data = appendU64(data, 5)
	data = appendU128(data, 1_000)
	data = appendU64(data, 20)
	data = appendU64(data, 5)
	data = appendU64(data, 75)
	data = append(data, []byte{0, 0, 0, 0}...)
	return data
}

func testMintBytes(supply uint64) []byte {
	data := make([]byte, 82)
	binary.LittleEndian.PutUint64(data[36:44], supply)
	return data
}

func testPoolBytes(pool *PumpSwapPool) []byte {
	data := make([]byte, 0, 8+PoolSize)
	data = append(data, make([]byte, 8)...)
	data = append(data, pool.PoolBump)
	buf2 := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf2, pool.Index)
	data = append(data, buf2...)
	data = append(data, pool.Creator.Bytes()...)
	data = append(data, pool.BaseMint.Bytes()...)
	data = append(data, pool.QuoteMint.Bytes()...)
	data = append(data, pool.LpMint.Bytes()...)
	data = append(data, pool.PoolBaseTokenAccount.Bytes()...)
	data = append(data, pool.PoolQuoteTokenAccount.Bytes()...)
	data = appendU64(data, pool.LpSupply)
	data = append(data, pool.CoinCreator.Bytes()...)
	if pool.IsMayhemMode {
		data = append(data, byte(1))
	} else {
		data = append(data, byte(0))
	}
	if pool.IsCashbackCoin {
		data = append(data, byte(1))
	} else {
		data = append(data, byte(0))
	}
	data = append(data, make([]byte, 7)...)
	return data
}

func TestPumpSwapDecodeFeeConfigAndComputeTier(t *testing.T) {
	baseMint := testPK(31)
	config := DecodeFeeConfig(testFeeConfigBytes())
	if config == nil {
		t.Fatalf("fee config decode failed")
	}
	supply := uint64(10_000)
	fees := ComputePumpSwapFeeBasisPoints(
		config,
		GetPumpPoolAuthorityPDA(baseMint),
		baseMint,
		&supply,
		1_000,
		1_000,
	)
	if fees.LPFeeBasisPoints != 20 ||
		fees.ProtocolFeeBasisPoints != 5 ||
		fees.CoinCreatorFeeBasisPoints != 75 {
		t.Fatalf("fees = %+v", fees)
	}
}

func TestPumpSwapDecodeMintSupply(t *testing.T) {
	supply, ok := DecodeMintSupply(testMintBytes(123_456))
	if !ok || supply != 123_456 {
		t.Fatalf("supply = %d ok=%v", supply, ok)
	}
	if _, ok := DecodeMintSupply(make([]byte, 10)); ok {
		t.Fatalf("short mint decoded")
	}
}

func TestNewPumpSwapParamsFromPoolAddressDiscoversFeeConfig(t *testing.T) {
	baseMint := testPK(31)
	poolAddress := testPK(32)
	coinCreator := testPK(33)
	pool := &PumpSwapPool{
		PoolBump:              1,
		Index:                 0,
		Creator:               GetPumpPoolAuthorityPDA(baseMint),
		BaseMint:              baseMint,
		QuoteMint:             constants.WSOL_TOKEN_ACCOUNT,
		LpMint:                testPK(34),
		PoolBaseTokenAccount:  GetAssociatedTokenAddress(poolAddress, baseMint, constants.TOKEN_PROGRAM),
		PoolQuoteTokenAccount: GetAssociatedTokenAddress(poolAddress, constants.WSOL_TOKEN_ACCOUNT, constants.TOKEN_PROGRAM),
		LpSupply:              100,
		CoinCreator:           coinCreator,
		IsCashbackCoin:        true,
	}
	fetcher := fakePumpSwapFetcher{
		accounts: map[solana.PublicKey][]byte{
			poolAddress: testPoolBytes(pool),
			baseMint:    testMintBytes(10_000),
			FEE_CONFIG:  testFeeConfigBytes(),
		},
		balances: map[solana.PublicKey]uint64{
			pool.PoolBaseTokenAccount:  1_000,
			pool.PoolQuoteTokenAccount: 1_000,
		},
	}
	params, err := NewPumpSwapParamsFromPoolAddress(fetcher, poolAddress, nil)
	if err != nil {
		t.Fatalf("NewPumpSwapParamsFromPoolAddress error: %v", err)
	}
	if params.FeeBasisPoints == nil ||
		params.FeeBasisPoints.LPFeeBasisPoints != 20 ||
		params.FeeBasisPoints.ProtocolFeeBasisPoints != 5 ||
		params.FeeBasisPoints.CoinCreatorFeeBasisPoints != 75 {
		t.Fatalf("fees = %+v", params.FeeBasisPoints)
	}
	if params.BaseMintSupply == nil || *params.BaseMintSupply != 10_000 {
		t.Fatalf("base mint supply = %+v", params.BaseMintSupply)
	}
	if !params.CoinCreatorVaultAuthority.Equals(GetCoinCreatorVaultAuthority(coinCreator)) {
		t.Fatalf("coin creator vault authority mismatch")
	}
	if !params.CoinCreatorVaultAta.Equals(GetCoinCreatorVaultAta(coinCreator, constants.WSOL_TOKEN_ACCOUNT)) {
		t.Fatalf("coin creator vault ata mismatch")
	}
}

func TestNewPumpSwapParamsFromPoolAddressPreservesManualFees(t *testing.T) {
	baseMint := testPK(35)
	poolAddress := testPK(36)
	pool := &PumpSwapPool{
		PoolBump:              1,
		Index:                 0,
		Creator:               GetPumpPoolAuthorityPDA(baseMint),
		BaseMint:              baseMint,
		QuoteMint:             constants.WSOL_TOKEN_ACCOUNT,
		LpMint:                testPK(37),
		PoolBaseTokenAccount:  GetAssociatedTokenAddress(poolAddress, baseMint, constants.TOKEN_PROGRAM),
		PoolQuoteTokenAccount: GetAssociatedTokenAddress(poolAddress, constants.WSOL_TOKEN_ACCOUNT, constants.TOKEN_PROGRAM),
		LpSupply:              100,
		CoinCreator:           testPK(38),
	}
	manual := &calc.PumpSwapFeeBasisPoints{
		LPFeeBasisPoints:          99,
		ProtocolFeeBasisPoints:    88,
		CoinCreatorFeeBasisPoints: 77,
	}
	fetcher := fakePumpSwapFetcher{
		accounts: map[solana.PublicKey][]byte{
			poolAddress: testPoolBytes(pool),
			baseMint:    testMintBytes(10_000),
			FEE_CONFIG:  testFeeConfigBytes(),
		},
		balances: map[solana.PublicKey]uint64{
			pool.PoolBaseTokenAccount:  1_000,
			pool.PoolQuoteTokenAccount: 1_000,
		},
	}
	params, err := NewPumpSwapParamsFromPoolAddress(fetcher, poolAddress, manual)
	if err != nil {
		t.Fatalf("NewPumpSwapParamsFromPoolAddress error: %v", err)
	}
	if params.FeeBasisPoints == nil ||
		params.FeeBasisPoints.LPFeeBasisPoints != 99 ||
		params.FeeBasisPoints.ProtocolFeeBasisPoints != 88 ||
		params.FeeBasisPoints.CoinCreatorFeeBasisPoints != 77 {
		t.Fatalf("fees = %+v", params.FeeBasisPoints)
	}
}

func TestMeteoraDammV2AcceptsSolAliasForWsolInput(t *testing.T) {
	fixedOutput := uint64(42)
	ixs, err := MeteoraDammV2BuildBuyInstructions(&MeteoraDammV2BuildBuyParams{
		Payer:               testPK(99),
		InputMint:           constants.SOL_TOKEN_ACCOUNT,
		OutputMint:          testPK(2),
		InputAmount:         100_000,
		SlippageBasisPoints: 300,
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		FixedOutputAmount:   &fixedOutput,
		ProtocolParams: &MeteoraDammV2Params{
			Pool:          testPK(1),
			TokenAMint:    constants.WSOL_TOKEN_ACCOUNT,
			TokenBMint:    testPK(2),
			TokenAVault:   testPK(3),
			TokenBVault:   testPK(4),
			TokenAProgram: constants.TOKEN_PROGRAM,
			TokenBProgram: constants.TOKEN_PROGRAM,
		},
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	accounts := ixs[len(ixs)-1].Accounts()
	if !accounts[6].PublicKey.Equals(constants.WSOL_TOKEN_ACCOUNT) {
		t.Fatalf("token A mint not normalized to WSOL")
	}
}

func TestPumpFunV2BuyUsesCurrentAccountLayout(t *testing.T) {
	ixs, err := PumpFunBuildBuyInstructions(&PumpFunBuildBuyParams{
		Payer:               testPK(99),
		InputMint:           constants.USDC_TOKEN_ACCOUNT,
		OutputMint:          testPK(2),
		InputAmount:         100_000,
		SlippageBasisPoints: 300,
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		ProtocolParams:      testPumpFunParams(constants.USDC_TOKEN_ACCOUNT),
		UseExactSolAmount:   true,
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	accounts := ixs[len(ixs)-1].Accounts()
	if len(accounts) != 27 {
		t.Fatalf("accounts len = %d", len(accounts))
	}
	if !accounts[16].PublicKey.Equals(testPK(8)) {
		t.Fatalf("creator vault account not at #16")
	}
	if accounts[18].IsWritable {
		t.Fatalf("sharing config should be readonly")
	}
}

func TestPumpFunV2FixedOutputUsesBuyV2(t *testing.T) {
	fixedOutput := uint64(42)
	ixs, err := PumpFunBuildBuyInstructions(&PumpFunBuildBuyParams{
		Payer:               testPK(99),
		InputMint:           constants.SOL_TOKEN_ACCOUNT,
		OutputMint:          testPK(2),
		InputAmount:         100_000,
		SlippageBasisPoints: 300,
		FixedOutputAmount:   &fixedOutput,
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		ProtocolParams:      testPumpFunParams(constants.WSOL_TOKEN_ACCOUNT),
		UseExactSolAmount:   true,
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	data, err := ixs[len(ixs)-1].Data()
	if err != nil {
		t.Fatalf("data error: %v", err)
	}
	if string(data[:8]) != string(PumpFunBuyV2Discriminator) {
		t.Fatalf("unexpected discriminator")
	}
	if got := binary.LittleEndian.Uint64(data[8:16]); got != fixedOutput {
		t.Fatalf("token amount = %d", got)
	}
	if got := binary.LittleEndian.Uint64(data[16:24]); got != 100_000 {
		t.Fatalf("max quote = %d", got)
	}
}

func TestPumpFunV2RegularWsolBuyWrapsMaxQuoteBudget(t *testing.T) {
	useExact := false
	ixs, err := PumpFunBuildBuyInstructions(&PumpFunBuildBuyParams{
		Payer:               testPK(99),
		InputMint:           constants.SOL_TOKEN_ACCOUNT,
		OutputMint:          testPK(2),
		InputAmount:         100_000,
		SlippageBasisPoints: 1000,
		CreateInputMintAta:  true,
		CreateOutputMintAta: false,
		ProtocolParams:      testPumpFunParams(constants.WSOL_TOKEN_ACCOUNT),
		UseExactSolAmount:   useExact,
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if len(ixs) < 4 {
		t.Fatalf("expected WSOL create/transfer/sync plus swap, got %d instructions", len(ixs))
	}
	if !ixs[1].ProgramID().Equals(constants.SYSTEM_PROGRAM) {
		t.Fatalf("expected system transfer as second instruction")
	}
}

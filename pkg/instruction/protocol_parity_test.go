package instruction

import (
	"encoding/binary"
	"testing"

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

package instruction

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/params"
	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
)

type reportedFailure struct {
	InstructionAccounts []string `json:"instruction_accounts"`
	InstructionData     string   `json:"instruction_data_base58"`
	CreatorATA          string   `json:"correct_creator_vault_ata"`
	CreatorAuthority    string   `json:"correct_creator_vault_authority"`
	Accounts            []struct {
		Address string `json:"address"`
		Data    string `json:"data_base64"`
	} `json:"accounts"`
}

func reportedPool(t *testing.T) (*PumpSwapParams, reportedFailure, solana.PublicKey) {
	t.Helper()
	raw, err := os.ReadFile("testdata/pumpswap_reported_failure.json")
	if err != nil {
		t.Fatal(err)
	}
	var f reportedFailure
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	key := func(i int) solana.PublicKey { return solana.MustPublicKeyFromBase58(f.InstructionAccounts[i]) }
	data := map[solana.PublicKey][]byte{}
	for _, a := range f.Accounts {
		b, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			t.Fatal(e)
		}
		data[solana.MustPublicKeyFromBase58(a.Address)] = b
	}
	state := DecodePoolAccount(data[key(0)])
	if state == nil {
		t.Fatal("invalid captured Pool")
	}
	fee, extra := key(9), key(24)
	pool := &PumpSwapParams{
		Pool: key(0), BaseMint: key(3), QuoteMint: key(4), PoolBaseTokenAccount: key(7), PoolQuoteTokenAccount: key(8),
		PoolBaseTokenReserves: binary.LittleEndian.Uint64(data[key(7)][64:72]), PoolQuoteTokenReserves: binary.LittleEndian.Uint64(data[key(8)][64:72]),
		VirtualQuoteReserves: state.VirtualQuoteReserves, BaseTokenProgram: key(11), QuoteTokenProgram: key(12),
		CoinCreator: state.CoinCreator, CoinCreatorKnown: true,
		CoinCreatorVaultAuthority:    GetCoinCreatorVaultAuthority(state.CoinCreator),
		CoinCreatorVaultAta:          GetCoinCreatorVaultAtaWithTokenProgram(state.CoinCreator, key(4), key(12)),
		ProtocolFeeRecipientOverride: &fee, ProtocolExtraFeeRecipientOverride: &extra,
	}
	wallet := solana.PrivateKey(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, 32)))
	return pool, f, wallet.PublicKey()
}

func TestPumpSwapReportedFailureOriginalBuy(t *testing.T) {
	p, f, payer := reportedPool(t)
	original, err := base58.Decode(f.InstructionData)
	if err != nil {
		t.Fatal(err)
	}
	amount := binary.LittleEndian.Uint64(original[8:16])
	ixs, err := BuildBuyInstructions(&BuildBuyParams{Payer: payer, ProtocolParams: p, InputMint: p.QuoteMint, OutputMint: p.BaseMint,
		InputAmount: binary.LittleEndian.Uint64(original[16:24]), FixedOutputAmount: &amount, SlippageBasisPoints: 100})
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := ixs[0].Data()
	if !bytes.Equal(wire, original) {
		t.Fatal("historical Buy payload changed")
	}
	if f.InstructionAccounts[17] != (solana.PublicKey{}).String() || f.InstructionAccounts[18] != (solana.PublicKey{}).String() {
		t.Fatal("not the original default creator fields")
	}
	keys := ixs[0].Accounts()
	if keys[17].PublicKey.String() != f.CreatorATA || keys[18].PublicKey.String() != f.CreatorAuthority {
		t.Fatal("creator accounts differ from independently captured vault")
	}
	for _, i := range []int{0, 3, 4, 7, 8, 11, 12} {
		if keys[i].PublicKey.String() != f.InstructionAccounts[i] {
			t.Fatalf("account %d mismatch", i)
		}
	}
}

func TestPumpSwapReportedFailureGuards(t *testing.T) {
	mutations := []func(*PumpSwapParams){
		func(p *PumpSwapParams) { p.CoinCreatorVaultAta = solana.PublicKey{} },
		func(p *PumpSwapParams) { p.CoinCreatorVaultAuthority = solana.PublicKey{} },
		func(p *PumpSwapParams) {
			p.CoinCreatorVaultAta = solana.PublicKey{}
			p.CoinCreatorVaultAuthority = solana.PublicKey{}
		},
		func(p *PumpSwapParams) {
			p.CoinCreatorVaultAta = GetCoinCreatorVaultAtaWithTokenProgram(p.CoinCreator, p.QuoteMint, constants.TOKEN_PROGRAM_2022)
		},
		func(p *PumpSwapParams) { p.PoolQuoteTokenAccount = p.PoolBaseTokenAccount },
		func(p *PumpSwapParams) {
			p.CoinCreator = solana.PublicKey{}
			p.CoinCreatorKnown = false
			p.CoinCreatorVaultAuthority = GetCoinCreatorVaultAuthority(p.CoinCreator)
			p.CoinCreatorVaultAta = GetCoinCreatorVaultAtaWithTokenProgram(p.CoinCreator, p.QuoteMint, p.QuoteTokenProgram)
		},
		func(p *PumpSwapParams) { p.ProtocolFeeRecipientOverride = &solana.PublicKey{} },
		func(p *PumpSwapParams) { p.ProtocolExtraFeeRecipientOverride = &solana.PublicKey{} },
	}
	for i, mutate := range mutations {
		p, _, payer := reportedPool(t)
		mutate(p)
		_, a := BuildBuyInstructions(&BuildBuyParams{Payer: payer, ProtocolParams: p, InputAmount: 10000, SlippageBasisPoints: 100})
		_, b := BuildSellInstructions(&BuildSellParams{Payer: payer, ProtocolParams: p, InputAmount: 10000, SlippageBasisPoints: 100})
		if a == nil || b == nil || !strings.Contains(a.Error(), "PumpSwap") {
			t.Fatalf("accepted malformed case %d: %v / %v", i, a, b)
		}
	}
	p, _, _ := reportedPool(t)
	p.CoinCreator = solana.PublicKey{}
	p.CoinCreatorKnown = true
	p.CoinCreatorVaultAuthority = GetCoinCreatorVaultAuthority(p.CoinCreator)
	p.CoinCreatorVaultAta = GetCoinCreatorVaultAtaWithTokenProgram(p.CoinCreator, p.QuoteMint, p.QuoteTokenProgram)
	if e := p.ValidateAccounts(); e != nil {
		t.Fatal("known zero creator must remain supported", e)
	}
}

func TestPumpSwapReportedFailureCurrentRecipientsAndExport(t *testing.T) {
	for _, mayhem := range []bool{false, true} {
		p, _, payer := reportedPool(t)
		p.IsMayhemMode = mayhem
		buy, e := BuildBuyInstructions(&BuildBuyParams{Payer: payer, ProtocolParams: p, InputAmount: 10000, SlippageBasisPoints: 100, UseExactQuoteAmount: true})
		if e != nil {
			t.Fatal(e)
		}
		sell, e := BuildSellInstructions(&BuildSellParams{Payer: payer, ProtocolParams: p, InputAmount: 10000, SlippageBasisPoints: 100})
		if e != nil {
			t.Fatal(e)
		}
		for _, ix := range []solana.Instruction{buy[0], sell[0]} {
			keys := ix.Accounts()
			if keys[9].PublicKey != *p.ProtocolFeeRecipientOverride || keys[len(keys)-2].PublicKey != *p.ProtocolExtraFeeRecipientOverride {
				t.Fatal("lost explicit current fee recipient")
			}
		}
		// The high-level adapter must not drop caller-selected recipients.
		converted := toPumpSwapParams(&params.PumpSwapParams{ProtocolFeeRecipientOverride: p.ProtocolFeeRecipientOverride, ProtocolExtraFeeRecipientOverride: p.ProtocolExtraFeeRecipientOverride})
		if converted.ProtocolFeeRecipientOverride != p.ProtocolFeeRecipientOverride || converted.ProtocolExtraFeeRecipientOverride != p.ProtocolExtraFeeRecipientOverride {
			t.Fatal("adapter lost recipient overrides")
		}
	}
	p, _, payer := reportedPool(t)
	amount := uint64(1000000)
	ixs, e := BuildBuyInstructions(&BuildBuyParams{Payer: payer, ProtocolParams: p, InputMint: p.QuoteMint, OutputMint: p.BaseMint, InputAmount: amount, FixedOutputAmount: &amount, SlippageBasisPoints: 100})
	if e != nil {
		t.Fatal(e)
	}
	sell, e := BuildSellInstructions(&BuildSellParams{Payer: payer, ProtocolParams: p, InputMint: p.QuoteMint, OutputMint: p.BaseMint, InputAmount: amount, FixedOutputAmount: &amount, SlippageBasisPoints: 100})
	if e != nil {
		t.Fatal(e)
	}
	data, _ := ixs[0].Data()
	other, _ := sell[0].Data()
	if !bytes.Equal(data, other) {
		t.Fatal("API label changed Buy payload")
	}
	// Pump AMM IDL marks global_volume_accumulator readonly in both Buy forms.
	if ixs[0].Accounts()[19].IsWritable || sell[0].Accounts()[19].IsWritable {
		t.Fatal("global volume accumulator must be readonly")
	}
	exactIn, e := BuildBuyInstructions(&BuildBuyParams{Payer: payer, ProtocolParams: p, InputMint: p.QuoteMint, OutputMint: p.BaseMint, InputAmount: amount, SlippageBasisPoints: 100})
	if e != nil {
		t.Fatal(e)
	}
	if exactIn[0].Accounts()[19].IsWritable {
		t.Fatal("exact-quote-in global volume accumulator must be readonly")
	}
	if directory := os.Getenv("PUMPSWAP_FIXTURE_EXPORT_DIR"); directory != "" {
		accounts := []map[string]any{}
		for _, a := range ixs[0].Accounts() {
			accounts = append(accounts, map[string]any{"pubkey": a.PublicKey.String(), "is_signer": a.IsSigner, "is_writable": a.IsWritable})
		}
		raw, e := json.MarshalIndent(map[string]any{"program": ixs[0].ProgramID().String(), "payer": payer.String(), "data_base64": base64.StdEncoding.EncodeToString(data), "accounts": accounts}, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.MkdirAll(directory, 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(directory, "golang.json"), raw, 0644); e != nil {
			t.Fatal(e)
		}
	}
}

package trading

// PumpSwap preparation binds an independent trade to a frozen subscription
// snapshot. No account discovery, RPC, simulation or submission occurs here.
import (
	"errors"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/serialization"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/subscription"
	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
)

type PreparedPumpSwapTrade struct {
	route         subscription.PreparedCachedRoute
	request       CachedTradeRequest
	minimumOutput uint64
	steps         []PumpSwapQuotedStep
}
type PumpSwapQuotedStep struct {
	subscription.PoolTradeHint
	AmountIn, MinimumNetAmountOut uint64
}

func (p *PreparedPumpSwapTrade) MinimumOutput() uint64 { return p.minimumOutput }
func (p *PreparedPumpSwapTrade) Steps() []PumpSwapQuotedStep {
	return append([]PumpSwapQuotedStep(nil), p.steps...)
}

func (p *PreparedPumpSwapTrade) ValidateFreshness(current subscription.CacheReadContext) error {
	if p == nil || p.request.Snapshot == nil {
		return errors.New("missing prepared PumpSwap trade")
	}
	original := p.request.Context
	age := min(original.MaximumSlotAge, current.MaximumSlotAge)
	if current.Epoch != original.Epoch || current.Slot < original.Slot || current.Slot-original.Slot > age {
		return errors.New("prepared PumpSwap quote expired; refresh snapshot and prepare again")
	}
	return p.request.Snapshot.AssertUsable()
}

func canonicalPumpMint(m solana.PublicKey) solana.PublicKey {
	if m == constants.SOL_TOKEN_ACCOUNT {
		return constants.WSOL_TOKEN_ACCOUNT
	}
	return m
}

// PreparePumpSwapTrade quotes explicit ordered hints using preceding protected
// credits. Amounts are raw token units; slippage is applied separately per hop.
func PreparePumpSwapTrade(r CachedTradeRequest) (*PreparedPumpSwapTrade, error) {
	return preparePumpSwapTradeWithLimits(r, nil)
}
func preparePumpSwapTradeWithLimits(r CachedTradeRequest, limits *subscription.BridgeLimits) (*PreparedPumpSwapTrade, error) {
	if r.Snapshot == nil || r.Payer.IsZero() || r.Amount == 0 || len(r.Hints) == 0 || r.FixedOutputAmount != nil || (r.TradeType != soltradesdk.TradeTypeBuy && r.TradeType != soltradesdk.TradeTypeSell) {
		return nil, errors.New("invalid exact-input PumpSwap preparation request")
	}
	r = r.withResolvedPumpFunVersion()
	r.Hints = append([]subscription.PoolTradeHint(nil), r.Hints...)
	for i := range r.Hints {
		r.Hints[i].InputMint = canonicalPumpMint(r.Hints[i].InputMint)
		r.Hints[i].OutputMint = canonicalPumpMint(r.Hints[i].OutputMint)
	}
	if canonicalPumpMint(r.InputMint) != r.Hints[0].InputMint || canonicalPumpMint(r.OutputMint) != r.Hints[len(r.Hints)-1].OutputMint {
		return nil, errors.New("PumpSwap route endpoints do not match request")
	}
	index := 0
	if r.TradeType == soltradesdk.TradeTypeBuy {
		index = len(r.Hints) - 1
	}
	if _, err := r.Snapshot.Get(r.Hints[index].Pool, r.Context, &instruction.PUMPSWAP_PROGRAM); err != nil {
		return nil, err
	}
	slip := uint16(100)
	if r.SlippageBps != nil {
		slip = *r.SlippageBps
	}
	if slip >= 10000 {
		return nil, errors.New("invalid slippage")
	}
	arrays := r.MaximumArrays
	if arrays == 0 {
		arrays = 8
	}
	route, err := r.Snapshot.PrepareRouteWithBridgeLimits(r.Hints, r.Context, r.UnixTimestamp, r.Payer, r.Amount, slip, arrays, limits)
	if err != nil {
		return nil, err
	}
	// Copy pointer configuration so later edits to the caller's request cannot
	// silently change the prepared quote's slippage or transaction policy.
	r.SlippageBps = &slip
	r.Candidates = nil
	r.DexType = soltradesdk.DexTypePumpSwap
	if r.V1Config != nil {
		c := *r.V1Config
		if c.PriorityFee != nil {
			v := *c.PriorityFee
			c.PriorityFee = &v
		}
		if c.ComputeUnitLimit != nil {
			v := *c.ComputeUnitLimit
			c.ComputeUnitLimit = &v
		}
		if c.LoadedAccountsDataSizeLimit != nil {
			v := *c.LoadedAccountsDataSizeLimit
			c.LoadedAccountsDataSizeLimit = &v
		}
		if c.HeapSize != nil {
			v := *c.HeapSize
			c.HeapSize = &v
		}
		r.V1Config = &c
	}
	r.NativeInput = r.InputMint == constants.SOL_TOKEN_ACCOUNT
	r.NativeOutput = r.OutputMint == constants.SOL_TOKEN_ACCOUNT
	p := &PreparedPumpSwapTrade{request: r, route: route, minimumOutput: route.MinimumNetAmountOut}
	for _, leg := range route.Legs {
		p.steps = append(p.steps, PumpSwapQuotedStep{leg.Hint, leg.AmountIn, leg.MinimumNetAmountOut})
	}
	return p, nil
}

// BuildTransaction retains the prepared request's hash, native settlement, tip,
// and V1 priority settings. V1 uses its existing 4096-byte transport limit.
func (p *PreparedPumpSwapTrade) BuildTransaction(current subscription.CacheReadContext, signers []solana.PrivateKey) ([]byte, error) {
	return p.BuildV1Transaction(current, signers, PumpSwapNonce{})
}

type PumpSwapNonce struct {
	Account, Authority solana.PublicKey
	Hash               solana.Hash
}

// BuildV1Transaction supports a caller-cached durable nonce. V1 encodes
// priority settings in its header; the advance instruction remains first.
func (p *PreparedPumpSwapTrade) BuildV1Transaction(current subscription.CacheReadContext, signers []solana.PrivateKey, nonce PumpSwapNonce) ([]byte, error) {
	if err := p.ValidateFreshness(current); err != nil {
		return nil, err
	}
	built, err := finalizeCachedRoute(p.request, p.route, false, false)
	if err != nil {
		return nil, err
	}
	blockhash := p.request.RecentBlockhash
	if !nonce.Account.IsZero() || !nonce.Authority.IsZero() || nonce.Hash != (solana.Hash{}) {
		if nonce.Account.IsZero() || nonce.Authority.IsZero() || nonce.Hash == (solana.Hash{}) {
			return nil, errors.New("incomplete cached durable nonce")
		}
		built.Instructions = append([]solana.Instruction{system.NewAdvanceNonceAccountInstruction(nonce.Account, solana.SysVarRecentBlockHashesPubkey, nonce.Authority).Build()}, built.Instructions...)
		blockhash = nonce.Hash
	}
	built.Compiled, err = compileCachedV1(p.request, built.Instructions, blockhash)
	if err != nil {
		return nil, err
	}
	required := built.Compiled.AccountKeys[:built.Compiled.RequiredSignatures]
	for _, signer := range signers {
		if len(signer) != 64 {
			return nil, errors.New("invalid Ed25519 signer")
		}
		found := false
		for _, key := range required {
			if key == signer.PublicKey() {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("extraneous prepared trade signer")
		}
	}
	return serialization.SignV1Transaction(built.Compiled, signers)
}

type PumpSwapV0Options struct {
	AddressLookupTables          map[solana.PublicKey]solana.PublicKeySlice
	GasConfig                    *GasFeeConfig
	NonceAccount, NonceAuthority solana.PublicKey
	NonceHash                    solana.Hash
}

// BuildV0Transaction uses the native Solana compiler and supplied cached ALTs.
// Durable nonce advance stays first; oversized V0 packets fail instead of
// silently switching versions. V0 gas settings are explicit in options.
func (p *PreparedPumpSwapTrade) BuildV0Transaction(current subscription.CacheReadContext, signers []solana.PrivateKey, options PumpSwapV0Options) ([]byte, error) {
	if err := p.ValidateFreshness(current); err != nil {
		return nil, err
	}
	if p.request.V1Config != nil {
		return nil, errors.New("V1 configuration cannot be applied to V0; prepare without V1Config and supply V0 GasConfig")
	}
	built, err := finalizeCachedRoute(p.request, p.route, false, false)
	if err != nil {
		return nil, err
	}
	instructions := []solana.Instruction{}
	blockhash := p.request.RecentBlockhash
	if !options.NonceAccount.IsZero() || !options.NonceAuthority.IsZero() || options.NonceHash != (solana.Hash{}) {
		if options.NonceAccount.IsZero() || options.NonceAuthority.IsZero() || options.NonceHash == (solana.Hash{}) {
			return nil, errors.New("incomplete cached durable nonce")
		}
		instructions = append(instructions, system.NewAdvanceNonceAccountInstruction(options.NonceAccount, solana.SysVarRecentBlockHashesPubkey, options.NonceAuthority).Build())
		blockhash = options.NonceHash
	}
	if g := options.GasConfig; g != nil {
		if g.PriorityFee != 0 {
			return nil, errors.New("V0 GasConfig.PriorityFee is ambiguous; set ComputeUnitPrice and request TipLamports explicitly")
		}
		if g.ComputeUnitLimit > 1400000 || g.ComputeUnitLimit == 0 {
			return nil, errors.New("invalid V0 compute unit limit")
		}
		limit, e := computebudget.NewSetComputeUnitLimitInstruction(uint32(g.ComputeUnitLimit)).ValidateAndBuild()
		if e != nil {
			return nil, e
		}
		instructions = append(instructions, limit)
		if g.ComputeUnitPrice != 0 {
			price, e := computebudget.NewSetComputeUnitPriceInstruction(g.ComputeUnitPrice).ValidateAndBuild()
			if e != nil {
				return nil, e
			}
			instructions = append(instructions, price)
		}
	}
	instructions = append(instructions, built.Instructions...)
	// solana-go's compiler coalesces permissions by mutating AccountMeta
	// pointers. Keep those writes local so prepared V0/V1 builds can overlap.
	for i, ix := range instructions {
		data, e := ix.Data()
		if e != nil {
			return nil, e
		}
		metas := ix.Accounts()
		owned := make(solana.AccountMetaSlice, len(metas))
		for j, m := range metas {
			copy := *m
			owned[j] = &copy
		}
		instructions[i] = solana.NewInstruction(ix.ProgramID(), owned, append([]byte(nil), data...))
	}
	tx, err := solana.NewTransaction(instructions, blockhash, solana.TransactionPayer(p.request.Payer), solana.TransactionAddressTables(options.AddressLookupTables))
	if err != nil {
		return nil, err
	}
	tx.Message.SetVersion(solana.MessageVersionV0)
	required := tx.Message.AccountKeys[:tx.Message.Header.NumRequiredSignatures]
	for _, s := range signers {
		if len(s) != 64 {
			return nil, errors.New("invalid Ed25519 signer")
		}
		found := false
		for _, k := range required {
			if k == s.PublicKey() {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("extraneous V0 signer")
		}
	}
	if _, err = tx.Sign(func(k solana.PublicKey) *solana.PrivateKey {
		for i := range signers {
			if signers[i].PublicKey() == k {
				return &signers[i]
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err = tx.VerifySignatures(); err != nil {
		return nil, err
	}
	wire, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if len(wire) > 1232 {
		return nil, errors.New("V0 exceeds 1232 bytes; provide cached ALTs or choose V1 explicitly")
	}
	return wire, nil
}

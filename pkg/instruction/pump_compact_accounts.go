package instruction

import (
	"fmt"
	"github.com/gagliardetto/solana-go"
)

var compactPump = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
var compactAMM = solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
var compactFees = solana.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ")
var compactATA = solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
var compactWSOL = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")

func compactNormalizeQuote(mint solana.PublicKey) solana.PublicKey {
	if mint == (solana.PublicKey{}) || mint == solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111111") {
		return compactWSOL
	}
	return mint
}
func compactNormalizeHop(hop PumpMultiHop) PumpMultiHop {
	hop.QuoteMint = compactNormalizeQuote(hop.QuoteMint)
	if hop.QuoteMint == compactWSOL {
		hop.QuoteTokenProgram = solana.TokenProgramID
	}
	return hop
}
func compactPDA(program solana.PublicKey, seed string, key *solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{[]byte(seed)}
	if key != nil {
		seeds = append(seeds, key[:])
	}
	k, _, e := solana.FindProgramAddress(seeds, program)
	return k, e
}
func compactAta(owner, mint, token solana.PublicKey) (solana.PublicKey, error) {
	k, _, e := solana.FindProgramAddress([][]byte{owner[:], token[:], mint[:]}, compactATA)
	return k, e
}

type PumpCompactAccountParams struct {
	User, BaseMint, QuoteMint, BaseTokenProgram, QuoteTokenProgram, BuybackRecipient solana.PublicKey
	Cashback, Complete                                                               bool
}

func DerivePumpV3Accounts(p PumpCompactAccountParams) (map[string]solana.PublicKey, error) {
	p.QuoteMint = compactNormalizeQuote(p.QuoteMint)
	if p.Cashback {
		return nil, fmt.Errorf("cashback requires legacy trades")
	}
	if p.Complete {
		return nil, fmt.Errorf("BondingCurveComplete")
	}
	curve, err := compactPDA(compactPump, "bonding-curve", &p.BaseMint)
	if err != nil {
		return nil, err
	}
	accounts := make(map[string]solana.PublicKey)
	accounts["base_mint"] = p.BaseMint
	accounts["quote_mint"] = p.QuoteMint
	accounts["base_token_program"] = p.BaseTokenProgram
	accounts["quote_token_program"] = p.QuoteTokenProgram
	accounts["bonding_curve"] = curve
	vassociated_base_bonding_curve, eassociated_base_bonding_curve := compactAta(curve, p.BaseMint, p.BaseTokenProgram)
	if eassociated_base_bonding_curve != nil {
		return nil, eassociated_base_bonding_curve
	}
	accounts["associated_base_bonding_curve"] = vassociated_base_bonding_curve
	vassociated_quote_bonding_curve, eassociated_quote_bonding_curve := compactAta(curve, p.QuoteMint, p.QuoteTokenProgram)
	if eassociated_quote_bonding_curve != nil {
		return nil, eassociated_quote_bonding_curve
	}
	accounts["associated_quote_bonding_curve"] = vassociated_quote_bonding_curve
	accounts["user"] = p.User
	vassociated_base_user, eassociated_base_user := compactAta(p.User, p.BaseMint, p.BaseTokenProgram)
	if eassociated_base_user != nil {
		return nil, eassociated_base_user
	}
	accounts["associated_base_user"] = vassociated_base_user
	vassociated_quote_user, eassociated_quote_user := compactAta(p.User, p.QuoteMint, p.QuoteTokenProgram)
	if eassociated_quote_user != nil {
		return nil, eassociated_quote_user
	}
	accounts["associated_quote_user"] = vassociated_quote_user
	vuser_volume_accumulator, euser_volume_accumulator := compactPDA(compactPump, "user_volume_accumulator", &p.User)
	if euser_volume_accumulator != nil {
		return nil, euser_volume_accumulator
	}
	accounts["user_volume_accumulator"] = vuser_volume_accumulator
	vfee_config, efee_config := compactPDA(compactFees, "fee_config", &compactPump)
	if efee_config != nil {
		return nil, efee_config
	}
	accounts["fee_config"] = vfee_config
	if p.QuoteMint == compactWSOL {
		accounts["buyback_fee_recipient"] = p.BuybackRecipient
	} else {
		v, e := compactAta(p.BuybackRecipient, p.QuoteMint, p.QuoteTokenProgram)
		if e != nil {
			return nil, e
		}
		accounts["buyback_fee_recipient"] = v
	}
	accounts["system_program"] = solana.PublicKey{}
	vevent_authority, eevent_authority := compactPDA(compactPump, "__event_authority", nil)
	if eevent_authority != nil {
		return nil, eevent_authority
	}
	accounts["event_authority"] = vevent_authority
	accounts["program"] = compactPump
	vglobal, eglobal := compactPDA(compactPump, "global", nil)
	if eglobal != nil {
		return nil, eglobal
	}
	accounts["global"] = vglobal
	return accounts, nil
}
func DerivePumpSwapV2Accounts(p PumpCompactAccountParams, pool, base_vault, quote_vault solana.PublicKey) (map[string]solana.PublicKey, error) {
	p.QuoteMint = compactNormalizeQuote(p.QuoteMint)
	if p.Cashback {
		return nil, fmt.Errorf("cashback requires legacy trades")
	}
	accounts := make(map[string]solana.PublicKey)
	accounts["pool"] = pool
	accounts["user"] = p.User
	vglobal_config, eglobal_config := compactPDA(compactAMM, "global_config", nil)
	if eglobal_config != nil {
		return nil, eglobal_config
	}
	accounts["global_config"] = vglobal_config
	accounts["base_mint"] = p.BaseMint
	accounts["quote_mint"] = p.QuoteMint
	vuser_base_token_account, euser_base_token_account := compactAta(p.User, p.BaseMint, p.BaseTokenProgram)
	if euser_base_token_account != nil {
		return nil, euser_base_token_account
	}
	accounts["user_base_token_account"] = vuser_base_token_account
	vuser_quote_token_account, euser_quote_token_account := compactAta(p.User, p.QuoteMint, p.QuoteTokenProgram)
	if euser_quote_token_account != nil {
		return nil, euser_quote_token_account
	}
	accounts["user_quote_token_account"] = vuser_quote_token_account
	accounts["pool_base_token_account"] = base_vault
	accounts["pool_quote_token_account"] = quote_vault
	accounts["base_token_program"] = p.BaseTokenProgram
	accounts["quote_token_program"] = p.QuoteTokenProgram
	accounts["system_program"] = solana.PublicKey{}
	vuser_volume_accumulator, euser_volume_accumulator := compactPDA(compactAMM, "user_volume_accumulator", &p.User)
	if euser_volume_accumulator != nil {
		return nil, euser_volume_accumulator
	}
	accounts["user_volume_accumulator"] = vuser_volume_accumulator
	vfee_config, efee_config := compactPDA(compactFees, "fee_config", &compactAMM)
	if efee_config != nil {
		return nil, efee_config
	}
	accounts["fee_config"] = vfee_config
	vbuyback_fee_recipient, ebuyback_fee_recipient := compactAta(p.BuybackRecipient, p.QuoteMint, p.QuoteTokenProgram)
	if ebuyback_fee_recipient != nil {
		return nil, ebuyback_fee_recipient
	}
	accounts["buyback_fee_recipient"] = vbuyback_fee_recipient
	vevent_authority, eevent_authority := compactPDA(compactAMM, "__event_authority", nil)
	if eevent_authority != nil {
		return nil, eevent_authority
	}
	accounts["event_authority"] = vevent_authority
	accounts["program"] = compactAMM
	return accounts, nil
}

type PumpMultiHop struct {
	Venue                                                                                             string
	BaseMint, QuoteMint, Address, BaseVault, QuoteVault, BaseTokenProgram, QuoteTokenProgram, Creator solana.PublicKey
	Mayhem, Cashback, Complete                                                                        bool
	Index                                                                                             uint16
}

// DerivePumpMultiHopAccounts validates decoded state. Four hops require v0 + ALT.
func DerivePumpMultiHopAccounts(user, inputMint, outputMint, buybackRecipient solana.PublicKey, hops []PumpMultiHop, useV0WithAlt bool) (map[string]solana.PublicKey, []*solana.AccountMeta, error) {
	fail := func(message string) (map[string]solana.PublicKey, []*solana.AccountMeta, error) {
		return nil, nil, fmt.Errorf("%s", message)
	}
	if len(hops) == 0 || (len(hops) >= 4 && !useV0WithAlt) {
		return fail("route requires hops and v0 with ALT for four or more hops")
	}
	inputMint = compactNormalizeQuote(inputMint)
	outputMint = compactNormalizeQuote(outputMint)
	normalized := make([]PumpMultiHop, len(hops))
	for i, h := range hops {
		normalized[i] = compactNormalizeHop(h)
	}
	hops = normalized
	current := inputMint
	side := false
	remaining := []*solana.AccountMeta{}
	for i, h := range hops {
		if h.Mayhem || h.BaseMint == h.QuoteMint {
			return fail("invalid multi-hop venue")
		}
		buy := current == h.QuoteMint
		if !buy && current != h.BaseMint {
			return fail("discontinuous route")
		}
		if i > 0 && buy != side {
			return fail("mixed route direction")
		}
		side = buy
		endpoint := len(hops) - 1
		if buy {
			endpoint = 0
		}
		if h.Cashback && i != endpoint {
			return fail("cashback must be at currency endpoint")
		}
		if h.Venue == "curve" {
			curve, e := compactPDA(compactPump, "bonding-curve", &h.BaseMint)
			if e != nil {
				return nil, nil, e
			}
			base, e := compactAta(curve, h.BaseMint, h.BaseTokenProgram)
			if e != nil {
				return nil, nil, e
			}
			quote, e := compactAta(curve, h.QuoteMint, h.QuoteTokenProgram)
			if e != nil {
				return nil, nil, e
			}
			if h.Complete || h.Address != curve || h.BaseVault != base || h.QuoteVault != quote {
				return fail("invalid curve accounts")
			}
		} else if h.Venue == "pool" {
			authority, e := compactPDA(compactPump, "pool-authority", &h.BaseMint)
			if e != nil {
				return nil, nil, e
			}
			pool, _, e := solana.FindProgramAddress([][]byte{[]byte("pool"), {0, 0}, authority[:], h.BaseMint[:], h.QuoteMint[:]}, compactAMM)
			if e != nil {
				return nil, nil, e
			}
			if h.Index != 0 || h.Creator != authority || h.Address != pool {
				return fail("noncanonical Pump pool")
			}
		} else {
			return fail("unknown venue")
		}
		for j, key := range []solana.PublicKey{h.BaseMint, h.QuoteMint, h.Address, h.BaseVault, h.QuoteVault} {
			remaining = append(remaining, &solana.AccountMeta{PublicKey: key, IsWritable: j >= 2})
		}
		if buy {
			current = h.BaseMint
		} else {
			current = h.QuoteMint
		}
	}
	if current != outputMint {
		return fail("wrong output mint")
	}
	first, last := hops[0], hops[len(hops)-1]
	currency := last
	inputToken, outputToken := first.BaseTokenProgram, last.QuoteTokenProgram
	if side {
		currency = first
		inputToken = first.QuoteTokenProgram
		outputToken = last.BaseTokenProgram
	}
	accounts := map[string]solana.PublicKey{"user": user, "program": compactAMM, "pump_program": compactPump, "system_program": {}, "token_program": solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"), "token_2022_program": solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")}
	for _, v := range []struct {
		name               string
		owner, mint, token solana.PublicKey
	}{{"user_in_token_account", user, inputMint, inputToken}, {"user_out_token_account", user, outputMint, outputToken}, {"buyback_fee_recipient", buybackRecipient, currency.QuoteMint, currency.QuoteTokenProgram}} {
		key, e := compactAta(v.owner, v.mint, v.token)
		if e != nil {
			return nil, nil, e
		}
		accounts[v.name] = key
	}
	for _, v := range []struct {
		name, seed string
		program    solana.PublicKey
		key        *solana.PublicKey
	}{{"global_config", "global_config", compactAMM, nil}, {"fee_config", "fee_config", compactFees, &compactAMM}, {"user_volume_accumulator", "user_volume_accumulator", compactAMM, &user}, {"event_authority", "__event_authority", compactAMM, nil}, {"pump_global", "global", compactPump, nil}, {"pump_fee_config", "fee_config", compactFees, &compactPump}, {"pump_event_authority", "__event_authority", compactPump, nil}} {
		key, e := compactPDA(v.program, v.seed, v.key)
		if e != nil {
			return nil, nil, e
		}
		accounts[v.name] = key
	}
	return accounts, remaining, nil
}

// DerivePumpCoinQuoteCreateAccounts derives extra create_v2 roles from decoded quote state.
func DerivePumpCoinQuoteCreateAccounts(newMint solana.PublicKey, quote PumpMultiHop, depth, maxDepth uint8, listedQuoteMints []solana.PublicKey) ([]*solana.AccountMeta, error) {
	quote = compactNormalizeHop(quote)
	if depth >= maxDepth {
		return nil, fmt.Errorf("CurveDepthExceeded")
	}
	eligible := depth > 0 || quote.QuoteMint == compactWSOL || quote.QuoteMint == solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	for _, mint := range listedQuoteMints {
		if mint == quote.QuoteMint {
			eligible = true
		}
	}
	if !eligible {
		return nil, fmt.Errorf("QuoteBondingCurveNotEligible")
	}
	if _, _, e := DerivePumpMultiHopAccounts(newMint, quote.QuoteMint, quote.BaseMint, newMint, []PumpMultiHop{quote}, false); e != nil {
		return nil, e
	}
	curve, e := compactPDA(compactPump, "bonding-curve", &newMint)
	if e != nil {
		return nil, e
	}
	vault, e := compactAta(curve, quote.BaseMint, quote.BaseTokenProgram)
	if e != nil {
		return nil, e
	}
	control, e := compactPDA(compactPump, "quote-control", nil)
	if e != nil {
		return nil, e
	}
	quoteCurve, e := compactPDA(compactPump, "bonding-curve", &quote.BaseMint)
	if e != nil {
		return nil, e
	}
	keys := []solana.PublicKey{quote.BaseMint, vault, quote.BaseTokenProgram, control, quoteCurve}
	if quote.Venue == "pool" {
		keys = append(keys, quote.Address, quote.BaseVault, quote.QuoteVault)
	}
	accounts := []*solana.AccountMeta{}
	for i, k := range keys {
		accounts = append(accounts, &solana.AccountMeta{PublicKey: k, IsWritable: i == 1})
	}
	return accounts, nil
}

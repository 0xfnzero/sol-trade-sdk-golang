package subscription

// Explicit SOL endpoint settlement through a fresh temporary WSOL account.
// Rent is supplied from cold initialization; seed must be unique per transaction.
// Existing WSOL ATAs are never funded or closed; plain WSOL skips this helper.
import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/gagliardetto/solana-go"
	"reflect"
	"unicode/utf8"
)

var routeWsolMint = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")

type NativeSolRoute struct {
	Instructions                          []solana.Instruction
	TemporaryWsolAccount                  solana.PublicKey
	RequiredLamports, MinimumNetAmountOut uint64
}

func missingNativeInstruction(ix solana.Instruction) bool {
	if ix == nil {
		return true
	}
	v := reflect.ValueOf(ix)
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return v.IsNil()
	}
	return false
}

func SettleCachedRouteWithNativeSol(route PreparedCachedRoute, payer solana.PublicKey, seed string, rentLamports uint64, nativeInput, nativeOutput bool) (NativeSolRoute, error) {
	var empty NativeSolRoute
	if nativeInput == nativeOutput {
		return empty, errors.New("choose exactly one native SOL endpoint")
	}
	if len(route.Legs) < 1 || len(route.Legs) > 5 || len(route.SwapInstructions) != len(route.Legs) {
		return empty, errors.New("invalid native SOL route instructions")
	}
	for i, leg := range route.Legs {
		ix, quoted := route.SwapInstructions[i], leg.Instruction
		if missingNativeInstruction(ix) || missingNativeInstruction(quoted) {
			return empty, errors.New("missing native SOL swap instruction")
		}
		a, err := ix.Data()
		if err != nil {
			return empty, err
		}
		b, err := quoted.Data()
		if err != nil {
			return empty, err
		}
		keys, expected := ix.Accounts(), quoted.Accounts()
		if ix.ProgramID() != quoted.ProgramID() || !bytes.Equal(a, b) || len(keys) != len(expected) {
			return empty, errors.New("native SOL instruction differs from quoted leg")
		}
		for j, key := range keys {
			if key == nil || expected[j] == nil {
				return empty, errors.New("missing native SOL swap account")
			}
			if *key != *expected[j] {
				return empty, errors.New("native SOL instruction differs from quoted leg")
			}
		}
		if leg.AmountIn == 0 || leg.MinimumNetAmountOut == 0 {
			return empty, errors.New("invalid native SOL amount or protection")
		}
		if i > 0 && (route.Legs[i-1].Hint.OutputMint != leg.Hint.InputMint || leg.AmountIn > route.Legs[i-1].MinimumNetAmountOut) {
			return empty, errors.New("native SOL route exceeds protected intermediate credit")
		}
	}
	if route.MinimumNetAmountOut != route.Legs[len(route.Legs)-1].MinimumNetAmountOut {
		return empty, errors.New("native SOL protection differs from quoted leg")
	}
	mint := route.Legs[len(route.Legs)-1].Hint.OutputMint
	if nativeInput {
		mint = route.Legs[0].Hint.InputMint
	}
	if mint != routeWsolMint {
		return empty, errors.New("native SOL endpoint must quote through WSOL")
	}
	if len(seed) < 1 || len(seed) > 32 || !utf8.ValidString(seed) {
		return empty, errors.New("temporary WSOL seed must contain 1..32 UTF-8 bytes")
	}
	if rentLamports == 0 {
		return empty, errors.New("supply current token-account rent from cold initialization")
	}
	funding := uint64(0)
	if nativeInput {
		funding = route.Legs[0].AmountIn
	}
	required := rentLamports + funding
	if required < rentLamports {
		return empty, errors.New("native SOL lamports overflow")
	}
	b := append(append(append([]byte{}, payer[:]...), []byte(seed)...), solana.TokenProgramID[:]...)
	temporary := solana.PublicKey(sha256.Sum256(b))
	old, _, e := solana.FindProgramAddress([][]byte{payer[:], solana.TokenProgramID[:], routeWsolMint[:]}, solana.SPLAssociatedTokenAccountProgramID)
	if e != nil {
		return empty, e
	}
	if temporary == old {
		return empty, errors.New("temporary account collides with existing WSOL ATA")
	}
	meta := func(k solana.PublicKey, writable, signer bool) *solana.AccountMeta {
		return &solana.AccountMeta{PublicKey: k, IsWritable: writable, IsSigner: signer}
	}
	d := make([]byte, 4+32+8+len(seed)+8+8+32)
	binary.LittleEndian.PutUint32(d, 3)
	copy(d[4:], payer[:])
	binary.LittleEndian.PutUint64(d[36:], uint64(len(seed)))
	copy(d[44:], []byte(seed))
	o := 44 + len(seed)
	binary.LittleEndian.PutUint64(d[o:], required)
	o += 8
	binary.LittleEndian.PutUint64(d[o:], 165)
	o += 8
	copy(d[o:], solana.TokenProgramID[:])
	create := solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{meta(payer, true, true), meta(temporary, true, false)}, d)
	initialize := solana.NewInstruction(solana.TokenProgramID, solana.AccountMetaSlice{meta(temporary, true, false), meta(routeWsolMint, false, false)}, append([]byte{18}, payer[:]...))
	sync := solana.NewInstruction(solana.TokenProgramID, solana.AccountMetaSlice{meta(temporary, true, false)}, []byte{17})
	instructions := []solana.Instruction{create, initialize, sync}
	for _, ix := range route.SetupInstructions {
		if missingNativeInstruction(ix) {
			return empty, errors.New("missing native SOL setup instruction")
		}
		keys := ix.Accounts()
		for _, key := range keys {
			if key == nil {
				return empty, errors.New("missing native SOL setup account")
			}
		}
		if ix.ProgramID() == solana.SPLAssociatedTokenAccountProgramID && len(keys) > 1 && keys[1].PublicKey == old {
			continue
		}
		instructions = append(instructions, ix)
	}
	replaced := false
	for _, ix := range route.SwapInstructions {
		keys := solana.AccountMetaSlice{}
		own := false
		for _, a := range ix.Accounts() {
			if a.PublicKey == payer && a.IsSigner {
				own = true
			}
			k := a.PublicKey
			if k == old {
				k = temporary
				replaced = true
			}
			keys = append(keys, meta(k, a.IsWritable, a.IsSigner))
		}
		if !own {
			return empty, errors.New("native SOL route belongs to a different wallet")
		}
		data, e := ix.Data()
		if e != nil {
			return empty, e
		}
		instructions = append(instructions, solana.NewInstruction(ix.ProgramID(), keys, append([]byte{}, data...)))
	}
	if !replaced {
		return empty, errors.New("native SOL route does not use the wallet WSOL account")
	}
	close := solana.NewInstruction(solana.TokenProgramID, solana.AccountMetaSlice{meta(temporary, true, false), meta(payer, true, false), meta(payer, false, true)}, []byte{9})
	instructions = append(instructions, close)
	return NativeSolRoute{instructions, temporary, required, route.MinimumNetAmountOut}, nil
}

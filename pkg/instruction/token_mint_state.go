package instruction

import (
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/gagliardetto/solana-go"
)

var mintToken2022Program = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")

var mintExtensionLengths = map[uint16]int{1: 108, 3: 32, 4: 65, 6: 1, 10: 52, 12: 32, 14: 64, 16: 129, 18: 64, 20: 64, 21: 80, 22: 64, 23: 72, 25: 56, 26: 33}

// TokenTransferFeeForEpoch validates public-transfer eligibility and epoch fees.
func TokenTransferFeeForEpoch(d []byte, owner solana.PublicKey, epoch uint64) (calc.TokenTransferFee, error) {
	fail := func(s string) (calc.TokenTransferFee, error) { return calc.TokenTransferFee{}, errors.New(s) }
	if owner != solana.TokenProgramID && owner != mintToken2022Program {
		return fail("unsupported mint token program")
	}
	if len(d) < 82 || d[45] != 1 || binary.LittleEndian.Uint32(d) > 1 || binary.LittleEndian.Uint32(d[46:]) > 1 {
		return fail("invalid or uninitialized cached mint")
	}
	if owner == solana.TokenProgramID {
		if len(d) != 82 {
			return fail("invalid classic mint size")
		}
		return calc.TokenTransferFee{}, nil
	}
	if len(d) == 82 {
		return calc.TokenTransferFee{}, nil
	}
	zero := func(b []byte) bool {
		for _, v := range b {
			if v != 0 {
				return false
			}
		}
		return true
	}
	if len(d) < 166 || len(d) == 355 || d[165] != 1 || !zero(d[82:165]) {
		return fail("invalid Token2022 mint layout")
	}
	extensions := map[uint16][]byte{}
	offset := 166
	for offset < len(d) {
		if d[offset] == 0 && zero(d[offset:]) {
			break
		}
		if offset+4 > len(d) {
			return fail("truncated mint extension")
		}
		kind, n := binary.LittleEndian.Uint16(d[offset:]), int(binary.LittleEndian.Uint16(d[offset+2:]))
		offset += 4
		if kind == 0 || extensions[kind] != nil || offset+n > len(d) {
			return fail("invalid/duplicate mint extension")
		}
		length, ok := mintExtensionLengths[kind]
		if kind != 19 && !ok {
			return fail("unsupported mint extension")
		}
		if kind != 19 && n != length {
			return fail("invalid mint extension size")
		}
		extensions[kind] = d[offset : offset+n]
		offset += n
	}
	if state, ok := extensions[6]; ok && state[0] != 1 {
		return fail("mint defaults to frozen/uninitialized accounts")
	}
	if pause, ok := extensions[26]; ok && pause[32] != 0 {
		return fail("mint transfers are paused")
	}
	if hook, ok := extensions[14]; ok && !zero(hook[32:]) {
		return fail("active transfer hook requires extra accounts")
	}
	fees, ok := extensions[1]
	if !ok {
		return calc.TokenTransferFee{}, nil
	}
	start := 72
	if epoch >= binary.LittleEndian.Uint64(fees[90:]) {
		start = 90
	}
	bps := binary.LittleEndian.Uint16(fees[start+16:])
	if bps > 10000 {
		return fail("invalid mint fee basis points")
	}
	return calc.TokenTransferFee{BasisPoints: bps, MaximumFee: binary.LittleEndian.Uint64(fees[start+8:])}, nil
}

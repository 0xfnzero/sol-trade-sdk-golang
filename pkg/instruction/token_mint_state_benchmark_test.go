package instruction

import (
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/gagliardetto/solana-go"
	"testing"
)

// previousMintDecoder reproduces the pre-optimization implementation solely
// for benchmarks; it is not a historical binary or a production entry point.
func previousMintDecoder(d []byte, owner solana.PublicKey, epoch uint64) (calc.TokenTransferFee, error) {
	fail := func(s string) (calc.TokenTransferFee, error) { return calc.TokenTransferFee{}, errors.New(s) }
	token2022 := solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	if owner != solana.TokenProgramID && owner != token2022 {
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
	lengths := map[uint16]int{1: 108, 3: 32, 4: 65, 6: 1, 10: 52, 12: 32, 14: 64, 16: 129, 18: 64, 20: 64, 21: 80, 22: 64, 23: 72, 25: 56, 26: 33}
	extensions := map[uint16][]byte{}
	offset := 166
	for offset < len(d) {
		if zero(d[offset:]) {
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
		length, ok := lengths[kind]
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

func BenchmarkReviewMintDecode(b *testing.B) {
	classic := make([]byte, 82)
	classic[45] = 1
	extended := make([]byte, 166)
	extended[45], extended[165] = 1, 1
	for _, kind := range []uint16{1, 3, 4, 6, 10, 12, 14, 16, 18, 20, 21, 22, 23, 25, 26} {
		payload := make([]byte, mintExtensionLengths[kind])
		if kind == 6 {
			payload[0] = 1
		}
		header := make([]byte, 4)
		binary.LittleEndian.PutUint16(header, kind)
		binary.LittleEndian.PutUint16(header[2:], uint16(len(payload)))
		extended = append(extended, header...)
		extended = append(extended, payload...)
	}
	for _, c := range []struct {
		name  string
		data  []byte
		owner solana.PublicKey
	}{{"classic", classic, solana.TokenProgramID}, {"extended", extended, mintToken2022Program}} {
		for _, fn := range []struct {
			name   string
			decode func([]byte, solana.PublicKey, uint64) (calc.TokenTransferFee, error)
		}{{"current", TokenTransferFeeForEpoch}, {"previous_reference", previousMintDecoder}} {
			b.Run(c.name+"/"+fn.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, e := fn.decode(c.data, c.owner, 1); e != nil {
						b.Fatal(e)
					}
				}
			})
		}
	}
}

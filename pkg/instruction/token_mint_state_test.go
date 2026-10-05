package instruction

import (
	"encoding/binary"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestNativeMintEpochAndPublicState(t *testing.T) {
	token := solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	mint := func(kind uint16, payload []byte) []byte {
		d := make([]byte, 170)
		d[45] = 1
		d[165] = 1
		binary.LittleEndian.PutUint16(d[166:], kind)
		binary.LittleEndian.PutUint16(d[168:], uint16(len(payload)))
		return append(d, payload...)
	}
	fees := make([]byte, 108)
	binary.LittleEndian.PutUint64(fees[80:], 500)
	binary.LittleEndian.PutUint16(fees[88:], 100)
	binary.LittleEndian.PutUint64(fees[90:], 4)
	binary.LittleEndian.PutUint64(fees[98:], 900)
	binary.LittleEndian.PutUint16(fees[106:], 300)
	for _, epoch := range []uint64{3, 4} {
		f, e := TokenTransferFeeForEpoch(mint(1, fees), token, epoch)
		want := uint16(100)
		if epoch == 4 {
			want = 300
		}
		if e != nil || f.BasisPoints != want {
			t.Fatal("epoch fee mismatch", e)
		}
	}
	if _, e := TokenTransferFeeForEpoch(mint(6, []byte{1}), token, 1); e != nil {
		t.Fatal(e)
	}
	if _, e := TokenTransferFeeForEpoch(mint(6, []byte{2}), token, 1); e == nil {
		t.Fatal("frozen default accepted")
	}
	pause := make([]byte, 33)
	pause[32] = 1
	if _, e := TokenTransferFeeForEpoch(mint(26, pause), token, 1); e == nil {
		t.Fatal("paused mint accepted")
	}
	hook := make([]byte, 64)
	hook[32] = 1
	if _, e := TokenTransferFeeForEpoch(mint(14, hook), token, 1); e == nil {
		t.Fatal("active hook accepted")
	}
	if _, e := TokenTransferFeeForEpoch(mint(6, nil), token, 1); e == nil {
		t.Fatal("invalid extension length accepted")
	}
}

func TestMintExtensionPaddingAndMalformedTails(t *testing.T) {
	base := make([]byte, 166)
	base[45], base[165] = 1, 1
	decode := func(tail []byte) error {
		d := append(append([]byte(nil), base...), tail...)
		_, err := TokenTransferFeeForEpoch(d, mintToken2022Program, 1)
		return err
	}
	for _, n := range []int{1, 2, 3, 4, 64} {
		tail := append([]byte{6, 0, 1, 0, 1, 19, 0, 0, 0}, make([]byte, n)...)
		if err := decode(tail); err != nil {
			t.Fatal(n, err)
		}
	}
	for _, tail := range [][]byte{
		{0, 1}, {0, 0, 1, 0}, {6, 0, 1}, {6, 0, 2, 0, 1},
		{6, 0, 1, 0, 1, 6, 0, 1, 0, 1}, {19, 0, 0, 0, 19, 0, 0, 0},
	} {
		if err := decode(tail); err == nil {
			t.Fatalf("accepted malformed tail %v", tail)
		}
	}
}

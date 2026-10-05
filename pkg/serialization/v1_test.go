package serialization

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"testing"
)

func TestV1RustGolden(t *testing.T) {
	seed := func(n byte) solana.PrivateKey {
		return solana.PrivateKey(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{n}, 32)))
	}
	pk := func(n byte) solana.PublicKey {
		var k solana.PublicKey
		copy(k[:], bytes.Repeat([]byte{n}, 32))
		return k
	}
	payer, co := seed(7), seed(8)
	a, b, program := pk(3), pk(4), pk(5)
	ix := []solana.Instruction{solana.NewInstruction(program, solana.AccountMetaSlice{solana.Meta(a).WRITE(), solana.Meta(co.PublicKey()).SIGNER(), solana.Meta(b), solana.Meta(payer.PublicKey()).SIGNER().WRITE()}, []byte{1, 2, 3, 4}), solana.NewInstruction(program, solana.AccountMetaSlice{solana.Meta(b).WRITE(), solana.Meta(a)}, []byte{9, 8})}
	priority, compute, loaded, heap := uint64(5000), uint32(300000), uint32(1000000), uint32(32768)
	compiled, err := CompileV1Message(payer.PublicKey(), ix, solana.Hash(pk(6)), V1Config{&priority, &compute, &loaded, &heap})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/v1_rust_4_4_1.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Message    []int
		Signatures [][]int
	}
	if err = json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	want := []byte{}
	for _, v := range golden.Message {
		want = append(want, byte(v))
	}
	if !bytes.Equal(compiled.Message, want) {
		t.Fatal("V1 message differs from Rust")
	}
	for _, sig := range golden.Signatures {
		for _, v := range sig {
			want = append(want, byte(v))
		}
	}
	signed, err := SignV1Transaction(compiled, []solana.PrivateKey{co, payer})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(signed, want) {
		t.Fatal("V1 signatures differ from Rust")
	}
	if _, err = SignV1Transaction(compiled, []solana.PrivateKey{payer}); err == nil {
		t.Fatal("missing signer accepted")
	}
	heap = 32769
	if _, err = CompileV1Message(payer.PublicKey(), ix, solana.Hash(pk(6)), V1Config{HeapSize: &heap}); err == nil {
		t.Fatal("invalid heap accepted")
	}
	if _, err = CompileV1Message(payer.PublicKey(), []solana.Instruction{solana.NewInstruction(program, nil, make([]byte, 4096))}, solana.Hash(pk(6)), V1Config{}); err == nil {
		t.Fatal("oversize transaction accepted")
	}
}
